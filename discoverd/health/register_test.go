package health

import (
	"errors"
	"fmt"
	"time"

	. "github.com/flynn/flynn/Godeps/_workspace/src/github.com/flynn/go-check"
	"github.com/flynn/flynn/discoverd/client"
	"github.com/flynn/flynn/pkg/random"
	"github.com/flynn/flynn/pkg/stream"
)

type RegisterSuite struct{}

var _ = Suite(&RegisterSuite{})

type RegistrarFunc func(service string, inst *discoverd.Instance) (discoverd.Heartbeater, error)

func (f RegistrarFunc) RegisterInstance(service string, inst *discoverd.Instance) (discoverd.Heartbeater, error) {
	return f(service, inst)
}

type HeartbeatFunc func(map[string]string) error

func (f HeartbeatFunc) SetMeta(meta map[string]string) error { return f(meta) }
func (f HeartbeatFunc) Close() error                         { return nil }
func (f HeartbeatFunc) Addr() string                         { return "" }

func init() {
	registerErrWait = time.Millisecond
}

func (RegisterSuite) TestRegister(c *C) {
	type step struct {
		event      bool // send an event
		up         bool // event type
		register   bool // event should trigger register
		unregister bool // event should unregister service
		setMeta    bool // attempt SetMeta
		success    bool // true if SetMeta or Register should succeed
	}

	type called struct {
		args      map[string]interface{}
		returnVal chan bool
	}

	run := func(c *C, steps []step) {
		check := CheckFunc(func() error { return nil })

		hbChan := make(chan called)
		heartbeater := HeartbeatFunc(func(meta map[string]string) error {
			success := make(chan bool)
			hbChan <- called{
				args: map[string]interface{}{
					"meta": meta,
				},
				returnVal: success,
			}
			if !<-success {
				return errors.New("SetMeta failed")
			}
			return nil
		})

		registrarChan := make(chan called)
		registrar := RegistrarFunc(func(service string, inst *discoverd.Instance) (discoverd.Heartbeater, error) {
			id := random.UUID()
			fmt.Println("registrar fn", id)
			success := make(chan bool)
			registrarChan <- called{
				args: map[string]interface{}{
					"service": service,
					"inst":    inst,
				},
				returnVal: success,
			}
			defer func() { registrarChan <- called{} }()
			if <-success {
				fmt.Println("registrar success", id)
				return heartbeater, nil
			}
			fmt.Println("registrar error", id)
			return nil, errors.New("registrar failure")
		})

		monitorChan := make(chan bool)
		monitor := func(c Check, ch chan MonitorEvent) stream.Stream {
			stream := stream.New()
			go func() {
				for up := range monitorChan {
					if up {
						ch <- MonitorEvent{
							Check:  check,
							Status: MonitorStatusUp,
						}
					} else {
						ch <- MonitorEvent{
							Check:  check,
							Status: MonitorStatusDown,
						}
					}
				}
			}()
			return stream
		}

		reg := Registration{
			Service: "test",
			Instance: &discoverd.Instance{
				Addr: "123",
			},
			Registrar: registrar,
			Monitor:   monitor,
			Check:     check,
		}
		hb := reg.Register()
		defer hb.Close()

		errCh := make(chan struct{})
		errCheck := func(ch chan called, stop chan bool) {
			go func() {
				select {
				case <-ch:
					errCh <- struct{}{}
				case <-stop:
				}
			}()
		}

		stop := make(chan bool)
		for _, step := range steps {
			fmt.Println("loop")
			close(stop)
			stop = make(chan bool)

			if step.event {
				monitorChan <- step.up
			}
			if step.register {
				call := <-registrarChan
				call.returnVal <- step.success
				<-registrarChan
			} else {
				fmt.Println("register errorcheck!")
				errCheck(registrarChan, stop)
			}
			if step.setMeta {
				go func() {
					call := <-hbChan
					call.returnVal <- step.success
				}()
				c.Assert(hb.SetMeta(map[string]string{"TEST": "hello world"}), IsNil)
			} else {
				errCheck(hbChan, stop)
			}

			// check whether functions were called on this step that should not
			// have ran
			select {
			case <-errCh:
				c.Error("Received data on a channel that should not be recieving on this step")
			default:
			}
		}
		fmt.Println("stopping")
		close(stop)
		close(monitorChan)
		close(registrarChan)
		close(hbChan)
		fmt.Println("end")
	}

	for _, t := range []struct {
		name  string
		steps []step
	}{
		/*{
			name: "register success up/down/up",
			steps: []step{
				{event: true, up: true, register: true, success: true},
				{event: true},
				{event: true, up: true, register: true, success: true},
			},
		},
		{
			name: "register fails then succeeds",
			steps: []step{
				{event: true, up: true, register: true, success: false},
				{register: true, success: true},
			},
		},
		{
			name: "setmeta while registered",
			steps: []step{
				{event: true, up: true, register: true, success: true},
				{setMeta: true, success: true},
			},
		},
		{
			name: "setmeta while offline",
			steps: []step{
				{setMeta: true, success: false},
			},
		},*/
		{
			name: "setmeta while erroring registration",
			steps: []step{
				{event: true, up: true, register: true, success: false},
				//{register: true, success: false},
				// {setMeta: true, success: false},
			},
		},
		{
			name: "register failing then offline",
			steps: []step{
				{event: true, up: true, register: true, success: false},
				{register: true, success: false},
				//{event: true},
			},
		},
	} {
		run(c, t.steps)
	}
}

// register success up/down/up
// register fails then succeeds
// setmeta while registered
// setmeta while offline
// setmeta while erroring registration
// register failing then offline
// event forwarding

package health

import (
	"sync"
	"time"

	"github.com/flynn/flynn/discoverd2/client"
	"github.com/flynn/flynn/pkg/stream"
)

type Registrar interface {
	RegisterInstance(service string, inst *discoverd.Instance) (discoverd.Heartbeater, error)
}

type Registration struct {
	Registrar Registrar
	Service   string
	Instance  *discoverd.Instance
	Monitor   func(Check, chan MonitorEvent) stream.Stream
	Check     Check
	Events    chan MonitorEvent
}

func (r *Registration) Register() discoverd.Heartbeater {
	events := make(chan MonitorEvent)
	hb := &heartbeater{Registration: r}
	hb.stream = r.Monitor(r.Check, events)
	go hb.run(events)
	return hb
}

type heartbeater struct {
	stream stream.Stream

	sync.Mutex
	hb discoverd.Heartbeater
	*Registration
}

func (h *heartbeater) run(events chan MonitorEvent) {
	var stopRegister chan struct{}
	for e := range events {
		if stopRegister != nil {
			close(stopRegister)
		}
		stopRegister = make(chan struct{})

		switch e.Status {
		case MonitorStatusUp:
			go h.register(stopRegister)
		case MonitorStatusDown:
			h.Lock()
			if h.hb != nil {
				h.hb.Close()
				h.hb = nil
			}
			h.Unlock()
		}
		if h.Events != nil {
			h.Events <- e
		}
	}
}

var registerErrWait = time.Second

func (h *heartbeater) register(stop chan struct{}) {
	for {
		select {
		case <-stop:
			return
		default:
		}
		var err error
		func() {
			h.Lock()
			defer h.Unlock()
			if h.hb != nil {
				return
			}
			h.hb, err = h.Registrar.RegisterInstance(h.Service, h.Instance)
		}()
		if err == nil {
			return
		}
		time.Sleep(registerErrWait)
	}
}

func (h *heartbeater) SetMeta(meta map[string]string) error {
	h.Lock()
	defer h.Unlock()
	h.Instance.Meta = meta
	if h.hb == nil {
		return nil
	}
	return h.hb.SetMeta(meta)
}

func (h *heartbeater) Close() error {
	h.stream.Close()
	return nil
}

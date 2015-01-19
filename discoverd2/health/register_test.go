package health

import (
	"time"

	. "github.com/flynn/flynn/Godeps/_workspace/src/github.com/flynn/go-check"
)

type RegisterSuite struct{}

var _ = Suite(&RegisterSuite{})

func init() {
	registerErrWait = time.Millisecond
}

func (RegisterSuite) TestRegister(c *C) {
	type step struct {
		event    bool
		up       bool
		register bool
		setMeta  bool
		success  bool
	}

	run := func(c *C, steps []step) {
		
	}

	for _, t := range []struct {
		name  string
		steps []step
	}{
		{
			name: "register success up/down/up",
			steps: []step{
				{event: true, up: true, register: true, success: true},
				{event: true},
				{event: true, up: true, register: true, success: true},
			},
		},
	} {

	}
}

// register success up/down/up
// register fails then succeeds
// setmeta while registered
// setmeta while offline
// setmeta while erroring registration
// register failing then offline
// event forwarding

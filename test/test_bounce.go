package main

import (
	c "github.com/flynn/flynn/Godeps/_workspace/src/github.com/flynn/go-check"
	"github.com/flynn/flynn/test/cluster/client"
)

/*
	Sanity check over our CI system to make sure we can in fact bounce hosts,
	and that does what we think it does.

	Note that any tests that involve raising or destroying hosts that join the
	main test bootstrap cluster cannot be run concurrently.
*/
type BounceSuite struct {
	Helper
}

var _ = c.Suite(&BounceSuite{})

func (BounceSuite) SetUpSuite(t *c.C) {
	if args.ClusterAPI == "" {
		t.Skip("cannot boot new hosts")
	}
}

func (s *BounceSuite) TestHostUpDown(t *c.C) {
	cl := &testcluster.Client{
		C:          httpClient,
		ClusterAPI: args.ClusterAPI,
	}
	hosts, err := cl.AddHosts(testCluster, s.clusterClient(t), 1)
	t.Assert(err, c.IsNil)
	t.Assert(hosts, c.HasLen, 1)
	err = cl.RemoveHosts(hosts)
	t.Assert(err, c.IsNil)
}

func (s *BounceSuite) TestBounceHostProcess(t *c.C) {
	// TODO
}

func (s *BounceSuite) TestBounceHostVM(t *c.C) {
	// TODO
}

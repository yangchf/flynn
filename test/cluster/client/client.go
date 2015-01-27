package testcluster

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/flynn/flynn/host/types"
	"github.com/flynn/flynn/pkg/cluster"
	tc "github.com/flynn/flynn/test/cluster"
)

type Client struct {
	C          *http.Client
	ClusterAPI string
}

func (c *Client) AddHosts(testCluster *tc.Cluster, cluster *cluster.Client, count int) ([]string, error) {
	ch := make(chan *host.HostEvent)
	stream, err := cluster.StreamHostEvents(ch)
	if err != nil {
		return nil, fmt.Errorf("error when attempting to StreamHostEvents: %s", err)
	}
	defer stream.Close()

	hosts := make([]string, 0, count)
	for i := 0; i < count; i++ {
		res, err := c.C.PostForm(c.ClusterAPI, url.Values{})
		if err != nil {
			return nil, fmt.Errorf("error in POST request to cluster api: %s", err)
		}
		defer res.Body.Close()
		if res.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("expected 200 status, got %s", res.Status)
		}
		instance := &tc.Instance{}
		err = json.NewDecoder(res.Body).Decode(instance)
		if err != nil {
			return nil, fmt.Errorf("could not decode new instance: %s", err)
		}

		select {
		case event := <-ch:
			testCluster.Instances = append(testCluster.Instances, instance)
			hosts = append(hosts, event.HostID)
		case <-time.After(60 * time.Second):
			return nil, fmt.Errorf("timed out waiting for new host")
		}
	}
	return hosts, nil
}

func (c *Client) RemoveHosts(ids []string) error {
	for _, id := range ids {
		req, err := http.NewRequest("DELETE", c.ClusterAPI+"?host="+id, nil)
		if err != nil {
			return fmt.Errorf("error in DELETE request to cluster api: %s", err)
		}
		res, err := c.C.Do(req)
		if err != nil {
			return fmt.Errorf("error in DELETE request to cluster api: %s", err)
		}
		res.Body.Close()
		if res.StatusCode != http.StatusOK {
			return fmt.Errorf("expected 200 status, got %s", res.Status)
		}
	}
	return nil
}

func (c *Client) BounceHost(id string) error {
	req, err := http.NewRequest("HEAD", c.ClusterAPI+"?host="+id, nil)
	if err != nil {
		return fmt.Errorf("error in 'bounce' request to cluster api: %s", err)
	}
	res, err := c.C.Do(req)
	if err != nil {
		return fmt.Errorf("error in 'bounce' request to cluster api: %s", err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("expected 200 status in 'bounce' request to cluster api, got %s", res.Status)
	}
	return nil
}

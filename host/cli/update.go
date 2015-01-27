package cli

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/flynn/flynn/controller/client"
	ct "github.com/flynn/flynn/controller/types"
	"github.com/flynn/flynn/discoverd/client"
	"github.com/flynn/flynn/pinkerton"
)

func init() {
	Register("update", runUpdate, `
	usage: flynn-host update

	Options:
	  --driver=<name>  storage driver [default: aufs]
	  --root=<path>    storage root [default: /var/lib/docker]

	Update the host.`)
}

func runUpdate() error {
	fmt.Println("Fetching the update manifest from https://updates.flynn.io")
	res, _ := http.Get("https://updates.flynn.io/manifest.json")
	if res.StatusCode != 200 {
		return "", fmt.Errorf("error fetching the update manifest, got status %d", res.StatusCode)
	}
	defer res.Body.Close()

	var manifest map[string]string
	body, _ := ioutil.ReadAll(res.Body)
	json.Unmarshal(body, &manifest)

	// TODO: download the images on all hosts
	ctx, err := pinkerton.BuildContext(args.String["--driver"], args.String["--root"])
	if err != nil {
		return err
	}
	for component, id := range manifest {
		if err := ctx.Pull(fmt.Sprintf("<uri>?name%s?id=%s", component, id), false); err != nil {
			return err
		}
	}

	instances, err := discoverd.GetInstances("flynn-controller", 15*time.Second)
	if err != nil {
		return err
	}
	client, err := controller.NewClient("", instances[0].Meta["AUTH_KEY"])
	if err != nil {
		return err
	}

	for component, id := range manifest {
		fmt.Printf("Updating %s to %s...\n", component, id)
		app, err := client.GetApp(component)
		if err != nil {
			return err
		}
		release, err := client.GetAppRelease(app.ID)
		if err != nil {
			return err
		}
		artifact := &ct.Artifact{
			Type: "tuf",
			URI:  fmt.Sprintf("<uri>?name%s?id=%s", component, id),
		}
		if err := client.CreateArtifact(artifact); err != nil {
			return err
		}
		release.ID = ""
		release.ArtifactID = artifact.ID
		if err := client.CreateRelease(release); err != nil {
			return err
		}
		if err := client.DeployAppRelease(app.ID, release.ID); err != nil {
			return err
		}
	}
	fmt.Println("Update complete.")
	return nil
}

package bootstrap

import (
	"fmt"

	ct "github.com/flynn/flynn/controller/types"
	"github.com/flynn/flynn/router/types"
)

type AddRouteAction struct {
	ID string `json:"id"`

	AppStep  string `json:"app_step"`
	CertStep string `json:"cert_step"`
	*router.Route
}

func init() {
	Register("add-route", &AddRouteAction{})
}

type AddRouteState struct {
	App   *ct.App       `json:"app"`
	Route *router.Route `json:"route"`
}

func (a *AddRouteAction) Run(s *State) error {
	s.debugf("AddRouteAction controllerClient")
	client, err := s.ControllerClient()
	if err != nil {
		return err
	}
	s.debugf("AddRouteAction getAppStep")
	data, err := getAppStep(s, a.AppStep)
	if err != nil {
		return err
	}
	if a.CertStep != "" {
		if a.Route.Type != "http" {
			return fmt.Errorf("bootstrap: invalid cert_step option for non-http route")
		}
		s.debugf("AddRouteAction getCertStep")
		cert, err := getCertStep(s, a.CertStep)
		if err != nil {
			return err
		}
		s.debugf("AddRouteAction HTTPRoute")
		route := a.Route.HTTPRoute()
		route.Domain = interpolate(s, route.Domain)
		route.TLSCert = cert.Cert
		route.TLSKey = cert.PrivateKey
		a.Route = route.ToRoute()
	}

	s.debugf("AddRouteAction CreateRoute")
	if err := client.CreateRoute(data.App.ID, a.Route); err != nil {
		s.debugf("AddRouteAction CreateRoute err: %s", err)
		return err
	}
	s.StepData[a.ID] = &AddRouteState{App: data.App, Route: a.Route}

	return nil
}

func getAppStep(s *State, step string) (*AppState, error) {
	data, ok := s.StepData[step].(*AppState)
	if !ok {
		return nil, fmt.Errorf("bootstrap: unable to find step %q", step)
	}
	return data, nil
}

func getCertStep(s *State, step string) (*TLSCert, error) {
	data, ok := s.StepData[step].(*TLSCert)
	if !ok {
		return nil, fmt.Errorf("bootstrap: unable to find step %q", step)
	}
	return data, nil
}

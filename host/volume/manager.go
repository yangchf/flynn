package volume

import (
	"errors"
	"sync"
)

/*
	volume.Manager providers interfaces for both provisioning volume backends, and then creating volumes using them.

	There is one volume.Manager per host daemon process.
*/
type Manager struct {
	mutex sync.Mutex

	defaultProvider Provider

	// `map[providerName]provider`
	//
	// It's possible to configure multiple volume providers for a flynn-host daemon.
	// This can be used to create volumes using providers backed by different storage resources,
	// or different volume backends entirely.
	providers map[string]Provider

	// `map[volume.Id]volume`
	volumes map[string]Volume

	// `map[wellKnownName]volume.Id`
	namedVolumes map[string]string
}

var NoSuchProvider = errors.New("no such provider")
var ProviderAlreadyExists = errors.New("that provider id already exists")

func NewManager(p Provider) *Manager {
	return &Manager{
		defaultProvider: p,
		providers:       map[string]Provider{"default": p},
		volumes:         map[string]Volume{},
		namedVolumes:    map[string]string{},
	}
}

func (m *Manager) AddProvider(id string, p Provider) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if _, ok := m.providers[id]; ok {
		return ProviderAlreadyExists
	}
	m.providers[id] = p
	return nil
}

func (m *Manager) Volumes() map[string]Volume {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	r := make(map[string]Volume)
	for k, v := range m.volumes {
		r[k] = v
	}
	return r
}

/*
	volume.Manager implements the volume.Provider interface by
	delegating NewVolume requests to the default Provider.
*/
func (m *Manager) NewVolume() (Volume, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	return m.newVolumeFromProviderLocked("")
}

/*
	volume.Manager implements the volume.Provider interface by
	delegating NewVolume requests to the named Provider.
*/
func (m *Manager) NewVolumeFromProvider(providerID string) (Volume, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	return m.newVolumeFromProviderLocked(providerID)
}

func (m *Manager) newVolumeFromProviderLocked(providerID string) (Volume, error) {
	if providerID == "" {
		return managerProviderProxy{m.defaultProvider, m}.NewVolume()
	}
	if p, ok := m.providers[providerID]; ok {
		return managerProviderProxy{p, m}.NewVolume()
	} else {
		return nil, NoSuchProvider
	}
}

func (m *Manager) GetVolume(id string) Volume {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	return m.volumes[id]
}

/*
	Proxies `volume.Provider` while making sure the manager remains
	apprised of all volume lifecycle events.
*/
type managerProviderProxy struct {
	Provider
	m *Manager
}

func (p managerProviderProxy) NewVolume() (Volume, error) {
	v, err := p.Provider.NewVolume()
	if err != nil {
		return v, err
	}
	p.m.volumes[v.Info().ID] = v
	return v, err
}

/*
	Gets a reference to a volume by name if that exists; if no volume is so named,
	it is created using the named provider (the zero string can be used to invoke
	the default provider).
*/
func (m *Manager) CreateOrGetNamedVolume(name string, providerID string) (Volume, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if v, ok := m.namedVolumes[name]; ok {
		return m.volumes[v], nil
	} else {
		v, err := m.newVolumeFromProviderLocked(providerID)
		if err != nil {
			return nil, err
		}
		m.namedVolumes[name] = v.Info().ID
		return v, nil
	}
}

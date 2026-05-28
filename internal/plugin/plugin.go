package plugin

import (
	"context"
	"image"
	"sync"

	"github.com/trodemaster/trmnl-byos/internal/device"
)

// Plugin renders a screen for a device. Implement this interface to create custom plugins.
type Plugin interface {
	Name() string
	Render(ctx context.Context, d *device.Device) (*image.Gray, error)
}

var (
	mu       sync.RWMutex
	registry = map[string]Plugin{}
)

func Register(p Plugin) {
	mu.Lock()
	defer mu.Unlock()
	registry[p.Name()] = p
}

func Get(name string) (Plugin, bool) {
	mu.RLock()
	defer mu.RUnlock()
	p, ok := registry[name]
	return p, ok
}

func Names() []string {
	mu.RLock()
	defer mu.RUnlock()
	names := make([]string, 0, len(registry))
	for n := range registry {
		names = append(names, n)
	}
	return names
}

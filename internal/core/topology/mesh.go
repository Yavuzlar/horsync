package topology

import (
	"context"
	"sync"

	"horsync/internal/models"
)

type Mesh struct {
	mu    sync.RWMutex
	nodes []models.Node
}

var instance *Mesh
var once sync.Once

// GetInstance returns the singleton mesh topology instance.
func GetInstance() *Mesh {
	once.Do(func() {
		instance = &Mesh{
			nodes: make([]models.Node, 0),
		}
	})
	return instance
}

// Start initializes the mesh. Currently a no-op.
func (m *Mesh) Start() {}

// Stop shuts down the mesh. Currently a no-op.
func (m *Mesh) Stop() {}

// GetNodes returns the list of mesh nodes, attempting to load from the database first.
func (m *Mesh) GetNodes() []models.Node {
	if devices, err := m.ListDevices(context.Background()); err == nil {
		return devices
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]models.Node, len(m.nodes))
	copy(result, m.nodes)
	return result
}


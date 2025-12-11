package balancer

import (
	"sync"
)

// Backend represents a backend server in the pool
type Backend struct {
	Address     string
	Weight      int
	Healthy     bool
	Connections int64
	mu          sync.RWMutex
}

// NewBackend creates a new backend instance
func NewBackend(address string, weight int) *Backend {
	if weight <= 0 {
		weight = 1
	}
	return &Backend{
		Address: address,
		Weight:  weight,
		Healthy: true,
	}
}

// IsHealthy returns the health status of the backend
func (b *Backend) IsHealthy() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.Healthy
}

// SetHealthy updates the health status of the backend
func (b *Backend) SetHealthy(healthy bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.Healthy = healthy
}

// GetConnections returns the current connection count
func (b *Backend) GetConnections() int64 {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.Connections
}

// IncrementConnections atomically increments the connection count
func (b *Backend) IncrementConnections() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.Connections++
}

// DecrementConnections atomically decrements the connection count
func (b *Backend) DecrementConnections() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.Connections > 0 {
		b.Connections--
	}
}

// Balancer interface defines the load balancing contract
type Balancer interface {
	// Next returns the next backend to use for a request
	Next() *Backend
	// Backends returns all backends in the pool
	Backends() []*Backend
	// MarkHealthy marks a backend as healthy
	MarkHealthy(address string)
	// MarkUnhealthy marks a backend as unhealthy
	MarkUnhealthy(address string)
}

// BaseBalancer provides common functionality for all balancers
type BaseBalancer struct {
	backends []*Backend
	mu       sync.RWMutex
}

// NewBaseBalancer creates a new base balancer with the given backends
func NewBaseBalancer(backends []*Backend) *BaseBalancer {
	return &BaseBalancer{
		backends: backends,
	}
}

// Backends returns all backends in the pool
func (b *BaseBalancer) Backends() []*Backend {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.backends
}

// MarkHealthy marks a backend as healthy by address
func (b *BaseBalancer) MarkHealthy(address string) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for _, backend := range b.backends {
		if backend.Address == address {
			backend.SetHealthy(true)
			return
		}
	}
}

// MarkUnhealthy marks a backend as unhealthy by address
func (b *BaseBalancer) MarkUnhealthy(address string) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for _, backend := range b.backends {
		if backend.Address == address {
			backend.SetHealthy(false)
			return
		}
	}
}

// healthyBackends returns a list of healthy backends
func (b *BaseBalancer) healthyBackends() []*Backend {
	b.mu.RLock()
	defer b.mu.RUnlock()

	var healthy []*Backend
	for _, backend := range b.backends {
		if backend.IsHealthy() {
			healthy = append(healthy, backend)
		}
	}
	return healthy
}

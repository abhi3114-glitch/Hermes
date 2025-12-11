package balancer

import (
	"sync/atomic"
)

// RoundRobin implements round-robin load balancing
type RoundRobin struct {
	*BaseBalancer
	current uint64
}

// NewRoundRobin creates a new round-robin balancer
func NewRoundRobin(backends []*Backend) *RoundRobin {
	return &RoundRobin{
		BaseBalancer: NewBaseBalancer(backends),
		current:      0,
	}
}

// Next returns the next healthy backend in round-robin order
func (r *RoundRobin) Next() *Backend {
	healthy := r.healthyBackends()
	if len(healthy) == 0 {
		return nil
	}

	// Atomic increment and modulo for thread-safe rotation
	idx := atomic.AddUint64(&r.current, 1) - 1
	return healthy[idx%uint64(len(healthy))]
}

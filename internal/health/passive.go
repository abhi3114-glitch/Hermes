package health

import (
	"log"
	"sync"

	"github.com/hermes-proxy/hermes/internal/balancer"
)

// PassiveMonitor tracks failures during actual request proxying
type PassiveMonitor struct {
	balancer           balancer.Balancer
	unhealthyThreshold int

	failureCounts map[string]int
	mu            sync.Mutex
}

// NewPassiveMonitor creates a new passive health monitor
func NewPassiveMonitor(b balancer.Balancer, unhealthyThreshold int) *PassiveMonitor {
	return &PassiveMonitor{
		balancer:           b,
		unhealthyThreshold: unhealthyThreshold,
		failureCounts:      make(map[string]int),
	}
}

// RecordSuccess records a successful request to a backend
func (p *PassiveMonitor) RecordSuccess(address string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Reset failure count on success
	p.failureCounts[address] = 0
}

// RecordFailure records a failed request to a backend
func (p *PassiveMonitor) RecordFailure(address string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.failureCounts[address]++

	if p.failureCounts[address] >= p.unhealthyThreshold {
		log.Printf("[PASSIVE] Backend %s marked UNHEALTHY after %d consecutive failures",
			address, p.failureCounts[address])
		p.balancer.MarkUnhealthy(address)
	}
}

// Reset clears all failure counts
func (p *PassiveMonitor) Reset(address string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.failureCounts[address] = 0
}

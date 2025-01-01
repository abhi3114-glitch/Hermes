package circuit

import (
	"sync"
	"time"
)

// BreakerPool manages circuit breakers for multiple backends
type BreakerPool struct {
	breakers         map[string]*Breaker
	failureThreshold int
	successThreshold int
	timeout          time.Duration
	mu               sync.RWMutex
}

// NewBreakerPool creates a new circuit breaker pool
func NewBreakerPool(failureThreshold, successThreshold int, timeoutSeconds int64) *BreakerPool {
	return &BreakerPool{
		breakers:         make(map[string]*Breaker),
		failureThreshold: failureThreshold,
		successThreshold: successThreshold,
		timeout:          time.Duration(timeoutSeconds) * time.Second,
	}
}

// Get returns the circuit breaker for a given backend address
func (p *BreakerPool) Get(address string) *Breaker {
	p.mu.RLock()
	breaker, exists := p.breakers[address]
	p.mu.RUnlock()

	if exists {
		return breaker
	}

	// Create new breaker
	p.mu.Lock()
	defer p.mu.Unlock()

	// Double-check after acquiring write lock
	if breaker, exists = p.breakers[address]; exists {
		return breaker
	}

	breaker = NewBreaker(
		p.failureThreshold,
		p.successThreshold,
		p.timeout,
	)
	p.breakers[address] = breaker
	return breaker
}

// AllBreakers returns a map of all breakers and their states
func (p *BreakerPool) AllBreakers() map[string]State {
	p.mu.RLock()
	defer p.mu.RUnlock()

	result := make(map[string]State)
	for addr, breaker := range p.breakers {
		result[addr] = breaker.State()
	}
	return result
}

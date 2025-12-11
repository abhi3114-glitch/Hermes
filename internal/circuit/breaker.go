package circuit

import (
	"log"
	"sync"
	"time"
)

// State represents the circuit breaker state
type State int

const (
	// StateClosed allows requests through normally
	StateClosed State = iota
	// StateOpen blocks all requests
	StateOpen
	// StateHalfOpen allows limited requests to test recovery
	StateHalfOpen
)

func (s State) String() string {
	switch s {
	case StateClosed:
		return "CLOSED"
	case StateOpen:
		return "OPEN"
	case StateHalfOpen:
		return "HALF-OPEN"
	default:
		return "UNKNOWN"
	}
}

// Breaker implements the circuit breaker pattern
type Breaker struct {
	state            State
	failureThreshold int
	successThreshold int
	timeout          time.Duration

	failures    int
	successes   int
	lastFailure time.Time
	mu          sync.RWMutex
}

// NewBreaker creates a new circuit breaker
func NewBreaker(failureThreshold, successThreshold int, timeout time.Duration) *Breaker {
	return &Breaker{
		state:            StateClosed,
		failureThreshold: failureThreshold,
		successThreshold: successThreshold,
		timeout:          timeout,
	}
}

// Allow checks if a request should be allowed through
func (b *Breaker) Allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	switch b.state {
	case StateClosed:
		return true
	case StateOpen:
		// Check if timeout has passed
		if time.Since(b.lastFailure) >= b.timeout {
			b.state = StateHalfOpen
			b.successes = 0
			log.Printf("[CIRCUIT] State changed to HALF-OPEN")
			return true
		}
		return false
	case StateHalfOpen:
		return true
	default:
		return false
	}
}

// RecordSuccess records a successful request
func (b *Breaker) RecordSuccess() {
	b.mu.Lock()
	defer b.mu.Unlock()

	switch b.state {
	case StateClosed:
		b.failures = 0
	case StateHalfOpen:
		b.successes++
		if b.successes >= b.successThreshold {
			b.state = StateClosed
			b.failures = 0
			log.Printf("[CIRCUIT] State changed to CLOSED (recovered)")
		}
	}
}

// RecordFailure records a failed request
func (b *Breaker) RecordFailure() {
	b.mu.Lock()
	defer b.mu.Unlock()

	switch b.state {
	case StateClosed:
		b.failures++
		if b.failures >= b.failureThreshold {
			b.state = StateOpen
			b.lastFailure = time.Now()
			log.Printf("[CIRCUIT] State changed to OPEN after %d failures", b.failures)
		}
	case StateHalfOpen:
		b.state = StateOpen
		b.lastFailure = time.Now()
		b.successes = 0
		log.Printf("[CIRCUIT] State changed to OPEN (half-open test failed)")
	}
}

// State returns the current state
func (b *Breaker) State() State {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.state
}

// Reset resets the circuit breaker to closed state
func (b *Breaker) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.state = StateClosed
	b.failures = 0
	b.successes = 0
}

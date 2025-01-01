package circuit

import (
	"testing"
	"time"
)

func TestBreaker_InitialState(t *testing.T) {
	breaker := NewBreaker(5, 3, 30*time.Second)

	if breaker.State() != StateClosed {
		t.Errorf("Expected initial state CLOSED, got %s", breaker.State())
	}

	if !breaker.Allow() {
		t.Error("Closed circuit should allow requests")
	}
}

func TestBreaker_OpensAfterFailures(t *testing.T) {
	breaker := NewBreaker(3, 2, 100*time.Millisecond)

	// Record failures up to threshold
	for i := 0; i < 3; i++ {
		breaker.RecordFailure()
	}

	if breaker.State() != StateOpen {
		t.Errorf("Expected OPEN after 3 failures, got %s", breaker.State())
	}

	if breaker.Allow() {
		t.Error("Open circuit should not allow requests")
	}
}

func TestBreaker_TransitionsToHalfOpen(t *testing.T) {
	breaker := NewBreaker(3, 2, 50*time.Millisecond)

	// Open the circuit
	for i := 0; i < 3; i++ {
		breaker.RecordFailure()
	}

	// Wait for timeout
	time.Sleep(60 * time.Millisecond)

	// Should allow request and transition to half-open
	if !breaker.Allow() {
		t.Error("Should allow request after timeout")
	}

	if breaker.State() != StateHalfOpen {
		t.Errorf("Expected HALF-OPEN, got %s", breaker.State())
	}
}

func TestBreaker_ClosesAfterSuccesses(t *testing.T) {
	breaker := NewBreaker(3, 2, 50*time.Millisecond)

	// Open the circuit
	for i := 0; i < 3; i++ {
		breaker.RecordFailure()
	}

	// Wait and transition to half-open
	time.Sleep(60 * time.Millisecond)
	breaker.Allow()

	// Record successes
	breaker.RecordSuccess()
	breaker.RecordSuccess()

	if breaker.State() != StateClosed {
		t.Errorf("Expected CLOSED after successes, got %s", breaker.State())
	}
}

func TestBreaker_HalfOpenFailureReopens(t *testing.T) {
	breaker := NewBreaker(3, 2, 50*time.Millisecond)

	// Open the circuit
	for i := 0; i < 3; i++ {
		breaker.RecordFailure()
	}

	// Wait and transition to half-open
	time.Sleep(60 * time.Millisecond)
	breaker.Allow()

	// Fail in half-open state
	breaker.RecordFailure()

	if breaker.State() != StateOpen {
		t.Errorf("Expected OPEN after half-open failure, got %s", breaker.State())
	}
}

func TestBreaker_SuccessResetsFailures(t *testing.T) {
	breaker := NewBreaker(3, 2, 30*time.Second)

	// Record some failures
	breaker.RecordFailure()
	breaker.RecordFailure()

	// Success should reset
	breaker.RecordSuccess()

	// Now 3 more failures needed
	breaker.RecordFailure()
	breaker.RecordFailure()

	if breaker.State() != StateClosed {
		t.Errorf("Expected CLOSED, got %s", breaker.State())
	}
}

func TestBreaker_Reset(t *testing.T) {
	breaker := NewBreaker(2, 2, 30*time.Second)

	// Open the circuit
	breaker.RecordFailure()
	breaker.RecordFailure()

	if breaker.State() != StateOpen {
		t.Fatal("Circuit should be open")
	}

	// Reset
	breaker.Reset()

	if breaker.State() != StateClosed {
		t.Errorf("Expected CLOSED after reset, got %s", breaker.State())
	}

	if !breaker.Allow() {
		t.Error("Should allow requests after reset")
	}
}

package balancer

import (
	"testing"
)

func TestRoundRobin_Next(t *testing.T) {
	backends := []*Backend{
		NewBackend("server1:8080", 1),
		NewBackend("server2:8080", 1),
		NewBackend("server3:8080", 1),
	}

	rr := NewRoundRobin(backends)

	// Test round-robin distribution
	expected := []string{"server1:8080", "server2:8080", "server3:8080", "server1:8080"}
	for i, exp := range expected {
		backend := rr.Next()
		if backend.Address != exp {
			t.Errorf("Request %d: expected %s, got %s", i, exp, backend.Address)
		}
	}
}

func TestRoundRobin_SkipsUnhealthy(t *testing.T) {
	backends := []*Backend{
		NewBackend("server1:8080", 1),
		NewBackend("server2:8080", 1),
		NewBackend("server3:8080", 1),
	}

	// Mark server2 as unhealthy
	backends[1].SetHealthy(false)

	rr := NewRoundRobin(backends)

	// Should only return healthy backends
	seen := make(map[string]int)
	for i := 0; i < 10; i++ {
		backend := rr.Next()
		seen[backend.Address]++
	}

	if _, ok := seen["server2:8080"]; ok {
		t.Error("Unhealthy backend was selected")
	}
	if seen["server1:8080"] == 0 || seen["server3:8080"] == 0 {
		t.Error("Healthy backends were not selected")
	}
}

func TestRoundRobin_NoHealthyBackends(t *testing.T) {
	backends := []*Backend{
		NewBackend("server1:8080", 1),
	}
	backends[0].SetHealthy(false)

	rr := NewRoundRobin(backends)

	backend := rr.Next()
	if backend != nil {
		t.Error("Expected nil when no healthy backends")
	}
}

func TestLeastConnections_Next(t *testing.T) {
	backends := []*Backend{
		NewBackend("server1:8080", 1),
		NewBackend("server2:8080", 1),
		NewBackend("server3:8080", 1),
	}

	// Simulate connections
	backends[0].IncrementConnections()
	backends[0].IncrementConnections()
	backends[1].IncrementConnections()
	// server3 has 0 connections

	lc := NewLeastConnections(backends)

	backend := lc.Next()
	if backend.Address != "server3:8080" {
		t.Errorf("Expected server3 (0 conns), got %s (%d conns)",
			backend.Address, backend.GetConnections())
	}
}

func TestLeastConnections_SkipsUnhealthy(t *testing.T) {
	backends := []*Backend{
		NewBackend("server1:8080", 1),
		NewBackend("server2:8080", 1),
	}

	// server1 has fewer connections but is unhealthy
	backends[1].IncrementConnections()
	backends[0].SetHealthy(false)

	lc := NewLeastConnections(backends)

	backend := lc.Next()
	if backend.Address != "server2:8080" {
		t.Errorf("Expected server2 (healthy), got %s", backend.Address)
	}
}

func TestBackend_ConnectionTracking(t *testing.T) {
	backend := NewBackend("test:8080", 1)

	if backend.GetConnections() != 0 {
		t.Error("Initial connections should be 0")
	}

	backend.IncrementConnections()
	backend.IncrementConnections()

	if backend.GetConnections() != 2 {
		t.Errorf("Expected 2 connections, got %d", backend.GetConnections())
	}

	backend.DecrementConnections()

	if backend.GetConnections() != 1 {
		t.Errorf("Expected 1 connection, got %d", backend.GetConnections())
	}
}

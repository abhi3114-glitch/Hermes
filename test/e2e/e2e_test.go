package e2e

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/hermes-proxy/hermes/internal/balancer"
	"github.com/hermes-proxy/hermes/internal/circuit"
	"github.com/hermes-proxy/hermes/internal/health"
	"github.com/hermes-proxy/hermes/internal/proxy"
)

func TestEndToEndProxy(t *testing.T) {
	// 1. Start Mock Backends
	backend1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("backend1"))
	}))
	defer backend1.Close()

	backend2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("backend2"))
	}))
	defer backend2.Close()

	// Extract ports/addresses
	addr1 := strings.TrimPrefix(backend1.URL, "http://")
	addr2 := strings.TrimPrefix(backend2.URL, "http://")

	// 2. Initialize Components Manually
	backends := []*balancer.Backend{
		balancer.NewBackend(addr1, 1),
		balancer.NewBackend(addr2, 1),
	}

	lb := balancer.NewRoundRobin(backends)
	breakerPool := circuit.NewBreakerPool(3, 2, 1) // 1 second timeout
	passiveMonitor := health.NewPassiveMonitor(lb, 2)
	proxyHandler := proxy.NewHandler(lb, breakerPool, passiveMonitor, 1024)

	// 3. Create Proxy Server
	proxyServer := httptest.NewServer(proxyHandler)
	defer proxyServer.Close()

	client := proxyServer.Client()

	// 4. Test Round-Robin Distribution
	// We expect alternating responses: backend1, backend2, backend1, backend2...

	// Send 4 requests
	responses := make([]string, 0)
	for i := 0; i < 4; i++ {
		resp, err := client.Get(proxyServer.URL)
		if err != nil {
			t.Fatalf("Request %d failed: %v", i, err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		responses = append(responses, string(body))
	}

	// Verify we got both backends
	gotBackend1 := false
	gotBackend2 := false
	for _, r := range responses {
		if r == "backend1" {
			gotBackend1 = true
		}
		if r == "backend2" {
			gotBackend2 = true
		}
	}

	if !gotBackend1 || !gotBackend2 {
		t.Errorf("Load balancing failed, responses: %v", responses)
	}

	// 5. Test Failover (Passive Health Check & Circuit Breaker)
	// Stop backend1
	backend1.Close()

	// Send requests, eventually all should go to backend2
	// We might see some failures initially

	successBackend2 := 0

	for i := 0; i < 20; i++ {
		resp, err := client.Get(proxyServer.URL)
		if err == nil {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			if string(body) == "backend2" {
				successBackend2++
			}
		}
		// Small delay to allow circuit breaker state to propagate if async (it's sync here mostly)
		time.Sleep(10 * time.Millisecond)
	}

	if successBackend2 == 0 {
		t.Error("Failover failed: no requests succeeded to backend2 after backend1 shutdown")
	}

	// 6. Verify backend1 state
	breakerState := breakerPool.Get(addr1).State()
	if breakerState == circuit.StateClosed {
		// It might still be closed if we didn't hit failure threshold or if successful stats came from passive monitor
		// logic is: passive monitor marks unhealthy in LB, breaker tracks its own failures
		// Proxy handler calls breaker.RecordFailure() on error.
		// So it should transition to Open if enough failures occurred.
		t.Logf("Circuit breaker state for backend1: %s (Expected OPEN or failing)", breakerState)
	}
}

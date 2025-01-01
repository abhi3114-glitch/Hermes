package admin

import (
	"encoding/json"
	"net/http"

	"github.com/hermes-proxy/hermes/internal/balancer"
	"github.com/hermes-proxy/hermes/internal/circuit"
	"github.com/hermes-proxy/hermes/internal/proxy"
)

// API provides admin/monitoring endpoints
type API struct {
	balancer    balancer.Balancer
	breakerPool *circuit.BreakerPool
	handler     *proxy.Handler
}

// NewAPI creates a new admin API
func NewAPI(b balancer.Balancer, breakerPool *circuit.BreakerPool, handler *proxy.Handler) *API {
	return &API{
		balancer:    b,
		breakerPool: breakerPool,
		handler:     handler,
	}
}

// Handler returns an http.Handler for the admin API
func (a *API) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/health", a.healthHandler)
	mux.HandleFunc("/backends", a.backendsHandler)
	mux.HandleFunc("/stats", a.statsHandler)
	mux.HandleFunc("/circuits", a.circuitsHandler)

	return mux
}

// BackendInfo represents backend status information
type BackendInfo struct {
	Address     string `json:"address"`
	Healthy     bool   `json:"healthy"`
	Connections int64  `json:"connections"`
	Weight      int    `json:"weight"`
}

// healthHandler returns the proxy health status
func (a *API) healthHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	backends := a.balancer.Backends()
	healthyCount := 0
	for _, b := range backends {
		if b.IsHealthy() {
			healthyCount++
		}
	}

	status := "healthy"
	httpStatus := http.StatusOK
	if healthyCount == 0 {
		status = "unhealthy"
		httpStatus = http.StatusServiceUnavailable
	} else if healthyCount < len(backends) {
		status = "degraded"
	}

	response := map[string]interface{}{
		"status":           status,
		"healthy_backends": healthyCount,
		"total_backends":   len(backends),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpStatus)
	json.NewEncoder(w).Encode(response)
}

// backendsHandler returns information about all backends
func (a *API) backendsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	backends := a.balancer.Backends()
	infos := make([]BackendInfo, len(backends))

	for i, b := range backends {
		infos[i] = BackendInfo{
			Address:     b.Address,
			Healthy:     b.IsHealthy(),
			Connections: b.GetConnections(),
			Weight:      b.Weight,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(infos)
}

// statsHandler returns request statistics
func (a *API) statsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	stats := a.handler.GetStats()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// circuitsHandler returns circuit breaker states
func (a *API) circuitsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	breakers := a.breakerPool.AllBreakers()

	response := make(map[string]string)
	for addr, state := range breakers {
		response[addr] = state.String()
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

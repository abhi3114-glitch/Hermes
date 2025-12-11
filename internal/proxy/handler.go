package proxy

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/hermes-proxy/hermes/internal/balancer"
	"github.com/hermes-proxy/hermes/internal/circuit"
	"github.com/hermes-proxy/hermes/internal/health"
)

// Handler handles HTTP proxying to backends
type Handler struct {
	balancer       balancer.Balancer
	breakerPool    *circuit.BreakerPool
	passiveMonitor *health.PassiveMonitor
	buffer         *Buffer
	client         *http.Client

	// Statistics
	TotalRequests  int64
	ActiveRequests int64
	FailedRequests int64
}

// NewHandler creates a new proxy handler
func NewHandler(
	b balancer.Balancer,
	breakerPool *circuit.BreakerPool,
	passiveMonitor *health.PassiveMonitor,
	maxRequestBody int64,
) *Handler {
	return &Handler{
		balancer:       b,
		breakerPool:    breakerPool,
		passiveMonitor: passiveMonitor,
		buffer:         NewBuffer(maxRequestBody),
		client: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				MaxIdleConnsPerHost: 100,
				IdleConnTimeout:     90 * time.Second,
				DisableCompression:  true,
			},
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse // Don't follow redirects
			},
		},
	}
}

// ServeHTTP implements the http.Handler interface
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&h.TotalRequests, 1)
	atomic.AddInt64(&h.ActiveRequests, 1)
	defer atomic.AddInt64(&h.ActiveRequests, -1)

	// Buffer the request body for potential retries
	var bodyBuf *bytes.Buffer
	var err error
	if r.Body != nil && r.ContentLength != 0 {
		bodyBuf, err = h.buffer.BufferRequest(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusRequestEntityTooLarge)
			return
		}
	}

	// Try to proxy the request
	if err := h.proxyRequest(w, r, bodyBuf); err != nil {
		atomic.AddInt64(&h.FailedRequests, 1)
		log.Printf("[PROXY] Error: %v", err)
		http.Error(w, "Bad Gateway", http.StatusBadGateway)
	}
}

func (h *Handler) proxyRequest(w http.ResponseWriter, r *http.Request, bodyBuf *bytes.Buffer) error {
	// Select a backend
	backend := h.balancer.Next()
	if backend == nil {
		return fmt.Errorf("no healthy backends available")
	}

	// Check circuit breaker
	breaker := h.breakerPool.Get(backend.Address)
	if !breaker.Allow() {
		return fmt.Errorf("circuit breaker open for %s", backend.Address)
	}

	// Track connection
	backend.IncrementConnections()
	defer backend.DecrementConnections()

	// Build the proxied request
	targetURL := fmt.Sprintf("http://%s%s", backend.Address, r.URL.RequestURI())

	var body io.Reader
	if bodyBuf != nil {
		body = bytes.NewReader(bodyBuf.Bytes())
	}

	proxyReq, err := http.NewRequestWithContext(r.Context(), r.Method, targetURL, body)
	if err != nil {
		return fmt.Errorf("failed to create proxy request: %w", err)
	}

	// Copy headers
	copyHeaders(proxyReq.Header, r.Header)

	// Add proxy headers
	h.setProxyHeaders(proxyReq, r)

	// Send the request
	resp, err := h.client.Do(proxyReq)
	if err != nil {
		breaker.RecordFailure()
		h.passiveMonitor.RecordFailure(backend.Address)
		return fmt.Errorf("failed to proxy request to %s: %w", backend.Address, err)
	}
	defer resp.Body.Close()

	// Record success
	breaker.RecordSuccess()
	h.passiveMonitor.RecordSuccess(backend.Address)

	// Copy response headers
	copyHeaders(w.Header(), resp.Header)

	// Set the status code
	w.WriteHeader(resp.StatusCode)

	// Copy response body
	if _, err := io.Copy(w, resp.Body); err != nil {
		log.Printf("[PROXY] Error copying response body: %v", err)
	}

	return nil
}

func (h *Handler) setProxyHeaders(proxyReq *http.Request, originalReq *http.Request) {
	// X-Forwarded-For
	clientIP := getClientIP(originalReq)
	if prior := originalReq.Header.Get("X-Forwarded-For"); prior != "" {
		clientIP = prior + ", " + clientIP
	}
	proxyReq.Header.Set("X-Forwarded-For", clientIP)

	// X-Real-IP
	proxyReq.Header.Set("X-Real-IP", getClientIP(originalReq))

	// X-Forwarded-Proto
	scheme := "http"
	if originalReq.TLS != nil {
		scheme = "https"
	}
	proxyReq.Header.Set("X-Forwarded-Proto", scheme)

	// X-Forwarded-Host
	proxyReq.Header.Set("X-Forwarded-Host", originalReq.Host)
}

func getClientIP(r *http.Request) string {
	// Check X-Real-IP header first
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}

	// Check X-Forwarded-For header
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		ips := strings.Split(xff, ",")
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}

	// Fall back to RemoteAddr
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func copyHeaders(dst, src http.Header) {
	for key, values := range src {
		for _, value := range values {
			dst.Add(key, value)
		}
	}
}

// GetStats returns current proxy statistics
func (h *Handler) GetStats() map[string]int64 {
	return map[string]int64{
		"total_requests":  atomic.LoadInt64(&h.TotalRequests),
		"active_requests": atomic.LoadInt64(&h.ActiveRequests),
		"failed_requests": atomic.LoadInt64(&h.FailedRequests),
	}
}

// Shutdown gracefully shuts down the proxy
func (h *Handler) Shutdown(ctx context.Context) error {
	// Wait for active requests to complete
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if atomic.LoadInt64(&h.ActiveRequests) == 0 {
				return nil
			}
		}
	}
}

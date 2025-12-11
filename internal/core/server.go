package core

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/hermes-proxy/hermes/internal/admin"
	"github.com/hermes-proxy/hermes/internal/balancer"
	"github.com/hermes-proxy/hermes/internal/circuit"
	"github.com/hermes-proxy/hermes/internal/health"
	"github.com/hermes-proxy/hermes/internal/proxy"
)

// Server is the main Hermes proxy server
type Server struct {
	config         *Config
	balancer       balancer.Balancer
	healthChecker  *health.Checker
	passiveMonitor *health.PassiveMonitor
	breakerPool    *circuit.BreakerPool
	proxyHandler   *proxy.Handler
	adminAPI       *admin.API

	proxyServer *http.Server
	adminServer *http.Server
}

// NewServer creates a new Hermes server
func NewServer(config *Config) (*Server, error) {
	// Create backends
	backends := make([]*balancer.Backend, len(config.Backends))
	for i, bc := range config.Backends {
		backends[i] = balancer.NewBackend(bc.Address, bc.Weight)
	}

	// Create the appropriate balancer
	var lb balancer.Balancer
	switch config.LoadBalancing.Algorithm {
	case "least-connections":
		lb = balancer.NewLeastConnections(backends)
	default:
		lb = balancer.NewRoundRobin(backends)
	}

	// Create circuit breaker pool
	breakerPool := circuit.NewBreakerPool(
		config.CircuitBreaker.FailureThreshold,
		config.CircuitBreaker.SuccessThreshold,
		int64(config.CircuitBreaker.Timeout.Seconds()),
	)

	// Create passive health monitor
	passiveMonitor := health.NewPassiveMonitor(lb, config.HealthCheck.UnhealthyThreshold)

	// Create proxy handler
	proxyHandler := proxy.NewHandler(lb, breakerPool, passiveMonitor, config.Buffer.MaxRequestBody)

	// Create health checker
	var healthChecker *health.Checker
	if config.HealthCheck.Enabled {
		healthChecker = health.NewChecker(
			lb,
			config.HealthCheck.Interval,
			config.HealthCheck.Timeout,
			config.HealthCheck.Path,
			config.HealthCheck.UnhealthyThreshold,
			config.HealthCheck.HealthyThreshold,
		)
	}

	// Create admin API
	adminAPI := admin.NewAPI(lb, breakerPool, proxyHandler)

	return &Server{
		config:         config,
		balancer:       lb,
		healthChecker:  healthChecker,
		passiveMonitor: passiveMonitor,
		breakerPool:    breakerPool,
		proxyHandler:   proxyHandler,
		adminAPI:       adminAPI,
	}, nil
}

// Run starts the server and blocks until shutdown
func (s *Server) Run() error {
	// Start health checker
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if s.healthChecker != nil {
		s.healthChecker.Start(ctx)
		log.Printf("[HERMES] Health checker started (interval: %v)", s.config.HealthCheck.Interval)
	}

	// Create proxy server
	s.proxyServer = &http.Server{
		Addr:         s.config.Server.Listen,
		Handler:      s.proxyHandler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Create admin server
	if s.config.Server.AdminListen != "" {
		s.adminServer = &http.Server{
			Addr:    s.config.Server.AdminListen,
			Handler: s.adminAPI.Handler(),
		}

		go func() {
			log.Printf("[HERMES] Admin API listening on %s", s.config.Server.AdminListen)
			if err := s.adminServer.ListenAndServe(); err != http.ErrServerClosed {
				log.Printf("[HERMES] Admin server error: %v", err)
			}
		}()
	}

	// Handle shutdown signals
	go s.handleShutdown(cancel)

	// Start proxy server
	log.Printf("[HERMES] Proxy listening on %s", s.config.Server.Listen)
	log.Printf("[HERMES] Load balancing algorithm: %s", s.config.LoadBalancing.Algorithm)
	log.Printf("[HERMES] Backends: %d configured", len(s.config.Backends))

	if err := s.proxyServer.ListenAndServe(); err != http.ErrServerClosed {
		return err
	}

	return nil
}

func (s *Server) handleShutdown(cancel context.CancelFunc) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	log.Println("[HERMES] Shutdown signal received")

	// Cancel context to stop health checker
	cancel()

	// Graceful shutdown with 30 second timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if s.adminServer != nil {
		s.adminServer.Shutdown(shutdownCtx)
	}

	if err := s.proxyServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("[HERMES] Shutdown error: %v", err)
	}

	log.Println("[HERMES] Server stopped")
}

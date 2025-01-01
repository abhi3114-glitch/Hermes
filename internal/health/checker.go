package health

import (
	"context"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/hermes-proxy/hermes/internal/balancer"
)

// Checker performs active health checks on backends
type Checker struct {
	balancer           balancer.Balancer
	interval           time.Duration
	timeout            time.Duration
	path               string
	unhealthyThreshold int
	healthyThreshold   int

	// Track consecutive successes/failures per backend
	failureCounts map[string]int
	successCounts map[string]int
	mu            sync.Mutex

	client *http.Client
	cancel context.CancelFunc
}

// NewChecker creates a new health checker
func NewChecker(
	b balancer.Balancer,
	interval, timeout time.Duration,
	path string,
	unhealthyThreshold, healthyThreshold int,
) *Checker {
	return &Checker{
		balancer:           b,
		interval:           interval,
		timeout:            timeout,
		path:               path,
		unhealthyThreshold: unhealthyThreshold,
		healthyThreshold:   healthyThreshold,
		failureCounts:      make(map[string]int),
		successCounts:      make(map[string]int),
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

// Start begins the health check loop
func (c *Checker) Start(ctx context.Context) {
	ctx, c.cancel = context.WithCancel(ctx)
	go c.run(ctx)
}

// Stop terminates the health check loop
func (c *Checker) Stop() {
	if c.cancel != nil {
		c.cancel()
	}
}

func (c *Checker) run(ctx context.Context) {
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	// Run initial check immediately
	c.checkAll()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.checkAll()
		}
	}
}

func (c *Checker) checkAll() {
	backends := c.balancer.Backends()
	var wg sync.WaitGroup

	for _, backend := range backends {
		wg.Add(1)
		go func(b *balancer.Backend) {
			defer wg.Done()
			c.checkBackend(b)
		}(backend)
	}

	wg.Wait()
}

func (c *Checker) checkBackend(backend *balancer.Backend) {
	url := "http://" + backend.Address + c.path

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		c.recordFailure(backend)
		return
	}

	resp, err := c.client.Do(req)
	if err != nil {
		c.recordFailure(backend)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		c.recordSuccess(backend)
	} else {
		c.recordFailure(backend)
	}
}

func (c *Checker) recordFailure(backend *balancer.Backend) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.successCounts[backend.Address] = 0
	c.failureCounts[backend.Address]++

	if c.failureCounts[backend.Address] >= c.unhealthyThreshold {
		if backend.IsHealthy() {
			log.Printf("[HEALTH] Backend %s marked UNHEALTHY after %d failures",
				backend.Address, c.failureCounts[backend.Address])
			backend.SetHealthy(false)
		}
	}
}

func (c *Checker) recordSuccess(backend *balancer.Backend) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.failureCounts[backend.Address] = 0
	c.successCounts[backend.Address]++

	if c.successCounts[backend.Address] >= c.healthyThreshold {
		if !backend.IsHealthy() {
			log.Printf("[HEALTH] Backend %s marked HEALTHY after %d successes",
				backend.Address, c.successCounts[backend.Address])
			backend.SetHealthy(true)
		}
	}
}

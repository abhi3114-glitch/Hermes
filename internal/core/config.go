package core

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config represents the complete proxy configuration
type Config struct {
	Server         ServerConfig         `yaml:"server"`
	Backends       []BackendConfig      `yaml:"backends"`
	LoadBalancing  LoadBalancingConfig  `yaml:"load_balancing"`
	HealthCheck    HealthCheckConfig    `yaml:"health_check"`
	CircuitBreaker CircuitBreakerConfig `yaml:"circuit_breaker"`
	Buffer         BufferConfig         `yaml:"buffer"`
}

// ServerConfig holds the main server settings
type ServerConfig struct {
	Listen      string `yaml:"listen"`
	AdminListen string `yaml:"admin_listen"`
}

// BackendConfig defines a single backend server
type BackendConfig struct {
	Address string `yaml:"address"`
	Weight  int    `yaml:"weight"`
}

// LoadBalancingConfig specifies the load balancing strategy
type LoadBalancingConfig struct {
	Algorithm string `yaml:"algorithm"` // "round-robin" or "least-connections"
}

// HealthCheckConfig controls health checking behavior
type HealthCheckConfig struct {
	Enabled            bool          `yaml:"enabled"`
	Interval           time.Duration `yaml:"interval"`
	Timeout            time.Duration `yaml:"timeout"`
	Path               string        `yaml:"path"`
	UnhealthyThreshold int           `yaml:"unhealthy_threshold"`
	HealthyThreshold   int           `yaml:"healthy_threshold"`
}

// CircuitBreakerConfig controls circuit breaker behavior
type CircuitBreakerConfig struct {
	Enabled          bool          `yaml:"enabled"`
	FailureThreshold int           `yaml:"failure_threshold"`
	SuccessThreshold int           `yaml:"success_threshold"`
	Timeout          time.Duration `yaml:"timeout"`
}

// BufferConfig controls request buffering
type BufferConfig struct {
	MaxRequestBody int64 `yaml:"max_request_body"`
}

// DefaultConfig returns sensible default configuration
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Listen:      ":8080",
			AdminListen: ":8081",
		},
		LoadBalancing: LoadBalancingConfig{
			Algorithm: "round-robin",
		},
		HealthCheck: HealthCheckConfig{
			Enabled:            true,
			Interval:           10 * time.Second,
			Timeout:            2 * time.Second,
			Path:               "/health",
			UnhealthyThreshold: 3,
			HealthyThreshold:   2,
		},
		CircuitBreaker: CircuitBreakerConfig{
			Enabled:          true,
			FailureThreshold: 5,
			SuccessThreshold: 3,
			Timeout:          30 * time.Second,
		},
		Buffer: BufferConfig{
			MaxRequestBody: 10 * 1024 * 1024, // 10MB
		},
	}
}

// LoadConfig reads configuration from a YAML file
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	config := DefaultConfig()
	if err := yaml.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return config, nil
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	if c.Server.Listen == "" {
		return fmt.Errorf("server.listen is required")
	}

	if len(c.Backends) == 0 {
		return fmt.Errorf("at least one backend is required")
	}

	for i, backend := range c.Backends {
		if backend.Address == "" {
			return fmt.Errorf("backend[%d].address is required", i)
		}
		if backend.Weight < 0 {
			return fmt.Errorf("backend[%d].weight must be non-negative", i)
		}
	}

	validAlgorithms := map[string]bool{
		"round-robin":       true,
		"least-connections": true,
	}
	if !validAlgorithms[c.LoadBalancing.Algorithm] {
		return fmt.Errorf("invalid load balancing algorithm: %s", c.LoadBalancing.Algorithm)
	}

	return nil
}

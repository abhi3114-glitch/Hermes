package balancer

// LeastConnections implements least-connections load balancing
type LeastConnections struct {
	*BaseBalancer
}

// NewLeastConnections creates a new least-connections balancer
func NewLeastConnections(backends []*Backend) *LeastConnections {
	return &LeastConnections{
		BaseBalancer: NewBaseBalancer(backends),
	}
}

// Next returns the healthy backend with the fewest active connections
func (l *LeastConnections) Next() *Backend {
	healthy := l.healthyBackends()
	if len(healthy) == 0 {
		return nil
	}

	var selected *Backend
	minConns := int64(-1)

	for _, backend := range healthy {
		conns := backend.GetConnections()
		if minConns == -1 || conns < minConns {
			minConns = conns
			selected = backend
		}
	}

	return selected
}

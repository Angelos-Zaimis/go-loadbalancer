package strategy

import (
	"math"

	"github.com/angeloszaimis/load-balancer/internal/backend"
)

type leastConnStrategy struct {
}

func (l *leastConnStrategy) SelectBackend(backends []*backend.Backend) *backend.Backend {
	if len(backends) == 0 {
		return nil
	}

	var bestBackend *backend.Backend
	bestConns := math.MaxInt32

	for _, backend := range backends {
		activeConns := backend.ActiveConnections()
		if activeConns < bestConns {
			bestConns = activeConns
			bestBackend = backend
		}
	}

	return bestBackend
}

func NewLeastConnStrategy() Strategy {
	return &leastConnStrategy{}
}

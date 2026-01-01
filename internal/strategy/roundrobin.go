package strategy

import (
	"sync/atomic"

	"github.com/angeloszaimis/load-balancer/internal/backend"
)

type roundRobinStrategy struct {
	current uint64
}

func (rb *roundRobinStrategy) SelectBackend(backends []*backend.Backend) *backend.Backend {
	if len(backends) == 0 {
		return nil
	}

	n := atomic.AddUint64(&rb.current, 1)

	index := (n - 1) % uint64(len(backends))

	return backends[index]
}

func NewRoundRobinStrategy() Strategy {
	return &roundRobinStrategy{
		current: 0,
	}
}

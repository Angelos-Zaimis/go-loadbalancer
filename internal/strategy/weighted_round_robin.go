package strategy

import (
	"sync"

	"github.com/angeloszaimis/load-balancer/internal/backend"
)

type weightedRoundRobinStrategy struct {
	mutex   sync.Mutex
	current map[*backend.Backend]int
}

func NewWeightedRoundRobinStrategy() Strategy {
	return &weightedRoundRobinStrategy{
		current: make(map[*backend.Backend]int),
	}
}

func (w *weightedRoundRobinStrategy) SelectBackend(backends []*backend.Backend) *backend.Backend {
	if len(backends) == 0 {
		return nil
	}

	w.mutex.Lock()
	defer w.mutex.Unlock()

	w.cleanup(backends)

	totalWeight := 0
	var chosen *backend.Backend

	for _, b := range backends {
		weight := b.Weight()
		if weight <= 0 {
			continue
		}

		w.current[b] += weight
		totalWeight += weight

		if chosen == nil || w.current[b] > w.current[chosen] {
			chosen = b
		}
	}

	if chosen == nil || totalWeight == 0 {
		return nil
	}

	w.current[chosen] -= totalWeight
	return chosen
}

func (w *weightedRoundRobinStrategy) cleanup(backends []*backend.Backend) {
	alive := make(map[*backend.Backend]struct{}, len(backends))

	for _, b := range backends {
		alive[b] = struct{}{}
	}

	for b := range w.current {
		if _, ok := alive[b]; !ok {
			delete(w.current, b)
		}
	}
}

package strategy

import (
	"sync"

	"github.com/angeloszaimis/load-balancer/internal/backend"
)

// weightedRoundRobinStrategy implements smooth weighted round-robin load balancing.
// Uses the Nginx algorithm: each backend accumulates its weight per selection cycle,
// the highest current value is chosen, then reduced by the sum of all weights.
type weightedRoundRobinStrategy struct {
	mutex   sync.Mutex
	current map[*backend.Backend]int // Tracks accumulated weight per backend
}

// NewWeightedRoundRobinStrategy creates a weighted round-robin strategy instance.
func NewWeightedRoundRobinStrategy() Strategy {
	return &weightedRoundRobinStrategy{
		current: make(map[*backend.Backend]int),
	}
}

// SelectBackend picks the backend with the highest accumulated weight.
// Distributes requests proportionally to configured weights while maintaining smooth distribution.
func (w *weightedRoundRobinStrategy) SelectBackend(backends []*backend.Backend) *backend.Backend {
	if len(backends) == 0 {
		return nil
	}

	w.mutex.Lock()
	defer w.mutex.Unlock()

	// Remove stale backends from tracking map
	w.cleanup(backends)

	totalWeight := 0
	var chosen *backend.Backend

	// Add each backend's weight to its current value and find the highest
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

	// No valid backend with positive weight
	if chosen == nil || totalWeight == 0 {
		return nil
	}

	// Reduce chosen backend's current value by total weight to balance future selections
	w.current[chosen] -= totalWeight
	return chosen
}

// cleanup removes entries for backends no longer in the active list.
// Prevents unbounded map growth when backends are removed from the pool.
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

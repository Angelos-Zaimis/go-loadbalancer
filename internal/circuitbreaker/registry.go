package circuitbreaker

import (
    "sync"
    "time"
)

type Registry struct {
	mutex 	  sync.RWMutex
	breakers  map[string]*CircuitBreaker
	threshold int
	timeout   time.Duration
}

func NewRegistry(threshold int, timeout time.Duration) *Registry {
	return &Registry{
		breakers: make(map[string]*CircuitBreaker),
		threshold: threshold,
		timeout: timeout,
	}
}

func (r *Registry) GetBreaker(backendURL string) *CircuitBreaker {
	r.mutex.RLock()
	cb, exists := r.breakers[backendURL]
	r.mutex.RUnlock()

	if exists {
		return cb
	}
	
	r.mutex.Lock()
	defer r.mutex.Unlock()

	// Double-check: another goroutine may have created it
	if cb, exists = r.breakers[backendURL]; exists {
		return cb
	}

	cb = NewCircuitBreaker(r.threshold, r.timeout)
	r.breakers[backendURL] = cb
	return cb
}

func (r *Registry) Reset() {
    r.mutex.Lock()
    defer r.mutex.Unlock()
    r.breakers = make(map[string]*CircuitBreaker)
}

func (r *Registry) Stats() map[string]State {
    r.mutex.RLock()
    defer r.mutex.RUnlock()

    stats := make(map[string]State, len(r.breakers))
    for url, cb := range r.breakers {
        stats[url] = cb.State()
    }
    return stats
}
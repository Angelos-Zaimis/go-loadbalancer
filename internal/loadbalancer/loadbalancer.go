package loadbalancer

import (
	"fmt"
	"sync"

	"github.com/angeloszaimis/load-balancer/internal/backend"
	"github.com/angeloszaimis/load-balancer/internal/strategy"
)

type LoadBalancer struct {
	strategy strategy.Strategy
	mutex    sync.Mutex
}

func NewLoadBalancer(strategy strategy.Strategy) *LoadBalancer {
	return &LoadBalancer{
		strategy: strategy,
		mutex:    sync.Mutex{},
	}
}

func (lb *LoadBalancer) GetAndReserveServer(backends []*backend.Backend) (*backend.Backend, error) {
	lb.mutex.Lock()

	healthyBackends := lb.filterHealthyBackends(backends)
	if len(healthyBackends) == 0 {
		lb.mutex.Unlock()
		return nil, fmt.Errorf("no healthy backends")
	}

	chosen := lb.strategy.SelectBackend(healthyBackends)
	lb.mutex.Unlock()

	if chosen == nil {
		return nil, fmt.Errorf("strategy returned nil backend")
	}

	chosen.IncrementConn()
	return chosen, nil
}

func (lb *LoadBalancer) GetAndReserveServerWithKey(backends []*backend.Backend, key string) (*backend.Backend, error) {
	lb.mutex.Lock()
	defer lb.mutex.Unlock()

	healthyBackends := lb.filterHealthyBackends(backends)
	if len(healthyBackends) == 0 {
		return nil, fmt.Errorf("no healthy backends")
	}

	// Set the key if strategy supports it
	if ks, ok := lb.strategy.(interface{ SetKey(string) }); ok {
		ks.SetKey(key)
	}

	chosen := lb.strategy.SelectBackend(healthyBackends)
	if chosen == nil {
		return nil, fmt.Errorf("strategy returned nil backend")
	}

	chosen.IncrementConn()
	return chosen, nil
}

func (lb *LoadBalancer) filterHealthyBackends(backends []*backend.Backend) []*backend.Backend {
	healthy := make([]*backend.Backend, 0, len(backends))

	for _, b := range backends {
		if b.IsHealthy() {
			healthy = append(healthy, b)
		}
	}

	return healthy
}

func (lb *LoadBalancer) LoadBalancerStrategy() strategy.Strategy {
	return lb.strategy
}

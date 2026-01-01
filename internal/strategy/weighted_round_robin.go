package strategy

import "github.com/angeloszaimis/load-balancer/internal/backend"

type weightedRoundRobinStradegy struct {
}

func (w *weightedRoundRobinStradegy) SelectBackend(backends []*backend.Backend) *backend.Backend {
	panic("unimplemented")
}

func NewWeightedRoundRobinStradegy() Strategy {
	return &weightedRoundRobinStradegy{}
}

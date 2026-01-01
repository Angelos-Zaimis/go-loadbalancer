package strategy

import (
	"math/rand/v2"

	"github.com/angeloszaimis/load-balancer/internal/backend"
)

type randomStrategy struct{}

func (r *randomStrategy) SelectBackend(backends []*backend.Backend) *backend.Backend {
	if len(backends) == 0 {
		return nil
	}

	index := rand.IntN(len(backends))
	return backends[index]
}

func NewRandomStrategy() Strategy {
	return &randomStrategy{}
}

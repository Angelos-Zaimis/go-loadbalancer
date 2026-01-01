package strategy

import (
	"time"

	"github.com/angeloszaimis/load-balancer/internal/backend"
)

type leastResponseStrategy struct{}

func (l *leastResponseStrategy) SelectBackend(backends []*backend.Backend) *backend.Backend {
	if len(backends) == 0 {
		return nil
	}

	var chosen *backend.Backend
	var best time.Duration

	for _, b := range backends {
		ewma := b.EWMATime()

		if ewma == 0 {
			return b
		}

		score := ewma * (time.Duration(b.ActiveConnections()) + 1)

		if chosen == nil {
			chosen = b
			best = score
			continue
		}

		if score < best {
			chosen = b
			best = score
		}
	}

	return chosen
}

func NewLeastResponseStrategy() Strategy {
	return &leastResponseStrategy{}
}

package strategy

import (
	"github.com/angeloszaimis/load-balancer/internal/backend"
)

type Strategy interface {
	SelectBackend(backends []*backend.Backend) *backend.Backend
}

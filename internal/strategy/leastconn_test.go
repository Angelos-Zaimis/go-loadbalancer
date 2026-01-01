package strategy_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/angeloszaimis/load-balancer/internal/backend"
	"github.com/angeloszaimis/load-balancer/internal/strategy"
)

var _ = Describe("Leastconn", func() {
	var (
		strat    strategy.Strategy
		backends []*backend.Backend
	)

	BeforeEach(func() {
		strat = strategy.NewLeastConnStrategy()
		backends = []*backend.Backend{
			backend.New(mustParseURL("http://localhost:8081")),
			backend.New(mustParseURL("http://localhost:8082")),
			backend.New(mustParseURL("http://localhost:8083")),
		}
		for _, b := range backends {
			b.SetHealthy(true)
		}
	})

	Describe("SelectBackend", func() {
		It("should select backend with fewest connections", func() {
			backends[0].IncrementConn()
			backends[0].IncrementConn()
			backends[1].IncrementConn()

			selected := strat.SelectBackend(backends)
			Expect(selected).To(Equal(backends[2]))
		})
	})
})

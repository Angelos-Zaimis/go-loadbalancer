package strategy_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/angeloszaimis/load-balancer/internal/backend"
	"github.com/angeloszaimis/load-balancer/internal/strategy"
)

var _ = Describe("ConsistentHash", func() {
	var (
		strat    strategy.Strategy
		backends []*backend.Backend
	)

	BeforeEach(func() {
		strat = strategy.NewConsistentHashStrategy(100)
		backends = []*backend.Backend{
			backend.New(mustParseURL("http://localhost:8081"), 1),
			backend.New(mustParseURL("http://localhost:8082"), 1),
			backend.New(mustParseURL("http://localhost:8083"), 1),
		}
		for _, b := range backends {
			b.SetHealthy(true)
		}
	})

	Describe("SelectBackend with SetKey", func() {
		It("should return same backend for same IP", func() {
			hasher, ok := strat.(interface{ SetKey(string) })
			Expect(ok).To(BeTrue())

			ip := "192.168.1.100"
			hasher.SetKey(ip)
			first := strat.SelectBackend(backends)

			for i := 0; i < 5; i++ {
				hasher.SetKey(ip)
				selected := strat.SelectBackend(backends)
				Expect(selected).To(Equal(first))
			}
		})
	})
})

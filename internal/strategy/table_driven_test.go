package strategy_test

import (
	"net/url"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/angeloszaimis/load-balancer/internal/backend"
	"github.com/angeloszaimis/load-balancer/internal/strategy"
)

// Demonstrate Go table-driven testing best practice using Ginkgo's DescribeTable
var _ = Describe("Table-Driven Strategy Tests", func() {
	DescribeTable("All strategies can be instantiated",
		func(createStrat func() strategy.Strategy) {
			strat := createStrat()
			Expect(strat).NotTo(BeNil())
		},
		Entry("Round Robin", func() strategy.Strategy { return strategy.NewRoundRobinStrategy() }),
		Entry("Random", func() strategy.Strategy { return strategy.NewRandomStrategy() }),
		Entry("Least Connections", func() strategy.Strategy { return strategy.NewLeastConnStrategy() }),
		Entry("Least Response Time", func() strategy.Strategy { return strategy.NewLeastResponseStrategy() }),
		Entry("Consistent Hash with 100 vnodes", func() strategy.Strategy { return strategy.NewConsistentHashStrategy(100) }),
		Entry("Weighted Round Robin", func() strategy.Strategy { return strategy.NewWeightedRoundRobinStrategy() }),
	)

	DescribeTable("All strategies select from healthy backends",
		func(createStrat func() strategy.Strategy) {
			strat := createStrat()
			backends := []*backend.Backend{
				backend.New(mustParseURLTable("http://localhost:8081"), 1),
				backend.New(mustParseURLTable("http://localhost:8082"), 1),
				backend.New(mustParseURLTable("http://localhost:8083"), 1),
			}

			for _, b := range backends {
				b.SetHealthy(true)
			}

			selected := strat.SelectBackend(backends)
			Expect(selected).NotTo(BeNil())
			Expect(backends).To(ContainElement(selected))
		},
		Entry("Round Robin", func() strategy.Strategy { return strategy.NewRoundRobinStrategy() }),
		Entry("Random", func() strategy.Strategy { return strategy.NewRandomStrategy() }),
		Entry("Least Connections", func() strategy.Strategy { return strategy.NewLeastConnStrategy() }),
		Entry("Least Response Time", func() strategy.Strategy { return strategy.NewLeastResponseStrategy() }),
		Entry("Consistent Hash", func() strategy.Strategy { return strategy.NewConsistentHashStrategy(100) }),
	)

	DescribeTable("Least-connection strategy behavior",
		func(createStrat func() strategy.Strategy) {
			strat := createStrat()
			backends := []*backend.Backend{
				backend.New(mustParseURLTable("http://localhost:8081"), 1),
				backend.New(mustParseURLTable("http://localhost:8082"), 1),
			}

			backends[0].SetHealthy(true)
			backends[1].SetHealthy(true)
			backends[0].IncrementConn()
			backends[0].IncrementConn()

			selected := strat.SelectBackend(backends)
			Expect(selected).To(Equal(backends[1]), "Should prefer backend with fewer connections")
		},
		Entry("Least Connections", func() strategy.Strategy { return strategy.NewLeastConnStrategy() }),
	)
})

func mustParseURLTable(rawURL string) *url.URL {
	u, err := url.Parse(rawURL)
	if err != nil {
		panic(err)
	}
	return u
}

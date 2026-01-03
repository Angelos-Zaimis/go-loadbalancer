package strategy_test

import (
	"net/url"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/angeloszaimis/load-balancer/internal/backend"
	"github.com/angeloszaimis/load-balancer/internal/strategy"
)

var _ = Describe("Roundrobin", func() {
	var (
		strat    strategy.Strategy
		backends []*backend.Backend
	)

	BeforeEach(func() {
		strat = strategy.NewRoundRobinStrategy()

		backends = []*backend.Backend{
			backend.New(mustParseURL("http://localhost:8081"), 1),
			backend.New(mustParseURL("http://localhost:8082"), 1),
			backend.New(mustParseURL("http://localhost:8083"), 1),
		}

		for _, b := range backends {
			b.SetHealthy(true)
		}
	})

	Describe("SelectBackend", func() {
		Context("with all healthy backends", func() {
			It("should cycle through backends in order", func() {
				Expect(strat.SelectBackend(backends)).To(Equal(backends[0]))
				Expect(strat.SelectBackend(backends)).To(Equal(backends[1]))
				Expect(strat.SelectBackend(backends)).To(Equal(backends[2]))
				Expect(strat.SelectBackend(backends)).To(Equal(backends[0]))
			})

			It("should distribute load evenly", func() {
				counts := make(map[string]int)
				for i := 0; i < 300; i++ {
					selected := strat.SelectBackend(backends)
					counts[selected.URL().String()]++
				}
				Expect(counts["http://localhost:8081"]).To(Equal(100))
				Expect(counts["http://localhost:8082"]).To(Equal(100))
				Expect(counts["http://localhost:8083"]).To(Equal(100))
			})
		})

		Context("with empty backend list", func() {
			It("should return nil", func() {
				Expect(strat.SelectBackend([]*backend.Backend{})).To(BeNil())
			})
		})
	})
})

var _ = Describe("LeastResponse", func() {
	var (
		strat    strategy.Strategy
		backends []*backend.Backend
	)

	BeforeEach(func() {
		strat = strategy.NewLeastResponseStrategy()
		backends = []*backend.Backend{
			backend.New(mustParseURL("http://localhost:8081"), 1),
			backend.New(mustParseURL("http://localhost:8082"), 1),
			backend.New(mustParseURL("http://localhost:8083"), 1),
		}
	})

	It("should select backend with lowest EWMA response time", func() {
		backends[0].RecordResponse(100 * time.Millisecond)
		backends[1].RecordResponse(50 * time.Millisecond)
		backends[2].RecordResponse(200 * time.Millisecond)

		selected := strat.SelectBackend(backends)
		Expect(selected).To(Equal(backends[1]))
	})

	It("should select first backend when all have zero EWMA", func() {
		selected := strat.SelectBackend(backends)
		Expect(selected).To(Equal(backends[0]))
	})

	It("should return nil for empty backend list", func() {
		selected := strat.SelectBackend([]*backend.Backend{})
		Expect(selected).To(BeNil())
	})
})

var _ = Describe("Random", func() {
	var (
		strat    strategy.Strategy
		backends []*backend.Backend
	)

	BeforeEach(func() {
		strat = strategy.NewRandomStrategy()
		backends = []*backend.Backend{
			backend.New(mustParseURL("http://localhost:8081"), 1),
			backend.New(mustParseURL("http://localhost:8082"), 1),
			backend.New(mustParseURL("http://localhost:8083"), 1),
		}
	})

	It("should select a backend", func() {
		selected := strat.SelectBackend(backends)
		Expect(selected).NotTo(BeNil())
		Expect(backends).To(ContainElement(selected))
	})

	It("should distribute across backends over multiple calls", func() {
		backendSet := make(map[*backend.Backend]bool)

		for i := 0; i < 100; i++ {
			selected := strat.SelectBackend(backends)
			backendSet[selected] = true
		}

		Expect(len(backendSet)).To(BeNumerically(">=", 2))
	})

	It("should return nil for empty backend list", func() {
		selected := strat.SelectBackend([]*backend.Backend{})
		Expect(selected).To(BeNil())
	})
})

var _ = Describe("WeightedRoundRobin", func() {
	var (
		strat    strategy.Strategy
		backends []*backend.Backend
	)

	BeforeEach(func() {
		backends = []*backend.Backend{
			backend.New(mustParseURL("http://localhost:8081"), 1),
			backend.New(mustParseURL("http://localhost:8082"), 1),
			backend.New(mustParseURL("http://localhost:8083"), 1),
		}
		strat = strategy.NewWeightedRoundRobinStrategy()
	})

	It("should create strategy", func() {
		Expect(strat).NotTo(BeNil())
	})

	It("should select backend based on weights", func() {
		backend := strat.SelectBackend(backends)
		Expect(backend).NotTo(BeNil())
		Expect(backends).To(ContainElement(backend))
	})

	It("should distribute requests proportionally to weights", func() {
		counts := make(map[*backend.Backend]int)
		iterations := 100

		for i := 0; i < iterations; i++ {
			backend := strat.SelectBackend(backends)
			counts[backend]++
		}

		// Each backend should receive some requests
		Expect(len(counts)).To(Equal(3))
		for _, count := range counts {
			Expect(count).To(BeNumerically(">", 0))
		}
	})
})

func mustParseURL(rawURL string) *url.URL {
	u, err := url.Parse(rawURL)
	if err != nil {
		panic(err)
	}
	return u
}

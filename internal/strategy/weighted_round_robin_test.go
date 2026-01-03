package strategy_test

import (
	"net/url"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/angeloszaimis/load-balancer/internal/backend"
	"github.com/angeloszaimis/load-balancer/internal/strategy"
)

var _ = Describe("WeightedRoundRobinStrategy", func() {
	var (
		strat    strategy.Strategy
		backends []*backend.Backend
	)

	BeforeEach(func() {
		strat = strategy.NewWeightedRoundRobinStrategy()
	})

	It("should create strategy", func() {
		Expect(strat).NotTo(BeNil())
	})

	Context("with equal weights", func() {
		BeforeEach(func() {
			backends = []*backend.Backend{
				backend.New(mustParseURLWeighted("http://localhost:8081"), 1),
				backend.New(mustParseURLWeighted("http://localhost:8082"), 1),
				backend.New(mustParseURLWeighted("http://localhost:8083"), 1),
			}
		})

		It("should distribute requests evenly", func() {
			counts := make(map[*backend.Backend]int)
			iterations := 300

			for i := 0; i < iterations; i++ {
				b := strat.SelectBackend(backends)
				Expect(b).NotTo(BeNil())
				counts[b]++
			}

			Expect(len(counts)).To(Equal(3))
			for _, count := range counts {
				Expect(count).To(BeNumerically("~", 100, 10))
			}
		})
	})

	Context("with different weights", func() {
		BeforeEach(func() {
			backends = []*backend.Backend{
				backend.New(mustParseURLWeighted("http://localhost:8081"), 5),
				backend.New(mustParseURLWeighted("http://localhost:8082"), 3),
				backend.New(mustParseURLWeighted("http://localhost:8083"), 1),
			}
		})

		It("should distribute requests proportionally to weights", func() {
			counts := make(map[*backend.Backend]int)
			iterations := 900

			for i := 0; i < iterations; i++ {
				b := strat.SelectBackend(backends)
				Expect(b).NotTo(BeNil())
				counts[b]++
			}

			Expect(len(counts)).To(Equal(3))
			Expect(counts[backends[0]]).To(BeNumerically("~", 500, 20))
			Expect(counts[backends[1]]).To(BeNumerically("~", 300, 20))
			Expect(counts[backends[2]]).To(BeNumerically("~", 100, 20))
		})

		It("should respect weight ratios", func() {
			counts := make(map[*backend.Backend]int)
			iterations := 450

			for i := 0; i < iterations; i++ {
				b := strat.SelectBackend(backends)
				counts[b]++
			}

			ratio1to2 := float64(counts[backends[0]]) / float64(counts[backends[1]])
			Expect(ratio1to2).To(BeNumerically("~", 5.0/3.0, 0.3))

			ratio2to3 := float64(counts[backends[1]]) / float64(counts[backends[2]])
			Expect(ratio2to3).To(BeNumerically("~", 3.0, 0.5))
		})
	})

	Context("with extreme weight differences", func() {
		BeforeEach(func() {
			backends = []*backend.Backend{
				backend.New(mustParseURLWeighted("http://localhost:8081"), 100),
				backend.New(mustParseURLWeighted("http://localhost:8082"), 1),
			}
		})

		It("should still distribute correctly", func() {
			counts := make(map[*backend.Backend]int)
			iterations := 1010

			for i := 0; i < iterations; i++ {
				b := strat.SelectBackend(backends)
				counts[b]++
			}

			Expect(counts[backends[0]]).To(BeNumerically("~", 1000, 20))
			Expect(counts[backends[1]]).To(BeNumerically("~", 10, 5))
		})
	})

	Context("edge cases", func() {
		It("should return nil for empty backends", func() {
			backends = []*backend.Backend{}
			b := strat.SelectBackend(backends)
			Expect(b).To(BeNil())
		})

		It("should return nil for nil backends", func() {
			b := strat.SelectBackend(nil)
			Expect(b).To(BeNil())
		})

		It("should handle single backend", func() {
			backends = []*backend.Backend{
				backend.New(mustParseURLWeighted("http://localhost:8081"), 10),
			}

			for i := 0; i < 10; i++ {
				b := strat.SelectBackend(backends)
				Expect(b).To(Equal(backends[0]))
			}
		})

		It("should skip backends with zero weight", func() {
			backends = []*backend.Backend{
				backend.New(mustParseURLWeighted("http://localhost:8081"), 0),
				backend.New(mustParseURLWeighted("http://localhost:8082"), 5),
				backend.New(mustParseURLWeighted("http://localhost:8083"), 0),
			}

			counts := make(map[*backend.Backend]int)
			for i := 0; i < 100; i++ {
				b := strat.SelectBackend(backends)
				Expect(b).To(Equal(backends[1]))
				counts[b]++
			}

			Expect(counts[backends[1]]).To(Equal(100))
			Expect(counts[backends[0]]).To(Equal(0))
			Expect(counts[backends[2]]).To(Equal(0))
		})

		It("should return nil when all backends have zero weight", func() {
			backends = []*backend.Backend{
				backend.New(mustParseURLWeighted("http://localhost:8081"), 0),
				backend.New(mustParseURLWeighted("http://localhost:8082"), 0),
			}

			b := strat.SelectBackend(backends)
			Expect(b).To(BeNil())
		})
	})

	Context("dynamic backend changes", func() {
		It("should handle backend removal correctly", func() {
			backends = []*backend.Backend{
				backend.New(mustParseURLWeighted("http://localhost:8081"), 1),
				backend.New(mustParseURLWeighted("http://localhost:8082"), 1),
				backend.New(mustParseURLWeighted("http://localhost:8083"), 1),
			}

			for i := 0; i < 10; i++ {
				strat.SelectBackend(backends)
			}

			backends = backends[:2]

			counts := make(map[*backend.Backend]int)
			for i := 0; i < 100; i++ {
				b := strat.SelectBackend(backends)
				Expect(b).NotTo(BeNil())
				Expect(backends).To(ContainElement(b))
				counts[b]++
			}

			Expect(len(counts)).To(Equal(2))
		})

		It("should handle backend addition correctly", func() {
			backends = []*backend.Backend{
				backend.New(mustParseURLWeighted("http://localhost:8081"), 1),
			}

			for i := 0; i < 10; i++ {
				strat.SelectBackend(backends)
			}

			backends = append(backends,
				backend.New(mustParseURLWeighted("http://localhost:8082"), 1),
				backend.New(mustParseURLWeighted("http://localhost:8083"), 1),
			)

			counts := make(map[*backend.Backend]int)
			for i := 0; i < 300; i++ {
				b := strat.SelectBackend(backends)
				counts[b]++
			}

			Expect(len(counts)).To(Equal(3))
			for _, count := range counts {
				Expect(count).To(BeNumerically("~", 100, 15))
			}
		})
	})

	Context("smooth weighted distribution", func() {
		It("should provide smooth distribution pattern", func() {
			backends = []*backend.Backend{
				backend.New(mustParseURLWeighted("http://localhost:8081"), 5),
				backend.New(mustParseURLWeighted("http://localhost:8082"), 1),
			}

			selections := make([]*backend.Backend, 18)
			for i := 0; i < 18; i++ {
				selections[i] = strat.SelectBackend(backends)
			}

			count1 := 0
			for _, b := range selections {
				if b == backends[0] {
					count1++
				}
			}

			Expect(count1).To(Equal(15))
		})
	})

	Context("concurrency safety", func() {
		It("should handle concurrent requests", func() {
			backends = []*backend.Backend{
				backend.New(mustParseURLWeighted("http://localhost:8081"), 1),
				backend.New(mustParseURLWeighted("http://localhost:8082"), 1),
			}

			done := make(chan bool)
			results := make(chan *backend.Backend, 100)

			for g := 0; g < 10; g++ {
				go func() {
					for i := 0; i < 10; i++ {
						b := strat.SelectBackend(backends)
						results <- b
					}
					done <- true
				}()
			}

			for g := 0; g < 10; g++ {
				<-done
			}
			close(results)

			counts := make(map[*backend.Backend]int)
			for b := range results {
				Expect(b).NotTo(BeNil())
				Expect(backends).To(ContainElement(b))
				counts[b]++
			}

			total := 0
			for _, count := range counts {
				total += count
			}
			Expect(total).To(Equal(100))
		})
	})
})

func mustParseURLWeighted(rawURL string) *url.URL {
	u, err := url.Parse(rawURL)
	if err != nil {
		panic(err)
	}
	return u
}

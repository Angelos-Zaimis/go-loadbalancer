package circuitbreaker_test

import (
	"sync"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/angeloszaimis/load-balancer/internal/circuitbreaker"
)

var _ = Describe("Registry", func() {
	var registry *circuitbreaker.Registry

	BeforeEach(func() {
		registry = circuitbreaker.NewRegistry(5, 30*time.Second)
	})

	Describe("NewRegistry", func() {
		It("should create a registry", func() {
			Expect(registry).NotTo(BeNil())
		})
	})

	Describe("GetBreaker", func() {
		It("should create a new breaker for unknown URL", func() {
			cb := registry.GetBreaker("http://localhost:8081")
			Expect(cb).NotTo(BeNil())
			Expect(cb.State()).To(Equal(circuitbreaker.StateClosed))
		})

		It("should return the same breaker for the same URL", func() {
			cb1 := registry.GetBreaker("http://localhost:8081")
			cb2 := registry.GetBreaker("http://localhost:8081")
			Expect(cb1).To(BeIdenticalTo(cb2))
		})

		It("should return different breakers for different URLs", func() {
			cb1 := registry.GetBreaker("http://localhost:8081")
			cb2 := registry.GetBreaker("http://localhost:8082")
			Expect(cb1).NotTo(BeIdenticalTo(cb2))
		})

		It("should use registry threshold for new breakers", func() {
			registry = circuitbreaker.NewRegistry(2, 100*time.Millisecond)
			cb := registry.GetBreaker("http://localhost:8081")

			// Should open after 2 failures (not default)
			cb.RecordFailure()
			cb.RecordFailure()
			Expect(cb.State()).To(Equal(circuitbreaker.StateOpen))
		})

		It("should use registry timeout for new breakers", func() {
			registry = circuitbreaker.NewRegistry(2, 50*time.Millisecond)
			cb := registry.GetBreaker("http://localhost:8081")

			// Trip the circuit
			cb.RecordFailure()
			cb.RecordFailure()
			Expect(cb.State()).To(Equal(circuitbreaker.StateOpen))

			// Wait for short timeout
			time.Sleep(60 * time.Millisecond)
			Expect(cb.Allow()).To(BeTrue())
			Expect(cb.State()).To(Equal(circuitbreaker.StateHalfOpen))
		})
	})

	Describe("Concurrent access", func() {
		It("should handle concurrent GetBreaker calls safely", func() {
			const goroutines = 100
			const urlsPerGoroutine = 10

			var wg sync.WaitGroup
			wg.Add(goroutines)

			for i := 0; i < goroutines; i++ {
				go func(id int) {
					defer wg.Done()
					for j := 0; j < urlsPerGoroutine; j++ {
						url := "http://localhost:8081" // Same URL
						cb := registry.GetBreaker(url)
						Expect(cb).NotTo(BeNil())
					}
				}(i)
			}

			wg.Wait()

			// Should only have one breaker for the URL
			stats := registry.Stats()
			Expect(stats).To(HaveLen(1))
		})

		It("should handle concurrent operations on same breaker", func() {
			const goroutines = 50

			var wg sync.WaitGroup
			wg.Add(goroutines * 2)

			cb := registry.GetBreaker("http://localhost:8081")

			// Half recording failures
			for i := 0; i < goroutines; i++ {
				go func() {
					defer wg.Done()
					cb.RecordFailure()
				}()
			}

			// Half recording successes
			for i := 0; i < goroutines; i++ {
				go func() {
					defer wg.Done()
					cb.RecordSuccess()
				}()
			}

			wg.Wait()

			// Should not panic and state should be valid
			state := cb.State()
			Expect(state).To(BeElementOf(
				circuitbreaker.StateClosed,
				circuitbreaker.StateOpen,
				circuitbreaker.StateHalfOpen,
			))
		})
	})

	Describe("Reset", func() {
		It("should clear all breakers", func() {
			registry.GetBreaker("http://localhost:8081")
			registry.GetBreaker("http://localhost:8082")
			registry.GetBreaker("http://localhost:8083")

			stats := registry.Stats()
			Expect(stats).To(HaveLen(3))

			registry.Reset()

			stats = registry.Stats()
			Expect(stats).To(HaveLen(0))
		})
	})

	Describe("Stats", func() {
		It("should return state of all breakers", func() {
			cb1 := registry.GetBreaker("http://localhost:8081")
			cb2 := registry.GetBreaker("http://localhost:8082")

			// Trip cb2
			for i := 0; i < 5; i++ {
				cb2.RecordFailure()
			}

			stats := registry.Stats()
			Expect(stats).To(HaveLen(2))
			Expect(stats["http://localhost:8081"]).To(Equal(circuitbreaker.StateClosed))
			Expect(stats["http://localhost:8082"]).To(Equal(circuitbreaker.StateOpen))

			// Ensure cb1 is used to avoid unused variable error
			Expect(cb1.State()).To(Equal(circuitbreaker.StateClosed))
		})
	})
})

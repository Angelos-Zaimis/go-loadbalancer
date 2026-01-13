package circuitbreaker_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/angeloszaimis/load-balancer/internal/circuitbreaker"
)

var _ = Describe("CircuitBreaker", func() {
	var cb *circuitbreaker.CircuitBreaker

	Describe("NewCircuitBreaker", func() {
		It("should create a circuit breaker in closed state", func() {
			cb = circuitbreaker.NewCircuitBreaker(5, 30*time.Second)
			Expect(cb).NotTo(BeNil())
			Expect(cb.State()).To(Equal(circuitbreaker.StateClosed))
		})
	})

	Describe("State transitions", func() {
		BeforeEach(func() {
			cb = circuitbreaker.NewCircuitBreaker(3, 100*time.Millisecond)
		})

		Context("when in CLOSED state", func() {
			It("should allow requests", func() {
				Expect(cb.Allow()).To(BeTrue())
			})

			It("should remain closed after failures below threshold", func() {
				cb.RecordFailure()
				cb.RecordFailure()
				Expect(cb.State()).To(Equal(circuitbreaker.StateClosed))
				Expect(cb.Allow()).To(BeTrue())
			})

			It("should transition to OPEN after reaching failure threshold", func() {
				cb.RecordFailure()
				cb.RecordFailure()
				cb.RecordFailure()
				Expect(cb.State()).To(Equal(circuitbreaker.StateOpen))
			})
		})

		Context("when in OPEN state", func() {
			BeforeEach(func() {
				// Trip the circuit
				cb.RecordFailure()
				cb.RecordFailure()
				cb.RecordFailure()
				Expect(cb.State()).To(Equal(circuitbreaker.StateOpen))
			})

			It("should block requests", func() {
				Expect(cb.Allow()).To(BeFalse())
			})

			It("should transition to HALF-OPEN after reset timeout", func() {
				time.Sleep(150 * time.Millisecond)
				Expect(cb.Allow()).To(BeTrue())
				Expect(cb.State()).To(Equal(circuitbreaker.StateHalfOpen))
			})

			It("should remain OPEN before reset timeout expires", func() {
				time.Sleep(50 * time.Millisecond)
				Expect(cb.Allow()).To(BeFalse())
				Expect(cb.State()).To(Equal(circuitbreaker.StateOpen))
			})
		})

		Context("when in HALF-OPEN state", func() {
			BeforeEach(func() {
				// Trip the circuit
				cb.RecordFailure()
				cb.RecordFailure()
				cb.RecordFailure()
				// Wait for timeout to transition to half-open
				time.Sleep(150 * time.Millisecond)
				cb.Allow() // This transitions to HALF-OPEN
				Expect(cb.State()).To(Equal(circuitbreaker.StateHalfOpen))
			})

			It("should allow the probe request", func() {
				Expect(cb.Allow()).To(BeTrue())
			})

			It("should transition to CLOSED on success", func() {
				cb.RecordSuccess()
				Expect(cb.State()).To(Equal(circuitbreaker.StateClosed))
			})

			It("should transition back to OPEN on failure", func() {
				cb.RecordFailure()
				Expect(cb.State()).To(Equal(circuitbreaker.StateOpen))
			})
		})
	})

	Describe("RecordSuccess", func() {
		BeforeEach(func() {
			cb = circuitbreaker.NewCircuitBreaker(3, 100*time.Millisecond)
		})

		It("should reset failure count", func() {
			cb.RecordFailure()
			cb.RecordFailure()
			cb.RecordSuccess()
			// Should not open after one more failure
			cb.RecordFailure()
			Expect(cb.State()).To(Equal(circuitbreaker.StateClosed))
		})

		It("should close the circuit from any state", func() {
			// Trip the circuit
			cb.RecordFailure()
			cb.RecordFailure()
			cb.RecordFailure()
			Expect(cb.State()).To(Equal(circuitbreaker.StateOpen))

			// Wait and transition to half-open
			time.Sleep(150 * time.Millisecond)
			cb.Allow()

			// Record success
			cb.RecordSuccess()
			Expect(cb.State()).To(Equal(circuitbreaker.StateClosed))
		})
	})

	Describe("State.String", func() {
		It("should return correct string representation", func() {
			Expect(circuitbreaker.StateClosed.String()).To(Equal("CLOSED"))
			Expect(circuitbreaker.StateOpen.String()).To(Equal("OPEN"))
			Expect(circuitbreaker.StateHalfOpen.String()).To(Equal("HALF-OPEN"))
		})
	})
})

package handler_test

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"sync/atomic"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/angeloszaimis/load-balancer/internal/backend"
	"github.com/angeloszaimis/load-balancer/internal/circuitbreaker"
	"github.com/angeloszaimis/load-balancer/internal/handler"
	"github.com/angeloszaimis/load-balancer/internal/loadbalancer"
	"github.com/angeloszaimis/load-balancer/internal/strategy"
)

var _ = Describe("Handler", func() {
	var (
		h            *handler.LoadBalancerHandler
		lb           *loadbalancer.LoadBalancer
		backends     []*backend.Backend
		mockBackend1 *httptest.Server
		log          *slog.Logger
	)

	BeforeEach(func() {
		log = slog.New(slog.NewTextHandler(os.Stdout, nil))

		mockBackend1 = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("backend1"))
		}))

		backends = []*backend.Backend{
			backend.New(mustParseURL(mockBackend1.URL), 1),
		}

		for _, b := range backends {
			b.SetHealthy(true)
		}

		strat := strategy.NewRoundRobinStrategy()
		lb = loadbalancer.NewLoadBalancer(strat)
		h = handler.NewLoadBalancerHandler(log, lb, backends, nil, nil, 2)
	})

	AfterEach(func() {
		mockBackend1.Close()
	})

	Describe("NewLoadBalancerHandler", func() {
		It("should create a handler", func() {
			Expect(h).NotTo(BeNil())
		})
	})

	Describe("ServeHTTP", func() {
		It("should proxy request to backend", func() {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			w := httptest.NewRecorder()

			h.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
		})

		Context("with no healthy backends", func() {
			BeforeEach(func() {
				backends[0].SetHealthy(false)
			})

			It("should return 503 Service Unavailable", func() {
				req := httptest.NewRequest(http.MethodGet, "/test", nil)
				w := httptest.NewRecorder()

				h.ServeHTTP(w, req)

				Expect(w.Code).To(Equal(http.StatusServiceUnavailable))
			})
		})
	})
})

var _ = Describe("Handler Retry Logic", func() {
	var (
		h            *handler.LoadBalancerHandler
		lb           *loadbalancer.LoadBalancer
		backends     []*backend.Backend
		mockBackend1 *httptest.Server
		mockBackend2 *httptest.Server
		log          *slog.Logger
		callCount1   int32
		callCount2   int32
	)

	BeforeEach(func() {
		log = slog.New(slog.NewTextHandler(io.Discard, nil))
		atomic.StoreInt32(&callCount1, 0)
		atomic.StoreInt32(&callCount2, 0)
	})

	AfterEach(func() {
		if mockBackend1 != nil {
			mockBackend1.Close()
		}
		if mockBackend2 != nil {
			mockBackend2.Close()
		}
	})

	Describe("Retry on backend failure", func() {
		Context("when first backend fails and second succeeds", func() {
			BeforeEach(func() {
				// Backend 1 always fails
				mockBackend1 = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					atomic.AddInt32(&callCount1, 1)
					// Simulate connection close
					hj, ok := w.(http.Hijacker)
					if ok {
						conn, _, _ := hj.Hijack()
						conn.Close()
					}
				}))

				// Backend 2 succeeds
				mockBackend2 = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					atomic.AddInt32(&callCount2, 1)
					w.WriteHeader(http.StatusOK)
					w.Write([]byte("backend2"))
				}))

				backends = []*backend.Backend{
					backend.New(mustParseURL(mockBackend1.URL), 1),
					backend.New(mustParseURL(mockBackend2.URL), 1),
				}

				for _, b := range backends {
					b.SetHealthy(true)
				}

				strat := strategy.NewRoundRobinStrategy()
				lb = loadbalancer.NewLoadBalancer(strat)
				h = handler.NewLoadBalancerHandler(log, lb, backends, nil, nil, 2)
			})

			It("should retry and succeed on second backend for GET request", func() {
				req := httptest.NewRequest(http.MethodGet, "/test", nil)
				w := httptest.NewRecorder()

				h.ServeHTTP(w, req)

				// Should have tried backend2 and succeeded
				Expect(atomic.LoadInt32(&callCount2)).To(BeNumerically(">=", 1))
			})
		})

		Context("when request is not idempotent", func() {
			BeforeEach(func() {
				// Backend 1 always fails
				mockBackend1 = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					atomic.AddInt32(&callCount1, 1)
					hj, ok := w.(http.Hijacker)
					if ok {
						conn, _, _ := hj.Hijack()
						conn.Close()
					}
				}))

				// Backend 2 succeeds
				mockBackend2 = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					atomic.AddInt32(&callCount2, 1)
					w.WriteHeader(http.StatusOK)
				}))

				backends = []*backend.Backend{
					backend.New(mustParseURL(mockBackend1.URL), 1),
					backend.New(mustParseURL(mockBackend2.URL), 1),
				}

				for _, b := range backends {
					b.SetHealthy(true)
				}

				strat := strategy.NewRoundRobinStrategy()
				lb = loadbalancer.NewLoadBalancer(strat)
				h = handler.NewLoadBalancerHandler(log, lb, backends, nil, nil, 2)
			})

			It("should NOT retry POST requests", func() {
				req := httptest.NewRequest(http.MethodPost, "/test", nil)
				w := httptest.NewRecorder()

				h.ServeHTTP(w, req)

				// Should only try once (no retry for POST)
				Expect(atomic.LoadInt32(&callCount1) + atomic.LoadInt32(&callCount2)).To(Equal(int32(1)))
			})
		})
	})
})

var _ = Describe("Handler with Circuit Breaker", func() {
	var (
		h            *handler.LoadBalancerHandler
		lb           *loadbalancer.LoadBalancer
		backends     []*backend.Backend
		mockBackend1 *httptest.Server
		mockBackend2 *httptest.Server
		registry     *circuitbreaker.Registry
		log          *slog.Logger
		callCount1   int32
		callCount2   int32
	)

	BeforeEach(func() {
		log = slog.New(slog.NewTextHandler(io.Discard, nil))
		registry = circuitbreaker.NewRegistry(2, 100*time.Millisecond)
		atomic.StoreInt32(&callCount1, 0)
		atomic.StoreInt32(&callCount2, 0)
	})

	AfterEach(func() {
		if mockBackend1 != nil {
			mockBackend1.Close()
		}
		if mockBackend2 != nil {
			mockBackend2.Close()
		}
	})

	Describe("Circuit breaker integration", func() {
		Context("when circuit is open for a backend", func() {
			BeforeEach(func() {
				mockBackend1 = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					atomic.AddInt32(&callCount1, 1)
					w.WriteHeader(http.StatusOK)
					w.Write([]byte("backend1"))
				}))

				mockBackend2 = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					atomic.AddInt32(&callCount2, 1)
					w.WriteHeader(http.StatusOK)
					w.Write([]byte("backend2"))
				}))

				backends = []*backend.Backend{
					backend.New(mustParseURL(mockBackend1.URL), 1),
					backend.New(mustParseURL(mockBackend2.URL), 1),
				}

				for _, b := range backends {
					b.SetHealthy(true)
				}

				strat := strategy.NewRoundRobinStrategy()
				lb = loadbalancer.NewLoadBalancer(strat)
				h = handler.NewLoadBalancerHandler(log, lb, backends, nil, registry, 2)

				// Trip circuit for backend1
				cb := registry.GetBreaker(mockBackend1.URL)
				cb.RecordFailure()
				cb.RecordFailure()
				Expect(cb.State()).To(Equal(circuitbreaker.StateOpen))
			})

			It("should skip backend with open circuit", func() {
				req := httptest.NewRequest(http.MethodGet, "/test", nil)
				w := httptest.NewRecorder()

				h.ServeHTTP(w, req)

				Expect(w.Code).To(Equal(http.StatusOK))
				// Backend1 should not be called due to open circuit
				// Backend2 should handle the request
				Expect(atomic.LoadInt32(&callCount2)).To(BeNumerically(">=", 1))
			})
		})

		Context("when circuit recovers", func() {
			BeforeEach(func() {
				mockBackend1 = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					atomic.AddInt32(&callCount1, 1)
					w.WriteHeader(http.StatusOK)
					w.Write([]byte("backend1"))
				}))

				backends = []*backend.Backend{
					backend.New(mustParseURL(mockBackend1.URL), 1),
				}

				for _, b := range backends {
					b.SetHealthy(true)
				}

				strat := strategy.NewRoundRobinStrategy()
				lb = loadbalancer.NewLoadBalancer(strat)
				h = handler.NewLoadBalancerHandler(log, lb, backends, nil, registry, 2)

				// Trip circuit
				cb := registry.GetBreaker(mockBackend1.URL)
				cb.RecordFailure()
				cb.RecordFailure()
				Expect(cb.State()).To(Equal(circuitbreaker.StateOpen))
			})

			It("should allow traffic after reset timeout", func() {
				// Wait for circuit to transition to half-open
				time.Sleep(150 * time.Millisecond)

				req := httptest.NewRequest(http.MethodGet, "/test", nil)
				w := httptest.NewRecorder()

				h.ServeHTTP(w, req)

				Expect(w.Code).To(Equal(http.StatusOK))
				Expect(atomic.LoadInt32(&callCount1)).To(Equal(int32(1)))

				// Circuit should be closed after success
				cb := registry.GetBreaker(mockBackend1.URL)
				Expect(cb.State()).To(Equal(circuitbreaker.StateClosed))
			})
		})
	})
})

func mustParseURL(rawURL string) *url.URL {
	u, err := url.Parse(rawURL)
	if err != nil {
		panic(err)
	}
	return u
}

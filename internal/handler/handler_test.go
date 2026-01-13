package handler_test

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/angeloszaimis/load-balancer/internal/backend"
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

func mustParseURL(rawURL string) *url.URL {
	u, err := url.Parse(rawURL)
	if err != nil {
		panic(err)
	}
	return u
}

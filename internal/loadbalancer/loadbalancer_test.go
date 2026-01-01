package loadbalancer_test

import (
	"net/url"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/angeloszaimis/load-balancer/internal/backend"
	"github.com/angeloszaimis/load-balancer/internal/loadbalancer"
	"github.com/angeloszaimis/load-balancer/internal/strategy"
)

var _ = Describe("LoadBalancer", func() {
	var (
		lb       *loadbalancer.LoadBalancer
		backends []*backend.Backend
	)

	BeforeEach(func() {
		strat := strategy.NewRoundRobinStrategy()
		lb = loadbalancer.NewLoadBalancer(strat)

		backends = []*backend.Backend{
			backend.New(mustParseURL("http://localhost:8081")),
			backend.New(mustParseURL("http://localhost:8082")),
			backend.New(mustParseURL("http://localhost:8083")),
		}
	})

	Describe("NewLoadBalancer", func() {
		It("should create a load balancer with given strategy", func() {
			Expect(lb).NotTo(BeNil())
		})
	})

	Describe("GetAndReserveServer", func() {
		Context("with all healthy backends", func() {
			BeforeEach(func() {
				for _, b := range backends {
					b.SetHealthy(true)
				}
			})

			It("should return a backend", func() {
				server, err := lb.GetAndReserveServer(backends)
				Expect(err).NotTo(HaveOccurred())
				Expect(server).NotTo(BeNil())
			})

			It("should increment connection count", func() {
				server, err := lb.GetAndReserveServer(backends)
				Expect(err).NotTo(HaveOccurred())
				Expect(server.ActiveConnections()).To(Equal(1))
			})
		})

		Context("with no healthy backends", func() {
			BeforeEach(func() {
				for _, b := range backends {
					b.SetHealthy(false)
				}
			})

			It("should return an error", func() {
				server, err := lb.GetAndReserveServer(backends)
				Expect(err).To(HaveOccurred())
				Expect(server).To(BeNil())
			})
		})
	})

	Describe("GetAndReserveServerWithKey", func() {
		BeforeEach(func() {
			for _, b := range backends {
				b.SetHealthy(true)
			}
		})

		Context("with consistent hash strategy", func() {
			BeforeEach(func() {
				strat := strategy.NewConsistentHashStrategy(100)
				lb = loadbalancer.NewLoadBalancer(strat)
			})

			It("should select backend based on key", func() {
				server1, err := lb.GetAndReserveServerWithKey(backends, "192.168.1.1")
				Expect(err).NotTo(HaveOccurred())
				Expect(server1).NotTo(BeNil())

				server2, err := lb.GetAndReserveServerWithKey(backends, "192.168.1.1")
				Expect(err).NotTo(HaveOccurred())
				Expect(server2).To(Equal(server1))
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

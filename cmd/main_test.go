package main

import (
	"context"
	"log/slog"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/angeloszaimis/load-balancer/config"
)

func TestMain(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Main Suite")
}

var _ = Describe("initializeBackends", func() {
	var (
		log    *slog.Logger
		ctx    context.Context
		cancel context.CancelFunc
		cfg    *config.Config
	)

	BeforeEach(func() {
		log = slog.Default()
		ctx, cancel = context.WithCancel(context.Background())
		cfg = &config.Config{
			HealthCheck: config.HealthCheckConfig{
				Interval: "5s",
			},
			Backends: []config.BackendConfig{},
		}
	})

	AfterEach(func() {
		if cancel != nil {
			cancel()
		}
	})

	Context("valid backend URLs", func() {
		It("should initialize single backend", func() {
			cfg.Backends = []config.BackendConfig{{URL: "http://localhost:8080", Weight: 1}}
			backends, err := initializeBackends(ctx, cfg, log)
			Expect(err).NotTo(HaveOccurred())
			Expect(backends).To(HaveLen(1))
			Expect(backends[0]).NotTo(BeNil())
		})

		It("should initialize multiple backends", func() {
			cfg.Backends = []config.BackendConfig{
				{URL: "http://localhost:8080", Weight: 1},
				{URL: "http://localhost:8081", Weight: 1},
				{URL: "http://localhost:8082", Weight: 1},
			}
			backends, err := initializeBackends(ctx, cfg, log)
			Expect(err).NotTo(HaveOccurred())
			Expect(backends).To(HaveLen(3))
		})

		It("should handle HTTPS backends", func() {
			cfg.Backends = []config.BackendConfig{{URL: "https://api.example.com", Weight: 1}}
			backends, err := initializeBackends(ctx, cfg, log)
			Expect(err).NotTo(HaveOccurred())
			Expect(backends).To(HaveLen(1))
		})

		It("should handle backends with paths", func() {
			cfg.Backends = []config.BackendConfig{{URL: "http://localhost:8080/api/v1", Weight: 1}}
			backends, err := initializeBackends(ctx, cfg, log)
			Expect(err).NotTo(HaveOccurred())
			Expect(backends).To(HaveLen(1))
		})
	})

	Context("invalid configurations", func() {
		It("should return error for invalid health check interval", func() {
			cfg.HealthCheck.Interval = "invalid"
			cfg.Backends = []config.BackendConfig{{URL: "http://localhost:8080", Weight: 1}}
			backends, err := initializeBackends(ctx, cfg, log)
			Expect(err).To(HaveOccurred())
			Expect(backends).To(BeNil())
		})

		It("should return error when no backends configured", func() {
			cfg.Backends = []config.BackendConfig{}
			backends, err := initializeBackends(ctx, cfg, log)
			Expect(err).To(HaveOccurred())
			Expect(backends).To(BeNil())
		})

		It("should skip invalid URLs but continue with valid ones", func() {
			cfg.Backends = []config.BackendConfig{
				{URL: "http://localhost:8080", Weight: 1},
				{URL: "http://localhost:8081", Weight: 1},
			}
			backends, err := initializeBackends(ctx, cfg, log)
			Expect(err).NotTo(HaveOccurred())
			Expect(backends).To(HaveLen(2))
		})

		It("should return error when all URLs are invalid", func() {
			cfg.Backends = []config.BackendConfig{
				{URL: "://invalid", Weight: 1},
			}
			backends, err := initializeBackends(ctx, cfg, log)
			Expect(err).To(HaveOccurred())
			Expect(backends).To(BeNil())
		})
	})

	Context("health check intervals", func() {
		It("should handle different interval formats", func() {
			cfg.Backends = []config.BackendConfig{{URL: "http://localhost:8080", Weight: 1}}

			cfg.HealthCheck.Interval = "1s"
			backends, err := initializeBackends(ctx, cfg, log)
			Expect(err).NotTo(HaveOccurred())
			Expect(backends).To(HaveLen(1))

			cfg.HealthCheck.Interval = "100ms"
			backends, err = initializeBackends(ctx, cfg, log)
			Expect(err).NotTo(HaveOccurred())
			Expect(backends).To(HaveLen(1))

			cfg.HealthCheck.Interval = "1m"
			backends, err = initializeBackends(ctx, cfg, log)
			Expect(err).NotTo(HaveOccurred())
			Expect(backends).To(HaveLen(1))

			cfg.HealthCheck.Interval = "500ms"
			backends, err = initializeBackends(ctx, cfg, log)
			Expect(err).NotTo(HaveOccurred())
			Expect(backends).To(HaveLen(1))
		})

		It("should handle hour format", func() {
			cfg.HealthCheck.Interval = "1h"
			cfg.Backends = []config.BackendConfig{{URL: "http://localhost:8080", Weight: 1}}
			backends, err := initializeBackends(ctx, cfg, log)
			Expect(err).NotTo(HaveOccurred())
			Expect(backends).To(HaveLen(1))
		})
	})
})

var _ = Describe("createStrategy", func() {
	var log *slog.Logger

	BeforeEach(func() {
		log = slog.Default()
	})

	Context("valid strategies", func() {
		It("should create round-robin strategy", func() {
			strat, err := createStrategy(log, "round-robin", 100)
			Expect(err).NotTo(HaveOccurred())
			Expect(strat).NotTo(BeNil())
		})

		It("should create random strategy", func() {
			strat, err := createStrategy(log, "random", 100)
			Expect(err).NotTo(HaveOccurred())
			Expect(strat).NotTo(BeNil())
		})

		It("should create least-conn strategy", func() {
			strat, err := createStrategy(log, "least-conn", 100)
			Expect(err).NotTo(HaveOccurred())
			Expect(strat).NotTo(BeNil())
		})

		It("should create least-response strategy", func() {
			strat, err := createStrategy(log, "least-response", 100)
			Expect(err).NotTo(HaveOccurred())
			Expect(strat).NotTo(BeNil())
		})

		It("should create consistent hash strategy with virtual nodes", func() {
			strat, err := createStrategy(log, "consistent_hash", 150)
			Expect(err).NotTo(HaveOccurred())
			Expect(strat).NotTo(BeNil())
		})

		It("should create weighted-round-robin strategy", func() {
			strat, err := createStrategy(log, "weighted-round-robin", 100)
			Expect(err).NotTo(HaveOccurred())
			Expect(strat).NotTo(BeNil())
		})
	})

	Context("default behavior", func() {
		It("should default to round-robin for unknown strategy", func() {
			strat, err := createStrategy(log, "unknown-strategy", 100)
			Expect(err).NotTo(HaveOccurred())
			Expect(strat).NotTo(BeNil())
		})

		It("should default to round-robin for empty strategy", func() {
			strat, err := createStrategy(log, "", 100)
			Expect(err).NotTo(HaveOccurred())
			Expect(strat).NotTo(BeNil())
		})

		It("should default to round-robin for invalid strategy name", func() {
			strat, err := createStrategy(log, "!!invalid!!", 100)
			Expect(err).NotTo(HaveOccurred())
			Expect(strat).NotTo(BeNil())
		})

		It("should default to round-robin for mixed case strategy", func() {
			strat, err := createStrategy(log, "Round-Robin", 100)
			Expect(err).NotTo(HaveOccurred())
			Expect(strat).NotTo(BeNil())
		})
	})

	Context("virtual nodes parameter", func() {
		It("should handle different virtual nodes parameters", func() {
			strat1, err := createStrategy(log, "consistent_hash", 50)
			Expect(err).NotTo(HaveOccurred())
			Expect(strat1).NotTo(BeNil())

			strat2, err := createStrategy(log, "consistent_hash", 200)
			Expect(err).NotTo(HaveOccurred())
			Expect(strat2).NotTo(BeNil())
		})

		It("should handle zero virtual nodes", func() {
			strat, err := createStrategy(log, "consistent_hash", 0)
			Expect(err).NotTo(HaveOccurred())
			Expect(strat).NotTo(BeNil())
		})

		It("should handle negative virtual nodes", func() {
			strat, err := createStrategy(log, "consistent_hash", -10)
			Expect(err).NotTo(HaveOccurred())
			Expect(strat).NotTo(BeNil())
		})

		It("should handle large virtual nodes value", func() {
			strat, err := createStrategy(log, "consistent_hash", 10000)
			Expect(err).NotTo(HaveOccurred())
			Expect(strat).NotTo(BeNil())
		})

		It("should ignore virtual nodes for non-hash strategies", func() {
			strat, err := createStrategy(log, "round-robin", 999)
			Expect(err).NotTo(HaveOccurred())
			Expect(strat).NotTo(BeNil())
		})
	})

	Context("strategy name variations", func() {
		It("should handle round-robin exactly", func() {
			strat, err := createStrategy(log, "round-robin", 100)
			Expect(err).NotTo(HaveOccurred())
			Expect(strat).NotTo(BeNil())
		})

		It("should handle consistent_hash with underscore", func() {
			strat, err := createStrategy(log, "consistent_hash", 100)
			Expect(err).NotTo(HaveOccurred())
			Expect(strat).NotTo(BeNil())
		})

		It("should handle weighted-round-robin with hyphens", func() {
			strat, err := createStrategy(log, "weighted-round-robin", 100)
			Expect(err).NotTo(HaveOccurred())
			Expect(strat).NotTo(BeNil())
		})
	})
})

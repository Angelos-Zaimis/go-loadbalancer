package main

import (
	"context"
	"log/slog"
	"net/http"
	_ "net/http/pprof"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/angeloszaimis/load-balancer/config"
	"github.com/angeloszaimis/load-balancer/internal/backend"
	"github.com/angeloszaimis/load-balancer/internal/handler"
	"github.com/angeloszaimis/load-balancer/internal/healthcheck"
	"github.com/angeloszaimis/load-balancer/internal/httpserver"
	"github.com/angeloszaimis/load-balancer/internal/loadbalancer"
	"github.com/angeloszaimis/load-balancer/internal/metrics"
	"github.com/angeloszaimis/load-balancer/internal/strategy"
	"github.com/angeloszaimis/load-balancer/pkg/logger"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", slog.Any("err", err))
		os.Exit(1)
	}

	log := logger.New(cfg.Logging.Level, true, cfg.Server.Environment)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	backends, err := initializeBackends(ctx, cfg, log)
	if err != nil {
		log.Error("Failed to initialize backends", slog.Any("err", err))
		os.Exit(1)
	}

	strat, err := createStrategy(log, cfg.Strategy.Type, cfg.Strategy.VirtualNodes)
	if err != nil {
		log.Error("Failed to create strategy",
			slog.String("strategy", cfg.Strategy.Type),
			slog.Any("err", err))
		os.Exit(1)
	}

	lb := loadbalancer.NewLoadBalancer(strat)

	metricsCollector := metrics.NewCollector(1000, log)
	metricsCollector.Start(ctx)

	loadBalancerHandler := handler.NewLoadBalancerHandler(log, lb, backends, metricsCollector)

	// Start pprof server on separate port for diagnostics
	go func() {
		log.Info("Starting pprof server on :6060")
		if err := http.ListenAndServe(":6060", nil); err != nil {
			log.Error("pprof server failed", slog.Any("err", err))
		}
	}()

	api := http.NewServeMux()

	api.HandleFunc("/", loadBalancerHandler.ServeHTTP)
	api.HandleFunc("/metrics", metricsCollector.Handler(cfg.Strategy.Type))

	srv, err := httpserver.New(cfg.Server.Address, api)
	if err != nil {
		log.Error("Failed to create server", slog.Any("err", err))
		os.Exit(1)
	}

	srvErrCh := make(chan error, 1)

	go func() {
		srvErrCh <- srv.Start()
	}()

	select {
	case <-ctx.Done():
		log.Info("Shutting down gracefully...")
		if err := srv.Shutdown(context.Background()); err != nil {
			log.Error("Error during shutdown", slog.Any("err", err))
		}
	case err := <-srvErrCh:
		if err != nil {
			log.Error("Error starting load balancer", slog.Any("err", err))
			os.Exit(1)
		}
	}
}

func initializeBackends(ctx context.Context, cfg *config.Config, log *slog.Logger) ([]*backend.Backend, error) {
	healthCheckInterval, err := time.ParseDuration(cfg.HealthCheck.Interval)
	if err != nil {
		return nil, err
	}

	var backends []*backend.Backend

	for _, backendCfg := range cfg.Backends {
		u, err := url.Parse(backendCfg.URL)

		if err != nil {
			log.Error("Failed to parse URL",
				slog.String("url", backendCfg.URL),
				slog.String("error", err.Error()))
			continue
		}

		backend := backend.New(u, backendCfg.Weight)
		backends = append(backends, backend)
		go healthcheck.HealthCheck(ctx, backend, healthCheckInterval, log)
	}

	if len(backends) == 0 {
		return nil, os.ErrInvalid
	}

	return backends, nil
}

func createStrategy(logger *slog.Logger, strategyType string, virtualNodes int) (strategy.Strategy, error) {
	switch strategyType {
	case "round-robin":
		return strategy.NewRoundRobinStrategy(), nil
	case "random":
		return strategy.NewRandomStrategy(), nil
	case "least-conn":
		return strategy.NewLeastConnStrategy(), nil
	case "least-response":
		return strategy.NewLeastResponseStrategy(), nil
	case "consistent_hash":
		return strategy.NewConsistentHashStrategy(virtualNodes), nil
	case "weighted-round-robin":
		return strategy.NewWeightedRoundRobinStrategy(), nil
	default:
		logger.Warn("Unkown strategy, defaulting to round-robin", slog.String("requested", strategyType))
		return strategy.NewRoundRobinStrategy(), nil
	}
}

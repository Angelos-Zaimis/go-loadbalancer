package main

import (
	"context"
	"log/slog"
	"net/http"
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
	"github.com/angeloszaimis/load-balancer/internal/strategy"
	"github.com/angeloszaimis/load-balancer/pkg/logger"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", slog.Any("err", err))
		os.Exit(1)
	}

	log := logger.New(cfg.LogLevel, true, cfg.Env)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	backends, err := initializeBackends(ctx, cfg, log)
	if err != nil {
		log.Error("Failed to initialize backends", slog.Any("err", err))
		os.Exit(1)
	}

	strat, err := createStrategy(log, cfg.Strategy, cfg.VirtualNodes)
	if err != nil {
		log.Error("Failed to create strategy",
			slog.String("strategy", cfg.Strategy),
			slog.Any("err", err))
		os.Exit(1)
	}

	lb := loadbalancer.NewLoadBalancer(strat)

	loadBalancerHandler := handler.NewLoadBalancerHandler(log, lb, backends)

	srv, err := httpserver.New(cfg.HTTPAddr, http.HandlerFunc(loadBalancerHandler.ServeHTTP))
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
	healthCheckInterval, err := time.ParseDuration(cfg.HealthCheckInterval)
	if err != nil {
		return nil, err
	}

	var backends []*backend.Backend

	for _, serverUrl := range cfg.Backends {
		u, err := url.Parse(serverUrl)

		if err != nil {
			log.Error("Failed to parse URL",
				slog.String("url", serverUrl),
				slog.String("error", err.Error()))
			continue
		}

		backend := backend.New(u)
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
		return strategy.NewWeightedRoundRobinStradegy(), nil
	default:
		logger.Warn("Unkown strategy, defaulting to round-robin", slog.String("requested", strategyType))
		return strategy.NewRoundRobinStrategy(), nil
	}
}

package healthcheck

import (
	"context"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"github.com/angeloszaimis/load-balancer/internal/backend"
)

func HealthCheck(
	ctx context.Context,
	backend *backend.Backend,
	interval time.Duration,
	logger *slog.Logger,
) {
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	// Perform initial health check immediately
	doHealthCheck(ctx, client, backend, logger, true)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Info("Health check stopped",
				slog.String("server", backend.URL().String()))
			return

		case <-ticker.C:
			doHealthCheck(ctx, client, backend, logger, false)
		}
	}
}

func doHealthCheck( ctx context.Context, client *http.Client, backend *backend.Backend, logger *slog.Logger, isInitial bool ) {
	healthURL := backend.URL().ResolveReference(&url.URL{Path: "/health"})

	req, err := http.NewRequestWithContext(
		ctx, http.MethodGet, healthURL.String(), nil)
	if err != nil {
		return
	}

	res, err := client.Do(req)
	if err != nil {
		backend.SetHealthy(false)
		if isInitial {
			logger.Warn("Server is down (initial check)",
				slog.String("server", backend.URL().String()),
				slog.Any("error", err))
		}
		return
	}
	defer res.Body.Close()

	healthy := res.StatusCode == http.StatusOK
	changed := backend.SetHealthy(healthy)

	if changed {
		if healthy {
			logger.Info("Server is back up",
				slog.String("server", backend.URL().String()))
		} else {
			logger.Warn("Server is down",
				slog.String("server", backend.URL().String()))
		}
	} else if isInitial && healthy {
		logger.Info("Server is up (initial check)",
			slog.String("server", backend.URL().String()))
	}
}

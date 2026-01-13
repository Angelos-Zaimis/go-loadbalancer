package handler

import (
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/angeloszaimis/load-balancer/internal/backend"
	"github.com/angeloszaimis/load-balancer/internal/loadbalancer"
	"github.com/angeloszaimis/load-balancer/internal/metrics"
	"github.com/angeloszaimis/load-balancer/internal/circuitbreaker"

)

type LoadBalancerHandler struct {
	logger           *slog.Logger
	balancer         *loadbalancer.LoadBalancer
	backends         []*backend.Backend
	metricsCollector *metrics.Collector
	circuitRegistry  *circuitbreaker.Registry
	maxRetries 		 int
}

type retryableWriter struct {
	http.ResponseWriter
	headerWritten bool
	statusCode    int
}

type statusRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (rw *retryableWriter) WriteHeader(code int) {
	rw.headerWritten = true
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *retryableWriter) Write(b []byte) (int, error) {
	if !rw.headerWritten {
		rw.headerWritten = true
		rw.statusCode = http.StatusOK
	}
	return rw.ResponseWriter.Write(b)
}

// isIdempotent returns true if the HTTP method is safe to retry.
// Based on RFC 7231.
func isIdempotent(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace:
		return true
	case http.MethodPut, http.MethodDelete:
		// Idempotent but not safe - typically okay to retry
		return true
	default:
		return false
	}
}

func (lb *LoadBalancerHandler) selectBackend(clientIP string, trackBackends map[string]bool) (*backend.Backend, error) {
	available := make([]*backend.Backend, 0, len(lb.backends))
	for _, b := range lb.backends {
		if !trackBackends[b.URL().String()] && b.IsHealthy() {
			available = append(available, b)
		}
	}

	if len(available) == 0 {
		return nil, http.ErrServerClosed
	}

	if _, ok := lb.balancer.LoadBalancerStrategy().(interface{ SetKey(string) }); ok {
        return lb.balancer.GetAndReserveServerWithKey(available, clientIP)
    }
    return lb.balancer.GetAndReserveServer(available)
}

func (lb *LoadBalancerHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    clientIP := extractClientIP(r)

    lb.logger.Info("Received request",
        slog.String("from", clientIP),
        slog.String("method", r.Method),
        slog.String("path", r.URL.Path),
        slog.String("proto", r.Proto),
        slog.String("host", r.Host),
        slog.String("user_agent", r.UserAgent()))

    // Determine max retries based on method idempotency
    maxAttempts := 1
    if isIdempotent(r.Method) && lb.maxRetries > 0 {
        maxAttempts = lb.maxRetries + 1
    }

    // Track which backends we've tried (to avoid retrying same one)
    triedBackends := make(map[string]bool)

    var lastErr error
    for attempt := 1; attempt <= maxAttempts; attempt++ {
        // Select a backend
        nextServer, err := lb.selectBackend(clientIP, triedBackends)
        if err != nil {
            lb.logger.Warn("No healthy backends available",
                slog.String("client", clientIP),
                slog.Int("attempt", attempt))
            lastErr = err
            break
        }

        backendURL := nextServer.URL().String()
        triedBackends[backendURL] = true

        // Check circuit breaker
        if lb.circuitRegistry != nil {
            cb := lb.circuitRegistry.GetBreaker(backendURL)
            if !cb.Allow() {
                lb.logger.Debug("Circuit breaker open, skipping backend",
                    slog.String("backend", backendURL),
                    slog.Int("attempt", attempt))
                continue // Try next backend
            }
        }

        // Emit metrics
        lb.emitEvent(metrics.MetricEvent{
            Type:      metrics.EventRequestReceived,
            Timestamp: time.Now(),
            Backend:   backendURL,
        })
        lb.emitEvent(metrics.MetricEvent{
            Type:      metrics.EventBackendSelected,
            Timestamp: time.Now(),
            Backend:   backendURL,
        })

        // Increment connection count
        nextServer.IncrementConn()

        lb.logger.Info("Forwarding to backend",
            slog.String("client", clientIP),
            slog.String("backend", backendURL),
            slog.Int("attempt", attempt))

        // Prepare for proxying
        w.Header().Set("X-Backend-Server", backendURL)

        wrapped := &retryableWriter{ResponseWriter: w, statusCode: http.StatusOK}
        start := time.Now()

        // Enable error capture from proxy
        reqWithCapture, proxyErr := backend.WithProxyErrorCapture(r)

        // Forward request to backend
        nextServer.ReverseProxy().ServeHTTP(wrapped, reqWithCapture)

        duration := time.Since(start)
        nextServer.DecrementConn()

        // Check if proxy succeeded
        if proxyErr.Err == nil {
            // Success!
            if lb.circuitRegistry != nil {
                lb.circuitRegistry.GetBreaker(backendURL).RecordSuccess()
            }

            lb.emitEvent(metrics.MetricEvent{
                Type:       metrics.EventResponseCompleted,
                Timestamp:  time.Now(),
                Backend:    backendURL,
                Duration:   duration,
                StatusCode: wrapped.statusCode,
            })
            nextServer.RecordResponse(duration)
            return // Done!
        }

        // Proxy failed
        lb.logger.Warn("Backend request failed",
            slog.String("backend", backendURL),
            slog.String("error", proxyErr.Err.Error()),
            slog.Int("attempt", attempt),
            slog.Bool("header_written", wrapped.headerWritten))

        if lb.circuitRegistry != nil {
            lb.circuitRegistry.GetBreaker(backendURL).RecordFailure()
        }

        lastErr = proxyErr.Err

        // Can we retry?
        if wrapped.headerWritten {
            // Headers already sent to client - cannot retry
            lb.logger.Warn("Cannot retry: headers already written",
                slog.String("backend", backendURL))
            return
        }

        // Will retry with next backend (if attempts remain)
        lb.logger.Info("Retrying with different backend",
            slog.Int("attempt", attempt),
            slog.Int("max_attempts", maxAttempts))
    }

    // All retries exhausted
    lb.logger.Error("All backends failed",
        slog.String("client", clientIP),
        slog.Any("error", lastErr))
    http.Error(w, "Service unavailable", http.StatusServiceUnavailable)
}

func extractClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return strings.TrimSpace(strings.Split(xff, ",")[0])
	}

	host, _, _ := net.SplitHostPort(r.RemoteAddr)
	return host
}

func (lb *LoadBalancerHandler) emitEvent(event metrics.MetricEvent) {
	if lb.metricsCollector == nil {
		return
	}

	select {
	case lb.metricsCollector.EventChannel() <- event:
	default:
	}
}

func (r *statusRecorder) WriteHeader(code int) {
	r.statusCode = code
	r.ResponseWriter.WriteHeader(code)
}

func NewLoadBalancerHandler(
    logger *slog.Logger,
    lb *loadbalancer.LoadBalancer,
    backends []*backend.Backend,
    collector *metrics.Collector,
    circuitRegistry *circuitbreaker.Registry,
    maxRetries int,
) *LoadBalancerHandler {
    return &LoadBalancerHandler{
        logger:           logger,
        balancer:         lb,
        backends:         backends,
        metricsCollector: collector,
        circuitRegistry:  circuitRegistry,
        maxRetries:       maxRetries,
    }
}

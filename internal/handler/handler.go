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
)

type LoadBalancerHandler struct {
	logger           *slog.Logger
	balancer         *loadbalancer.LoadBalancer
	backends         []*backend.Backend
	metricsCollector *metrics.Collector
}

type statusRecorder struct {
	http.ResponseWriter
	statusCode int
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

	var nextServer *backend.Backend
	var err error

	// Check if strategy needs a key (consistent hash)
	if _, ok := lb.balancer.LoadBalancerStrategy().(interface{ SetKey(string) }); ok {
		// For IP-based strategies: atomically set key and select backend
		nextServer, err = lb.balancer.GetAndReserveServerWithKey(lb.backends, clientIP)
	} else {
		// For other strategies: use regular method
		nextServer, err = lb.balancer.GetAndReserveServer(lb.backends)
	}

	if err != nil {
		lb.logger.Warn("No healthy backends available", slog.String("client", clientIP))
		http.Error(w, "No healthy server available", http.StatusServiceUnavailable)
		return
	}

	lb.emitEvent(metrics.MetricEvent{
		Type:      metrics.EventRequestReceived,
		Timestamp: time.Now(),
		Backend:   nextServer.URL().String(),
	})

	lb.emitEvent(metrics.MetricEvent{
		Type:      metrics.EventBackendSelected,
		Timestamp: time.Now(),
		Backend:   nextServer.URL().String(),
	})

	defer nextServer.DecrementConn()
	start := time.Now()

	lb.logger.Info("Forwarding to backend",
		slog.String("client", clientIP),
		slog.String("backend", nextServer.URL().String()))

	w.Header().Set("X-Backend-Server", nextServer.URL().String())

	wrapped := &statusRecorder{ResponseWriter: w, statusCode: http.StatusOK}
	nextServer.ReverseProxy().ServeHTTP(wrapped, r)

	duration := time.Since(start)
	lb.emitEvent(metrics.MetricEvent{
		Type:       metrics.EventResponseCompleted,
		Timestamp:  time.Now(),
		Backend:    nextServer.URL().String(),
		Duration:   duration,
		StatusCode: wrapped.statusCode,
	})
	nextServer.RecordResponse(duration)
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

func NewLoadBalancerHandler(logger *slog.Logger, lb *loadbalancer.LoadBalancer, backends []*backend.Backend, collector *metrics.Collector) *LoadBalancerHandler {
	return &LoadBalancerHandler{
		logger:           logger,
		balancer:         lb,
		backends:         backends,
		metricsCollector: collector,
	}
}

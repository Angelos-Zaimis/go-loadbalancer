package handler

import (
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/angeloszaimis/load-balancer/internal/backend"
	"github.com/angeloszaimis/load-balancer/internal/loadbalancer"
)

// LoadBalancerHandler handles incoming HTTP requests and distributes them
// to backend servers using the configured load balancing strategy.
type LoadBalancerHandler struct {
	logger   *slog.Logger
	balancer *loadbalancer.LoadBalancer
	backends []*backend.Backend
}

// NewLoadBalancerHandler creates a new HTTP handler for load balancing.
func NewLoadBalancerHandler(logger *slog.Logger, lb *loadbalancer.LoadBalancer, backends []*backend.Backend) *LoadBalancerHandler {
	return &LoadBalancerHandler{
		logger:   logger,
		balancer: lb,
		backends: backends,
	}
}

// ServeHTTP handles HTTP requests by selecting a backend using the configured
// strategy, forwarding the request, and tracking response times.
func (lb *LoadBalancerHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var nextServer *backend.Backend
	var err error

	// Check if strategy needs a key (consistent hash)
	if _, ok := lb.balancer.LoadBalancerStrategy().(interface{ SetKey(string) }); ok {
		// For IP-based strategies: atomically set key and select backend
		ip := extractClientIP(r)
		nextServer, err = lb.balancer.GetAndReserveServerWithKey(lb.backends, ip)
	} else {
		// For other strategies: use regular method
		nextServer, err = lb.balancer.GetAndReserveServer(lb.backends)
	}

	if err != nil {
		http.Error(w, "No healthy server available", http.StatusServiceUnavailable)
		return
	}
	defer nextServer.DecrementConn()
	start := time.Now()

	lb.logger.Info("Forwarding request", "method", r.Method, "url", r.URL.String(), "backend", nextServer.URL().String())

	w.Header().Set("X-Backend-Server", nextServer.URL().String())
	nextServer.ReverseProxy().ServeHTTP(w, r)

	duration := time.Since(start)
	nextServer.RecordResponse(duration)
}

func extractClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return strings.TrimSpace(strings.Split(xff, ",")[0])
	}

	host, _, _ := net.SplitHostPort(r.RemoteAddr)
	return host
}

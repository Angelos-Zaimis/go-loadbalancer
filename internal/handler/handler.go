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
    clientIP := extractClientIP(r)

    // Log incoming request details (matching challenge format)
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
    defer nextServer.DecrementConn()
    start := time.Now()

    lb.logger.Info("Forwarding to backend",
        slog.String("client", clientIP),
        slog.String("backend", nextServer.URL().String()))

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

package main

import (
	"net/http"

	"github.com/angeloszaimis/load-balancer/internal/handler"
	"github.com/angeloszaimis/load-balancer/internal/metrics"
)

func setupRouter(loadBalancerHandler *handler.LoadBalancerHandler, metricsCollector *metrics.Collector, strategy string) *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("/", loadBalancerHandler.ServeHTTP)
	mux.HandleFunc("/metrics", metricsCollector.Handler(strategy))

	return mux
}

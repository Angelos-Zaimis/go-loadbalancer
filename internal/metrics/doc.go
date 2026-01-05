// Package metrics provides real-time metrics collection for the load balancer.
//
// It uses a channel-based event pipeline to asynchronously collect metrics about:
//   - Request counts per backend
//   - Backend selection frequencies
//   - Response times with percentile calculations (P50, P95, P99)
//   - HTTP status code distribution
//   - Health status tracking
//
// The collector runs in a dedicated goroutine and processes events without blocking
// the request path. Events are sent via buffered channels with non-blocking semantics
// to prevent performance degradation under load.
//
// Example usage:
//
//	collector := metrics.NewCollector(1000, logger)
//	collector.Start(ctx)
//
//	// Emit events during request handling
//	collector.EventChannel() <- metrics.MetricEvent{
//		Type:       metrics.EventResponseCompleted,
//		Backend:    "http://localhost:8081",
//		Duration:   150 * time.Millisecond,
//		StatusCode: 200,
//	}
//
//	// Get metrics snapshot
//	snapshot := collector.Snapshot("round-robin")
//
// The package provides thread-safe metrics storage using sync.RWMutex and supports
// graceful shutdown with event draining to prevent data loss.
package metrics

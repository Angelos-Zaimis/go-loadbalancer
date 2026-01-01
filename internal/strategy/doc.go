// Package strategy defines the load balancing strategy interface and
// implements various algorithms:
//
//   - Round Robin: Sequential distribution across backends
//   - Random: Random backend selection
//   - Least Connections: Routes to backend with fewest active connections
//   - Least Response Time: Routes based on exponentially weighted moving average (EWMA) response times
//   - IP Hash: Consistent hashing for session affinity
//   - Weighted Round Robin: Distribution proportional to backend weights
//
// All strategies respect backend health status and only select healthy backends.
package strategy

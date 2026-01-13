// Package circuitbreaker implements the circuit breaker pattern for backend failover.
//
// A circuit breaker prevents cascading failures by temporarily blocking requests
// to failing backends. It has three states:
//
//   - CLOSED: Normal operation, requests pass through
//   - OPEN: Backend failing, requests blocked
//   - HALF-OPEN: Testing if backend recovered
//
// Usage:
//
//	registry := circuitbreaker.NewRegistry(5, 30*time.Second)
//	cb := registry.GetBreaker("http://localhost:8081")
//	if cb.Allow() {
//	    // Make request...
//	    if err != nil {
//	        cb.RecordFailure()
//	    } else {
//	        cb.RecordSuccess()
//	    }
//	}
package circuitbreaker

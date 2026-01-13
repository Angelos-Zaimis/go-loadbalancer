package circuitbreaker

import (
	"sync"
	"time"
)

type State int

const (
	StateClosed State = iota // Normal operation
	StateOpen              // Blocking Requests
	StateHalfOpen          // Testing with one request
)

type CircuitBreaker struct {
	mutex 		sync.Mutex
	state       State
	failures    int 
	lastFailure time.Time
	failureThreshold int
	resetTimeout     time.Duration
}

func NewCircuitBreaker(threshold int, timeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		state: StateClosed,
		failureThreshold: threshold,
		resetTimeout: timeout,
	}
}

func (cb *CircuitBreaker) Allow() bool {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	switch cb.state {
	case StateClosed:
		return true
	case StateOpen:
		if time.Since(cb.lastFailure) >= cb.resetTimeout {
			cb.state = StateHalfOpen
			return true
		}

		return false
	case StateHalfOpen:
		return true
	default:
		return true 
	}
}

func (cb *CircuitBreaker) RecordFailure() {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	cb.failures++
	cb.lastFailure = time.Now()

	if cb.state == StateHalfOpen {
		cb.state = StateOpen
	}

	if cb.failures >= cb.failureThreshold {
		cb.state = StateOpen
	}
}

func (cb *CircuitBreaker) State() State {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()
	return cb.state
}

func (s State) String() string {
	switch s {
		case StateClosed:
			return "CLOSED"
		case StateOpen:
			return "OPEN"
		case StateHalfOpen:
			return "HALF-OPEN"
		default:
			return "UNKNOWN"
	}
}

func (cb *CircuitBreaker) RecordSuccess() {
    cb.mutex.Lock()
    defer cb.mutex.Unlock()

    cb.failures = 0
    cb.state = StateClosed
}
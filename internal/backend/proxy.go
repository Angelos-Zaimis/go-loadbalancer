package backend

import (
	"net/http/httputil"
	"net/url"
	"sync"
	"time"
)

// Backend represents a backend server with health status, connection tracking,
// and response time monitoring.
type Backend struct {
	url               *url.URL
	proxy             *httputil.ReverseProxy
	mutex             sync.Mutex
	isHealthy         bool
	activeConnections int
	weight            int
	ewmaResponseTime  time.Duration
	hasEWMA           bool
}

const ewmaAlpha = 0.2

// ReverseProxy returns the HTTP reverse proxy for this backend.
func (b *Backend) ReverseProxy() *httputil.ReverseProxy {
	return b.proxy
}

// IncrementConn increments the active connection count.
func (b *Backend) IncrementConn() {
	b.mutex.Lock()
	b.activeConnections++
	b.mutex.Unlock()
}

// DecrementConn decrements the active connection count.
func (b *Backend) DecrementConn() {
	b.mutex.Lock()
	if b.activeConnections > 0 {
		b.activeConnections--
	}
	b.mutex.Unlock()
}

// ActiveConnections returns the current number of active connections.
func (b *Backend) ActiveConnections() int {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	return b.activeConnections
}

// URL returns the backend server URL.
func (b *Backend) URL() *url.URL {
	return b.url
}

// IsHealthy returns true if the backend is currently healthy.
// IsHealthy returns true if the backend is currently healthy.
func (b *Backend) IsHealthy() bool {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	return b.isHealthy
}

// SetHealthy updates the backend's health status.
// Returns true if the status changed, false if it was already in that state.
func (b *Backend) SetHealthy(healthy bool) (changed bool) {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	if b.isHealthy == healthy {
		return false
	}

	b.isHealthy = healthy
	return true
}

// RecordResponse updates the exponentially weighted moving average (EWMA)
// response time using the latest request duration.
func (b *Backend) RecordResponse(duration time.Duration) {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	if !b.hasEWMA {
		b.ewmaResponseTime = duration
		b.hasEWMA = true
		return
	}
	//ewma = (1 - α) * ewma + α * latest
	b.ewmaResponseTime = time.Duration((1-ewmaAlpha)*float64(b.ewmaResponseTime) + ewmaAlpha*float64(duration))
}

// EWMATime returns the exponentially weighted moving average response time.
// Returns 0 if no responses have been recorded yet.
func (b *Backend) EWMATime() time.Duration {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	if !b.hasEWMA {
		return 0
	}

	return b.ewmaResponseTime
}

func (b *Backend) Weight() int {
	return b.weight
}

// New creates a new Backend with the given URL.
// The backend starts in an unhealthy state until the first health check passes.
func New(url *url.URL, weight int) *Backend {
	return &Backend{
		url:       url,
		proxy:     httputil.NewSingleHostReverseProxy(url),
		isHealthy: false,
		weight: weight,
	}
}

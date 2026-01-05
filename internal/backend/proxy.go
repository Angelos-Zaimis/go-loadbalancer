package backend

import (
	"net/http/httputil"
	"net/url"
	"sync"
	"time"
)

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

type bufferPool struct {
	pool *sync.Pool
}

func (bp *bufferPool) Get() []byte {
	return bp.pool.Get().([]byte)
}

func (bp *bufferPool) Put(b []byte) {
    bp.pool.Put(b)
}

var sharedBufferPool = &bufferPool{
    pool: &sync.Pool{
        New: func() interface{} {
            return make([]byte, 32*1024)
        },
    },
}

func (b *Backend) ReverseProxy() *httputil.ReverseProxy {
	return b.proxy
}

func (b *Backend) IncrementConn() {
	b.mutex.Lock()
	b.activeConnections++
	b.mutex.Unlock()
}

func (b *Backend) DecrementConn() {
	b.mutex.Lock()
	if b.activeConnections > 0 {
		b.activeConnections--
	}
	b.mutex.Unlock()
}

func (b *Backend) ActiveConnections() int {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	return b.activeConnections
}

func (b *Backend) URL() *url.URL {
	return b.url
}

func (b *Backend) IsHealthy() bool {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	return b.isHealthy
}

func (b *Backend) SetHealthy(healthy bool) (changed bool) {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	if b.isHealthy == healthy {
		return false
	}

	b.isHealthy = healthy
	return true
}

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

func New(url *url.URL, weight int) *Backend {
	proxy := httputil.NewSingleHostReverseProxy(url)
	proxy.BufferPool = sharedBufferPool

	return &Backend{
		url:       url,
		proxy:     proxy,
		isHealthy: false,
		weight: weight,
	}
}

package metrics

import (
	"context"
	"log/slog"
	"time"
)

type EventType string

const (
    EventRequestReceived   EventType = "request_received"
    EventBackendSelected   EventType = "backend_selected"
    EventResponseCompleted EventType = "response_completed"
    EventHealthChanged     EventType = "health_changed"
)

type MetricEvent struct {
	Type EventType
	Timestamp time.Time
	Backend string
	Duration time.Duration
	StatusCode int
	Healthy bool
}

type Collector struct {
	eventCh 	  chan MetricEvent
	metrics 	  *Metrics
	logger 		  *slog.Logger
}

func NewCollector(bufferSize int, logger *slog.Logger) *Collector {
	return &Collector{
		eventCh: make(chan MetricEvent, bufferSize),
		metrics: NewMetrics(),
		logger: logger,
	}
}

func (c *Collector) EventChannel() chan<- MetricEvent {
	return c.eventCh
}

func (c *Collector) Start(ctx context.Context) {
	go c.run(ctx)
}

func (c *Collector) run(ctx context.Context) {
	c.logger.Info("Metrics collector started")
    defer c.logger.Info("Metrics collector stopped")

	for {
		select {
		case event:= <-c.eventCh:
			c.processEvent(event)
		case <-ctx.Done():
			// Drain remaining events before shutdown
			c.drain()
			return
		}
	}
}

func (c *Collector) processEvent(event MetricEvent) {
    switch event.Type {
    case EventRequestReceived:
        c.metrics.IncrementRequests(event.Backend)
        
    case EventBackendSelected:
        c.metrics.RecordBackendSelection(event.Backend)
        
    case EventResponseCompleted:
        c.metrics.RecordResponse(event.Backend, event.Duration, event.StatusCode)
        
    case EventHealthChanged:
        c.metrics.UpdateHealthStatus(event.Backend, event.Healthy)
    }
}

func (c *Collector) drain() {
	for {
		select {
		case event := <-c.eventCh:
			c.processEvent(event)
		default:
			return
		}
	}
}
func (c *Collector) Snapshot(algorithm string) Snapshot {
return c.metrics.Snapshot(algorithm)
}

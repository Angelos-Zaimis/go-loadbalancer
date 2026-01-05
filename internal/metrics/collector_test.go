package metrics_test

import (
	"context"
	"log/slog"
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/angeloszaimis/load-balancer/internal/metrics"
)

var _ = Describe("Collector", func() {
	var (
		collector *metrics.Collector
		log       *slog.Logger
		ctx       context.Context
		cancel    context.CancelFunc
	)

	BeforeEach(func() {
		log = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelError, // Suppress logs in tests
		}))
		ctx, cancel = context.WithCancel(context.Background())
		collector = metrics.NewCollector(100, log)
	})

	AfterEach(func() {
		cancel()
		time.Sleep(10 * time.Millisecond) // Allow goroutine to finish
	})

	Describe("NewCollector", func() {
		It("should create a collector with specified buffer size", func() {
			c := metrics.NewCollector(500, log)
			Expect(c).NotTo(BeNil())
		})
	})

	Describe("EventChannel", func() {
		It("should return a write-only channel", func() {
			ch := collector.EventChannel()
			Expect(ch).NotTo(BeNil())
		})
	})

	Describe("Start and event processing", func() {
		It("should process EventRequestReceived", func() {
			collector.Start(ctx)

			event := metrics.MetricEvent{
				Type:      metrics.EventRequestReceived,
				Timestamp: time.Now(),
				Backend:   "http://localhost:8081",
			}

			collector.EventChannel() <- event
			time.Sleep(10 * time.Millisecond)

			snap := collector.Snapshot("round-robin")
			Expect(snap.Backends["http://localhost:8081"].Requests).To(Equal(int64(1)))
		})

		It("should process EventBackendSelected", func() {
			collector.Start(ctx)

			event := metrics.MetricEvent{
				Type:      metrics.EventBackendSelected,
				Timestamp: time.Now(),
				Backend:   "http://localhost:8081",
			}

			collector.EventChannel() <- event
			time.Sleep(10 * time.Millisecond)

			snap := collector.Snapshot("round-robin")
			Expect(snap.Backends["http://localhost:8081"].Selections).To(Equal(int64(1)))
		})

		It("should process EventResponseCompleted", func() {
			collector.Start(ctx)

			event := metrics.MetricEvent{
				Type:       metrics.EventResponseCompleted,
				Timestamp:  time.Now(),
				Backend:    "http://localhost:8081",
				Duration:   100 * time.Millisecond,
				StatusCode: 200,
			}

			collector.EventChannel() <- event
			time.Sleep(10 * time.Millisecond)

			snap := collector.Snapshot("round-robin")
			backend := snap.Backends["http://localhost:8081"]
			Expect(backend.AvgResponse).To(Equal(100 * time.Millisecond))
			Expect(backend.StatusCodes[200]).To(Equal(int64(1)))
		})

		It("should process EventHealthChanged", func() {
			collector.Start(ctx)

			event := metrics.MetricEvent{
				Type:      metrics.EventHealthChanged,
				Timestamp: time.Now(),
				Backend:   "http://localhost:8081",
				Healthy:   true,
			}

			collector.EventChannel() <- event
			time.Sleep(10 * time.Millisecond)

			snap := collector.Snapshot("round-robin")
			Expect(snap.Backends["http://localhost:8081"].Healthy).To(BeTrue())
		})

		It("should process multiple events in sequence", func() {
			collector.Start(ctx)

			events := []metrics.MetricEvent{
				{
					Type:      metrics.EventRequestReceived,
					Timestamp: time.Now(),
					Backend:   "http://localhost:8081",
				},
				{
					Type:      metrics.EventBackendSelected,
					Timestamp: time.Now(),
					Backend:   "http://localhost:8081",
				},
				{
					Type:       metrics.EventResponseCompleted,
					Timestamp:  time.Now(),
					Backend:    "http://localhost:8081",
					Duration:   50 * time.Millisecond,
					StatusCode: 201,
				},
			}

			for _, event := range events {
				collector.EventChannel() <- event
			}
			time.Sleep(20 * time.Millisecond)

			snap := collector.Snapshot("round-robin")
			backend := snap.Backends["http://localhost:8081"]
			Expect(backend.Requests).To(Equal(int64(1)))
			Expect(backend.Selections).To(Equal(int64(1)))
			Expect(backend.AvgResponse).To(Equal(50 * time.Millisecond))
			Expect(backend.StatusCodes[201]).To(Equal(int64(1)))
		})

		It("should drain events on context cancellation", func() {
			collector.Start(ctx)

			// Send events before cancellation
			for i := 0; i < 5; i++ {
				collector.EventChannel() <- metrics.MetricEvent{
					Type:      metrics.EventRequestReceived,
					Timestamp: time.Now(),
					Backend:   "http://localhost:8081",
				}
			}

			cancel()
			time.Sleep(20 * time.Millisecond)

			snap := collector.Snapshot("round-robin")
			// All events should be processed via drain
			Expect(snap.Backends["http://localhost:8081"].Requests).To(Equal(int64(5)))
		})
	})

	Describe("Handler", func() {
		It("should return a valid http.HandlerFunc", func() {
			handler := collector.Handler("round-robin")
			Expect(handler).NotTo(BeNil())
		})
	})

	Describe("Snapshot", func() {
		It("should return current metrics snapshot", func() {
			collector.Start(ctx)

			collector.EventChannel() <- metrics.MetricEvent{
				Type:      metrics.EventRequestReceived,
				Timestamp: time.Now(),
				Backend:   "http://localhost:8081",
			}
			time.Sleep(10 * time.Millisecond)

			snap := collector.Snapshot("least-conn")
			Expect(snap.Algorithm).To(Equal("least-conn"))
			Expect(snap.TotalRequests).To(Equal(int64(1)))
		})
	})
})

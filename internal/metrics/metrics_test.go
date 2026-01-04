package metrics_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/angeloszaimis/load-balancer/internal/metrics"
)

var _ = Describe("Metrics", func() {
	var m *metrics.Metrics

	BeforeEach(func() {
		m = metrics.NewMetrics()
	})

	Describe("NewMetrics", func() {
		It("should create a new metrics instance", func() {
			Expect(m).NotTo(BeNil())
		})
	})

	Describe("IncrementRequests", func() {
		It("should increment request count for a backend", func() {
			m.IncrementRequests("http://localhost:8081")
			m.IncrementRequests("http://localhost:8081")

			snap := m.Snapshot("round-robin")
			Expect(snap.TotalRequests).To(Equal(int64(2)))
			Expect(snap.Backends["http://localhost:8081"].Requests).To(Equal(int64(2)))
		})

		It("should track multiple backends separately", func() {
			m.IncrementRequests("http://localhost:8081")
			m.IncrementRequests("http://localhost:8082")
			m.IncrementRequests("http://localhost:8081")

			snap := m.Snapshot("round-robin")
			Expect(snap.TotalRequests).To(Equal(int64(3)))
			Expect(snap.Backends["http://localhost:8081"].Requests).To(Equal(int64(2)))
			Expect(snap.Backends["http://localhost:8082"].Requests).To(Equal(int64(1)))
		})
	})

	Describe("RecordBackendSelection", func() {
		It("should track backend selections", func() {
			m.RecordBackendSelection("http://localhost:8081")
			m.RecordBackendSelection("http://localhost:8081")
			m.RecordBackendSelection("http://localhost:8082")

			snap := m.Snapshot("round-robin")
			Expect(snap.Backends["http://localhost:8081"].Selections).To(Equal(int64(2)))
			Expect(snap.Backends["http://localhost:8082"].Selections).To(Equal(int64(1)))
		})
	})

	Describe("RecordResponse", func() {
		It("should record response time and status code", func() {
			m.RecordResponse("http://localhost:8081", 100*time.Millisecond, 200)
			m.RecordResponse("http://localhost:8081", 200*time.Millisecond, 200)

			snap := m.Snapshot("round-robin")
			backend := snap.Backends["http://localhost:8081"]

			Expect(backend.AvgResponse).To(Equal(150 * time.Millisecond))
			Expect(backend.StatusCodes[200]).To(Equal(int64(2)))
		})

		It("should track different status codes", func() {
			m.RecordResponse("http://localhost:8081", 100*time.Millisecond, 200)
			m.RecordResponse("http://localhost:8081", 150*time.Millisecond, 201)
			m.RecordResponse("http://localhost:8081", 200*time.Millisecond, 500)

			snap := m.Snapshot("round-robin")
			backend := snap.Backends["http://localhost:8081"]

			Expect(backend.StatusCodes[200]).To(Equal(int64(1)))
			Expect(backend.StatusCodes[201]).To(Equal(int64(1)))
			Expect(backend.StatusCodes[500]).To(Equal(int64(1)))
		})

		It("should calculate percentiles correctly", func() {
			for i := 1; i <= 100; i++ {
				m.RecordResponse("http://localhost:8081", time.Duration(i)*time.Millisecond, 200)
			}

			snap := m.Snapshot("round-robin")
			backend := snap.Backends["http://localhost:8081"]

			Expect(backend.P50Response).To(BeNumerically("~", 50*time.Millisecond, 1*time.Millisecond))
			Expect(backend.P95Response).To(BeNumerically("~", 95*time.Millisecond, 1*time.Millisecond))
			Expect(backend.P99Response).To(BeNumerically("~", 99*time.Millisecond, 1*time.Millisecond))
		})

		It("should limit stored response times to 1000", func() {
			for i := 1; i <= 1500; i++ {
				m.RecordResponse("http://localhost:8081", time.Duration(i)*time.Millisecond, 200)
			}

			snap := m.Snapshot("round-robin")
			backend := snap.Backends["http://localhost:8081"]

			Expect(backend.AvgResponse).To(BeNumerically(">", 500*time.Millisecond))
		})
	})

	Describe("UpdateHealthStatus", func() {
		It("should update backend health status", func() {
			m.UpdateHealthStatus("http://localhost:8081", true)

			snap := m.Snapshot("round-robin")
			Expect(snap.Backends["http://localhost:8081"].Healthy).To(BeTrue())
		})

		It("should track health status changes", func() {
			m.UpdateHealthStatus("http://localhost:8081", true)
			snap1 := m.Snapshot("round-robin")
			Expect(snap1.Backends["http://localhost:8081"].Healthy).To(BeTrue())

			m.UpdateHealthStatus("http://localhost:8081", false)
			snap2 := m.Snapshot("round-robin")
			Expect(snap2.Backends["http://localhost:8081"].Healthy).To(BeFalse())
		})
	})

	Describe("Snapshot", func() {
		It("should return a snapshot with algorithm", func() {
			m.IncrementRequests("http://localhost:8081")

			snap := m.Snapshot("least-conn")
			Expect(snap.Algorithm).To(Equal("least-conn"))
		})

		It("should include uptime", func() {
			time.Sleep(10 * time.Millisecond)

			snap := m.Snapshot("round-robin")
			Expect(snap.Uptime).To(BeNumerically(">", 0))
		})

		It("should handle empty metrics", func() {
			snap := m.Snapshot("round-robin")

			Expect(snap.TotalRequests).To(Equal(int64(0)))
			Expect(snap.Backends).To(BeEmpty())
		})

		It("should return independent snapshot", func() {
			m.IncrementRequests("http://localhost:8081")

			snap1 := m.Snapshot("round-robin")
			m.IncrementRequests("http://localhost:8081")
			snap2 := m.Snapshot("round-robin")

			Expect(snap1.TotalRequests).To(Equal(int64(1)))
			Expect(snap2.TotalRequests).To(Equal(int64(2)))
		})
	})
})

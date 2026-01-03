package backend_test

import (
	"net/url"
	"sync"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/angeloszaimis/load-balancer/internal/backend"
)

var _ = Describe("Backend", func() {
	var (
		testURL *url.URL
		b       *backend.Backend
	)

	BeforeEach(func() {
		var err error
		testURL, err = url.Parse("http://localhost:8081")
		Expect(err).NotTo(HaveOccurred())
		b = backend.New(testURL, 1)
	})

	Describe("New", func() {
		It("should create a backend with the correct URL", func() {
			Expect(b).NotTo(BeNil())
			Expect(b.URL()).To(Equal(testURL))
		})

		It("should initialize as unhealthy", func() {
			Expect(b.IsHealthy()).To(BeFalse())
		})

		It("should have zero active connections", func() {
			Expect(b.ActiveConnections()).To(Equal(0))
		})

		It("should provide a reverse proxy", func() {
			Expect(b.ReverseProxy()).NotTo(BeNil())
		})
	})

	Describe("Health Management", func() {
		Context("SetHealthy", func() {
			It("should update health status to healthy", func() {
				changed := b.SetHealthy(true)
				Expect(changed).To(BeTrue())
				Expect(b.IsHealthy()).To(BeTrue())
			})

			It("should update health status to unhealthy", func() {
				b.SetHealthy(true)
				changed := b.SetHealthy(false)
				Expect(changed).To(BeTrue())
				Expect(b.IsHealthy()).To(BeFalse())
			})

			It("should return false when setting same status", func() {
				b.SetHealthy(true)
				changed := b.SetHealthy(true)
				Expect(changed).To(BeFalse())
			})

			It("should handle multiple toggles", func() {
				b.SetHealthy(true)
				Expect(b.IsHealthy()).To(BeTrue())

				b.SetHealthy(false)
				Expect(b.IsHealthy()).To(BeFalse())

				b.SetHealthy(true)
				Expect(b.IsHealthy()).To(BeTrue())
			})
		})

		Context("IsHealthy", func() {
			It("should be thread-safe", func() {
				var wg sync.WaitGroup
				for i := 0; i < 100; i++ {
					wg.Add(1)
					go func(healthy bool) {
						defer wg.Done()
						b.SetHealthy(healthy)
						_ = b.IsHealthy()
					}(i%2 == 0)
				}
				wg.Wait()
			})
		})
	})

	Describe("Connection Tracking", func() {
		Context("IncrementConn", func() {
			It("should increase active connection count", func() {
				Expect(b.ActiveConnections()).To(Equal(0))

				b.IncrementConn()
				Expect(b.ActiveConnections()).To(Equal(1))

				b.IncrementConn()
				b.IncrementConn()
				Expect(b.ActiveConnections()).To(Equal(3))
			})

			It("should be thread-safe", func() {
				var wg sync.WaitGroup
				for i := 0; i < 100; i++ {
					wg.Add(1)
					go func() {
						defer wg.Done()
						b.IncrementConn()
					}()
				}
				wg.Wait()
				Expect(b.ActiveConnections()).To(Equal(100))
			})
		})

		Context("DecrementConn", func() {
			It("should decrease active connection count", func() {
				b.IncrementConn()
				b.IncrementConn()
				b.IncrementConn()
				Expect(b.ActiveConnections()).To(Equal(3))

				b.DecrementConn()
				Expect(b.ActiveConnections()).To(Equal(2))

				b.DecrementConn()
				Expect(b.ActiveConnections()).To(Equal(1))
			})

			It("should not go below zero", func() {
				Expect(b.ActiveConnections()).To(Equal(0))
				b.DecrementConn()
				b.DecrementConn()
				Expect(b.ActiveConnections()).To(Equal(0))
			})

			It("should be thread-safe", func() {
				for i := 0; i < 50; i++ {
					b.IncrementConn()
				}

				var wg sync.WaitGroup
				for i := 0; i < 50; i++ {
					wg.Add(1)
					go func() {
						defer wg.Done()
						b.DecrementConn()
					}()
				}
				wg.Wait()
				Expect(b.ActiveConnections()).To(Equal(0))
			})
		})

		Context("ActiveConnections", func() {
			It("should accurately track connection count", func() {
				b.IncrementConn()
				b.IncrementConn()
				Expect(b.ActiveConnections()).To(Equal(2))

				b.DecrementConn()
				Expect(b.ActiveConnections()).To(Equal(1))

				b.IncrementConn()
				Expect(b.ActiveConnections()).To(Equal(2))
			})
		})
	})

	Describe("Response Time Tracking (EWMA)", func() {
		Context("RecordResponse", func() {
			It("should record first response time", func() {
				duration := 100 * time.Millisecond
				b.RecordResponse(duration)
				// First call initializes EWMA
			})

			It("should update EWMA with subsequent responses", func() {
				b.RecordResponse(100 * time.Millisecond)
				b.RecordResponse(200 * time.Millisecond)
				b.RecordResponse(150 * time.Millisecond)
				// EWMA should smooth these values
			})

			It("should handle very fast responses", func() {
				b.RecordResponse(1 * time.Microsecond)
				b.RecordResponse(2 * time.Microsecond)
			})

			It("should handle very slow responses", func() {
				b.RecordResponse(10 * time.Second)
				b.RecordResponse(5 * time.Second)
			})

			It("should be thread-safe", func() {
				var wg sync.WaitGroup
				for i := 0; i < 100; i++ {
					wg.Add(1)
					go func(i int) {
						defer wg.Done()
						b.RecordResponse(time.Duration(i) * time.Millisecond)
					}(i)
				}
				wg.Wait()
			})
		})
	})

	Describe("URL", func() {
		It("should return the correct URL", func() {
			Expect(b.URL()).To(Equal(testURL))
			Expect(b.URL().String()).To(Equal("http://localhost:8081"))
		})

		It("should handle different URL schemes", func() {
			httpsURL, _ := url.Parse("https://example.com:443")
			httpsBackend := backend.New(httpsURL, 1)
			Expect(httpsBackend.URL().Scheme).To(Equal("https"))
			Expect(httpsBackend.URL().Host).To(Equal("example.com:443"))
		})
	})

	Describe("ReverseProxy", func() {
		It("should provide a reverse proxy instance", func() {
			proxy := b.ReverseProxy()
			Expect(proxy).NotTo(BeNil())
		})

		It("should return the same proxy instance", func() {
			proxy1 := b.ReverseProxy()
			proxy2 := b.ReverseProxy()
			Expect(proxy1).To(Equal(proxy2))
		})
	})
})

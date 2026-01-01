package httpserver_test

import (
	"context"
	"io"
	"net/http"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/angeloszaimis/load-balancer/internal/httpserver"
)

var _ = Describe("HTTP Server", func() {
	Context("server creation", func() {
		It("creates server with valid address", func() {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
			srv, err := httpserver.New("localhost:9999", handler)
			Expect(err).NotTo(HaveOccurred())
			Expect(srv).NotTo(BeNil())
		})

		It("creates server with IP address", func() {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
			srv, err := httpserver.New("127.0.0.1:9999", handler)
			Expect(err).NotTo(HaveOccurred())
			Expect(srv).NotTo(BeNil())
		})

		It("handles port-only address", func() {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
			srv, err := httpserver.New(":9999", handler)
			Expect(err).NotTo(HaveOccurred())
			Expect(srv).NotTo(BeNil())
		})

		It("rejects invalid address", func() {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
			srv, err := httpserver.New("invalid:host:port", handler)
			Expect(err).To(HaveOccurred())
			Expect(srv).To(BeNil())
		})
	})

	Context("server lifecycle", func() {
		var testServer *httpserver.Server
		var testPort = ":19999"

		AfterEach(func() {
			if testServer != nil {
				ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
				defer cancel()
				_ = testServer.Shutdown(ctx)
			}
		})

		It("starts and handles requests", func() {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("test"))
			})
			var err error
			testServer, err = httpserver.New(testPort, handler)
			Expect(err).NotTo(HaveOccurred())

			go func() {
				testServer.Start()
			}()
			time.Sleep(100 * time.Millisecond)

			resp, err := http.Get("http://localhost" + testPort)
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			body, _ := io.ReadAll(resp.Body)
			Expect(string(body)).To(Equal("test"))
		})

		It("shuts down gracefully", func() {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
			var err error
			testServer, err = httpserver.New(":19998", handler)
			Expect(err).NotTo(HaveOccurred())

			go func() {
				testServer.Start()
			}()
			time.Sleep(100 * time.Millisecond)

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			err = testServer.Shutdown(ctx)
			Expect(err).NotTo(HaveOccurred())
		})
	})
})

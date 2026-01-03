package healthcheck_test

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/angeloszaimis/load-balancer/internal/backend"
	"github.com/angeloszaimis/load-balancer/internal/healthcheck"
)

var _ = Describe("Healthcheck", func() {
	var (
		backends     []*backend.Backend
		mockBackend1 *httptest.Server
		log          *slog.Logger
	)

	BeforeEach(func() {
		log = slog.New(slog.NewTextHandler(os.Stdout, nil))

		mockBackend1 = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/health" {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("OK"))
			}
		}))

		backends = []*backend.Backend{
			backend.New(mustParseURL(mockBackend1.URL), 1),
		}
		backends[0].SetHealthy(false)
	})

	AfterEach(func() {
		mockBackend1.Close()
	})

	Describe("HealthCheck", func() {
		It("should mark healthy backend as healthy", func() {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			go healthcheck.HealthCheck(ctx, backends[0], 100*time.Millisecond, log)

			time.Sleep(250 * time.Millisecond)
			cancel()

			Expect(backends[0].IsHealthy()).To(BeTrue())
		})

		It("should stop when context is cancelled", func() {
			ctx, cancel := context.WithCancel(context.Background())

			go healthcheck.HealthCheck(ctx, backends[0], 100*time.Millisecond, log)

			time.Sleep(150 * time.Millisecond)
			cancel()
			time.Sleep(100 * time.Millisecond)

			// Should not panic
		})
	})
})

func mustParseURL(rawURL string) *url.URL {
	u, err := url.Parse(rawURL)
	if err != nil {
		panic(err)
	}
	return u
}

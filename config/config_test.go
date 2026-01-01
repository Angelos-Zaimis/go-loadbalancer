package config_test

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/angeloszaimis/load-balancer/config"
)

var _ = Describe("Config", func() {
	var tempDir string

	BeforeEach(func() {
		var err error
		tempDir, err = os.MkdirTemp("", "config-test-*")
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		os.RemoveAll(tempDir)
		os.Unsetenv("STRATEGY")
		os.Unsetenv("BACKENDS")
	})

	Describe("Load", func() {
		Context("with valid config file", func() {
			BeforeEach(func() {
				configContent := `
ENV: "dev"
HTTP_ADDR: ":8080"
LOG_LEVEL: "info"
HEALTH_CHECK_INTERVAL: "10s"
STRATEGY: "round-robin"
BACKENDS: "http://localhost:8081,http://localhost:8082"
`
				configPath := filepath.Join(tempDir, "config.yaml")
				err := os.WriteFile(configPath, []byte(configContent), 0644)
				Expect(err).NotTo(HaveOccurred())

				err = os.Chdir(tempDir)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should load configuration successfully", func() {
				cfg, err := config.Load()
				Expect(err).NotTo(HaveOccurred())
				Expect(cfg).NotTo(BeNil())
			})

			It("should parse strategy correctly", func() {
				cfg, _ := config.Load()
				Expect(cfg.Strategy).To(Equal("round-robin"))
			})

			It("should parse health check interval", func() {
				cfg, _ := config.Load()
				Expect(cfg.HealthCheckInterval).To(Equal("10s"))
			})
		})

		Context("with environment variables", func() {
			BeforeEach(func() {
				err := os.Chdir(tempDir)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should use defaults when config file missing", func() {
				cfg, err := config.Load()
				Expect(err).NotTo(HaveOccurred())
				Expect(cfg.Strategy).To(Equal("round-robin"))
			})
		})
	})
})

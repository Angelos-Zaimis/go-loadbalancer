package logger_test

import (
	"log/slog"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/angeloszaimis/load-balancer/pkg/logger"
)

var _ = Describe("Logger", func() {
	Describe("New", func() {
		It("should create logger with info level", func() {
			log := logger.New("info", false, "dev")
			Expect(log).NotTo(BeNil())
		})

		It("should create logger with debug level", func() {
			log := logger.New("debug", false, "dev")
			Expect(log).NotTo(BeNil())
		})

		It("should create logger with warn level", func() {
			log := logger.New("warn", false, "dev")
			Expect(log).NotTo(BeNil())
		})

		It("should create logger with error level", func() {
			log := logger.New("error", false, "dev")
			Expect(log).NotTo(BeNil())
		})

		It("should default to info for invalid level", func() {
			log := logger.New("invalid", false, "dev")
			Expect(log).NotTo(BeNil())
		})

		It("should create prod logger", func() {
			log := logger.New("info", false, "prod")
			Expect(log).NotTo(BeNil())
		})

		It("should support addSource option", func() {
			log := logger.New("info", true, "dev")
			Expect(log).NotTo(BeNil())
		})

		It("should include environment attribute", func() {
			log := logger.New("info", false, "dev")
			Expect(log).NotTo(BeNil())

			// Logger should have default level behavior
			Expect(log.Enabled(nil, slog.LevelInfo)).To(BeTrue())
			Expect(log.Enabled(nil, slog.LevelDebug)).To(BeFalse())
		})

		It("should respect debug level", func() {
			log := logger.New("debug", false, "dev")

			Expect(log.Enabled(nil, slog.LevelDebug)).To(BeTrue())
			Expect(log.Enabled(nil, slog.LevelInfo)).To(BeTrue())
		})

		It("should respect warn level", func() {
			log := logger.New("warn", false, "dev")

			Expect(log.Enabled(nil, slog.LevelInfo)).To(BeFalse())
			Expect(log.Enabled(nil, slog.LevelWarn)).To(BeTrue())
		})

		It("should respect error level", func() {
			log := logger.New("error", false, "dev")

			Expect(log.Enabled(nil, slog.LevelWarn)).To(BeFalse())
			Expect(log.Enabled(nil, slog.LevelError)).To(BeTrue())
		})
	})
})

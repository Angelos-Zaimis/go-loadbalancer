package config

import (
	"log/slog"
	"net"
	"net/url"
	"strings"
	"time"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/go-ozzo/ozzo-validation/v4/is"
	"github.com/spf13/viper"
)

const (
	EnvDev     = "dev"
	EnvStaging = "staging"
	EnvProd    = "prod"
)

const (
	LogLevelDebug = "debug"
	LogLevelInfo  = "info"
	LogLevelWarn  = "warn"
	LogLevelError = "error"
)

// Config holds the application configuration loaded from YAML or environment variables.
type Config struct {
	Env                 string   `mapstructure:"ENV"`
	HTTPAddr            string   `mapstructure:"HTTP_ADDR"`
	LogLevel            string   `mapstructure:"LOG_LEVEL"`
	HealthCheckInterval string   `mapstructure:"HEALTH_CHECK_INTERVAL"`
	Backends            []string `mapstructure:"BACKENDS"`
	Strategy            string   `mapstructure:"STRATEGY"`
	VirtualNodes        int      `mapstructure:"VIRTUAL_NODES"`
}

// Load reads configuration from config.yaml and environment variables.
// Environment variables take precedence over file configuration.
func Load() (*Config, error) {
	viper.SetDefault("ENV", EnvDev)
	viper.SetDefault("HTTP_ADDR", ":8080")
	viper.SetDefault("HEALTH_CHECK_INTERVAL", "2s")
	viper.SetDefault("BACKENDS", "")
	viper.SetDefault("STRATEGY", "round-robin")
	viper.SetDefault("VIRTUAL_NODES", "100")

	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./config")
	viper.AddConfigPath(".")

	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			slog.Error("failed to read config file", slog.String("error", err.Error()))
			return nil, err
		}
		slog.Error("config file not found, using defaults and environment variables")
	} else {
		slog.Info("loaded config file", slog.String("file", viper.ConfigFileUsed()))
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		slog.Error("failed to unmarshal config", slog.String("error", err.Error()))
		return nil, err
	}

	if err := cfg.Validate(); err != nil {
		slog.Error("invalid configuration", slog.String("error", err.Error()))
		return nil, err
	}

	return &cfg, nil
}

func (c *Config) Validate() error {
	return validation.ValidateStruct(c,
		validation.Field(&c.Env,
			validation.Required,
			validation.In(EnvDev, EnvStaging, EnvProd),
		),
		validation.Field(&c.HTTPAddr,
			validation.Required,
			validation.By(validateHostPort),
		),
		validation.Field(&c.LogLevel,
			validation.Required,
			validation.In(LogLevelDebug, LogLevelInfo, LogLevelWarn, LogLevelError),
		),
		validation.Field(&c.HealthCheckInterval,
			validation.Required,
			validation.By(validateDuration),
		),
		validation.Field(&c.Backends,
			validation.Required,
			validation.Length(1, 0),
			validation.Each(validation.By(validateServerURL)),
		),
		validation.Field(&c.Strategy,
			validation.Required,
			validation.In("round-robin", "least-conn", "least-response", "random", "consistent_hash", "weighted-round-robin"),
		),
		validation.Field(&c.VirtualNodes,
			validation.Required,
			validation.Min(1),
		),
	)
}

func validateHostPort(value interface{}) error {
	addr, ok := value.(string)
	if !ok {
		return validation.NewError("validation_invalid_type", "must be a string")
	}

	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return validation.NewError("validation_invalid_hostport", "must be in host:port format")
	}

	if port == "" {
		return validation.NewError("validation_invalid_port", "port cannot be empty")
	}

	if host != "" {
		if err := is.Host.Validate(host); err != nil {
			return validation.NewError("validation_invalid_host", "invalid host")
		}
	}

	return nil
}

func validateDuration(value interface{}) error {
	durationStr, ok := value.(string)
	if !ok {
		return validation.NewError("validation_invalid_type", "must be a string")
	}

	if _, err := time.ParseDuration(durationStr); err != nil {
		return validation.NewError("validation_invalid_duration", "must be a valid duration (e.g., 2s, 5m, 1h)")
	}

	return nil
}

func validateServerURL(value interface{}) error {
	serverURL, ok := value.(string)
	if !ok {
		return validation.NewError("validation_invalid_type", "must be a string")
	}

	if serverURL == "" {
		return validation.NewError("validation_empty_url", "server URL cannot be empty")
	}

	parsedURL, err := url.Parse(serverURL)
	if err != nil {
		return validation.NewError("validation_invalid_url", "must be a valid URL")
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return validation.NewError("validation_invalid_scheme", "URL must use http or https scheme")
	}

	if parsedURL.Host == "" {
		return validation.NewError("validation_missing_host", "URL must have a host")
	}

	return nil
}

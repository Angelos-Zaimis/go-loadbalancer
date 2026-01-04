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

type ServerConfig struct {
	Address     string `mapstructure:"address"`
	Environment string `mapstructure:"environment"`
}

type HealthCheckConfig struct {
	Interval string `mapstructure:"interval"`
}

type StrategyConfig struct {
	Type         string `mapstructure:"type"`
	VirtualNodes int    `mapstructure:"virtual_nodes"`
}

type BackendConfig struct {
	URL    string `mapstructure:"url"`
	Weight int    `mapstructure:"weight"`
}

type LoggingConfig struct {
	Level string `mapstructure:"level"`
}

type Config struct {
	Server      ServerConfig      `mapstructure:"server"`
	HealthCheck HealthCheckConfig `mapstructure:"health_check"`
	Strategy    StrategyConfig    `mapstructure:"strategy"`
	Backends    []BackendConfig   `mapstructure:"backends"`
	Logging     LoggingConfig     `mapstructure:"logging"`
}

func Load() (*Config, error) {
	viper.SetDefault("server.environment", EnvDev)
	viper.SetDefault("server.address", ":8080")
	viper.SetDefault("health_check.interval", "2s")
	viper.SetDefault("strategy.type", "round-robin")
	viper.SetDefault("strategy.virtual_nodes", 100)
	viper.SetDefault("logging.level", LogLevelInfo)

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
		validation.Field(&c.Server,
			validation.Required,
			validation.By(func(value interface{}) error {
				sc, ok := value.(ServerConfig)
				if !ok {
					return validation.NewError("validation_invalid_type", "must be a ServerConfig")
				}
				return validation.ValidateStruct(&sc,
					validation.Field(&sc.Environment,
						validation.Required,
						validation.In(EnvDev, EnvStaging, EnvProd),
					),
					validation.Field(&sc.Address,
						validation.Required,
						validation.By(validateHostPort),
					),
				)
			}),
		),
		validation.Field(&c.Logging,
			validation.Required,
			validation.By(func(value interface{}) error {
				lc, ok := value.(LoggingConfig)
				if !ok {
					return validation.NewError("validation_invalid_type", "must be a LoggingConfig")
				}
				return validation.ValidateStruct(&lc,
					validation.Field(&lc.Level,
						validation.Required,
						validation.In(LogLevelDebug, LogLevelInfo, LogLevelWarn, LogLevelError),
					),
				)
			}),
		),
		validation.Field(&c.HealthCheck,
			validation.Required,
			validation.By(func(value interface{}) error {
				hc, ok := value.(HealthCheckConfig)
				if !ok {
					return validation.NewError("validation_invalid_type", "must be a HealthCheckConfig")
				}
				return validation.ValidateStruct(&hc,
					validation.Field(&hc.Interval,
						validation.Required,
						validation.By(validateDuration),
					),
				)
			}),
		),
		validation.Field(&c.Backends,
			validation.Required,
			validation.Length(1, 0),
			validation.Each(validation.By(validateBackendConfig)),
		),
		validation.Field(&c.Strategy,
			validation.Required,
			validation.By(func(value interface{}) error {
				sc, ok := value.(StrategyConfig)
				if !ok {
					return validation.NewError("validation_invalid_type", "must be a StrategyConfig")
				}
				return validation.ValidateStruct(&sc,
					validation.Field(&sc.Type,
						validation.Required,
						validation.In("round-robin", "least-conn", "least-response", "random", "consistent_hash", "weighted-round-robin"),
					),
					validation.Field(&sc.VirtualNodes,
						validation.Required,
						validation.Min(1),
					),
				)
			}),
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

func validateBackendConfig(value interface{}) error {
	backend, ok := value.(BackendConfig)
	if !ok {
		return validation.NewError("validation_invalid_type", "must be a BackendConfig")
	}

	if backend.URL == "" {
		return validation.NewError("validation_empty_url", "backend URL cannot be empty")
	}

	parsedURL, err := url.Parse(backend.URL)
	if err != nil {
		return validation.NewError("validation_invalid_url", "must be a valid URL")
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return validation.NewError("validation_invalid_scheme", "URL must use http or https scheme")
	}

	if parsedURL.Host == "" {
		return validation.NewError("validation_missing_host", "URL must have a host")
	}

	if backend.Weight < 1 {
		return validation.NewError("validation_invalid_weight", "weight must be at least 1")
	}

	return nil
}

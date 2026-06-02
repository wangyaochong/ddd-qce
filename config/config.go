package config

import (
	"context"
	"os"

	"github.com/pelletier/go-toml/v2"
)

// Config holds all application configuration sections.
type Config struct {
	App    AppConfig    `toml:"app"`
	Store  StoreConfig  `toml:"store"`
	Aspect AspectConfig `toml:"aspect"`
	Log    LogConfig    `toml:"logging"`
}

// AppConfig holds application-level settings.
type AppConfig struct {
	Name string `toml:"name"`
	Env  string `toml:"env"`
}

// StoreConfig holds data store settings.
type StoreConfig struct {
	Type string `toml:"type"`
	DSN  string `toml:"dsn"`
}

// AspectConfig controls which cross-cutting aspects are enabled.
type AspectConfig struct {
	EnableLogging     bool `toml:"enable-logging"`
	EnableTracing     bool `toml:"enable-tracing"`
	EnableMetrics     bool `toml:"enable-metrics"`
	EnableTransaction bool `toml:"enable-transaction"`
}

// LogConfig holds logging settings.
type LogConfig struct {
	Level string `toml:"level"`
}

// DefaultConfig returns a Config with sensible defaults for local development.
func DefaultConfig() *Config {
	return &Config{
		App: AppConfig{
			Name: "ddq-app",
			Env:  "development",
		},
		Store: StoreConfig{
			Type: "memory",
			DSN:  "",
		},
		Aspect: AspectConfig{
			EnableLogging:     true,
			EnableTracing:     true,
			EnableMetrics:     true,
			EnableTransaction: true,
		},
		Log: LogConfig{
			Level: "debug",
		},
	}
}

// ConfigLoader loads configuration from a TOML file with default fallbacks.
type ConfigLoader struct {
	config *Config
}

// NewConfigLoader creates a ConfigLoader initialized with default configuration.
func NewConfigLoader() *ConfigLoader {
	return &ConfigLoader{
		config: DefaultConfig(),
	}
}

// Load reads and parses a TOML configuration file, replacing the current config.
func (c *ConfigLoader) Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	c.config = &cfg
	return c.config, nil
}

// Get returns the current loaded configuration.
func (c *ConfigLoader) Get() *Config {
	return c.config
}

// ResolveStoreConfig reads store configuration from DDD_STORE_TYPE and DDD_POSTGRES_URI environment variables.
func ResolveStoreConfig() StoreConfig {
	return StoreConfig{
		Type: os.Getenv("DDD_STORE_TYPE"),
		DSN:  os.Getenv("DDD_POSTGRES_URI"),
	}
}

type configKey struct{}

// ContextWithConfig returns a context carrying the given configuration.
func ContextWithConfig(ctx context.Context, cfg *Config) context.Context {
	return context.WithValue(ctx, configKey{}, cfg)
}

// ConfigFromContext extracts the configuration from the context.
func ConfigFromContext(ctx context.Context) (*Config, bool) {
	cfg, ok := ctx.Value(configKey{}).(*Config)
	return cfg, ok
}

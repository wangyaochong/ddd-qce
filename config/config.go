package config

import (
	"context"
	"errors"
	"os"

	"github.com/pelletier/go-toml/v2"
)

var ErrInvalidConfig = errors.New("invalid configuration")

type Config struct {
	App    AppConfig    `toml:"app"`
	Store  StoreConfig  `toml:"store"`
	Aspect AspectConfig `toml:"aspect"`
	Log    LogConfig    `toml:"logging"`
}

type AppConfig struct {
	Name string `toml:"name"`
	Env  string `toml:"env"`
}

type StoreConfig struct {
	Type string `toml:"type"`
	DSN  string `toml:"dsn"`
}

type AspectConfig struct {
	EnableLogging     bool `toml:"enable-logging"`
	EnableTracing     bool `toml:"enable-tracing"`
	EnableMetrics     bool `toml:"enable-metrics"`
	EnableTransaction bool `toml:"enable-transaction"`
}

type LogConfig struct {
	Level string `toml:"level"`
}

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

type HandlerConfig struct {
	Commands map[string]string `toml:"commands"`
	Queries  map[string]string `toml:"queries"`
	Events   map[string]string `toml:"events"`
}

type ConfigLoader struct {
	config *Config
}

func NewConfigLoader() *ConfigLoader {
	return &ConfigLoader{
		config: DefaultConfig(),
	}
}

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

func (c *ConfigLoader) Get() *Config {
	return c.config
}

func ResolveStoreConfig() StoreConfig {
	return StoreConfig{
		Type: os.Getenv("DDD_STORE_TYPE"),
		DSN:  os.Getenv("DDD_POSTGRES_URI"),
	}
}

type configKey struct{}

func ContextWithConfig(ctx context.Context, cfg *Config) context.Context {
	return context.WithValue(ctx, configKey{}, cfg)
}

func ConfigFromContext(ctx context.Context) (*Config, bool) {
	cfg, ok := ctx.Value(configKey{}).(*Config)
	return cfg, ok
}
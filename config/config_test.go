package config

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.App.Name != "ddq-app" {
		t.Errorf("DefaultConfig().App.Name = %q, want %q", cfg.App.Name, "ddq-app")
	}
	if cfg.App.Env != "development" {
		t.Errorf("DefaultConfig().App.Env = %q, want %q", cfg.App.Env, "development")
	}
	if cfg.Store.Type != "memory" {
		t.Errorf("DefaultConfig().Store.Type = %q, want %q", cfg.Store.Type, "memory")
	}
	if !cfg.Aspect.EnableLogging {
		t.Error("DefaultConfig().Aspect.EnableLogging should be true")
	}
	if !cfg.Aspect.EnableTracing {
		t.Error("DefaultConfig().Aspect.EnableTracing should be true")
	}
	if !cfg.Aspect.EnableMetrics {
		t.Error("DefaultConfig().Aspect.EnableMetrics should be true")
	}
	if !cfg.Aspect.EnableTransaction {
		t.Error("DefaultConfig().Aspect.EnableTransaction should be true")
	}
	if cfg.Log.Level != "debug" {
		t.Errorf("DefaultConfig().Log.Level = %q, want %q", cfg.Log.Level, "debug")
	}
}

func TestNewConfigLoader(t *testing.T) {
	loader := NewConfigLoader()
	if loader == nil {
		t.Error("NewConfigLoader() returned nil")
	}
	if loader.config == nil {
		t.Error("NewConfigLoader().config is nil")
	}
}

func TestConfigLoader_Get(t *testing.T) {
	loader := NewConfigLoader()
	cfg := loader.Get()

	if cfg == nil {
		t.Error("Get() returned nil")
	}
	// Should return default config
	if cfg.App.Name != "ddq-app" {
		t.Errorf("Get().App.Name = %q, want %q", cfg.App.Name, "ddq-app")
	}
}

func TestConfigLoader_Load_FileNotFound(t *testing.T) {
	loader := NewConfigLoader()
	_, err := loader.Load("/nonexistent/path/config.toml")

	if err == nil {
		t.Error("Load() should return error for nonexistent file")
	}
}

func TestConfigLoader_Load_InvalidTOML(t *testing.T) {
	// Create temp file with invalid TOML
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "config.toml")

	if err := os.WriteFile(tmpFile, []byte("invalid toml [[["), 0644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	loader := NewConfigLoader()
	_, err := loader.Load(tmpFile)

	if err == nil {
		t.Error("Load() should return error for invalid TOML")
	}
}

func TestConfigLoader_Load_ValidTOML(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "config.toml")

	tomlContent := `
[app]
name = "test-app"
env = "production"

[store]
type = "postgres"
dsn = "postgres://localhost/db"

[aspect]
enable-logging = false
enable-tracing = false
enable-metrics = false
enable-transaction = false

[logging]
level = "error"
`
	if err := os.WriteFile(tmpFile, []byte(tomlContent), 0644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	loader := NewConfigLoader()
	cfg, err := loader.Load(tmpFile)

	if err != nil {
		t.Errorf("Load() error = %v, want nil", err)
	}

	if cfg.App.Name != "test-app" {
		t.Errorf("cfg.App.Name = %q, want %q", cfg.App.Name, "test-app")
	}
	if cfg.App.Env != "production" {
		t.Errorf("cfg.App.Env = %q, want %q", cfg.App.Env, "production")
	}
	if cfg.Store.Type != "postgres" {
		t.Errorf("cfg.Store.Type = %q, want %q", cfg.Store.Type, "postgres")
	}
	if cfg.Store.DSN != "postgres://localhost/db" {
		t.Errorf("cfg.Store.DSN = %q, want %q", cfg.Store.DSN, "postgres://localhost/db")
	}
	if cfg.Aspect.EnableLogging {
		t.Error("cfg.Aspect.EnableLogging should be false")
	}
	if cfg.Log.Level != "error" {
		t.Errorf("cfg.Log.Level = %q, want %q", cfg.Log.Level, "error")
	}
}

func TestConfigLoader_Load_OverridesDefaults(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "config.toml")

	tomlContent := `
[app]
name = "override-app"
`
	if err := os.WriteFile(tmpFile, []byte(tomlContent), 0644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	loader := NewConfigLoader()
	cfg, err := loader.Load(tmpFile)

	if err != nil {
		t.Errorf("Load() error = %v, want nil", err)
	}

	// Should override specified fields
	if cfg.App.Name != "override-app" {
		t.Errorf("cfg.App.Name = %q, want %q", cfg.App.Name, "override-app")
	}
}

func TestContextWithConfig(t *testing.T) {
	ctx := context.Background()
	cfg := &Config{
		App: AppConfig{Name: "test"},
	}

	newCtx := ContextWithConfig(ctx, cfg)
	if newCtx == nil {
		t.Error("ContextWithConfig() returned nil")
	}
}

func TestConfigFromContext(t *testing.T) {
	ctx := context.Background()
	cfg := &Config{
		App: AppConfig{Name: "test"},
	}

	// Empty context should return false
	_, ok := ConfigFromContext(ctx)
	if ok {
		t.Error("ConfigFromContext() should return false for empty context")
	}

	// Context with config
	ctx = ContextWithConfig(ctx, cfg)
	retrieved, ok := ConfigFromContext(ctx)
	if !ok {
		t.Error("ConfigFromContext() should return true for context with config")
	}
	if retrieved.App.Name != "test" {
		t.Errorf("retrieved.App.Name = %q, want %q", retrieved.App.Name, "test")
	}
}

func TestResolveStoreConfig_EnvVars(t *testing.T) {
	os.Setenv("DDD_STORE_TYPE", "postgres")
	os.Setenv("DDD_POSTGRES_URI", "postgres://localhost/db")
	t.Cleanup(func() {
		os.Unsetenv("DDD_STORE_TYPE")
		os.Unsetenv("DDD_POSTGRES_URI")
	})

	cfg := ResolveStoreConfig()
	if cfg.Type != "postgres" {
		t.Errorf("Type = %q, want %q", cfg.Type, "postgres")
	}
	if cfg.DSN != "postgres://localhost/db" {
		t.Errorf("DSN = %q, want %q", cfg.DSN, "postgres://localhost/db")
	}
}

func TestResolveStoreConfig_EmptyEnvVars(t *testing.T) {
	os.Unsetenv("DDD_STORE_TYPE")
	os.Unsetenv("DDD_POSTGRES_URI")

	cfg := ResolveStoreConfig()
	if cfg.Type != "" {
		t.Errorf("Type = %q, want empty", cfg.Type)
	}
	if cfg.DSN != "" {
		t.Errorf("DSN = %q, want empty", cfg.DSN)
	}
}

func TestConfigFromContext_NilConfig(t *testing.T) {
	ctx := context.Background()
	// Context with nil config value
	ctx = context.WithValue(ctx, configKey{}, nil)

	cfg, ok := ConfigFromContext(ctx)
	if ok {
		t.Error("ConfigFromContext() should return false for nil config in context")
	}
	if cfg != nil {
		t.Error("ConfigFromContext() should return nil config")
	}
}

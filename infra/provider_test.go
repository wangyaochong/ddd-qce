package infra

import (
	"testing"

	"github.com/ddd-qce/core/config"
)

func TestNewBackendFromConfig_Memory(t *testing.T) {
	b, err := NewBackendFromConfig(config.StoreConfig{Type: "memory"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if b.TransactionManager == nil {
		t.Error("expected TransactionManager to be set")
	}
	if b.JobStore == nil {
		t.Error("expected JobStore to be set")
	}
	if err := b.Close(); err != nil {
		t.Errorf("memory backend Close() should be nil, got %v", err)
	}
}

func TestNewBackendFromConfig_EmptyType(t *testing.T) {
	b, err := NewBackendFromConfig(config.StoreConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if b.TransactionManager == nil {
		t.Error("expected TransactionManager to be set")
	}
}

func TestNewBackendFromConfig_Postgres_MissingDSN(t *testing.T) {
	_, err := NewBackendFromConfig(config.StoreConfig{Type: "postgres"})
	if err == nil {
		t.Fatal("expected error when DSN is empty")
	}
	if err.Error() != "postgres DSN is required when DDD_STORE_TYPE=postgres" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestNewBackendFromConfig_Postgres_InvalidDSN(t *testing.T) {
	_, err := NewBackendFromConfig(config.StoreConfig{
		Type: "postgres",
		DSN:  "postgres://nonexistent:5432/invalid",
	})
	if err == nil {
		t.Fatal("expected error for invalid DSN")
	}
}

func TestNewBackendFromConfig_UnsupportedType(t *testing.T) {
	_, err := NewBackendFromConfig(config.StoreConfig{Type: "redis"})
	if err == nil {
		t.Fatal("expected error for unsupported type")
	}
	if err.Error() != `unsupported DDD_STORE_TYPE "redis": must be "memory" or "postgres"` {
		t.Errorf("unexpected error message: %v", err)
	}
}

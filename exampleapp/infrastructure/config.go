package infrastructure

import (
	"os"
)

const (
	StoreTypeMemory     = "memory"
	StoreTypePostgreSQL = "postgresql"
)

type Config struct {
	StoreType    string
	PostgresURI  string
}

func LoadConfig() *Config {
	return &Config{
		StoreType:   envOr("DDD_STORE_TYPE", StoreTypePostgreSQL),
		PostgresURI: os.Getenv("DDD_POSTGRES_URI"),
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

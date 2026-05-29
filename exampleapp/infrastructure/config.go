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
		PostgresURI: envOr("DDD_POSTGRES_URI", "postgres://wyc:wyc@localhost:5432/ddd_qce"),
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

package infra

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/ddd-qce/core/config"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func NewBackendFromConfig(cfg config.StoreConfig, opts ...BackendOption) (*Backend, error) {
	switch cfg.Type {
	case "memory", "":
		return NewMemoryBackend(opts...), nil
	case "postgres":
		if cfg.DSN == "" {
			return nil, errors.New("postgres DSN is required when DDD_STORE_TYPE=postgres")
		}
		db, err := sql.Open("pgx", cfg.DSN)
		if err != nil {
			return nil, fmt.Errorf("open postgres connection: %w", err)
		}
		if err := db.Ping(); err != nil {
			db.Close()
			return nil, fmt.Errorf("ping postgres: %w", err)
		}
		return NewPgBackend(db, opts...), nil
	default:
		return nil, fmt.Errorf("unsupported DDD_STORE_TYPE %q: must be \"memory\" or \"postgres\"", cfg.Type)
	}
}

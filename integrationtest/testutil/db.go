package testutil

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	corepg "github.com/ddd-qce/core/pg"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	_ "github.com/jackc/pgx/v5/stdlib"
)

var (
	containerOnce sync.Once
	containerDSN  string
	containerErr  error
)

func startPostgresContainer(ctx context.Context) (string, error) {
	containerOnce.Do(func() {
		c, err := postgres.Run(ctx,
			"postgres:17-alpine",
			postgres.WithDatabase("ddd_qce_test"),
			postgres.WithUsername("ddd"),
			postgres.WithPassword("ddd"),
			testcontainers.WithWaitStrategy(
				wait.ForAll(
					wait.ForListeningPort("5432/tcp"),
					wait.ForLog("database system is ready to accept connections").WithOccurrence(2),
				).WithStartupTimeout(30*time.Second),
			),
		)
		if err != nil {
			containerErr = fmt.Errorf("start postgres container: %w", err)
			return
		}
		dsn, err := c.ConnectionString(ctx, "sslmode=disable")
		if err != nil {
			containerErr = fmt.Errorf("get container connection string: %w", err)
			return
		}
		containerDSN = dsn
	})
	return containerDSN, containerErr
}

func OpenTestDB(t *testing.T) *sql.DB {
	t.Helper()
	ctx := context.Background()

	dsn := os.Getenv("TEST_PG_DSN")
	if dsn == "" {
		var err error
		dsn, err = startPostgresContainer(ctx)
		if err != nil {
			t.Fatalf("start postgres container: %v", err)
		}
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open db failed: %v", err)
	}
	if err := db.Ping(); err != nil {
		t.Fatalf("ping db failed: %v", err)
	}
	t.Cleanup(func() {
		corepg.DropAll(db)
		db.Close()
	})
	if err := corepg.Migrate(db); err != nil {
		t.Fatalf("migrate failed: %v", err)
	}
	return db
}

func CleanDB(t *testing.T, db *sql.DB) {
	t.Helper()
	if err := corepg.TruncateAll(db); err != nil {
		t.Fatalf("truncate tables: %v", err)
	}
}

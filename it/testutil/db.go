package testutil

import (
	"database/sql"
	"os"
	"testing"

	corepg "github.com/ddd-qce/core/pg"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func OpenTestDB(t *testing.T, dbname string) *sql.DB {
	t.Helper()
	dsn := os.Getenv("TEST_PG_DSN")
	if dsn == "" {
		dsn = "host=/var/run/postgresql dbname=" + dbname + " user=" + os.Getenv("USER") + " sslmode=disable"
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

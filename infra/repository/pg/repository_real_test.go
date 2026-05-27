package pg

import (
	"database/sql"
	"os"
	"testing"

	"github.com/ddd-qce/core/pg"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func openTestDBForRepo(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("TEST_PG_DSN")
	if dsn == "" {
		dsn = "host=/var/run/postgresql dbname=test_repo user=" + os.Getenv("USER") + " sslmode=disable"
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open db failed: %v", err)
	}
	if err := db.Ping(); err != nil {
		t.Fatalf("ping db failed: %v", err)
	}
	return db
}

func TestPgRepository_RealDB_MigrateAndDropAll(t *testing.T) {
	if os.Getenv("RUN_REAL_DB_TESTS") != "1" {
		t.Skip("Set RUN_REAL_DB_TESTS=1 to run real DB tests")
	}

	db := openTestDBForRepo(t)
	defer db.Close()

	err := pg.Migrate(db)
	if err != nil {
		t.Fatalf("Migrate failed: %v", err)
	}

	err = pg.DropAll(db)
	if err != nil {
		t.Fatalf("DropAll failed: %v", err)
	}
}
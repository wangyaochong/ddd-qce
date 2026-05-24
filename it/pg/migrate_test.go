package pg

import (
	"database/sql"
	"testing"

	corepg "github.com/ddd-qce/core/pg"
	"github.com/ddd-qce/it/testutil"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestMigrate(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	var count int
	err := db.QueryRow("SELECT count(*) FROM information_schema.tables WHERE table_schema = 'public' AND table_name LIKE 'ddd_%'").Scan(&count)
	if err != nil {
		t.Fatalf("count tables failed: %v", err)
	}
	if count != 9 {
		t.Errorf("expected 9 ddd_ tables, got %d", count)
	}
}

func TestDropAll(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	if err := corepg.DropAll(db); err != nil {
		t.Fatalf("drop all failed: %v", err)
	}

	var count int
	db.QueryRow("SELECT count(*) FROM information_schema.tables WHERE table_schema = 'public' AND table_name LIKE 'ddd_%'").Scan(&count)
	if count != 0 {
		t.Errorf("expected 0 tables after drop, got %d", count)
	}

	if err := corepg.Migrate(db); err != nil {
		t.Fatalf("re-migrate failed: %v", err)
	}
}

func openTestDB(t *testing.T) *sql.DB {
	return testutil.OpenTestDB(t, "ddd_qce_test")
}

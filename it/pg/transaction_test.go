package pg

import (
	"context"
	"database/sql"
	"os"
	"testing"

	corepg "github.com/ddd-qce/core/pg"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func openTestDBForTx(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("TEST_PG_DSN")
	if dsn == "" {
		dsn = "host=/var/run/postgresql dbname=ddd_qce_tx_test user=root password=root sslmode=disable"
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

func TestPgTransactionManager_SimpleBeginCommit(t *testing.T) {
	db := openTestDBForTx(t)
	m := corepg.NewTransactionManager(db)
	ctx := context.Background()

	txCtx, err := m.Begin(ctx)
	if err != nil {
		t.Fatalf("Begin failed: %v", err)
	}

	q := corepg.GetQuerier(txCtx, db)
	_, err = q.ExecContext(txCtx, `INSERT INTO ddd_jobs (id, command, command_type, status, created_at, timeout_ns, retry_count, max_retries) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		"tx-test-1", "{}", "test", "pending", "now", 0, 0, 0)
	if err != nil {
		t.Fatalf("insert failed: %v", err)
	}

	if err := m.Commit(txCtx); err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	var count int
	db.QueryRow("SELECT count(*) FROM ddd_jobs WHERE id = $1", "tx-test-1").Scan(&count)
	if count != 1 {
		t.Errorf("expected 1 row after commit, got %d", count)
	}
}

func TestPgTransactionManager_SimpleRollback(t *testing.T) {
	db := openTestDBForTx(t)
	m := corepg.NewTransactionManager(db)
	ctx := context.Background()

	txCtx, err := m.Begin(ctx)
	if err != nil {
		t.Fatalf("Begin failed: %v", err)
	}

	q := corepg.GetQuerier(txCtx, db)
	q.ExecContext(txCtx, `INSERT INTO ddd_jobs (id, command, command_type, status, created_at, timeout_ns, retry_count, max_retries) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		"tx-test-rb", "{}", "test", "pending", "now", 0, 0, 0)

	if err := m.Rollback(txCtx); err != nil {
		t.Fatalf("Rollback failed: %v", err)
	}

	var count int
	db.QueryRow("SELECT count(*) FROM ddd_jobs WHERE id = $1", "tx-test-rb").Scan(&count)
	if count != 0 {
		t.Errorf("expected 0 rows after rollback, got %d", count)
	}
}

func TestPgTransactionManager_NestedBeginCommit(t *testing.T) {
	db := openTestDBForTx(t)
	m := corepg.NewTransactionManager(db)
	ctx := context.Background()

	txCtx, err := m.Begin(ctx)
	if err != nil {
		t.Fatalf("outer Begin failed: %v", err)
	}

	innerCtx, err := m.Begin(txCtx)
	if err != nil {
		t.Fatalf("inner Begin failed: %v", err)
	}
	if innerCtx != txCtx {
		t.Error("nested Begin should return same context")
	}

	q := corepg.GetQuerier(txCtx, db)
	q.ExecContext(txCtx, `INSERT INTO ddd_jobs (id, command, command_type, status, created_at, timeout_ns, retry_count, max_retries) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		"tx-nested-1", "{}", "test", "pending", "now", 0, 0, 0)

	if err := m.Commit(innerCtx); err != nil {
		t.Fatalf("inner Commit failed: %v", err)
	}

	var count int
	db.QueryRow("SELECT count(*) FROM ddd_jobs WHERE id = $1", "tx-nested-1").Scan(&count)
	if count != 0 {
		t.Error("expected 0 rows before outer commit (nested still in tx)")
	}

	if err := m.Commit(txCtx); err != nil {
		t.Fatalf("outer Commit failed: %v", err)
	}

	db.QueryRow("SELECT count(*) FROM ddd_jobs WHERE id = $1", "tx-nested-1").Scan(&count)
	if count != 1 {
		t.Errorf("expected 1 row after outer commit, got %d", count)
	}
}

func TestPgTransactionManager_NestedRollbackAbortsOuter(t *testing.T) {
	db := openTestDBForTx(t)
	m := corepg.NewTransactionManager(db)
	ctx := context.Background()

	txCtx, err := m.Begin(ctx)
	if err != nil {
		t.Fatalf("outer Begin failed: %v", err)
	}

	innerCtx, err := m.Begin(txCtx)
	if err != nil {
		t.Fatalf("inner Begin failed: %v", err)
	}

	q := corepg.GetQuerier(txCtx, db)
	q.ExecContext(txCtx, `INSERT INTO ddd_jobs (id, command, command_type, status, created_at, timeout_ns, retry_count, max_retries) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		"tx-abort-1", "{}", "test", "pending", "now", 0, 0, 0)

	if err := m.Rollback(innerCtx); err != nil {
		t.Fatalf("inner Rollback failed: %v", err)
	}

	err = m.Commit(txCtx)
	if err == nil {
		t.Fatal("expected outer Commit to fail after inner Rollback")
	}

	var count int
	db.QueryRow("SELECT count(*) FROM ddd_jobs WHERE id = $1", "tx-abort-1").Scan(&count)
	if count != 0 {
		t.Errorf("expected 0 rows after aborted transaction, got %d", count)
	}
}

func TestPgTransactionManager_NoTransaction(t *testing.T) {
	db := openTestDBForTx(t)
	m := corepg.NewTransactionManager(db)
	ctx := context.Background()

	if err := m.Commit(ctx); err == nil {
		t.Error("expected error when Commit without transaction")
	}
	if err := m.Rollback(ctx); err == nil {
		t.Error("expected error when Rollback without transaction")
	}
}

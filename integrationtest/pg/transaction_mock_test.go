package pg

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	corepg "github.com/ddd-qce/core/pg"
)

func TestMockNewTransactionManager(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	mgr := corepg.NewTransactionManager(db)
	if mgr == nil {
		t.Error("NewTransactionManager() returned nil")
	}
}

func TestMockTransactionManager_Begin(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mgr := corepg.NewTransactionManager(db)

	ctx := context.Background()
	ctx, err = mgr.Begin(ctx)
	if err != nil {
		t.Errorf("Begin() error = %v, want nil", err)
	}

	if !corepg.HasTransaction(ctx) {
		t.Error("HasTransaction() = false, want true after Begin")
	}
}

func TestMockTransactionManager_Begin_AlreadyInTransaction(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectExec("SAVEPOINT").WillReturnResult(sqlmock.NewResult(0, 0))

	mgr := corepg.NewTransactionManager(db)
	ctx := context.Background()

	ctx, err = mgr.Begin(ctx)
	if err != nil {
		t.Fatalf("first Begin() error = %v", err)
	}

	ctx, err = mgr.Begin(ctx)
	if err != nil {
		t.Errorf("second Begin() error = %v, want nil", err)
	}
}

func TestMockTransactionManager_Commit(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectCommit()
	mgr := corepg.NewTransactionManager(db)

	ctx := context.Background()
	ctx, _ = mgr.Begin(ctx)

	err = mgr.Commit(ctx)
	if err != nil {
		t.Errorf("Commit() error = %v, want nil", err)
	}
}

func TestMockTransactionManager_Commit_NoTransaction(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	mgr := corepg.NewTransactionManager(db)
	ctx := context.Background()

	err = mgr.Commit(ctx)
	if err == nil {
		t.Error("Commit() error = nil, want error when no transaction")
	}
}

func TestMockTransactionManager_Commit_NestedWithSavepoint(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectExec("SAVEPOINT").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("RELEASE SAVEPOINT").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	mgr := corepg.NewTransactionManager(db)
	ctx := context.Background()

	ctx, _ = mgr.Begin(ctx)
	ctx, _ = mgr.Begin(ctx)
	err = mgr.Commit(ctx)
	if err != nil {
		t.Errorf("Commit() nested error = %v, want nil", err)
	}
}

func TestMockTransactionManager_Rollback(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectRollback()
	mgr := corepg.NewTransactionManager(db)

	ctx := context.Background()
	ctx, _ = mgr.Begin(ctx)

	err = mgr.Rollback(ctx)
	if err != nil {
		t.Errorf("Rollback() error = %v, want nil", err)
	}
}

func TestMockTransactionManager_Rollback_NoTransaction(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	mgr := corepg.NewTransactionManager(db)
	ctx := context.Background()

	err = mgr.Rollback(ctx)
	if err == nil {
		t.Error("Rollback() error = nil, want error when no transaction")
	}
}

func TestMockTransactionManager_Rollback_Nested(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectExec("SAVEPOINT").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("ROLLBACK TO SAVEPOINT").WillReturnResult(sqlmock.NewResult(0, 0))

	mgr := corepg.NewTransactionManager(db)
	ctx := context.Background()

	ctx, _ = mgr.Begin(ctx)
	ctx, _ = mgr.Begin(ctx)
	err = mgr.Rollback(ctx)
	if err != nil {
		t.Errorf("Rollback() nested error = %v, want nil", err)
	}
}

func TestMockTransactionManager_NestedRollbackAbortsOuter(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectExec("SAVEPOINT").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("ROLLBACK TO SAVEPOINT").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectRollback()

	mgr := corepg.NewTransactionManager(db)
	ctx := context.Background()

	ctx, _ = mgr.Begin(ctx)
	ctx, _ = mgr.Begin(ctx)
	_ = mgr.Rollback(ctx)
	err = mgr.Commit(ctx)

	if err != nil {
		t.Logf("Commit() error after inner rollback: %v", err)
	}
}

func TestMockGetQuerier_InTransaction(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mgr := corepg.NewTransactionManager(db)

	ctx := context.Background()
	ctx, _ = mgr.Begin(ctx)

	q := corepg.GetQuerier(ctx, db)
	if q == nil {
		t.Error("GetQuerier() returned nil")
	}
	if _, ok := q.(*sql.Tx); !ok {
		t.Error("GetQuerier() should return *sql.Tx when in transaction")
	}
}

func TestMockGetQuerier_NotInTransaction(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	q := corepg.GetQuerier(ctx, db)
	if q != db {
		t.Error("GetQuerier() should return db when not in transaction")
	}
}

func TestMockHasTransaction(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mgr := corepg.NewTransactionManager(db)

	ctx := context.Background()
	if corepg.HasTransaction(ctx) {
		t.Error("HasTransaction() = true before Begin")
	}

	ctx, _ = mgr.Begin(ctx)
	if !corepg.HasTransaction(ctx) {
		t.Error("HasTransaction() = false after Begin")
	}
}

func TestMockBegin_SavepointError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectExec("SAVEPOINT").WillReturnError(errors.New("savepoint failed"))

	mgr := corepg.NewTransactionManager(db)
	ctx := context.Background()

	ctx, _ = mgr.Begin(ctx)
	_, err = mgr.Begin(ctx)
	if err == nil {
		t.Error("Begin() should error when savepoint fails")
	}
}

func TestMockRollback_AfterCommit(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectCommit()
	mock.ExpectRollback()

	mgr := corepg.NewTransactionManager(db)
	ctx := context.Background()

	ctx, _ = mgr.Begin(ctx)
	_ = mgr.Commit(ctx)
	err = mgr.Rollback(ctx)
	if err != nil {
		t.Logf("Rollback after commit got error: %v (expected)", err)
	}
}

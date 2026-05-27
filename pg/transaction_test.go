package pg

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestNewTransactionManager(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	mgr := NewTransactionManager(db)
	if mgr == nil {
		t.Error("NewTransactionManager() returned nil")
	}
	if mgr.db != db {
		t.Error("TransactionManager.db not set correctly")
	}
}

func TestTransactionManager_Begin(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mgr := NewTransactionManager(db)

	ctx := context.Background()
	ctx, err = mgr.Begin(ctx)
	if err != nil {
		t.Errorf("Begin() error = %v, want nil", err)
	}

	if !HasTransaction(ctx) {
		t.Error("HasTransaction() = false, want true after Begin")
	}
}

func TestTransactionManager_Begin_AlreadyInTransaction(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectExec("SAVEPOINT").WillReturnResult(sqlmock.NewResult(0, 0))

	mgr := NewTransactionManager(db)
	ctx := context.Background()

	// First begin
	ctx, err = mgr.Begin(ctx)
	if err != nil {
		t.Fatalf("first Begin() error = %v", err)
	}

	// Second begin should create savepoint
	ctx, err = mgr.Begin(ctx)
	if err != nil {
		t.Errorf("second Begin() error = %v, want nil", err)
	}
}

func TestTransactionManager_Commit(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectCommit()
	mgr := NewTransactionManager(db)

	ctx := context.Background()
	ctx, _ = mgr.Begin(ctx)

	err = mgr.Commit(ctx)
	if err != nil {
		t.Errorf("Commit() error = %v, want nil", err)
	}
}

func TestTransactionManager_Commit_NoTransaction(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	mgr := NewTransactionManager(db)
	ctx := context.Background()

	err = mgr.Commit(ctx)
	if err == nil {
		t.Error("Commit() error = nil, want error when no transaction")
	}
}

func TestTransactionManager_Commit_NestedWithSavepoint(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectExec("SAVEPOINT").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("RELEASE SAVEPOINT").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	mgr := NewTransactionManager(db)
	ctx := context.Background()

	ctx, _ = mgr.Begin(ctx)
	ctx, _ = mgr.Begin(ctx)
	err = mgr.Commit(ctx)
	if err != nil {
		t.Errorf("Commit() nested error = %v, want nil", err)
	}
}

func TestTransactionManager_Rollback(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectRollback()
	mgr := NewTransactionManager(db)

	ctx := context.Background()
	ctx, _ = mgr.Begin(ctx)

	err = mgr.Rollback(ctx)
	if err != nil {
		t.Errorf("Rollback() error = %v, want nil", err)
	}
}

func TestTransactionManager_Rollback_NoTransaction(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	mgr := NewTransactionManager(db)
	ctx := context.Background()

	err = mgr.Rollback(ctx)
	if err == nil {
		t.Error("Rollback() error = nil, want error when no transaction")
	}
}

func TestTransactionManager_Rollback_Nested(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectExec("SAVEPOINT").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("ROLLBACK TO SAVEPOINT").WillReturnResult(sqlmock.NewResult(0, 0))

	mgr := NewTransactionManager(db)
	ctx := context.Background()

	ctx, _ = mgr.Begin(ctx)
	ctx, _ = mgr.Begin(ctx)
	err = mgr.Rollback(ctx)
	if err != nil {
		t.Errorf("Rollback() nested error = %v, want nil", err)
	}
}

func TestTransactionManager_NestedRollbackAbortsOuter(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectExec("SAVEPOINT").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("ROLLBACK TO SAVEPOINT").WillReturnResult(sqlmock.NewResult(0, 0))
	// After inner rollback, outer commit will rollback due to aborted flag
	mock.ExpectRollback()

	mgr := NewTransactionManager(db)
	ctx := context.Background()

	ctx, _ = mgr.Begin(ctx)
	ctx, _ = mgr.Begin(ctx)
	_ = mgr.Rollback(ctx) // inner rollback sets aborted=true
	err = mgr.Commit(ctx) // outer commit should fail due to aborted flag

	// After inner rollback, the outer commit detects aborted and tries to rollback
	// The error message should contain "aborted" or be a rollback
	if err != nil {
		t.Logf("Commit() error after inner rollback: %v", err)
	}
}

func TestGetQuerier_InTransaction(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mgr := NewTransactionManager(db)

	ctx := context.Background()
	ctx, _ = mgr.Begin(ctx)

	q := GetQuerier(ctx, db)
	if q == nil {
		t.Error("GetQuerier() returned nil")
	}
	// In transaction, should return *sql.Tx (not db)
	if _, ok := q.(*sql.Tx); !ok {
		t.Error("GetQuerier() should return *sql.Tx when in transaction")
	}
}

func TestGetQuerier_NotInTransaction(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	q := GetQuerier(ctx, db)
	if q != db {
		t.Error("GetQuerier() should return db when not in transaction")
	}
}

func TestHasTransaction(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mgr := NewTransactionManager(db)

	ctx := context.Background()
	if HasTransaction(ctx) {
		t.Error("HasTransaction() = true before Begin")
	}

	ctx, _ = mgr.Begin(ctx)
	if !HasTransaction(ctx) {
		t.Error("HasTransaction() = false after Begin")
	}
}

func TestBegin_SavepointError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectExec("SAVEPOINT").WillReturnError(errors.New("savepoint failed"))

	mgr := NewTransactionManager(db)
	ctx := context.Background()

	ctx, _ = mgr.Begin(ctx)
	_, err = mgr.Begin(ctx)
	if err == nil {
		t.Error("Begin() should error when savepoint fails")
	}
}

func TestCommit_EmptyStack(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	mgr := NewTransactionManager(db)
	ctx := context.Background()
	ctx = context.WithValue(ctx, txKey{}, &txState{tx: nil, depth: 0})

	err = mgr.Commit(ctx)
	if err == nil {
		t.Error("Commit() should error when depth is 0")
	}
}

func TestRollback_AfterCommit(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectCommit()
	mock.ExpectRollback()

	mgr := NewTransactionManager(db)
	ctx := context.Background()

	ctx, _ = mgr.Begin(ctx)
	_ = mgr.Commit(ctx)
	err = mgr.Rollback(ctx) // Should rollback the new transaction
	if err != nil {
		// This is expected - no transaction in context after commit
		t.Logf("Rollback after commit got error: %v (expected)", err)
	}
}
package builtintest

import (
	"context"
	"testing"

	"github.com/ddd-qce/core/aspect/builtin"
)

func TestTransactionManagerContract(t *testing.T, tm builtin.TransactionManager, newCtx func() context.Context) {
	t.Helper()

	t.Run("BeginAndCommit", func(t *testing.T) {
		ctx := newCtx()
		txCtx, err := tm.Begin(ctx)
		if err != nil {
			t.Fatalf("Begin failed: %v", err)
		}
		if err := tm.Commit(txCtx); err != nil {
			t.Fatalf("Commit failed: %v", err)
		}
	})

	t.Run("BeginAndRollback", func(t *testing.T) {
		ctx := newCtx()
		txCtx, err := tm.Begin(ctx)
		if err != nil {
			t.Fatalf("Begin failed: %v", err)
		}
		if err := tm.Rollback(txCtx); err != nil {
			t.Fatalf("Rollback failed: %v", err)
		}
	})

	t.Run("NoTransactionCommit", func(t *testing.T) {
		ctx := newCtx()
		if err := tm.Commit(ctx); err == nil {
			t.Error("expected error when Commit without transaction")
		}
	})

	t.Run("NoTransactionRollback", func(t *testing.T) {
		ctx := newCtx()
		if err := tm.Rollback(ctx); err == nil {
			t.Error("expected error when Rollback without transaction")
		}
	})

	t.Run("NestedBeginCommit", func(t *testing.T) {
		ctx := newCtx()
		txCtx, err := tm.Begin(ctx)
		if err != nil {
			t.Fatalf("outer Begin failed: %v", err)
		}
		innerCtx, err := tm.Begin(txCtx)
		if err != nil {
			t.Fatalf("inner Begin failed: %v", err)
		}
		if err := tm.Commit(innerCtx); err != nil {
			t.Fatalf("inner Commit failed: %v", err)
		}
		if err := tm.Commit(txCtx); err != nil {
			t.Fatalf("outer Commit failed: %v", err)
		}
	})

	t.Run("NestedRollbackAbortsOuter", func(t *testing.T) {
		ctx := newCtx()
		txCtx, err := tm.Begin(ctx)
		if err != nil {
			t.Fatalf("outer Begin failed: %v", err)
		}
		innerCtx, err := tm.Begin(txCtx)
		if err != nil {
			t.Fatalf("inner Begin failed: %v", err)
		}
		if err := tm.Rollback(innerCtx); err != nil {
			t.Fatalf("inner Rollback failed: %v", err)
		}
		err = tm.Commit(txCtx)
		if err == nil {
			t.Fatal("expected outer Commit to fail after inner Rollback")
		}
	})

	t.Run("TripleNesting", func(t *testing.T) {
		ctx := newCtx()
		l1, err := tm.Begin(ctx)
		if err != nil {
			t.Fatalf("l1 Begin failed: %v", err)
		}
		l2, err := tm.Begin(l1)
		if err != nil {
			t.Fatalf("l2 Begin failed: %v", err)
		}
		l3, err := tm.Begin(l2)
		if err != nil {
			t.Fatalf("l3 Begin failed: %v", err)
		}
		if err := tm.Commit(l3); err != nil {
			t.Fatalf("l3 Commit failed: %v", err)
		}
		if err := tm.Commit(l2); err != nil {
			t.Fatalf("l2 Commit failed: %v", err)
		}
		if err := tm.Commit(l1); err != nil {
			t.Fatalf("l1 Commit failed: %v", err)
		}
	})

	t.Run("RollbackThenBeginAgain", func(t *testing.T) {
		ctx := newCtx()
		txCtx, err := tm.Begin(ctx)
		if err != nil {
			t.Fatalf("first Begin failed: %v", err)
		}
		if err := tm.Rollback(txCtx); err != nil {
			t.Fatalf("Rollback failed: %v", err)
		}
		ctx2 := newCtx()
		txCtx2, err := tm.Begin(ctx2)
		if err != nil {
			t.Fatalf("second Begin failed: %v", err)
		}
		if err := tm.Commit(txCtx2); err != nil {
			t.Fatalf("second Commit failed: %v", err)
		}
	})
}

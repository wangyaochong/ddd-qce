package repository

import (
	"errors"
	"testing"

	ddderror "github.com/ddd-qce/core/error"
)

func TestOptimisticLockError(t *testing.T) {
	err := &OptimisticLockError{
		AggregateID:     "agg-123",
		ExpectedVersion: 5,
	}

	if err.Error() != "optimistic lock error: aggregate agg-123 version 5 was already updated by another transaction" {
		t.Errorf("Error() = %q, want expected message", err.Error())
	}

	unwrap := err.Unwrap()
	if !errors.Is(unwrap, ddderror.ErrConcurrency) {
		t.Error("Unwrap() should return ErrConcurrency")
	}
}

func TestOptimisticLockError_ZeroValues(t *testing.T) {
	err := &OptimisticLockError{}

	msg := err.Error()
	if msg == "" {
		t.Error("Error() should not be empty")
	}
	// Should not panic
	_ = msg
}
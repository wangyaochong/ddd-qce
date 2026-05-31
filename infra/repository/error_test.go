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
	_ = msg
}

func TestOptimisticLockError_VersionMismatch(t *testing.T) {
	err := &OptimisticLockError{
		AggregateID:     "agg-456",
		ExpectedVersion: 3,
		VersionMismatch: true,
	}

	msg := err.Error()
	if msg != "optimistic lock error: aggregate agg-456 version 3 conflicts with existing version" {
		t.Errorf("Error() = %q, want version mismatch message", msg)
	}
}

func TestOptimisticLockError_Unwrap(t *testing.T) {
	err := &OptimisticLockError{
		AggregateID:     "agg-789",
		ExpectedVersion: 1,
	}

	if !errors.Is(err, ddderror.ErrConcurrency) {
		t.Error("errors.Is(err, ErrConcurrency) should be true")
	}
}

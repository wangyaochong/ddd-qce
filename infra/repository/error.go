package repository

import (
	"fmt"

	ddderror "github.com/ddd-qce/core/error"
)

type OptimisticLockError struct {
	AggregateID     string
	ExpectedVersion int
	VersionMismatch bool
}

func (e *OptimisticLockError) Error() string {
	if e.VersionMismatch {
		return fmt.Sprintf("optimistic lock error: aggregate %s version %d conflicts with existing version", e.AggregateID, e.ExpectedVersion)
	}
	return fmt.Sprintf("optimistic lock error: aggregate %s version %d was already updated by another transaction", e.AggregateID, e.ExpectedVersion)
}

func (e *OptimisticLockError) Unwrap() error {
	return ddderror.ErrConcurrency
}

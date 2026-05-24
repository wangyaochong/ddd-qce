package ddderror

import "errors"

var (
	ErrNotFound         = NewDomainError("NOT_FOUND", "resource not found")
	ErrAlreadyExists    = NewDomainError("ALREADY_EXISTS", "resource already exists")
	ErrInvalidState     = NewDomainError("INVALID_STATE", "invalid state for operation")
	ErrConcurrency      = NewDomainError("CONCURRENCY", "concurrent modification conflict")
	ErrPermissionDenied = NewDomainError("PERMISSION_DENIED", "permission denied")

	ErrJobNotFound = errors.New("job not found")
)

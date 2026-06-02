package ddderror

var (
	// ErrNotFound is returned when a requested resource does not exist.
	ErrNotFound = NewDomainError("NOT_FOUND", "resource not found")
	// ErrAlreadyExists is returned when attempting to create a resource that already exists.
	ErrAlreadyExists = NewDomainError("ALREADY_EXISTS", "resource already exists")
	// ErrInvalidState is returned when an operation is not valid for the current state.
	ErrInvalidState = NewDomainError("INVALID_STATE", "invalid state for operation")
	// ErrConcurrency is returned on optimistic concurrency conflicts.
	ErrConcurrency = NewDomainError("CONCURRENCY", "concurrent modification conflict")
	// ErrPermissionDenied is returned when the caller lacks required permissions.
	ErrPermissionDenied = NewDomainError("PERMISSION_DENIED", "permission denied")
)

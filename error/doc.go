// Package error provides DDD error types: DomainError with code/message/cause
// chaining, sentinel errors (ErrNotFound, ErrAlreadyExists, etc.), and
// MultiError for aggregating multiple errors.
//
// Use NewDomainError(code, msg) for business errors that carry a semantic
// code. Use NewMultiError(errs...) to combine errors from fan-out operations.
package ddderror

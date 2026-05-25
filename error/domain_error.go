package ddderror

import (
	"errors"
	"fmt"
)

type DomainError struct {
	code    string
	message string
	cause   error
}

func (e *DomainError) Code() string    { return e.code }
func (e *DomainError) Message() string { return e.message }
func (e *DomainError) Cause() error    { return e.cause }

func NewDomainError(code, msg string) *DomainError {
	return &DomainError{code: code, message: msg}
}

func NewDomainErrorWithCause(code, msg string, cause error) *DomainError {
	return &DomainError{code: code, message: msg, cause: cause}
}

func (e *DomainError) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("[%s] %s: %v", e.code, e.message, e.cause)
	}
	return fmt.Sprintf("[%s] %s", e.code, e.message)
}

func (e *DomainError) Unwrap() error {
	return e.cause
}

func (e *DomainError) Is(target error) bool {
	t, ok := target.(*DomainError)
	if !ok {
		return false
	}
	return e.code == t.code
}

func IsDomainError(err error) bool {
	var de *DomainError
	return errors.As(err, &de)
}

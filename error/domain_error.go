package ddderror

import (
	"errors"
	"fmt"
)

type DomainError struct {
	Code    string
	Message string
	Cause   error
}

func NewDomainError(code, msg string) *DomainError {
	return &DomainError{Code: code, Message: msg}
}

func NewDomainErrorWithCause(code, msg string, cause error) *DomainError {
	return &DomainError{Code: code, Message: msg, Cause: cause}
}

func (e *DomainError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

func (e *DomainError) Unwrap() error {
	return e.Cause
}

func IsDomainError(err error) bool {
	var de *DomainError
	return errors.As(err, &de)
}

package ddderror

import (
	"fmt"
	"strings"
)

// MultiError aggregates multiple errors into a single error value.
type MultiError struct {
	Errors []error
}

// NewMultiError creates a MultiError from the given errors.
func NewMultiError(errs ...error) *MultiError {
	return &MultiError{Errors: errs}
}

func (e *MultiError) Error() string {
	var b strings.Builder
	b.WriteString("multiple errors:")
	for i, err := range e.Errors {
		if err != nil {
			fmt.Fprintf(&b, "\n  [%d] %v", i+1, err)
		}
	}
	return b.String()
}

func (e *MultiError) Unwrap() []error {
	return e.Errors
}

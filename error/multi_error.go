package ddderror

import (
	"fmt"
	"strings"
)

type MultiError struct {
	Errors []error
}

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

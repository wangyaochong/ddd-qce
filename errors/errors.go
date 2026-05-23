package errors

import (
	"fmt"
	"strings"
)

type MultiError struct {
	Errors []error
}

func (e *MultiError) Error() string {
	if len(e.Errors) == 1 {
		return e.Errors[0].Error()
	}
	msgs := make([]string, len(e.Errors))
	for i, err := range e.Errors {
		msgs[i] = err.Error()
	}
	return fmt.Sprintf("%d errors: %s", len(e.Errors), strings.Join(msgs, "; "))
}

func (e *MultiError) Unwrap() []error {
	return e.Errors
}

func Join(errs ...error) error {
	var nonNil []error
	for _, err := range errs {
		if err != nil {
			nonNil = append(nonNil, err)
		}
	}
	switch len(nonNil) {
	case 0:
		return nil
	case 1:
		return nonNil[0]
	default:
		return &MultiError{Errors: nonNil}
	}
}

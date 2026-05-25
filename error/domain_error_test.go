package ddderror

import (
	"errors"
	"testing"
)

func TestDomainError_Error(t *testing.T) {
	e := NewDomainError("TEST_CODE", "test message")
	want := "[TEST_CODE] test message"
	if got := e.Error(); got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestDomainError_ErrorWithCause(t *testing.T) {
	cause := errors.New("root cause")
	e := NewDomainErrorWithCause("CODE", "msg", cause)
	want := "[CODE] msg: root cause"
	if got := e.Error(); got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestDomainError_Unwrap(t *testing.T) {
	cause := errors.New("root")
	e := NewDomainErrorWithCause("X", "y", cause)
	if unwrapped := e.Unwrap(); unwrapped != cause {
		t.Errorf("Unwrap() = %v, want %v", unwrapped, cause)
	}
}

func TestDomainError_UnwrapNil(t *testing.T) {
	e := NewDomainError("X", "y")
	if unwrapped := e.Unwrap(); unwrapped != nil {
		t.Errorf("Unwrap() = %v, want nil", unwrapped)
	}
}

func TestIsDomainError(t *testing.T) {
	e := NewDomainError("X", "y")
	if !IsDomainError(e) {
		t.Error("IsDomainError(DomainError) = false, want true")
	}
	plainErr := errors.New("plain")
	if IsDomainError(plainErr) {
		t.Error("IsDomainError(plain error) = true, want false")
	}
}

func TestDomainError_CodeAccess(t *testing.T) {
	e := NewDomainError("MY_CODE", "msg")
	if e.Code() != "MY_CODE" {
		t.Errorf("Code = %q, want %q", e.Code(), "MY_CODE")
	}
	if e.Message() != "msg" {
		t.Errorf("Message = %q, want %q", e.Message(), "msg")
	}
}

func TestDomainError_Is_SameCodeDifferentInstances(t *testing.T) {
	e1 := NewDomainError("NOT_FOUND", "resource not found")
	e2 := NewDomainError("NOT_FOUND", "another not found")
	if !errors.Is(e1, e2) {
		t.Error("errors.Is should match DomainErrors with the same code")
	}
}

func TestDomainError_Is_DifferentCode(t *testing.T) {
	e1 := NewDomainError("NOT_FOUND", "not found")
	e2 := NewDomainError("ALREADY_EXISTS", "already exists")
	if errors.Is(e1, e2) {
		t.Error("errors.Is should not match DomainErrors with different codes")
	}
}

func TestDomainError_Is_SentinelErrors(t *testing.T) {
	err := NewDomainError("NOT_FOUND", "custom not found")
	if !errors.Is(err, ErrNotFound) {
		t.Error("errors.Is should match sentinel ErrNotFound by code")
	}
	if errors.Is(err, ErrConcurrency) {
		t.Error("errors.Is should not match sentinel ErrConcurrency")
	}
}

func TestDomainError_Is_NonDomainError(t *testing.T) {
	e := NewDomainError("X", "y")
	plainErr := errors.New("plain")
	if errors.Is(e, plainErr) {
		t.Error("errors.Is should not match non-DomainError")
	}
}

func TestDomainError_WithCause_ErrorsIs(t *testing.T) {
	cause := errors.New("base")
	e := NewDomainErrorWithCause("X", "y", cause)
	if !errors.Is(e, cause) {
		t.Error("errors.Is(wrapped, cause) = false, want true")
	}
}

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
	if e.Code != "MY_CODE" {
		t.Errorf("Code = %q, want %q", e.Code, "MY_CODE")
	}
	if e.Message != "msg" {
		t.Errorf("Message = %q, want %q", e.Message, "msg")
	}
}

func TestDomainError_WithCause_ErrorsIs(t *testing.T) {
	cause := errors.New("base")
	e := NewDomainErrorWithCause("X", "y", cause)
	if !errors.Is(e, cause) {
		t.Error("errors.Is(wrapped, cause) = false, want true")
	}
}

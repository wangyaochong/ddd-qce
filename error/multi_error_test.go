package ddderror

import (
	"errors"
	"strings"
	"testing"
)

func TestMultiError_Error(t *testing.T) {
	me := NewMultiError(
		errors.New("err1"),
		errors.New("err2"),
	)
	got := me.Error()
	if !strings.Contains(got, "multiple errors:") {
		t.Errorf("Error() missing prefix, got %q", got)
	}
	if !strings.Contains(got, "[1] err1") {
		t.Errorf("Error() missing first error, got %q", got)
	}
	if !strings.Contains(got, "[2] err2") {
		t.Errorf("Error() missing second error, got %q", got)
	}
}

func TestMultiError_ErrorSkipsNil(t *testing.T) {
	me := NewMultiError(nil, errors.New("only this"))
	got := me.Error()
	if strings.Contains(got, "[1] <nil>") || strings.Contains(got, "[1] nil") {
		t.Errorf("Error() should skip nil, got %q", got)
	}
	if !strings.Contains(got, "[2] only this") {
		t.Errorf("Error() missing non-nil error, got %q", got)
	}
}

func TestMultiError_Unwrap(t *testing.T) {
	e1 := errors.New("a")
	e2 := errors.New("b")
	me := NewMultiError(e1, e2)
	unwrapped := me.Unwrap()
	if len(unwrapped) != 2 || unwrapped[0] != e1 || unwrapped[1] != e2 {
		t.Errorf("Unwrap() = %v, want [%v, %v]", unwrapped, e1, e2)
	}
}

func TestMultiError_ErrorsIs(t *testing.T) {
	target := errors.New("target")
	me := NewMultiError(errors.New("other"), target)
	if !errors.Is(me, target) {
		t.Error("errors.Is(MultiError, target) = false, want true")
	}
}

func TestMultiError_ErrorsAs(t *testing.T) {
	de := NewDomainError("X", "y")
	me := NewMultiError(de, errors.New("plain"))
	var got *DomainError
	if !errors.As(me, &got) {
		t.Error("errors.As(MultiError, *DomainError) = false, want true")
	}
	if got.Code != "X" {
		t.Errorf("as DomainError Code = %q, want %q", got.Code, "X")
	}
}

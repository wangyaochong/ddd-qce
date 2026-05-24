package ddderror

import (
	"errors"
	"testing"
)

func TestSentinelDomainErrors(t *testing.T) {
	sentinels := []struct {
		name string
		err  *DomainError
		code string
	}{
		{"ErrNotFound", ErrNotFound, "NOT_FOUND"},
		{"ErrAlreadyExists", ErrAlreadyExists, "ALREADY_EXISTS"},
		{"ErrInvalidState", ErrInvalidState, "INVALID_STATE"},
		{"ErrConcurrency", ErrConcurrency, "CONCURRENCY"},
		{"ErrPermissionDenied", ErrPermissionDenied, "PERMISSION_DENIED"},
	}
	for _, tt := range sentinels {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.Code() != tt.code {
				t.Errorf("Code = %q, want %q", tt.err.Code(), tt.code)
			}
		})
	}
}

func TestSentinelDomainErrors_ErrorsIs(t *testing.T) {
	if !errors.Is(ErrNotFound, ErrNotFound) {
		t.Error("errors.Is(ErrNotFound, ErrNotFound) = false")
	}
	if errors.Is(ErrNotFound, ErrAlreadyExists) {
		t.Error("errors.Is(ErrNotFound, ErrAlreadyExists) = true, want false")
	}
}

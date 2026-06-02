package pg

import (
	"errors"
	"testing"

	corepg "github.com/ddd-qce/core/pg"
)

func TestIsUniqueViolation(t *testing.T) {
	err23505 := &testSQLError{sqlState: "23505"}
	err22001 := &testSQLError{sqlState: "22001"}

	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"unique violation", err23505, true},
		{"other error", err22001, false},
		{"nil error", nil, false},
		{"non-SQL error", errors.New("some error"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := corepg.IsUniqueViolation(tt.err)
			if result != tt.expected {
				t.Errorf("IsUniqueViolation(%v) = %v, want %v", tt.err, result, tt.expected)
			}
		})
	}
}

type testSQLError struct {
	err      error
	sqlState string
}

func (e *testSQLError) Error() string {
	if e.err != nil {
		return e.err.Error()
	}
	return "sql error"
}

func (e *testSQLError) Unwrap() error {
	return e.err
}

func (e *testSQLError) SQLState() string {
	return e.sqlState
}

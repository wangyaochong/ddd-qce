package pg

import (
	"encoding/json"
	"fmt"
	"time"
)

// NullString returns nil for empty strings, otherwise the string value, suitable for nullable SQL columns.
func NullString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// NullTime returns nil for zero times, otherwise the time value, suitable for nullable SQL columns.
func NullTime(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t
}

// JSONOrNull marshals v to JSON bytes, returning nil for nil values.
func JSONOrNull(v any) ([]byte, error) {
	if v == nil {
		return nil, nil
	}
	data, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("marshal JSON: %w", err)
	}
	return data, nil
}

// IsUniqueViolation returns true if the error is a PostgreSQL unique constraint violation (SQL state 23505).
func IsUniqueViolation(err error) bool {
	if sq, ok := err.(interface{ SQLState() string }); ok {
		return sq.SQLState() == "23505"
	}
	return false
}

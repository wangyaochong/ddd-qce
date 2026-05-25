package pg

import (
	"encoding/json"
	"fmt"
	"time"
)

func NullString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func NullTime(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t
}

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

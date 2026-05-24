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

func JSONOrNull(v any) any {
	if v == nil {
		return nil
	}
	data, err := json.Marshal(v)
	if err != nil {
		return []byte(fmt.Sprintf(`{"_marshal_error":%q}`, err.Error()))
	}
	return data
}

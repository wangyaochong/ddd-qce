package pg

import (
	"testing"
	"time"
)

func TestNullString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected any
	}{
		{"empty string returns nil", "", nil},
		{"non-empty string returns value", "hello", "hello"},
		{"whitespace returns value", " ", " "},
		{"zero-length empty string", "", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NullString(tt.input)
			if result != tt.expected {
				t.Errorf("NullString(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestNullTime(t *testing.T) {
	now := time.Now()
	zeroTime := time.Time{}

	tests := []struct {
		name     string
		input    time.Time
		expected any
	}{
		{"zero time returns nil", zeroTime, nil},
		{"non-zero time returns value", now, now},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NullTime(tt.input)
			if result != tt.expected {
				t.Errorf("NullTime() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestJSONOrNull(t *testing.T) {
	type TestStruct struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	tests := []struct {
		name      string
		input     any
		wantNil   bool
		wantError bool
	}{
		{"nil returns nil", nil, true, false},
		{"empty struct", TestStruct{}, false, false},
		{"struct with values", TestStruct{Name: "test", Age: 10}, false, false},
		{"pointer to struct", &TestStruct{Name: "ptr"}, false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := JSONOrNull(tt.input)
			if (err != nil) != tt.wantError {
				t.Errorf("JSONOrNull() error = %v, wantError %v", err, tt.wantError)
				return
			}
			if tt.wantNil && result != nil {
				t.Errorf("JSONOrNull() = %v, want nil", result)
			}
			if !tt.wantNil && result == nil {
				t.Errorf("JSONOrNull() = nil, want non-nil")
			}
		})
	}
}
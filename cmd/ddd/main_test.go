package main

import (
	"os"
	"strings"
	"testing"
)

func TestReorderFlags(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		expected []string
	}{
		{
			name:     "module flag before name",
			args:     []string{"--module", "github.com/test/test", "Order"},
			expected: []string{"--module", "github.com/test/test", "Order"},
		},
		{
			name:     "module flag after name",
			args:     []string{"Order", "--module", "github.com/test/test"},
			expected: []string{"--module", "github.com/test/test", "Order"},
		},
		{
			name:     "no flags",
			args:     []string{"Order"},
			expected: []string{"Order"},
		},
		{
			name:     "multiple flags after name",
			args:     []string{"Order", "--module", "github.com/test/test", "--verbose"},
			expected: []string{"--module", "github.com/test/test", "--verbose", "Order"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := reorderFlags(tt.args)
			if len(result) != len(tt.expected) {
				t.Fatalf("expected %d args, got %d", len(tt.expected), len(result))
			}
			for i, v := range result {
				if v != tt.expected[i] {
					t.Errorf("arg[%d]: expected %q, got %q", i, tt.expected[i], v)
				}
			}
		})
	}
}

func TestUsage(t *testing.T) {
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	usage()

	w.Close()
	os.Stdout = oldStdout
	_ = r
}

func TestRunMissingArgs(t *testing.T) {
	err := run([]string{"ddd"})
	if err == nil {
		t.Fatal("expected error for missing arguments")
	}
	if !strings.Contains(err.Error(), "missing arguments") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunHelp(t *testing.T) {
	err := run([]string{"ddd", "--help"})
	if err != nil {
		t.Fatalf("expected no error for --help, got: %v", err)
	}
}

func TestRunUnknownCommand(t *testing.T) {
	err := run([]string{"ddd", "foo", "bar"})
	if err == nil {
		t.Fatal("expected error for unknown command")
	}
	if !strings.Contains(err.Error(), "unknown command") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestHandleNewAggregateMissingModule(t *testing.T) {
	err := handleNewAggregate([]string{"Order"})
	if err == nil {
		t.Fatal("expected error for missing module")
	}
	if !strings.Contains(err.Error(), "--module is required") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestHandleNewAggregateMissingName(t *testing.T) {
	err := handleNewAggregate([]string{"--module", "github.com/test/test"})
	if err == nil {
		t.Fatal("expected error for missing name")
	}
	if !strings.Contains(err.Error(), "aggregate name is required") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestHandleNewAggregateInvalidName(t *testing.T) {
	err := handleNewAggregate([]string{"--module", "github.com/test/test", "order"})
	if err == nil {
		t.Fatal("expected error for lowercase name")
	}
	if !strings.Contains(err.Error(), "must start with uppercase letter") {
		t.Errorf("unexpected error: %v", err)
	}
}

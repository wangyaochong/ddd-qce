package generator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateAggregate(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "scaffold-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	if err := GenerateAggregate("TestOrder", "github.com/test/mymodule"); err != nil {
		t.Fatalf("GenerateAggregate failed: %v", err)
	}

	files := []string{
		"domain/testorder.go",
		"domain/testorder_events.go",
		"domain/testorder_test.go",
		"application/testorder_commands.go",
		"application/testorder_cmd_handler.go",
		"application/testorder_query_handler.go",
		"application/testorder_event_handler.go",
		"application/testorder_repository.go",
	}

	for _, f := range files {
		if _, err := os.Stat(filepath.Join(tmpDir, f)); os.IsNotExist(err) {
			t.Errorf("expected file %s not found", f)
		}
	}

	testOrderGo, _ := os.ReadFile(filepath.Join(tmpDir, "domain/testorder.go"))
	content := string(testOrderGo)

	if !strings.Contains(content, "type TestOrder struct") {
		t.Error("missing TestOrder struct definition")
	}
	if !strings.Contains(content, "func NewTestOrder(") {
		t.Error("missing NewTestOrder constructor")
	}
	if !strings.Contains(content, "func (o *TestOrder) When(") {
		t.Error("missing When method for event sourcing")
	}
}

func TestAggregateData(t *testing.T) {
	data := AggregateData{
		Name:          "Order",
		NameLower:     "order",
		NamePlural:    "Orders",
		Module:        "github.com/test/mymodule",
	}

	if data.Name != "Order" {
		t.Errorf("expected Name=Order, got %s", data.Name)
	}
	if data.NameLower != "order" {
		t.Errorf("expected NameLower=order, got %s", data.NameLower)
	}
	if data.NamePlural != "Orders" {
		t.Errorf("expected NamePlural=Orders, got %s", data.NamePlural)
	}
}

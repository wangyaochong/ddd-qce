package observability

import (
	"testing"
)

func TestTypePrototypeRegistry_Register(t *testing.T) {
	r := NewTypePrototypeRegistry()
	r.Register("command", "TestCommand", []FieldInfo{{Name: "ID", Type: "string"}}, "TestResult", nil)

	proto := r.Get("TestCommand")
	if proto == nil {
		t.Fatal("expected prototype to be registered")
	}
	if proto.Name != "TestCommand" {
		t.Errorf("expected name TestCommand, got %s", proto.Name)
	}
	if proto.Category != "command" {
		t.Errorf("expected category command, got %s", proto.Category)
	}
	if len(proto.Fields) != 1 {
		t.Errorf("expected 1 field, got %d", len(proto.Fields))
	}
}

func TestTypePrototypeRegistry_RegisterFromSample(t *testing.T) {
	r := NewTypePrototypeRegistry()

	type TestCommand struct {
		UserID string
		Items  []string
		Count  int
	}
	type TestResult struct {
		OrderID string
		Success bool
	}

	r.RegisterFromSample("command", "TestCommand", TestCommand{}, TestResult{})

	proto := r.Get("TestCommand")
	if proto == nil {
		t.Fatal("expected prototype to be registered")
	}
	if proto.Name != "TestCommand" {
		t.Errorf("expected name TestCommand, got %s", proto.Name)
	}
	if len(proto.Fields) != 3 {
		t.Errorf("expected 3 fields, got %d: %v", len(proto.Fields), proto.Fields)
	}
	if proto.Fields[0].Name != "UserID" {
		t.Errorf("expected first field UserID, got %s", proto.Fields[0].Name)
	}
	if proto.Fields[1].Type != "[]string" {
		t.Errorf("expected Items field type []string, got %s", proto.Fields[1].Type)
	}
	if proto.ResultType != "TestResult" {
		t.Errorf("expected result type TestResult, got %s", proto.ResultType)
	}
	if len(proto.Result) != 2 {
		t.Errorf("expected 2 result fields, got %d", len(proto.Result))
	}
}

func TestTypePrototypeRegistry_ListByCategory(t *testing.T) {
	r := NewTypePrototypeRegistry()
	r.Register("command", "CmdA", nil, "", nil)
	r.Register("command", "CmdB", nil, "", nil)
	r.Register("query", "QueryA", nil, "", nil)

	cmds := r.ListByCategory("command")
	if len(cmds) != 2 {
		t.Errorf("expected 2 commands, got %d", len(cmds))
	}
	if cmds[0].Name != "CmdA" {
		t.Errorf("expected sorted order CmdA first, got %s", cmds[0].Name)
	}

	querys := r.ListByCategory("query")
	if len(querys) != 1 {
		t.Errorf("expected 1 query, got %d", len(querys))
	}

	events := r.ListByCategory("event")
	if len(events) != 0 {
		t.Errorf("expected 0 events, got %d", len(events))
	}
}

func TestTypePrototypeRegistry_CountByCategory(t *testing.T) {
	r := NewTypePrototypeRegistry()
	r.Register("command", "CmdA", nil, "", nil)
	r.Register("command", "CmdB", nil, "", nil)
	r.Register("query", "QueryA", nil, "", nil)

	if r.CountByCategory("command") != 2 {
		t.Errorf("expected 2 commands count, got %d", r.CountByCategory("command"))
	}
	if r.CountByCategory("query") != 1 {
		t.Errorf("expected 1 query count, got %d", r.CountByCategory("query"))
	}
}

func TestExtractFields_NestedStruct(t *testing.T) {
	type Item struct {
		ID    string
		Name  string
		Price float64
	}
	type Order struct {
		OrderID string
		Items   []Item
	}

	r := NewTypePrototypeRegistry()
	r.RegisterFromSample("command", "Order", Order{}, nil)

	proto := r.Get("Order")
	if proto == nil {
		t.Fatal("expected prototype")
	}
	if len(proto.Fields) != 2 {
		t.Errorf("expected 2 fields, got %d", len(proto.Fields))
	}
	itemsField := proto.Fields[1]
	if itemsField.Name != "Items" {
		t.Errorf("expected Items field, got %s", itemsField.Name)
	}
	if itemsField.Type != "[]observability.Item" {
		t.Errorf("expected Items field type []observability.Item, got %s", itemsField.Type)
	}
}

func TestExtractFields_Pointer(t *testing.T) {
	type Cmd struct {
		ID *string
	}

	r := NewTypePrototypeRegistry()
	r.RegisterFromSample("command", "Cmd", Cmd{}, nil)

	proto := r.Get("Cmd")
	if proto == nil {
		t.Fatal("expected prototype")
	}
	if len(proto.Fields) != 1 {
		t.Errorf("expected 1 field, got %d", len(proto.Fields))
	}
	if proto.Fields[0].Type != "*string" {
		t.Errorf("expected *string type, got %s", proto.Fields[0].Type)
	}
}

func TestExtractFields_Map(t *testing.T) {
	type Cmd struct {
		Metadata map[string]string
	}

	r := NewTypePrototypeRegistry()
	r.RegisterFromSample("command", "Cmd", Cmd{}, nil)

	proto := r.Get("Cmd")
	if proto == nil {
		t.Fatal("expected prototype")
	}
	if proto.Fields[0].Type != "map[string]string" {
		t.Errorf("expected map[string]string type, got %s", proto.Fields[0].Type)
	}
}

func TestShortPackageName(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{"github.com/ddd-qce/exampleapp/ddd/order/domain", "domain"},
		{"github.com/ddd-qce/core/cqrs/command", "command"},
		{"command", "command"},
	}

	for _, tt := range tests {
		result := shortPackageName(tt.path)
		if result != tt.expected {
			t.Errorf("shortPackageName(%s) = %s, want %s", tt.path, result, tt.expected)
		}
	}
}

func TestInferDomainFromPkgPath(t *testing.T) {
	tests := []struct {
		pkgPath  string
		expected string
	}{
		{"github.com/ddd-qce/exampleapp/ddd/order/command", "order"},
		{"github.com/ddd-qce/exampleapp/ddd/inventory/query", "inventory"},
		{"github.com/ddd-qce/exampleapp/ddd/order/event", "order"},
		{"exampleapp/ddd/order/command", "order"},
		{"ddd/order/command", "order"},
		{"github.com/ddd-qce/core/cqrs/command", ""},
		{"core/cqrs/command", ""},
		{"command", ""},
		{"", ""},
	}

	for _, tt := range tests {
		result := inferDomainFromPkgPath(tt.pkgPath)
		if result != tt.expected {
			t.Errorf("inferDomainFromPkgPath(%s) = %s, want %s", tt.pkgPath, result, tt.expected)
		}
	}
}

func TestTypePrototypeRegistry_RegisterFromSample_AutoDomain(t *testing.T) {
	r := NewTypePrototypeRegistry()

	type OrderCommand struct{ UserID string }
	type OrderResult struct{ OrderID string }

	r.RegisterFromSample("command", "OrderCommand", OrderCommand{}, OrderResult{})

	proto := r.Get("OrderCommand")
	if proto == nil {
		t.Fatal("expected prototype")
	}
	if proto.Domain != "" {
		t.Errorf("expected empty domain for non-ddd package path, got %s", proto.Domain)
	}

	domains := r.ListDomains()
	if len(domains) != 0 {
		t.Errorf("expected 0 domains for non-ddd package, got %d", len(domains))
	}
}

func TestInferDomainFromSample(t *testing.T) {
	type TestCmd struct{}

	result := inferDomainFromSample(TestCmd{})
	if result != "" {
		t.Errorf("expected empty domain for test type, got %s", result)
	}

	result = inferDomainFromSample(nil)
	if result != "" {
		t.Errorf("expected empty domain for nil, got %s", result)
	}

	result = inferDomainFromSample((*TestCmd)(nil))
	if result != "" {
		t.Errorf("expected empty domain for nil pointer, got %s", result)
	}
}

func TestNormalizeTypeName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Command/*ordercommand.PlaceOrderCommand", "PlaceOrderCommand"},
		{"Query/*orderquery.GetOrderQuery", "GetOrderQuery"},
		{"Event/*orderevent.OrderPlacedEvent", "OrderPlacedEvent"},
		{"Command/ordercommand.PlaceOrderCommand", "PlaceOrderCommand"},
		{"Command/PlaceOrderCommand", "PlaceOrderCommand"},
		{"PlaceOrderCommand", "PlaceOrderCommand"},
		{"*ordercommand.PlaceOrderCommand", "PlaceOrderCommand"},
		{"Command/*inventorycommand.ReserveInventoryCommand", "ReserveInventoryCommand"},
	}

	for _, tt := range tests {
		result := normalizeTypeName(tt.input)
		if result != tt.expected {
			t.Errorf("normalizeTypeName(%s) = %s, want %s", tt.input, result, tt.expected)
		}
	}
}

func TestGetTypeDomain_WithMetricsStyleName(t *testing.T) {
	r := NewTypePrototypeRegistry()
	r.RegisterWithDomain("command", "order", "PlaceOrderCommand", nil, "", nil)
	r.RegisterWithDomain("query", "inventory", "GetInventoryQuery", nil, "", nil)
	r.RegisterWithDomain("event", "order", "OrderPlacedEvent", nil, "", nil)

	tests := []struct {
		name     string
		expected string
	}{
		{"Command/*ordercommand.PlaceOrderCommand", "order"},
		{"Query/*inventoryquery.GetInventoryQuery", "inventory"},
		{"Event/*orderevent.OrderPlacedEvent", "order"},
		{"Command/PlaceOrderCommand", "order"},
		{"PlaceOrderCommand", "order"},
		{"UnknownCommand", ""},
	}

	for _, tt := range tests {
		result := r.GetTypeDomain(tt.name)
		if result != tt.expected {
			t.Errorf("GetTypeDomain(%s) = %s, want %s", tt.name, result, tt.expected)
		}
	}
}

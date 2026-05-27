package valueobject

import (
	"encoding/json"
	"fmt"
	"testing"
)

type money struct {
	Amount   float64
	Currency string
}

func validateMoney(m money) error {
	if m.Amount < 0 {
		return fmt.Errorf("amount cannot be negative")
	}
	if m.Currency == "" {
		return fmt.Errorf("currency is required")
	}
	return nil
}

type email string

func validateEmail(e email) error {
	if e == "" {
		return fmt.Errorf("email cannot be empty")
	}
	return nil
}

func TestNew_Valid(t *testing.T) {
	vo, err := New(money{Amount: 100, Currency: "USD"}, validateMoney)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if vo.Value().Amount != 100 || vo.Value().Currency != "USD" {
		t.Errorf("expected Money{100, USD}, got %v", vo.Value())
	}
}

func TestNew_ValidationError(t *testing.T) {
	_, err := New(money{Amount: -1, Currency: "USD"}, validateMoney)
	if err == nil {
		t.Fatal("expected validation error for negative amount")
	}
}

func TestNew_MissingCurrency(t *testing.T) {
	_, err := New(money{Amount: 100, Currency: ""}, validateMoney)
	if err == nil {
		t.Fatal("expected validation error for empty currency")
	}
}

func TestNew_NilValidate(t *testing.T) {
	vo, err := New(money{Amount: -1, Currency: ""}, nil)
	if err != nil {
		t.Fatalf("unexpected error with nil validator: %v", err)
	}
	if vo.Value().Amount != -1 {
		t.Errorf("expected -1, got %v", vo.Value().Amount)
	}
}

func TestMustNew_Valid(t *testing.T) {
	vo := MustNew(money{Amount: 100, Currency: "USD"}, validateMoney)
	if vo.Value().Amount != 100 {
		t.Errorf("expected 100, got %v", vo.Value().Amount)
	}
}

func TestMustNew_Panics(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic for invalid value")
		}
	}()
	MustNew(money{Amount: -1, Currency: "USD"}, validateMoney)
}

func TestValueObject_Equals_SameValue(t *testing.T) {
	m1 := MustNew(money{Amount: 100, Currency: "USD"}, validateMoney)
	m2 := MustNew(money{Amount: 100, Currency: "USD"}, validateMoney)
	if !m1.Equals(m2) {
		t.Error("expected equal values to be equal")
	}
}

func TestValueObject_Equals_DifferentValue(t *testing.T) {
	m1 := MustNew(money{Amount: 100, Currency: "USD"}, validateMoney)
	m2 := MustNew(money{Amount: 200, Currency: "USD"}, validateMoney)
	if m1.Equals(m2) {
		t.Error("expected different values to not be equal")
	}
}

func TestValueObject_Equals_ZeroValue(t *testing.T) {
	var m1 ValueObject[money]
	var m2 ValueObject[money]
	if !m1.Equals(m2) {
		t.Error("expected zero values to be equal")
	}
}

func TestValueObject_Equals_PrimitiveType(t *testing.T) {
	e1 := MustNew(email("test@example.com"), validateEmail)
	e2 := MustNew(email("test@example.com"), validateEmail)
	if !e1.Equals(e2) {
		t.Error("expected equal emails to be equal")
	}

	e3 := MustNew(email("other@example.com"), validateEmail)
	if e1.Equals(e3) {
		t.Error("expected different emails to not be equal")
	}
}

func TestValueObject_Value(t *testing.T) {
	vo := MustNew(money{Amount: 50, Currency: "EUR"}, validateMoney)
	v := vo.Value()
	if v.Amount != 50 || v.Currency != "EUR" {
		t.Errorf("expected Money{50, EUR}, got %v", v)
	}
}

func TestValueObject_String(t *testing.T) {
	vo := MustNew(money{Amount: 100, Currency: "USD"}, validateMoney)
	s := vo.String()
	if s == "" {
		t.Error("expected non-empty string representation")
	}
}

func TestValueObject_ZeroValue(t *testing.T) {
	var vo ValueObject[money]
	if vo.Value().Amount != 0 || vo.Value().Currency != "" {
		t.Error("expected zero value for uninitialized ValueObject")
	}
}

func TestValueObject_ImmutableByConvention(t *testing.T) {
	vo := MustNew(money{Amount: 100, Currency: "USD"}, validateMoney)
	v := vo.Value()
	v.Amount = 999
	if vo.Value().Amount == 999 {
		t.Error("Value() should return a copy for value types, original should not be modified")
	}
}

func TestValueObject_MarshalJSON_StructType(t *testing.T) {
	vo := MustNew(money{Amount: 100, Currency: "USD"}, validateMoney)
	data, err := json.Marshal(vo)
	if err != nil {
		t.Fatalf("MarshalJSON: %v", err)
	}
	var parsed struct {
		Value money `json:"value"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("parse marshaled data: %v", err)
	}
	if parsed.Value.Amount != 100 || parsed.Value.Currency != "USD" {
		t.Errorf("expected {100, USD}, got %v", parsed.Value)
	}
}

func TestValueObject_MarshalJSON_PrimitiveType(t *testing.T) {
	vo := MustNew(email("test@example.com"), validateEmail)
	data, err := json.Marshal(vo)
	if err != nil {
		t.Fatalf("MarshalJSON: %v", err)
	}
	var parsed struct {
		Value email `json:"value"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("parse marshaled data: %v", err)
	}
	if parsed.Value != "test@example.com" {
		t.Errorf("expected test@example.com, got %s", parsed.Value)
	}
}

func TestValueObject_UnmarshalJSON_RoundTrip(t *testing.T) {
	original := MustNew(money{Amount: 250, Currency: "EUR"}, validateMoney)
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("MarshalJSON: %v", err)
	}
	var restored ValueObject[money]
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("UnmarshalJSON: %v", err)
	}
	if !original.Equals(restored) {
		t.Errorf("round-trip failed: original=%v, restored=%v", original.Value(), restored.Value())
	}
}

func TestValueObject_UnmarshalJSON_ZeroValue(t *testing.T) {
	var original ValueObject[money]
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("MarshalJSON zero value: %v", err)
	}
	var restored ValueObject[money]
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("UnmarshalJSON zero value: %v", err)
	}
	if !original.Equals(restored) {
		t.Errorf("zero value round-trip failed: original=%v, restored=%v", original.Value(), restored.Value())
	}
}

func TestValueObject_UnmarshalJSON_NilValidator(t *testing.T) {
	vo, _ := New(money{Amount: 100, Currency: "USD"}, nil)
	data, err := json.Marshal(vo)
	if err != nil {
		t.Fatalf("MarshalJSON: %v", err)
	}
	var restored ValueObject[money]
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("UnmarshalJSON: %v", err)
	}
	if !vo.Equals(restored) {
		t.Errorf("round-trip with nil validator failed: original=%v, restored=%v", vo.Value(), restored.Value())
	}
}

func TestDeepEquals_SameValues(t *testing.T) {
	m1 := money{Amount: 100, Currency: "USD"}
	m2 := money{Amount: 100, Currency: "USD"}
	if !DeepEquals(m1, m2) {
		t.Error("expected DeepEquals to return true for identical values")
	}
}

func TestDeepEquals_DifferentValues(t *testing.T) {
	m1 := money{Amount: 100, Currency: "USD"}
	m2 := money{Amount: 200, Currency: "USD"}
	if DeepEquals(m1, m2) {
		t.Error("expected DeepEquals to return false for different values")
	}
}

func TestDeepEquals_DifferentTypes(t *testing.T) {
	m := money{Amount: 100, Currency: "USD"}
	e := email("test@example.com")
	if DeepEquals(m, e) {
		t.Error("expected DeepEquals to return false for different types")
	}
}

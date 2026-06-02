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

func TestValueObject_Equals(t *testing.T) {
	m100USD := MustNew(money{Amount: 100, Currency: "USD"}, validateMoney)
	m200USD := MustNew(money{Amount: 200, Currency: "USD"}, validateMoney)
	m100USD2 := MustNew(money{Amount: 100, Currency: "USD"}, validateMoney)
	e1 := MustNew(email("a@b.com"), validateEmail)
	e2 := MustNew(email("a@b.com"), validateEmail)

	tests := []struct {
		name string
		a, b any
		want bool
	}{
		{"same struct value", m100USD, m100USD2, true},
		{"different struct value", m100USD, m200USD, false},
		{"zero values", ValueObject[money]{}, ValueObject[money]{}, true},
		{"same primitive value", e1, e2, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			switch a := tt.a.(type) {
			case ValueObject[money]:
				if got := a.Equals(tt.b.(ValueObject[money])); got != tt.want {
					t.Errorf("Equals() = %v, want %v", got, tt.want)
				}
			case ValueObject[email]:
				if got := a.Equals(tt.b.(ValueObject[email])); got != tt.want {
					t.Errorf("Equals() = %v, want %v", got, tt.want)
				}
			}
		})
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
	if err := json.Unmarshal(data, &restored); err == nil {
		t.Error("UnmarshalJSON should fail for zero value")
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

func TestValueObject_Validate_WithCustomValidator(t *testing.T) {
	vo, err := New(money{Amount: 100, Currency: "USD"}, validateMoney)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := vo.Validate(); err != nil {
		t.Errorf("Validate() should pass for valid value, got: %v", err)
	}
}

func TestValueObject_Validate_WithCustomValidator_Invalid(t *testing.T) {
	vo := ValueObject[money]{
		value:    money{Amount: -1, Currency: "USD"},
		validate: validateMoney,
	}
	if err := vo.Validate(); err == nil {
		t.Error("Validate() should fail for negative amount with custom validator")
	}
}

func TestValueObject_Validate_ZeroValue_NoValidator(t *testing.T) {
	var vo ValueObject[money]
	if err := vo.Validate(); err == nil {
		t.Error("Validate() should fail for zero value without validator")
	}
}

func TestValueObject_Validate_NonZeroValue_NoValidator(t *testing.T) {
	vo, _ := New(money{Amount: 100, Currency: "USD"}, nil)
	if err := vo.Validate(); err != nil {
		t.Errorf("Validate() should pass for non-zero value without validator, got: %v", err)
	}
}

func TestValueObject_UnmarshalJSON_InvalidValue(t *testing.T) {
	vo, _ := New(money{Amount: 100, Currency: "USD"}, validateMoney)
	_, err := json.Marshal(vo)
	if err != nil {
		t.Fatalf("MarshalJSON: %v", err)
	}

	invalidData := `{"value":{"Amount":-1,"Currency":"USD"}}`
	var restored ValueObject[money]
	restored.validate = validateMoney
	if err := json.Unmarshal([]byte(invalidData), &restored); err == nil {
		t.Error("UnmarshalJSON should fail for invalid value when validator is set")
	}
}

func TestValueObject_UnmarshalJSON_ZeroValueFails(t *testing.T) {
	zeroData := `{"value":{"Amount":0,"Currency":""}}`
	var restored ValueObject[money]
	if err := json.Unmarshal([]byte(zeroData), &restored); err == nil {
		t.Error("UnmarshalJSON should fail for zero value (no validator)")
	}
}

func TestDeepEquals(t *testing.T) {
	tests := []struct {
		name string
		a, b any
		want bool
	}{
		{"same struct values", money{100, "USD"}, money{100, "USD"}, true},
		{"different struct values", money{100, "USD"}, money{200, "USD"}, false},
		{"different types", money{100, "USD"}, email("test@example.com"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// DeepEquals requires comparable types; test with same type only
			if m1, ok := tt.a.(money); ok {
				if m2, ok2 := tt.b.(money); ok2 {
					if got := DeepEquals(m1, m2); got != tt.want {
						t.Errorf("DeepEquals() = %v, want %v", got, tt.want)
					}
					return
				}
			}
			if e1, ok := tt.a.(email); ok {
				if e2, ok2 := tt.b.(email); ok2 {
					if got := DeepEquals(e1, e2); got != tt.want {
						t.Errorf("DeepEquals() = %v, want %v", got, tt.want)
					}
				}
			}
		})
	}
}

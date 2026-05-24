package valueobject

import (
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

func TestValueObject_Validate_Valid(t *testing.T) {
	vo := MustNew(money{Amount: 100, Currency: "USD"}, validateMoney)
	if err := vo.Validate(); err != nil {
		t.Errorf("unexpected validation error: %v", err)
	}
}

func TestValueObject_Validate_Invalid(t *testing.T) {
	vo := ValueObject[money]{value: money{Amount: -1, Currency: ""}, validate: validateMoney}
	if err := vo.Validate(); err == nil {
		t.Fatal("expected validation error for manually constructed invalid value")
	}
}

func TestValueObject_Validate_NilValidator(t *testing.T) {
	vo, _ := New(money{Amount: 100, Currency: "USD"}, nil)
	if err := vo.Validate(); err != nil {
		t.Errorf("unexpected validation error with nil validator: %v", err)
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

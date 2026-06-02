package valueobject

import (
	"encoding/json"
	"fmt"
	"reflect"
)

// ValueObject wraps an immutable value of type T with optional validation.
// Value objects are compared by value rather than by identity.
// Use New or MustNew to construct instances with validation.
type ValueObject[T comparable] struct {
	value    T
	validate func(T) error
}

// New creates a ValueObject with the given value and optional validation function.
// If validate is non-nil, it is called with the value; an error prevents construction.
func New[T comparable](value T, validate func(T) error) (ValueObject[T], error) {
	if validate != nil {
		if err := validate(value); err != nil {
			return ValueObject[T]{}, err
		}
	}
	return ValueObject[T]{value: value, validate: validate}, nil
}

// MustNew creates a ValueObject, panicking if validation fails.
func MustNew[T comparable](value T, validate func(T) error) ValueObject[T] {
	vo, err := New(value, validate)
	if err != nil {
		panic(err)
	}
	return vo
}

// Value returns the wrapped value.
func (v ValueObject[T]) Value() T {
	return v.value
}

// Equals returns true if both value objects wrap the same value.
func (v ValueObject[T]) Equals(other ValueObject[T]) bool {
	return v.value == other.value
}

func (v ValueObject[T]) String() string {
	return fmt.Sprintf("%v", v.value)
}

func (v ValueObject[T]) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Value T `json:"value"`
	}{Value: v.value})
}

func (v *ValueObject[T]) UnmarshalJSON(data []byte) error {
	var aux struct {
		Value T `json:"value"`
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	v.value = aux.Value
	return v.Validate()
}

// Validate runs the validation function against the wrapped value.
// If no custom validator was provided, it rejects zero values.
func (v ValueObject[T]) Validate() error {
	if v.validate != nil {
		return v.validate(v.value)
	}
	var zero T
	if v.value == zero {
		return fmt.Errorf("value object: value cannot be zero")
	}
	return nil
}

// DeepEquals compares two values using reflect.DeepEqual.
// Useful for value objects containing slices, maps, or structs
// where == cannot be used.
func DeepEquals(a, b any) bool {
	return reflect.DeepEqual(a, b)
}

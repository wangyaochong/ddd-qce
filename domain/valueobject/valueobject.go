package valueobject

import (
	"encoding/json"
	"fmt"
	"reflect"
)

type ValueObject[T comparable] struct {
	value T
}

func New[T comparable](value T, validate func(T) error) (ValueObject[T], error) {
	if validate != nil {
		if err := validate(value); err != nil {
			return ValueObject[T]{}, err
		}
	}
	return ValueObject[T]{value: value}, nil
}

func MustNew[T comparable](value T, validate func(T) error) ValueObject[T] {
	vo, err := New(value, validate)
	if err != nil {
		panic(err)
	}
	return vo
}

func (v ValueObject[T]) Value() T {
	return v.value
}

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
	return nil
}

func DeepEquals(a, b any) bool {
	return reflect.DeepEqual(a, b)
}

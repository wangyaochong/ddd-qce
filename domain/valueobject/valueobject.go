package valueobject

import (
	"fmt"
	"reflect"
)

type ValueObject[T comparable] struct {
	value    T
	validate func(T) error
}

func New[T comparable](value T, validate func(T) error) (ValueObject[T], error) {
	if validate != nil {
		if err := validate(value); err != nil {
			return ValueObject[T]{}, err
		}
	}
	return ValueObject[T]{value: value, validate: validate}, nil
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

func (v ValueObject[T]) Validate() error {
	if v.validate == nil {
		return nil
	}
	return v.validate(v.value)
}

func (v ValueObject[T]) String() string {
	return fmt.Sprintf("%v", v.value)
}

func DeepEquals(a, b any) bool {
	return reflect.DeepEqual(a, b)
}

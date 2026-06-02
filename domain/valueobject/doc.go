// Package valueobject provides ValueObject[T comparable] — a generic wrapper
// for DDD value objects with validation, deep equality, and JSON serialization.
//
// Use New(value, validate) to create a validated value object, or MustNew to
// panic on validation failure. Value objects are immutable by convention —
// Value() returns a copy.
package valueobject

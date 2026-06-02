package entity

// IDGenerator is a function type that generates unique entity IDs as strings.
type IDGenerator func() string

// DefaultIDGenerator returns an IDGenerator that creates UUID-based hex strings.
func DefaultIDGenerator() IDGenerator {
	return func() string {
		return NewEntityWithID().ID()
	}
}

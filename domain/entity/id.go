package entity

type IDGenerator func() string

func DefaultIDGenerator() IDGenerator {
	return func() string {
		return NewEntityWithID().ID()
	}
}

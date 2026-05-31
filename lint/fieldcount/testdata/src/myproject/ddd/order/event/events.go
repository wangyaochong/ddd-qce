package event

import "time"

type BaseEvent struct {
	aggregateID   string
	occurredAt    time.Time
	correlationID string
	causationID   string
}

func (e BaseEvent) AggregateID() string   { return e.aggregateID }
func (e BaseEvent) OccurredAt() time.Time { return e.occurredAt }
func (e BaseEvent) CorrelationID() string { return e.correlationID }
func (e BaseEvent) CausationID() string   { return e.causationID }

type ValidEvent struct {
	BaseEvent
	Field1 string
	Field2 int
	Field3 bool
	Field4 float64
	Field5 []string
}

type TooManyFieldsEvent struct { // want "dddfieldcount"
	BaseEvent
	Field1 string
	Field2 int
	Field3 bool
	Field4 float64
	Field5 []string
	Field6 string
}

type ThreeFieldsEvent struct {
	BaseEvent
	Field1 string
	Field2 int
	Field3 bool
}

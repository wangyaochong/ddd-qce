package event

import (
	"time"

	cqrsevent "github.com/ddd-qce/core/cqrs/event"
)

// 合法：标准嵌入
type OrderPlacedEvent struct {
	cqrsevent.BaseEvent
	UserID      string
	TotalAmount float64
}

func NewOrderPlaced(aggID, userID string, total float64) *OrderPlacedEvent {
	return &OrderPlacedEvent{
		BaseEvent:   cqrsevent.NewDomainEvent(aggID),
		UserID:      userID,
		TotalAmount: total,
	}
}

// 合法：复合字面量构造
func NewOrderPlacedWithLiteral(aggID string) OrderPlacedEvent {
	return OrderPlacedEvent{
		BaseEvent: cqrsevent.BaseEvent{
			AggregateID: aggID,
			OccurredAt:  time.Now(),
		},
		UserID: "u-1",
	}
}

func BadDirectAssign() *OrderPlacedEvent {
	e := NewOrderPlaced("order-1", "u-1", 100)
	e.AggregateID = "tampered" // want "dddeventimmutable"
	return e
}

func BadMutateOccurredAt() {
	e := &OrderPlacedEvent{}
	e.OccurredAt = time.Now() // want "dddeventimmutable"
}

func BadMutateCorrelation() {
	e := &OrderPlacedEvent{}
	e.CorrelationID = "fake" // want "dddeventimmutable"
}

func BadMutateCausation() {
	e := &OrderPlacedEvent{}
	e.CausationID = "fake" // want "dddeventimmutable"
}

func (e *OrderPlacedEvent) BadSetter(id string) {
	e.AggregateID = id // want "dddeventimmutable"
}

// 合法：字段遮蔽不触发
type ShadowedEvent struct {
	cqrsevent.BaseEvent
	AggregateID string // 用户的字段遮蔽了 BaseEvent.AggregateID
}

func SetUserField(e *ShadowedEvent, id string) {
	e.AggregateID = id // OK：修改的是 ShadowedEvent.AggregateID，不是 BaseEvent 的
}

// 合法：读取不触发
func ReadOnly(e *OrderPlacedEvent) string {
	return e.AggregateID // OK：读取
}

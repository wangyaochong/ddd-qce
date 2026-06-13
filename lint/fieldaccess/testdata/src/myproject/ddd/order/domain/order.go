package domain

type OrderStatus string

const (
	OrderStatusPending   OrderStatus = "pending"
	OrderStatusCompleted OrderStatus = "completed"
	OrderStatusFailed    OrderStatus = "failed"
)

type Order struct {
	ID           string
	Status       OrderStatus
	ErrorMessage string
	StartedAt    string
	EndedAt      string
}

func NewOrder(id string) *Order {
	return &Order{
		ID:     id,
		Status: OrderStatusPending,
	}
}

func (o *Order) Complete() error {
	if o.Status != OrderStatusPending {
		return nil
	}
	o.Status = OrderStatusCompleted
	return nil
}

func (o *Order) Fail(errMsg string) error {
	if o.Status != OrderStatusPending {
		return nil
	}
	o.Status = OrderStatusFailed
	o.ErrorMessage = errMsg
	return nil
}

package command

import "myproject/ddd/order/domain"

func BadDirectStatusAssign(o *domain.Order) {
	o.Status = domain.OrderStatusCompleted // want "dddfieldaccess"
}

func BadDirectErrorMessageAssign(o *domain.Order) {
	o.ErrorMessage = "something" // want "dddfieldaccess"
}

func BadDirectStartedAtAssign(o *domain.Order) {
	o.StartedAt = "2024-01-01" // want "dddfieldaccess"
}

func BadDirectEndedAtAssign(o *domain.Order) {
	o.EndedAt = "2024-01-01" // want "dddfieldaccess"
}

func GoodUseDomainMethod(o *domain.Order) error {
	return o.Complete()
}

func GoodReadStatus(o *domain.Order) bool {
	return o.Status == domain.OrderStatusPending
}

func GoodReadErrorMessage(o *domain.Order) string {
	return o.ErrorMessage
}

func GoodAssignNonProtectedField(o *domain.Order, id string) {
	o.ID = id
}

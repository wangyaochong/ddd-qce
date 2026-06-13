package pg

import "myproject/ddd/order/domain"

func ScanFromDB(row struct {
	ID           string
	Status       string
	ErrorMessage string
	StartedAt    string
	EndedAt      string
}) *domain.Order {
	o := &domain.Order{
		ID: row.ID,
	}
	o.Status = domain.OrderStatus(row.Status)
	o.ErrorMessage = row.ErrorMessage
	o.StartedAt = row.StartedAt
	o.EndedAt = row.EndedAt
	return o
}

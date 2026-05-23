package query

import (
	"context"
	"fmt"

	corequery "github.com/ddd-qce/core/cqrs/query"
	querymemory "github.com/ddd-qce/core/cqrs/query/memory"
)

type GetUserQuery struct {
	corequery.BaseQuery
	UserID string
}

type GetUserResult struct {
	ID    string
	Name  string
	Email string
}

type GetUserHandler struct{}

func (h *GetUserHandler) Handle(ctx context.Context, query *GetUserQuery) (*GetUserResult, error) {
	return &GetUserResult{ID: query.UserID, Name: "张三", Email: "zhangsan@example.com"}, nil
}

type ListUsersQuery struct {
	corequery.BaseQuery
	Page     int
	PageSize int
}

type ListUsersResult struct {
	Users []GetUserResult
	Total int
}

type ListUsersHandler struct{}

func (h *ListUsersHandler) Handle(ctx context.Context, query *ListUsersQuery) (*ListUsersResult, error) {
	return &ListUsersResult{
		Users: []GetUserResult{
			{ID: "1", Name: "张三", Email: "zhangsan@example.com"},
			{ID: "2", Name: "李四", Email: "lisi@example.com"},
		},
		Total: 2,
	}, nil
}

type GetOrderQuery struct {
	corequery.BaseQuery
	OrderID string
}

type GetOrderResult struct {
	ID     string
	UserID string
	Amount float64
	Status string
}

type GetOrderHandler struct{}

func (h *GetOrderHandler) Handle(ctx context.Context, query *GetOrderQuery) (*GetOrderResult, error) {
	return &GetOrderResult{ID: query.OrderID, UserID: "1", Amount: 99.99, Status: "paid"}, nil
}

func RegisterHandlers(bus *querymemory.QueryBus) {
	querymemory.RegisterQuery(bus, &GetUserHandler{})
	querymemory.RegisterQuery(bus, &ListUsersHandler{})
	querymemory.RegisterQuery(bus, &GetOrderHandler{})
}

func RunExample(ctx context.Context, bus *querymemory.QueryBus) {
	fmt.Println("=== Query: GetUser ===")
	result, err := querymemory.Ask[*GetUserQuery, *GetUserResult](bus, ctx, &GetUserQuery{UserID: "123"})
	if err != nil {
		panic(err)
	}
	fmt.Printf("User: ID=%s, Name=%s, Email=%s\n", result.ID, result.Name, result.Email)

	fmt.Println("\n=== Query: ListUsers ===")
	listResult, err := querymemory.Ask[*ListUsersQuery, *ListUsersResult](bus, ctx, &ListUsersQuery{Page: 1, PageSize: 10})
	if err != nil {
		panic(err)
	}
	fmt.Printf("Total users: %d\n", listResult.Total)
	for _, u := range listResult.Users {
		fmt.Printf("  - %s (%s)\n", u.Name, u.Email)
	}

	fmt.Println("\n=== Query: GetOrder ===")
	orderResult, err := querymemory.Ask[*GetOrderQuery, *GetOrderResult](bus, ctx, &GetOrderQuery{OrderID: "ORD-001"})
	if err != nil {
		panic(err)
	}
	fmt.Printf("Order: ID=%s, Amount=%.2f, Status=%s\n", orderResult.ID, orderResult.Amount, orderResult.Status)
}

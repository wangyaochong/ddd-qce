package traceexample

import (
	"context"
	"fmt"
	"time"

	"github.com/ddd-qce/core/cqrs/command"
	"github.com/ddd-qce/core/cqrs/event"
	commandmemory "github.com/ddd-qce/core/cqrs/impl/memory"
	eventmemory "github.com/ddd-qce/core/cqrs/impl/memory"
	"github.com/ddd-qce/core/cqrs/query"
	querymemory "github.com/ddd-qce/core/cqrs/impl/memory"
	"github.com/ddd-qce/core/trace"
)

type PlaceOrderCommand struct {
	command.BaseCommand
	UserID  string
	Product string
	Amount  float64
}

type PlaceOrderResult struct {
	OrderID string
}

type PlaceOrderHandler struct {
	eventBus *eventmemory.EventBus
}

func (h *PlaceOrderHandler) Handle(ctx context.Context, cmd *PlaceOrderCommand) (*PlaceOrderResult, error) {
	orderID := "ORD-" + time.Now().Format("20060102150405")

	h.eventBus.Publish(ctx, &OrderPlacedEvent{
		BaseEvent: event.NewBaseEvent(orderID, time.Now()),
		UserID:          cmd.UserID,
		Product:         cmd.Product,
		Amount:          cmd.Amount,
	})

	return &PlaceOrderResult{OrderID: orderID}, nil
}

type OrderPlacedEvent struct {
	event.BaseEvent
	UserID  string
	Product string
	Amount  float64
}

type OrderPlacedEventHandler struct {
	cmdBus *commandmemory.CommandBus
}

func (h *OrderPlacedEventHandler) Handle(ctx context.Context, event *OrderPlacedEvent) error {
	fmt.Printf("  [EventHandler] Processing OrderPlaced for order %s\n", event.AggregateID())

	command.Dispatch[*SendNotificationCommand, *SendNotificationResult](ctx, h.cmdBus, &SendNotificationCommand{
		UserID:  event.UserID,
		Message: fmt.Sprintf("Order %s placed: %s ($%.2f)", event.AggregateID(), event.Product, event.Amount),
	})

	command.Dispatch[*UpdateInventoryCommand, *UpdateInventoryResult](ctx, h.cmdBus, &UpdateInventoryCommand{
		Product:  event.Product,
		Quantity: 1,
	})

	return nil
}

type SendNotificationCommand struct {
	command.BaseCommand
	UserID  string
	Message string
}

type SendNotificationResult struct {
	Sent bool
}

type SendNotificationHandler struct{}

func (h *SendNotificationHandler) Handle(ctx context.Context, cmd *SendNotificationCommand) (*SendNotificationResult, error) {
	fmt.Printf("    [CommandHandler] Sending notification to user %s: %s\n", cmd.UserID, cmd.Message)
	return &SendNotificationResult{Sent: true}, nil
}

type UpdateInventoryCommand struct {
	command.BaseCommand
	Product  string
	Quantity int
}

type UpdateInventoryResult struct {
	Remaining int
}

type UpdateInventoryHandler struct{}

func (h *UpdateInventoryHandler) Handle(ctx context.Context, cmd *UpdateInventoryCommand) (*UpdateInventoryResult, error) {
	fmt.Printf("    [CommandHandler] Updating inventory for %s, quantity: %d\n", cmd.Product, cmd.Quantity)
	return &UpdateInventoryResult{Remaining: 99}, nil
}

type GetOrderStatusQuery struct {
	query.BaseQuery
	OrderID string
}

type GetOrderStatusResult struct {
	OrderID string
	Status  string
}

type GetOrderStatusHandler struct{}

func (h *GetOrderStatusHandler) Handle(ctx context.Context, query *GetOrderStatusQuery) (*GetOrderStatusResult, error) {
	return &GetOrderStatusResult{OrderID: query.OrderID, Status: "confirmed"}, nil
}

func RegisterHandlers(cmdBus *commandmemory.CommandBus, eventBus *eventmemory.EventBus, qBus *querymemory.QueryBus) {
	commandmemory.RegisterCommand(cmdBus, &PlaceOrderHandler{eventBus: eventBus})
	commandmemory.RegisterCommand(cmdBus, &SendNotificationHandler{})
	commandmemory.RegisterCommand(cmdBus, &UpdateInventoryHandler{})

	eventmemory.RegisterHandler[*OrderPlacedEvent](eventBus, &OrderPlacedEventHandler{cmdBus: cmdBus})

	querymemory.RegisterQuery(qBus, &GetOrderStatusHandler{})
}

func RunExample(ctx context.Context, cmdBus *commandmemory.CommandBus, eventBus *eventmemory.EventBus, qBus *querymemory.QueryBus) {
	RegisterHandlers(cmdBus, eventBus, qBus)

	fmt.Println("========================================")
	fmt.Println("  Cross-Domain Trace Example")
	fmt.Println("========================================")
	fmt.Println()
	fmt.Println("=== PlaceOrder (Command → Event → Commands) ===")

	result, err := command.Dispatch[*PlaceOrderCommand, *PlaceOrderResult](ctx, cmdBus, &PlaceOrderCommand{
		UserID:  "user-001",
		Product: "Laptop",
		Amount:  999.99,
	})
	if err != nil {
		panic(err)
	}
	fmt.Printf("Order placed: %s\n", result.OrderID)

	fmt.Println()
	fmt.Println("=== GetOrderStatus (Query) ===")
	qResult, err := query.Dispatch[*GetOrderStatusQuery, *GetOrderStatusResult](ctx, qBus, &GetOrderStatusQuery{OrderID: result.OrderID})
	if err != nil {
		panic(err)
	}
	fmt.Printf("Order %s status: %s\n", qResult.OrderID, qResult.Status)
}

func PrintTraces(ctx context.Context, store *trace.InMemoryTraceStore) {
	traceIDs, err := store.ListTraces(ctx, trace.TraceFilter{})
	if err != nil {
		fmt.Println("No traces found")
		return
	}

	for _, traceID := range traceIDs {
		spans, err := store.GetTrace(ctx, traceID)
		if err != nil {
			continue
		}
		fmt.Printf("\nTrace ID: %s (%d spans)\n", traceID, len(spans))
		printSpanTree(spans, 0)
	}
}

func printSpanTree(spans []*trace.Span, indent int) {
	printNodes(spans, "", indent)
}

func printNodes(allSpans []*trace.Span, parentID string, indent int) {
	prefix := ""
	for i := 0; i < indent; i++ {
		prefix += "  "
	}

	for _, span := range allSpans {
		if span.ParentID != parentID {
			continue
		}

		statusIcon := "OK"
		if span.Status == trace.SpanStatusError {
			statusIcon = "ERR"
		}

		typeLabel := ""
		switch span.Type {
		case trace.SpanTypeCommand:
			typeLabel = "CMD"
		case trace.SpanTypeQuery:
			typeLabel = "QRY"
		case trace.SpanTypeEvent:
			typeLabel = "EVT"
		}

		fmt.Printf("%s[%s] %s %s (%v)\n",
			prefix, typeLabel, span.Name, statusIcon, span.Duration)

		printNodes(allSpans, span.ID, indent+1)
	}
}

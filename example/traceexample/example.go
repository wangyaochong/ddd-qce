package traceexample

import (
	"context"
	"fmt"
	"time"

	"github.com/ddd-qce/core/domain/event"
	"github.com/ddd-qce/core/trace"
	commandmemory "github.com/ddd-qce/core/cqrs/command/memory"
	eventmemory "github.com/ddd-qce/core/cqrs/event/memory"
	querymemory "github.com/ddd-qce/core/cqrs/query/memory"
)

type PlaceOrderCommand struct {
	UserID  string
	Product string
	Amount  float64
}

type PlaceOrderResult struct {
	OrderID string
}

type PlaceOrderHandler struct {
	eventGroup *eventmemory.EventBusGroup
}

func (h *PlaceOrderHandler) Handle(ctx context.Context, cmd *PlaceOrderCommand) (*PlaceOrderResult, error) {
	orderID := "ORD-" + time.Now().Format("20060102150405")

	eventmemory.EventGroupPublish[*OrderPlacedEvent](h.eventGroup, ctx, &OrderPlacedEvent{
		OrderID: orderID,
		UserID:  cmd.UserID,
		Product: cmd.Product,
		Amount:  cmd.Amount,
		Time:    time.Now(),
	})

	return &PlaceOrderResult{OrderID: orderID}, nil
}

type OrderPlacedEvent struct {
	OrderID string
	UserID  string
	Product string
	Amount  float64
	Time    time.Time
}

func (e *OrderPlacedEvent) AggregateID() string   { return e.OrderID }
func (e *OrderPlacedEvent) EventType() string     { return event.EventTypeOf(e) }
func (e *OrderPlacedEvent) OccurredAt() time.Time { return e.Time }

type OrderPlacedEventHandler struct {
	cmdBus *commandmemory.CommandBus
}

func (h *OrderPlacedEventHandler) Handle(ctx context.Context, event *OrderPlacedEvent) error {
	fmt.Printf("  [EventHandler] Processing OrderPlaced for order %s\n", event.OrderID)

	commandmemory.Dispatch[*SendNotificationCommand, *SendNotificationResult](h.cmdBus, ctx, &SendNotificationCommand{
		UserID:  event.UserID,
		Message: fmt.Sprintf("Order %s placed: %s ($%.2f)", event.OrderID, event.Product, event.Amount),
	})

	commandmemory.Dispatch[*UpdateInventoryCommand, *UpdateInventoryResult](h.cmdBus, ctx, &UpdateInventoryCommand{
		Product:  event.Product,
		Quantity: 1,
	})

	return nil
}

type SendNotificationCommand struct {
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

func RegisterHandlers(cmdBus *commandmemory.CommandBus, eventGroup *eventmemory.EventBusGroup, qBus *querymemory.QueryBus) {
	commandmemory.RegisterCommand(cmdBus, &PlaceOrderHandler{eventGroup: eventGroup})
	commandmemory.RegisterCommand(cmdBus, &SendNotificationHandler{})
	commandmemory.RegisterCommand(cmdBus, &UpdateInventoryHandler{})

	eventmemory.EventGroupBus[*OrderPlacedEvent](eventGroup).Subscribe(&OrderPlacedEventHandler{cmdBus: cmdBus})

	querymemory.RegisterQuery(qBus, &GetOrderStatusHandler{})
}

func RunExample(ctx context.Context, cmdBus *commandmemory.CommandBus, eventGroup *eventmemory.EventBusGroup, qBus *querymemory.QueryBus) {
	RegisterHandlers(cmdBus, eventGroup, qBus)

	fmt.Println("========================================")
	fmt.Println("  Cross-Domain Trace Example")
	fmt.Println("========================================")
	fmt.Println()
	fmt.Println("=== PlaceOrder (Command → Event → Commands) ===")

	result, err := commandmemory.Dispatch[*PlaceOrderCommand, *PlaceOrderResult](cmdBus, ctx, &PlaceOrderCommand{
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
	qResult, err := querymemory.Ask[*GetOrderStatusQuery, *GetOrderStatusResult](qBus, ctx, &GetOrderStatusQuery{OrderID: result.OrderID})
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

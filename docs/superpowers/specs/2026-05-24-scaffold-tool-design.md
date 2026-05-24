# DDD-QCE 脚手架工具设计

**Date**: 2026-05-24
**Status**: Draft
**Related**: 无

## 1. 背景与目标

### 1.1 问题陈述

当前 `ddd-qce` 框架没有脚手架工具。用户创建新的聚合（Aggregate）时，需要手动参照 `exampleapp` 目录创建大量样板代码：
- domain/model.go - 聚合根 + 实体 + 值对象
- domain/events.go - 领域事件
- application/commands.go - Command 定义
- application/command_handlers.go - Command Handler
- application/query_handlers.go - Query Handler
- application/event_handlers.go - Event Handler
- application/repositories.go - Repository 适配器

手动创建存在以下问题：
1. **容易遗漏文件**：每次新建聚合需要创建 8-10 个文件，容易遗漏
2. **违反分层规则**：应用层直接 import 具体实现，破坏分层架构
3. **AI 生成不稳定**：AI 可能自行编造结构，导致违反框架约定
4. **新用户门槛高**：必须完整阅读 exampleapp 才能开始

### 1.2 目标

提供 `ddd new aggregate <Name>` 命令，自动生成符合框架约定的聚合骨架代码。

**核心价值**：
- 确保 AI 生成代码符合约定结构，防止越界
- 降低新用户入门门槛
- 作为框架完整性的标志

### 1.3 成功标准

1. 执行 `go run ./cmd/ddd new aggregate Order --module github.com/myorg/myapp` 能在当前目录生成完整的聚合骨架
2. 生成的代码符合 ddd-qce 框架的所有架构约定（分层、接口隔离、事件溯源模式等）
3. 生成后可以直接编译运行（只需补充业务逻辑）
4. docs/guide.md 新增脚手架使用说明章节

## 2. 技术方案

### 2.1 工具形式

- **Go CLI**：位于 `cmd/ddd/main.go`
- **运行方式**：`go run ./cmd/ddd new aggregate Order --module github.com/myorg/myapp`
- **技术栈**：Go + `text/template`（标准库内嵌模板）

### 2.2 为什么选择 Go CLI + 内嵌模板

| 方案 | 优点 | 缺点 |
|------|------|------|
| 方案 A: Go CLI + 内嵌模板 | 与项目技术栈一致；可被 AI 直接调用；模板与框架版本同步 | 模板写在代码中，维护时需改 Go 代码 |
| 方案 B: Go CLI + 外部模板 | 模板与逻辑分离，模板更易读易改 | 多一层间接；模板目录结构需要约定 |
| 方案 C: Shell 脚本 | 最简单，无依赖 | 跨平台差；模板逻辑弱；维护困难 |

**选择方案 A**：实现简洁，与框架 Go 技术栈一致，AI 可直接调用，模板内嵌避免外部依赖问题。

### 2.3 参数设计

| 参数 | 必填 | 说明 |
|------|------|------|
| `new aggregate` | 是 | 子命令，表示新建聚合 |
| `<Name>` | 是 | 聚合名称，采用 PascalCase，如 `Order`、`Inventory` |
| `--module` | 是 | 目标模块名，如 `github.com/myorg/myapp`，用于生成正确的 import 路径 |

## 3. 生成的代码结构

### 3.1 文件清单

执行 `ddd new aggregate Order --module github.com/myorg/myapp` 将在当前目录生成以下文件：

```
<current-dir>/
├── domain/
│   ├── order.go              # 聚合根 + 实体 + 状态常量
│   ├── order_events.go       # 领域事件定义
│   └── order_test.go         # 基础测试用例
├── application/
│   ├── order_commands.go    # Command + Result 定义
│   ├── order_cmd_handler.go # Command Handler
│   ├── order_query_handler.go # Query Handler
│   ├── order_event_handler.go # Event Handler（空实现模板）
│   └── order_repository.go  # Repository 适配器
└── （infrastructure/wire.go 注册代码通过 stdout 输出）
```

### 3.2 模板内容规范

#### domain/order.go

```go
package domain

import (
    "fmt"
    "time"

    "github.com/ddd-qce/core/domain/aggregate"
    "github.com/ddd-qce/core/domain/entity"
    "github.com/ddd-qce/core/domain/event"
)

type OrderStatus string

const (
    OrderStatusPending   OrderStatus = "pending"
    OrderStatusConfirmed OrderStatus = "confirmed"
    OrderStatusShipped   OrderStatus = "shipped"
    OrderStatusCancelled OrderStatus = "cancelled"
)

type OrderItem struct {
    entity.Entity
    ProductName string
    Price       float64
    Quantity    int
}

func NewOrderItem(id, productName string, price float64, quantity int) *OrderItem {
    return &OrderItem{
        Entity:      *entity.NewEntity(id),
        ProductName: productName,
        Price:       price,
        Quantity:    quantity,
    }
}

func (i *OrderItem) Subtotal() float64 {
    return i.Price * float64(i.Quantity)
}

type Order struct {
    aggregate.AggregateRoot
    UserID      string
    Items       []*OrderItem
    Status      OrderStatus
    TotalAmount float64
    CreatedAt   time.Time
}

func NewOrder(id, userID string, items []*OrderItem) (*Order, error) {
    o := &Order{
        UserID:    userID,
        Items:     items,
        Status:    OrderStatusPending,
        CreatedAt: time.Now(),
    }
    o.AggregateRoot = *aggregate.NewAggregateRootWithApplier(id, o)
    if err := o.validate(); err != nil {
        return nil, err
    }
    o.TotalAmount = o.calculateTotal()
    if err := o.Apply(&OrderCreatedEvent{
        BaseEvent:   event.NewBaseEvent(o.GetID(), time.Now()),
        UserID:      o.UserID,
        TotalAmount: o.TotalAmount,
    }); err != nil {
        return nil, err
    }
    return o, nil
}

func NewOrderForReplay(id string) *Order {
    o := &Order{}
    o.AggregateRoot = *aggregate.NewAggregateRootWithApplier(id, o)
    return o
}

func (o *Order) When(evt event.DomainEvent) {
    switch e := evt.(type) {
    case *OrderCreatedEvent:
        o.UserID = e.UserID
        o.TotalAmount = e.TotalAmount
        o.Status = OrderStatusPending
        o.CreatedAt = e.OccurredAt()
    // TODO: Add event handlers for other events
    }
}

func (o *Order) Confirm() error {
    if o.Status != OrderStatusPending {
        return fmt.Errorf("order can only be confirmed from pending status")
    }
    if err := o.Apply(&OrderConfirmedEvent{
        BaseEvent: event.NewBaseEvent(o.GetID(), time.Now()),
    }); err != nil {
        return err
    }
    return nil
}

func (o *Order) Cancel() error {
    if o.Status == OrderStatusShipped {
        return fmt.Errorf("cannot cancel shipped order")
    }
    if err := o.Apply(&OrderCancelledEvent{
        BaseEvent: event.NewBaseEvent(o.GetID(), time.Now()),
    }); err != nil {
        return err
    }
    return nil
}

func (o *Order) validate() error {
    if err := o.AggregateRoot.Validate(); err != nil {
        return err
    }
    if o.UserID == "" {
        return fmt.Errorf("order must have a user ID")
    }
    if len(o.Items) == 0 {
        return fmt.Errorf("order must have at least one item")
    }
    for _, item := range o.Items {
        if item.IsEmpty() {
            return fmt.Errorf("order item has empty product ID")
        }
    }
    return nil
}

func (o *Order) calculateTotal() float64 {
    var total float64
    for _, item := range o.Items {
        total += item.Subtotal()
    }
    return total
}
```

#### domain/order_events.go

```go
package domain

import (
    "github.com/ddd-qce/core/domain/event"
)

type OrderCreatedEvent struct {
    event.BaseEvent
    UserID      string
    TotalAmount float64
}

type OrderConfirmedEvent struct {
    event.BaseEvent
}

type OrderShippedEvent struct {
    event.BaseEvent
}

type OrderCancelledEvent struct {
    event.BaseEvent
    Reason string
}
```

#### domain/order_test.go

```go
package domain

import (
    "testing"
    "time"

    "github.com/ddd-qce/core/domain/aggregate"
    "github.com/ddd-qce/core/domain/event"
)

func TestOrderAggregate_Create(t *testing.T) {
    items := []*OrderItem{
        NewOrderItem("prod-1", "Product A", 100.0, 2),
    }
    order, err := NewOrder("order-1", "user-1", items)
    if err != nil {
        t.Fatalf("failed to create order: %v", err)
    }
    if order.Status != OrderStatusPending {
        t.Errorf("expected pending status, got %s", order.Status)
    }
    if order.TotalAmount != 200.0 {
        t.Errorf("expected 200.0, got %f", order.TotalAmount)
    }
    events := order.UncommittedEvents()
    if len(events) != 1 {
        t.Fatalf("expected 1 event, got %d", len(events))
    }
}

func TestOrderAggregate_Confirm(t *testing.T) {
    items := []*OrderItem{NewOrderItem("prod-1", "Product A", 100.0, 1)}
    order, _ := NewOrder("order-1", "user-1", items)
    order.MarkEventsAsCommitted()

    if err := order.Confirm(); err != nil {
        t.Fatalf("confirm failed: %v", err)
    }
    if order.Status != OrderStatusConfirmed {
        t.Errorf("expected confirmed, got %s", order.Status)
    }
}

func TestOrderAggregate_Cancel(t *testing.T) {
    items := []*OrderItem{NewOrderItem("prod-1", "Product A", 100.0, 1)}
    order, _ := NewOrder("order-1", "user-1", items)
    order.MarkEventsAsCommitted()

    if err := order.Cancel(); err != nil {
        t.Fatalf("cancel failed: %v", err)
    }
    if order.Status != OrderStatusCancelled {
        t.Errorf("expected cancelled, got %s", order.Status)
    }
}

func TestOrderAggregate_When(t *testing.T) {
    o := &Order{}
    o.AggregateRoot = *aggregate.NewAggregateRootWithApplier("order-1", o)
    _ = o.LoadFromHistory([]event.DomainEvent{
        &OrderCreatedEvent{BaseEvent: event.NewBaseEvent("order-1", time.Now()), UserID: "user-1", TotalAmount: 100},
    })
    if o.Status != OrderStatusPending {
        t.Errorf("expected pending, got %s", o.Status)
    }
    if o.UserID != "user-1" {
        t.Errorf("expected user-1, got %s", o.UserID)
    }
}
```

#### application/order_commands.go

```go
package application

import (
    "github.com/ddd-qce/core/cqrs/command"
    "github.com/ddd-qce/core/cqrs/query"
)

type ItemInput struct {
    ProductID   string
    ProductName string
    Price       float64
    Quantity    int
}

type CreateOrderCommand struct {
    command.BaseCommand
    UserID string
    Items  []ItemInput
}

type CreateOrderResult struct {
    OrderID     string
    TotalAmount float64
}

type GetOrderQuery struct {
    query.BaseQuery
    OrderID string
}

type OrderViewItem struct {
    ProductID   string
    ProductName string
    Price       float64
    Quantity    int
    Subtotal    float64
}

type GetOrderResult struct {
    OrderID     string
    UserID      string
    Status      string
    TotalAmount float64
    Items       []OrderViewItem
    CreatedAt   string
}

type ListOrdersQuery struct {
    query.BaseQuery
}

type ListOrdersResult struct {
    Orders []GetOrderResult
}
```

#### application/order_cmd_handler.go

```go
package application

import (
    "context"
    "time"

    "github.com/google/uuid"
    "github.com/ddd-qce/core/cqrs/command"
    cqrsevent "github.com/ddd-qce/core/cqrs/event"
    domainevent "github.com/ddd-qce/core/domain/event"
    "github.com/ddd-qce/exampleapp/domain"
)

type CreateOrderHandler struct {
    repo     OrderRepositoryAdapter
    eventBus cqrsevent.EventBus
}

func NewCreateOrderHandler(repo OrderRepositoryAdapter, eventBus cqrsevent.EventBus) *CreateOrderHandler {
    return &CreateOrderHandler{repo: repo, eventBus: eventBus}
}

func (h *CreateOrderHandler) Handle(ctx context.Context, cmd *CreateOrderCommand) (*CreateOrderResult, error) {
    items := make([]*domain.OrderItem, len(cmd.Items))
    for i, input := range cmd.Items {
        items[i] = domain.NewOrderItem(uuid.New().String(), input.ProductName, input.Price, input.Quantity)
    }

    orderID := uuid.New().String()
    order, err := domain.NewOrder(orderID, cmd.UserID, items)
    if err != nil {
        return nil, err
    }

    if err := h.repo.Save(ctx, order); err != nil {
        return nil, err
    }

    cqrsevent.Dispatch[*domain.OrderCreatedEvent](ctx, h.eventBus, &domain.OrderCreatedEvent{
        BaseEvent:   domainevent.NewBaseEvent(order.GetID(), time.Now()),
        UserID:      order.UserID,
        TotalAmount: order.TotalAmount,
    })

    return &CreateOrderResult{OrderID: order.GetID(), TotalAmount: order.TotalAmount}, nil
}

var _ command.CommandHandler[*CreateOrderCommand, *CreateOrderResult] = (*CreateOrderHandler)(nil)
```

#### application/order_query_handler.go

package application

```go
import (
    "context"
    "time"

    "github.com/ddd-qce/core/cqrs/query"
    "github.com/ddd-qce/exampleapp/domain"
)

type GetOrderHandler struct {
    repo OrderRepositoryAdapter
}

func NewGetOrderHandler(repo OrderRepositoryAdapter) *GetOrderHandler {
    return &GetOrderHandler{repo: repo}
}

func (h *GetOrderHandler) Handle(ctx context.Context, q *GetOrderQuery) (*GetOrderResult, error) {
    order, err := h.repo.FindByID(ctx, q.OrderID)
    if err != nil {
        return nil, err
    }
    return toOrderView(order), nil
}

type ListOrdersHandler struct {
    repo OrderRepositoryAdapter
}

func NewListOrdersHandler(repo OrderRepositoryAdapter) *ListOrdersHandler {
    return &ListOrdersHandler{repo: repo}
}

func (h *ListOrdersHandler) Handle(ctx context.Context, q *ListOrdersQuery) (*ListOrdersResult, error) {
    orders := h.repo.FindAll()
    result := make([]GetOrderResult, len(orders))
    for i, o := range orders {
        result[i] = *toOrderView(o)
    }
    return &ListOrdersResult{Orders: result}, nil
}

var _ query.QueryHandler[*GetOrderQuery, *GetOrderResult] = (*GetOrderHandler)(nil)
var _ query.QueryHandler[*ListOrdersQuery, *ListOrdersResult] = (*ListOrdersHandler)(nil)

func toOrderView(o *domain.Order) *GetOrderResult {
    items := make([]OrderViewItem, len(o.Items))
    for i, item := range o.Items {
        items[i] = OrderViewItem{
            ProductID:   item.GetID(),
            ProductName: item.ProductName,
            Price:       item.Price,
            Quantity:    item.Quantity,
            Subtotal:    item.Subtotal(),
        }
    }
    result := &GetOrderResult{
        OrderID:     o.GetID(),
        UserID:      o.UserID,
        Status:      string(o.Status),
        TotalAmount: o.TotalAmount,
        Items:       items,
    }
    if !o.CreatedAt.IsZero() {
        result.CreatedAt = o.CreatedAt.Format(time.RFC3339)
    }
    return result
}
```

#### application/order_event_handler.go

```go
package application

import (
    "context"
    "log"

    "github.com/ddd-qce/exampleapp/domain"
)

type OrderCreatedNotificationHandler struct{}

func NewOrderCreatedNotificationHandler() *OrderCreatedNotificationHandler {
    return &OrderCreatedNotificationHandler{}
}

func (h *OrderCreatedNotificationHandler) Handle(ctx context.Context, evt *domain.OrderCreatedEvent) error {
    log.Printf("[Notification] Order %s created by user %s, total: $%.2f",
        evt.AggregateID(), evt.UserID, evt.TotalAmount)
    return nil
}
```

#### application/order_repository.go

```go
package application

import (
    "context"
    "fmt"
    "sync"

    "github.com/ddd-qce/core/domain/event"
    "github.com/ddd-qce/core/domain/repository"
    "github.com/ddd-qce/exampleapp/domain"
)

type OrderRepositoryAdapter interface {
    Save(ctx context.Context, order *domain.Order) error
    FindByID(ctx context.Context, id string) (*domain.Order, error)
    Delete(ctx context.Context, id string) error
    FindAll() []*domain.Order
}

type OrderRepository struct {
    mu     sync.RWMutex
    orders map[string]*domain.Order
}

func NewOrderRepository() *OrderRepository {
    return &OrderRepository{orders: make(map[string]*domain.Order)}
}

func (r *OrderRepository) Save(ctx context.Context, order *domain.Order) error {
    r.mu.Lock()
    defer r.mu.Unlock()
    r.orders[order.GetID()] = order
    return nil
}

func (r *OrderRepository) FindByID(ctx context.Context, id string) (*domain.Order, error) {
    r.mu.RLock()
    defer r.mu.RUnlock()
    order, ok := r.orders[id]
    if !ok {
        return nil, fmt.Errorf("order %s not found", id)
    }
    return order, nil
}

func (r *OrderRepository) Delete(ctx context.Context, id string) error {
    r.mu.Lock()
    defer r.mu.Unlock()
    if _, ok := r.orders[id]; !ok {
        return fmt.Errorf("order %s not found", id)
    }
    delete(r.orders, id)
    return nil
}

func (r *OrderRepository) FindAll() []*domain.Order {
    r.mu.RLock()
    defer r.mu.RUnlock()
    result := make([]*domain.Order, 0, len(r.orders))
    for _, o := range r.orders {
        result = append(result, o)
    }
    return result
}

var _ repository.Repository[*domain.Order] = (*OrderRepository)(nil)

type OrderEventSourcedRepository struct {
    eventStore event.EventStore[event.DomainEvent]
    orderRepo  OrderRepositoryAdapter
}

func NewOrderEventSourcedRepository(
    eventStore event.EventStore[event.DomainEvent],
    orderRepo OrderRepositoryAdapter,
) *OrderEventSourcedRepository {
    return &OrderEventSourcedRepository{
        eventStore: eventStore,
        orderRepo:  orderRepo,
    }
}

func (r *OrderEventSourcedRepository) Save(ctx context.Context, order *domain.Order) error {
    uncommitted := order.UncommittedEvents()
    if len(uncommitted) > 0 {
        if err := r.eventStore.Append(ctx, order.GetID(), order.Version()-len(uncommitted), uncommitted); err != nil {
            return err
        }
        order.MarkEventsAsCommitted()
    }
    return r.orderRepo.Save(ctx, order)
}

func (r *OrderEventSourcedRepository) Load(ctx context.Context, id string) (*domain.Order, error) {
    events, err := r.eventStore.Load(ctx, id, 0)
    if err != nil {
        return nil, err
    }
    if len(events) == 0 {
        return nil, fmt.Errorf("order %s not found in event store", id)
    }
    order := domain.NewOrderForReplay(id)
    if err := order.LoadFromHistory(events); err != nil {
        return nil, fmt.Errorf("load order %s from history: %w", id, err)
    }
    return order, nil
}

var _ repository.EventSourcingRepository[*domain.Order] = (*OrderEventSourcedRepository)(nil)
```

#### infrastructure/wire.go 注册代码（stdout 输出）

```
// ============================================================
// Wire Registration Snippet - Copy to infrastructure/wire.go
// ============================================================

// In WireAppWithStore function, add after existing handler registrations:

// Order handlers
if err := cmdBus.RegisterHandler(application.NewCreateOrderHandler(orderRepo, eventBus)); err != nil {
    return nil, fmt.Errorf("register CreateOrderHandler: %w", err)
}

if err := queryBus.RegisterHandler(application.NewGetOrderHandler(orderRepo)); err != nil {
    return nil, fmt.Errorf("register GetOrderHandler: %w", err)
}
if err := queryBus.RegisterHandler(application.NewListOrdersHandler(orderRepo)); err != nil {
    return nil, fmt.Errorf("register ListOrdersHandler: %w", err)
}

if err := eventBus.SubscribeHandler(application.NewOrderCreatedNotificationHandler()); err != nil {
    return nil, fmt.Errorf("register OrderCreatedNotificationHandler: %w", err)
}
```

## 4. 实现计划

### 4.1 阶段一：CLI 框架（优先级：高）

1. 创建 `cmd/ddd/main.go`
2. 实现基础 CLI 框架（cobra 或标准库 flag）
3. 支持 `new aggregate <Name> --module <module>` 命令
4. 添加 `--help` 和基本错误处理

### 4.2 阶段二：模板引擎（优先级：高）

1. 创建 `cmd/ddd/templates/` 目录
2. 实现 `text/template` 模板生成逻辑
3. 为每个输出文件编写 Go 模板
4. 处理 PascalCase → camelCase 转换（`Order` → `order`）

### 4.3 阶段三：测试与验证（优先级：中）

1. 编写单元测试验证模板输出
2. 实际运行 `go run ./cmd/ddd new aggregate TestAgg --module github.com/test/test` 并验证生成代码的可编译性

### 4.4 阶段四：文档完善（优先级：中）

1. 在 `docs/guide.md` 新增章节：快速开始 - 使用脚手架
2. 在 `docs/architecture.md` 新增"脚手架工具"章节

## 5. 风险与限制

### 5.1 已知限制

1. **目标模块必须是 Go workspace 当前成员**：脚手架假设运行在已配置好的 Go workspace 环境中，无法处理任意路径的模块
2. **不会自动修改 infrastructure/wire.go**：生成注册代码片段，需要用户手动复制粘贴
3. **不处理多模块场景**：假设单模块应用，多模块场景需多次运行
4. **业务逻辑需手动补充**：生成的只是骨架，业务逻辑必须手动编写

### 5.2 潜在风险

1. **模板与框架 API 不同步**：框架 API 变更时需要同步更新模板
   - 缓解：模板尽量使用稳定的核心接口
2. **用户可能混淆模块名**：需要清晰说明 `--module` 参数的格式

## 6. 验收标准

### 6.1 功能验收

- [ ] 执行 `go run ./cmd/ddd new aggregate Order --module github.com/myorg/myapp` 成功生成文件
- [ ] 生成的 domain/order.go 包含完整的聚合根结构（含 NewOrder、When、Confirm、Cancel 方法）
- [ ] 生成的 domain/order_events.go 包含 4 个基础事件定义
- [ ] 生成的 application/* 包含完整的 Command/Query Handler
- [ ] 生成的代码可以直接编译（go build）
- [ ] 生成的代码包含正确的 import 路径（基于 --module 参数）

### 6.2 文档验收

- [ ] docs/guide.md 包含"使用脚手架创建聚合"章节
- [ ] docs/architecture.md 包含脚手架工具说明

### 6.3 架构验收

- [ ] 生成的代码遵循分层规则：application 层不直接 import infrastructure 实现
- [ ] 生成的代码使用框架核心接口（command.CommandHandler, query.QueryHandler, event.EventHandler）
- [ ] 生成的代码包含 `var _ Interface = (*Impl)(nil)` 编译检查模式

## 7. 后续扩展（Out of Scope）

以下功能在当前版本中不考虑，但为未来扩展预留设计空间：

1. **Entity 脚手架**：`ddd new entity <Name>` 生成独立的 Entity 模板
2. **ValueObject 脚手架**：`ddd new valueobject <Name>` 生成值对象模板
3. **Event Handler 脚手架**：交互式生成 Event Handler 模板
4. **交互式模式**：`ddd new` 进入交互式问答模式
5. **模板自定义**：用户可以提供自定义模板目录覆盖默认模板
# 实战指南

## 一、Aggregate Root 使用指南

### 1. 定义聚合根

```go
package domain

import "github.com/ddd-qce/core/core"

type Order struct {
    core.AggregateRoot
    Items   []OrderItem
    Status  OrderStatus
    Total   float64
}

type OrderItem struct {
    ProductID string
    Quantity  int
    Price     float64
}

type OrderStatus string

const (
    OrderStatusPending   OrderStatus = "pending"
    OrderStatusConfirmed OrderStatus = "confirmed"
    OrderStatusShipped   OrderStatus = "shipped"
    OrderStatusCancelled OrderStatus = "cancelled"
)
```

### 2. 聚合根工厂方法

```go
func NewOrder(orderID string, items []OrderItem) *Order {
    order := &Order{
        AggregateRoot: *core.NewAggregateRoot(orderID),
        Items:         items,
        Status:        OrderStatusPending,
    }

    // 计算总价
    var total float64
    for _, item := range items {
        total += item.Price * float64(item.Quantity)
    }
    order.Total = total

    // 产生领域事件
    order.Apply(OrderCreatedEvent{
        OrderID: orderID,
        Items:   items,
        Total:   total,
    })

    return order
}
```

### 3. 聚合根业务方法

```go
func (o *Order) Confirm() error {
    if o.Status != OrderStatusPending {
        return fmt.Errorf("order can only be confirmed from pending status")
    }

    o.Status = OrderStatusConfirmed

    o.Apply(OrderConfirmedEvent{
        OrderID: o.ID,
    })

    return nil
}

func (o *Order) Cancel() error {
    if o.Status == OrderStatusShipped {
        return fmt.Errorf("cannot cancel shipped order")
    }

    oldStatus := o.Status
    o.Status = OrderStatusCancelled

    o.Apply(OrderCancelledEvent{
        OrderID:  o.ID,
        OldStatus: oldStatus,
    })

    return nil
}
```

### 4. 获取未提交的事件

```go
func (h *CreateOrderHandler) Handle(ctx context.Context, cmd CreateOrderCommand) (string, error) {
    order := NewOrder(uuid.New().String(), cmd.Items)

    // 获取未提交的领域事件
    events := order.UncommittedEvents()

    // 保存聚合根
    if err := h.repo.Save(ctx, order); err != nil {
        return "", err
    }

    // 发布事件
    for _, event := range events {
        h.eventBus.Publish(ctx, event)
    }

    // 标记事件已提交
    order.MarkEventsAsCommitted()

    return order.ID, nil
}
```

### 5. 事件溯源模式（从事件流重建聚合）

```go
func (o *Order) ApplyEvent(event core.DomainEvent) {
    switch e := event.(type) {
    case OrderCreatedEvent:
        o.Items = e.Items
        o.Total = e.Total
        o.Status = OrderStatusPending
    case OrderConfirmedEvent:
        o.Status = OrderStatusConfirmed
    case OrderCancelledEvent:
        o.Status = OrderStatusCancelled
    }
}

// 从事件存储加载
func (r *OrderRepository) Load(ctx context.Context, id string) (*Order, error) {
    events, err := r.eventStore.Load(ctx, id, 0)
    if err != nil {
        return nil, err
    }

    order := &Order{
        AggregateRoot: *core.NewAggregateRoot(id),
    }

    // 重放事件重建状态
    for _, event := range events {
        order.ApplyEvent(event)
        order.Version++
    }

    return order, nil
}
```

---

## 二、Entity 使用指南

### 1. 定义实体

```go
package domain

import "github.com/ddd-qce/core/core"

type Product struct {
    core.Entity
    Name        string
    Price       float64
    Description string
}

func NewProduct(id, name string, price float64) *Product {
    return &Product{
        Entity: *core.NewEntity(id),
        Name:   name,
        Price:  price,
    }
}
```

### 2. 实体相等性判断

```go
func main() {
    p1 := NewProduct("prod-1", "Product A", 100.0)
    p2 := NewProduct("prod-1", "Product A Updated", 120.0)
    p3 := NewProduct("prod-2", "Product B", 50.0)

    // 基于 ID 判断相等性（即使其他字段不同）
    fmt.Println(p1.Equals(&p2.Entity)) // true
    fmt.Println(p1.Equals(&p3.Entity)) // false
}
```

### 3. 空值判断

```go
func (r *ProductRepository) FindByID(ctx context.Context, id string) (*Product, error) {
    product := r.cache.Get(id)
    if product != nil && !product.IsEmpty() {
        return product, nil
    }
    return nil, ErrNotFound
}
```

---

## 三、Repository 使用指南

### 1. 标准仓储接口

```go
type Repository[T any] interface {
    Save(ctx context.Context, aggregate T) error
    FindByID(ctx context.Context, id string) (T, error)
    Delete(ctx context.Context, id string) error
}
```

### 2. 实现仓储

```go
package repository

import (
    "context"
    "github.com/ddd-qce/core/core"
)

type OrderRepository struct {
    db       *sql.DB
    eventBus event.Bus
}

func NewOrderRepository(db *sql.DB, eventBus event.Bus) *OrderRepository {
    return &OrderRepository{db: db, eventBus: eventBus}
}

func (r *OrderRepository) Save(ctx context.Context, order *Order) error {
    tx, err := r.db.BeginTx(ctx, nil)
    if err != nil {
        return err
    }
    defer tx.Rollback()

    // 保存聚合根状态
    _, err = tx.ExecContext(ctx,
        "INSERT OR REPLACE INTO orders (id, status, total, version) VALUES (?, ?, ?, ?)",
        order.ID, order.Status, order.Total, order.Version,
    )
    if err != nil {
        return err
    }

    // 保存订单项
    for _, item := range order.Items {
        _, err = tx.ExecContext(ctx,
            "INSERT INTO order_items (order_id, product_id, quantity, price) VALUES (?, ?, ?, ?)",
            order.ID, item.ProductID, item.Quantity, item.Price,
        )
        if err != nil {
            return err
        }
    }

    return tx.Commit()
}

func (r *OrderRepository) FindByID(ctx context.Context, id string) (*Order, error) {
    var order Order
    err := r.db.QueryRowContext(ctx,
        "SELECT id, status, total, version FROM orders WHERE id = ?", id,
    ).Scan(&order.ID, &order.Status, &order.Total, &order.Version)

    if err != nil {
        return nil, err
    }

    // 加载订单项
    rows, err := r.db.QueryContext(ctx,
        "SELECT product_id, quantity, price FROM order_items WHERE order_id = ?", id,
    )
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    for rows.Next() {
        var item OrderItem
        if err := rows.Scan(&item.ProductID, &item.Quantity, &item.Price); err != nil {
            return nil, err
        }
        order.Items = append(order.Items, item)
    }

    return &order, nil
}

func (r *OrderRepository) Delete(ctx context.Context, id string) error {
    _, err := r.db.ExecContext(ctx, "DELETE FROM orders WHERE id = ?", id)
    return err
}
```

### 3. 事件溯源仓储接口

```go
type EventSourcingRepository[T any] interface {
    Save(ctx context.Context, aggregate T) error
    Load(ctx context.Context, id string) (T, error)
}
```

### 4. 实现事件溯源仓储

```go
type OrderESRepository struct {
    eventStore core.EventStore[core.DomainEvent]
    eventBus   event.Bus
}

func NewOrderESRepository(eventStore core.EventStore[core.DomainEvent], eventBus event.Bus) *OrderESRepository {
    return &OrderESRepository{eventStore: eventStore, eventBus: eventBus}
}

func (r *OrderESRepository) Save(ctx context.Context, order *Order) error {
    events := order.UncommittedEvents()
    if len(events) == 0 {
        return nil
    }

    // 追加事件到事件存储
    if err := r.eventStore.Append(ctx, events); err != nil {
        return err
    }

    // 发布事件
    for _, event := range events {
        r.eventBus.Publish(ctx, event)
    }

    // 标记事件已提交
    order.MarkEventsAsCommitted()

    return nil
}

func (r *OrderESRepository) Load(ctx context.Context, id string) (*Order, error) {
    events, err := r.eventStore.Load(ctx, id, 0)
    if err != nil {
        return nil, err
    }

    order := &Order{
        AggregateRoot: *core.NewAggregateRoot(id),
    }

    // 重放事件重建状态
    for _, event := range events {
        order.ApplyEvent(event)
        order.Version++
    }

    return order, nil
}
```

### 5. 使用仓储

```go
func (h *ConfirmOrderHandler) Handle(ctx context.Context, cmd ConfirmOrderCommand) error {
    // 加载聚合
    order, err := h.repo.FindByID(ctx, cmd.OrderID)
    if err != nil {
        return err
    }

    // 执行业务逻辑
    if err := order.Confirm(); err != nil {
        return err
    }

    // 保存聚合
    return h.repo.Save(ctx, order)
}
```

---

## 四、Command 使用指南

### 1. 定义 Command 结构

```go
package command

type CreateUserCommand struct {
    Name  string
    Email string
    Age   int
}
```

### 2. 定义 Handler

```go
package command

import (
    "context"
    "github.com/ddd-qce/core"
)

type CreateUserHandler struct {
    userRepo UserRepository
}

func (h *CreateUserHandler) Handle(ctx context.Context, cmd CreateUserCommand) (string, error) {
    user := NewUser(cmd.Name, cmd.Email, cmd.Age)
    
    if err := h.userRepo.Save(ctx, user); err != nil {
        return "", err
    }
    
    return user.ID, nil
}
```

### 3. 注册与执行

```go
import (
    "context"
    commandmemory "github.com/ddd-qce/core/command/memory"
    "github.com/ddd-qce/core/aspect"
)

func main() {
    ctx := context.Background()
    
    // 创建切面链和 Bus
    chain := aspect.NewAspectChain()
    bus := commandmemory.NewCommandBus(chain)
    
    // 注册 Handler
    commandmemory.RegisterCommand(bus, &CreateUserHandler{userRepo: repo})
    
    // 执行 Command
    userID, err := commandmemory.Dispatch[CreateUserCommand, string](
        ctx, bus, CreateUserCommand{
            Name:  "Alice",
            Email: "alice@example.com",
            Age:   25,
        },
    )
}
```

### 4. 无返回值的 Command

```go
type DeleteUserCommand struct {
    UserID string
}

type DeleteUserHandler struct{}

func (h *DeleteUserHandler) Handle(ctx context.Context, cmd DeleteUserCommand) (struct{}, error) {
    // 执行删除...
    return struct{}{}, nil
}

// 执行
_, err := commandmemory.Dispatch[DeleteUserCommand, struct{}](ctx, bus, cmd)
```

---

## 五、Query 使用指南

### 1. 定义 Query 结构

```go
package query

type GetUserQuery struct {
    UserID string
}

type GetUserResult struct {
    ID    string
    Name  string
    Email string
}
```

### 2. 定义 Handler

```go
package query

import "context"

type GetUserHandler struct {
    userRepo UserRepository
}

func (h *GetUserHandler) Handle(ctx context.Context, q GetUserQuery) (*GetUserResult, error) {
    user, err := h.userRepo.FindByID(ctx, q.UserID)
    if err != nil {
        return nil, err
    }
    
    return &GetUserResult{
        ID:    user.ID,
        Name:  user.Name,
        Email: user.Email,
    }, nil
}
```

### 3. 注册与执行

```go
import (
    "context"
    querymemory "github.com/ddd-qce/core/query/memory"
    "github.com/ddd-qce/core/aspect"
)

func main() {
    ctx := context.Background()
    
    chain := aspect.NewAspectChain()
    bus := querymemory.NewQueryBus(chain)
    
    // 注册 Handler
    querymemory.RegisterQuery(bus, &GetUserHandler{userRepo: repo})
    
    // 执行 Query
    result, err := querymemory.Ask[GetUserQuery, *GetUserResult](
        ctx, bus, GetUserQuery{UserID: "user-123"},
    )
}
```

---

## 六、Event 使用指南

### 1. 定义 Event 结构

```go
package event

import "time"

type UserCreatedEvent struct {
    UserID    string
    Name      string
    Email     string
    CreatedAt time.Time
}

// 实现 DomainEvent 接口
func (e UserCreatedEvent) AggregateID() string {
    return e.UserID
}

func (e UserCreatedEvent) EventType() string {
    return "UserCreated"
}

func (e UserCreatedEvent) OccurredAt() time.Time {
    return e.CreatedAt
}
```

### 2. 定义 Handler

```go
package event

import "context"

type SendWelcomeEmailHandler struct {
    emailService EmailService
}

func (h *SendWelcomeEmailHandler) Handle(ctx context.Context, event UserCreatedEvent) error {
    return h.emailService.SendWelcome(event.Email, event.Name)
}

type UpdateSearchIndexHandler struct {
    searchClient SearchClient
}

func (h *UpdateSearchIndexHandler) Handle(ctx context.Context, event UserCreatedEvent) error {
    return h.searchClient.IndexUser(event.UserID, event.Name, event.Email)
}
```

### 3. 注册与发布

```go
import (
    "context"
    eventmemory "github.com/ddd-qce/core/event/memory"
    "github.com/ddd-qce/core/aspect"
)

func main() {
    ctx := context.Background()
    
    chain := aspect.NewAspectChain()
    bus := eventmemory.NewEventBus(chain)
    
    // 订阅 Handler（同一事件可多个订阅者）
    bus.Subscribe(&SendWelcomeEmailHandler{emailService: svc})
    bus.Subscribe(&UpdateSearchIndexHandler{searchClient: client})
    
    // 发布事件
    err := bus.Publish[UserCreatedEvent](ctx, UserCreatedEvent{
        UserID:    "user-123",
        Name:      "Alice",
        Email:     "alice@example.com",
        CreatedAt: time.Now(),
    })
}
```

### 4. EventStore 使用

```go
import eventmemory "github.com/ddd-qce/core/event/memory"

func main() {
    // 创建事件存储
    store := eventmemory.NewEventStore[UserCreatedEvent]()
    
    // 追加事件
    events := []UserCreatedEvent{
        {UserID: "user-123", Name: "Alice", Email: "alice@example.com", CreatedAt: time.Now()},
    }
    store.Append(ctx, events)
    
    // 加载事件（用于事件溯源重建聚合）
    loadedEvents, err := store.Load(ctx, "user-123", 0)
}
```

---

## 七、Aspect 系统

### 洋葱模型执行顺序

```
┌─────────────────────────────────────────────┐
│  TracingAspect (Order: 0)                   │
│  ┌───────────────────────────────────────┐  │
│  │  TransactionAspect (Order: 10)        │  │
│  │  ┌───────────────────────────────┐    │  │
│  │  │  LoggingAspect (Order: 50)    │    │  │
│  │  │  ┌───────────────────────┐    │    │  │
│  │  │  │  MetricsAspect (100)  │    │    │  │
│  │  │  │  ┌───────────────┐    │    │    │  │
│  │  │  │  │  Handler      │    │    │    │  │
│  │  │  │  └───────────────┘    │    │    │  │
│  │  │  │  After (100)          │    │    │  │
│  │  │  After (50)               │    │    │  │
│  │  After (10)                   │    │    │  │
│  After (0)                        │    │    │  │
└─────────────────────────────────────────────┘
```

- **Before 钩子**: 按 Order 升序执行（0 → 10 → 50 → 100）
- **After 钩子**: 按 Order 降序执行（100 → 50 → 10 → 0）

### 1. TracingAspect（链路追踪）

```go
import (
    "github.com/ddd-qce/core/trace"
    "github.com/ddd-qce/core/aspect/builtin"
)

func main() {
    traceStore := trace.NewInMemoryTraceStore()
    
    tracingAspect := &builtin.TracingAspect{Store: traceStore}
    
    chain := aspect.NewAspectChain()
    chain.RegisterCommandAspect(tracingAspect)
    chain.RegisterQueryAspect(tracingAspect)
    chain.RegisterEventAspect(tracingAspect)
    
    // 执行后会自动记录 Span
}
```

### 2. TransactionAspect（事务管理）

```go
import (
    "github.com/ddd-qce/core/aspect/builtin"
    "github.com/ddd-qce/core/aspect"
)

type MyTransactionManager struct{}

func (m *MyTransactionManager) Begin(ctx context.Context) (context.Context, error) {
    // 开启事务
    return ctx, nil
}

func (m *MyTransactionManager) Commit(ctx context.Context) error {
    // 提交事务
    return nil
}

func (m *MyTransactionManager) Rollback(ctx context.Context) error {
    // 回滚事务
    return nil
}

func main() {
    txAspect := &builtin.TransactionAspect{Manager: &MyTransactionManager{}}
    
    chain := aspect.NewAspectChain()
    chain.RegisterCommandAspect(txAspect) // 仅注册到 Command
    
    // Command 执行自动包裹在事务中
    // Handler 成功 → Commit
    // Handler 失败 → Rollback
}
```

### 3. LoggingAspect（日志记录）

```go
import (
    "github.com/ddd-qce/core/aspect/builtin"
)

type MyLogger struct{}

func (l *MyLogger) Info(msg string, args ...interface{}) {
    log.Printf("[INFO] "+msg, args...)
}

func (l *MyLogger) Error(msg string, args ...interface{}) {
    log.Printf("[ERROR] "+msg, args...)
}

func (l *MyLogger) Debug(msg string, args ...interface{}) {
    log.Printf("[DEBUG] "+msg, args...)
}

func main() {
    loggingAspect := &builtin.LoggingAspect{Logger: &MyLogger{}}
    
    chain := aspect.NewAspectChain()
    chain.RegisterCommandAspect(loggingAspect)
    chain.RegisterQueryAspect(loggingAspect)
    chain.RegisterEventAspect(loggingAspect)
}
```

### 4. MetricsAspect（指标采集）

```go
import (
    "time"
    "github.com/ddd-qce/core/aspect/builtin"
)

type MyMetricsRecorder struct{}

func (r *MyMetricsRecorder) RecordDuration(name string, duration time.Duration) {
    prometheus.HistogramVec.
        WithLabelValues(name).
        Observe(duration.Seconds())
}

func (r *MyMetricsRecorder) RecordError(name string, err error) {
    prometheus.CounterVec.
        WithLabelValues(name, "error").
        Inc()
}

func main() {
    metricsAspect := &builtin.MetricsAspect{Recorder: &MyMetricsRecorder{}}
    
    chain := aspect.NewAspectChain()
    chain.RegisterCommandAspect(metricsAspect)
    chain.RegisterQueryAspect(metricsAspect)
    chain.RegisterEventAspect(metricsAspect)
}
```

### 5. 自定义 Aspect

```go
import "github.com/ddd-qce/core/aspect"

type MyAuthAspect struct{}

func (a *MyAuthAspect) Name() string {
    return "Auth"
}

func (a *MyAuthAspect) Order() int {
    return 5 // 在 Tracing 之后，Logging 之前
}

// 实现 CommandAspect
func (a *MyAuthAspect) BeforeCommand(ctx context.Context, cmd any) (context.Context, error) {
    // 验证权限
    if !hasPermission(ctx, cmd) {
        return ctx, errors.New("permission denied")
    }
    return ctx, nil
}

func (a *MyAuthAspect) AfterCommand(ctx context.Context, cmd any, result any, err error, duration time.Duration) error {
    // 记录审计日志
    auditLog(ctx, cmd, err)
    return nil
}

// 注册
chain.RegisterCommandAspect(&MyAuthAspect{})
```

---

## 八、Job 系统

### 1. 提交异步任务

```go
import (
    "context"
    "github.com/ddd-qce/core/job/core"
    jobmemory "github.com/ddd-qce/core/job/memory"
    "github.com/ddd-qce/core/aspect"
)

func main() {
    ctx := context.Background()
    
    chain := aspect.NewAspectChain()
    jobStore := jobmemory.NewJobStore()
    jobManager := jobmemory.NewJobManager(jobStore, chain)
    
    // 提交任务
    job, err := jobManager.Submit(ctx, GenerateReportCommand{
        ReportType: "monthly",
        UserID:     "user-123",
    })
    
    fmt.Printf("Job submitted: %s, status: %s\n", job.ID, job.Status)
}
```

### 2. 查询任务状态

```go
// 获取状态
job, err := jobManager.GetStatus(ctx, jobID)

// 等待完成（带超时）
job, err = jobManager.Wait(ctx, jobID, 30*time.Second)

// 列出所有待执行任务
pendingJobs, err := jobManager.ListByStatus(ctx, jobcore.JobStatusPending)
```

### 3. 取消任务

```go
err := jobManager.Cancel(ctx, jobID)
```

### 4. 重试失败任务

```go
err := jobManager.Retry(ctx, jobID)
```

### 5. 配置重试和超时

```go
job, err := jobManager.Submit(ctx, cmd,
    jobcore.WithMaxRetries(3),          // 最多重试 3 次
    jobcore.WithTimeout(5*time.Minute), // 超时时间 5 分钟
)
```

---

## 九、链路追踪

### 1. 跨 Command → Event → Command 传播

```go
import (
    "context"
    "github.com/ddd-qce/core/trace"
)

func main() {
    ctx := context.Background()
    traceStore := trace.NewInMemoryTraceStore()
    
    // 创建切面链并注册 TracingAspect
    chain := aspect.NewAspectChain()
    chain.RegisterCommandAspect(&builtin.TracingAspect{Store: traceStore})
    chain.RegisterEventAspect(&builtin.TracingAspect{Store: traceStore})
    
    cBus := commandmemory.NewCommandBus(chain)
    eBus := eventmemory.NewEventBus(chain)
    
    // 执行 Command 1
    cBus.Dispatch[CreateUserCommand, string](ctx, cmd)
    // ↓ 自动创建 Span (TraceID: xxx, SpanID: aaa)
    
    // Command 1 内部发布 Event
    eBus.Publish[UserCreatedEvent](ctx, event)
    // ↓ 自动创建 Span (TraceID: xxx, SpanID: bbb, ParentID: aaa)
    
    // Event Handler 内部执行 Command 2
    cBus.Dispatch[SendEmailCommand, struct{}](ctx, emailCmd)
    // ↓ 自动创建 Span (TraceID: xxx, SpanID: ccc, ParentID: bbb)
    
    // 查看完整调用链
    spans, _ := traceStore.GetTrace(ctx, traceID)
    // spans: [aaa → bbb → ccc]
}
```

### 2. 手动传播 Trace 上下文

```go
import "github.com/ddd-qce/core/trace"

func someFunction(ctx context.Context) {
    // 获取当前 TraceID
    traceID := trace.GetTraceID(ctx)
    spanID := trace.GetSpanID(ctx)
    parentSpanID := trace.GetParentSpanID(ctx)
    
    // 手动设置 Trace 上下文（用于 HTTP/gRPC 传播）
    newCtx := trace.WithTrace(ctx, traceID, newSpanID)
    newCtx = trace.WithParentSpan(newCtx, spanID)
}
```

### 3. 查询追踪数据

```go
import "github.com/ddd-qce/core/trace"

func main() {
    traceStore := trace.NewInMemoryTraceStore()
    ctx := context.Background()
    
    // 获取完整调用链
    spans, err := traceStore.GetTrace(ctx, "trace-id-xxx")
    
    // 按条件过滤
    filter := &trace.TraceFilter{
        Type:          "command",
        Status:        "error",
        NameContains:  "CreateUser",
        StartTime:     time.Now().Add(-1 * time.Hour),
        EndTime:       time.Now(),
    }
    traceIDs, err := traceStore.ListTraces(ctx, filter)
}
```

### 4. Span 结构

```go
type Span struct {
    ID         string        // Span 唯一标识
    TraceID    string        // 调用链唯一标识
    ParentID   string        // 父 Span ID
    Type       string        // command / query / event
    Name       string        // 操作名称
    Status     string        // success / error
    Error      string        // 错误信息
    StartedAt  time.Time     // 开始时间
    Duration   time.Duration // 耗时
}
```

---

## 十、完整集成示例

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"

    "github.com/ddd-qce/core/aspect"
    "github.com/ddd-qce/core/aspect/builtin"
    commandmemory "github.com/ddd-qce/core/command/memory"
    eventmemory "github.com/ddd-qce/core/event/memory"
    querymemory "github.com/ddd-qce/core/query/memory"
    jobmemory "github.com/ddd-qce/core/job/memory"
    "github.com/ddd-qce/core/trace"
)

func main() {
    ctx := context.Background()

    // 1. 创建 Trace 存储
    traceStore := trace.NewInMemoryTraceStore()

    // 2. 创建切面链
    chain := aspect.NewAspectChain()
    chain.RegisterCommandAspect(&builtin.TracingAspect{Store: traceStore})
    chain.RegisterCommandAspect(&builtin.MetricsAspect{Recorder: &SimpleMetricsRecorder{}})
    chain.RegisterCommandAspect(&builtin.LoggingAspect{Logger: &SimpleLogger{}})
    chain.RegisterQueryAspect(&builtin.TracingAspect{Store: traceStore})
    chain.RegisterQueryAspect(&builtin.MetricsAspect{Recorder: &SimpleMetricsRecorder{}})
    chain.RegisterQueryAspect(&builtin.LoggingAspect{Logger: &SimpleLogger{}})
    chain.RegisterEventAspect(&builtin.TracingAspect{Store: traceStore})
    chain.RegisterEventAspect(&builtin.MetricsAspect{Recorder: &SimpleMetricsRecorder{}})
    chain.RegisterEventAspect(&builtin.LoggingAspect{Logger: &SimpleLogger{}})

    // 3. 创建 Bus
    qBus := querymemory.NewQueryBus(chain)
    cBus := commandmemory.NewCommandBus(chain)
    eBus := eventmemory.NewEventBus(chain)

    // 4. 创建 Job 管理器
    jobStore := jobmemory.NewJobStore()
    jobManager := jobmemory.NewJobManager(jobStore, chain)

    // 5. 注册 Handler
    // command.RegisterHandlers(cBus)
    // query.RegisterHandlers(qBus)
    // event.SubscribeHandlers(eBus)

    // 6. 执行
    // cBus.Dispatch[...]
    // querymemory.Ask[...]
    // eBus.Publish[...]
    // jobManager.Submit(...)

    // 7. 查看追踪
    // traceStore.GetTrace(ctx, traceID)
}

type SimpleMetricsRecorder struct{}

func (r *SimpleMetricsRecorder) RecordDuration(name string, d time.Duration) {
    log.Printf("[Metrics] %s took %v", name, d)
}

func (r *SimpleMetricsRecorder) RecordError(name string, err error) {
    log.Printf("[Metrics] %s error: %v", name, err)
}

type SimpleLogger struct{}

func (l *SimpleLogger) Info(msg string, args ...interface{}) {
    log.Printf("[INFO] "+msg, args...)
}

func (l *SimpleLogger) Error(msg string, args ...interface{}) {
    log.Printf("[ERROR] "+msg, args...)
}

func (l *SimpleLogger) Debug(msg string, args ...interface{}) {
    log.Printf("[DEBUG] "+msg, args...)
}
```

---

## 十一、最佳实践

### 1. Command 命名规范

- 使用动词开头：`CreateUser`、`UpdateOrder`、`CancelPayment`
- 包含完整意图：避免模糊名称如 `Process`、`Handle`
- 一个 Command 对应一个 Handler

### 2. Query 命名规范

- 使用 `Get`、`List`、`Find` 开头：`GetUser`、`ListOrders`、`FindInactiveUsers`
- Query 应该是只读的，不修改状态

### 3. Event 命名规范

- 使用过去时态：`UserCreated`、`OrderCancelled`、`PaymentCompleted`
- Event 表示已发生的事实，不可拒绝

### 4. 跨领域交互原则

```
✅ 推荐                                    ❌ 避免
─────────────────                          ─────────────────
通过 CommandBus 发送 Command               直接调用其他领域的 Service
通过 EventBus 发布 Event                   直接操作其他领域的 Repository
通过 QueryBus 查询数据                     直接访问其他领域的 Domain 对象
只引入其他领域的 command/query/event 包    引入其他领域的 domain/service/repository
```

### 5. Aspect 注册顺序

```go
// 推荐顺序（Order 值从小到大）
chain.RegisterCommandAspect(&builtin.TracingAspect{...})     // Order: 0
chain.RegisterCommandAspect(&builtin.TransactionAspect{...}) // Order: 10
chain.RegisterCommandAspect(&MyAuthAspect{})                 // Order: 20 (自定义)
chain.RegisterCommandAspect(&builtin.LoggingAspect{...})     // Order: 50
chain.RegisterCommandAspect(&builtin.MetricsAspect{...})     // Order: 100
```

### 6. 错误处理

```go
// Command Handler 返回错误
func (h *CreateUserHandler) Handle(ctx context.Context, cmd CreateUserCommand) (string, error) {
    if cmd.Email == "" {
        return "", errors.New("email is required") // 业务错误
    }
    
    if err := h.repo.Save(ctx, user); err != nil {
        return "", fmt.Errorf("save user: %w", err) // 包装系统错误
    }
    
    return user.ID, nil
}

// Event Handler 错误不影响其他订阅者
// EventBus 会独立处理每个 Handler 的错误
```

### 7. 上下文传播

```go
// 始终传递 context
func (h *Handler) Handle(ctx context.Context, cmd Command) (Result, error) {
    // ctx 包含 TraceID、SpanID、超时、取消信号等
    return h.repo.Save(ctx, entity) // 传递 ctx 给下游
}
```

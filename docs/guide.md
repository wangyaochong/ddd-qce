# 实战指南

## 一、Entity 使用指南

### 1. 基础实体

```go
package domain

import "github.com/ddd-qce/core/domain/entity"

type Product struct {
    entity.Entity
    Name        string
    Price       float64
    Description string
}

func NewProduct(id, name string, price float64) *Product {
    return &Product{
        Entity: *entity.NewEntity(id),
        Name:   name,
        Price:  price,
    }
}
```

### 2. 自动生成 ID

```go
// 使用 DefaultIDGenerator（UUID v4）
product := &Product{
    Entity: *entity.NewEntityWithID(),
    Name:   name,
    Price:  price,
}

// 自定义 ID 生成器
entity.SetIDGenerator(func() string {
    return "prod-" + uuid.New().String()
})
```

### 3. 实体方法

```go
p := NewProduct("prod-1", "Product A", 100.0)

p.ID()         // "prod-1"
p.Equals(other)   // 基于 ID 判断相等性
p.IsEmpty()       // ID 为空时返回 true
p.Validate()      // 返回 ID 为空的错误（可由子类覆写）
```

### 4. AuditableEntity（审计实体）

```go
type Document struct {
    entity.AuditableEntity
    Title string
}

func NewDocument(id, title string) *Document {
    doc := &Document{
        AuditableEntity: *entity.NewAuditableEntity(id),
        Title:           title,
    }
    return doc
}

// 更新时自动刷新时间戳
doc.Touch()  // doc.UpdatedAt = time.Now()
```

### 5. SoftDeletableEntity（软删除实体）

```go
type Article struct {
    entity.SoftDeletableEntity
    Title string
}

func NewArticle(id, title string) *Article {
    return &Article{
        SoftDeletableEntity: *entity.NewSoftDeletableEntity(id),
        Title:               title,
    }
}

article.SoftDelete()  // article.DeletedAt = &now, article.IsDeleted() == true
article.Restore()     // article.DeletedAt = nil, article.IsDeleted() == false
```

---

## 二、ValueObject 使用指南

### 1. 定义值对象

```go
package domain

import (
    "errors"
    "github.com/ddd-qce/core/domain/valueobject"
)

email, err := valueobject.New("user@example.com", func(v string) error {
    if !strings.Contains(v, "@") {
        return errors.New("invalid email")
    }
    return nil
})
```

### 2. 使用值对象

```go
email.Value()            // "user@example.com"
email.Validate()         // nil
email.Equals(otherEmail) // 基于值比较
email.String()           // "user@example.com"

// 强制版本（panic on error）
email := valueobject.MustNew("user@example.com", validateFunc)
```

### 3. DeepEquals 深度比较

```go
import "github.com/ddd-qce/core/domain/valueobject"

// 比较任意两个值（递归比较结构体字段）
valueobject.DeepEquals(addr1, addr2)
```

---

## 三、AggregateRoot 使用指南

### 1. 定义聚合根

```go
package domain

import (
    "github.com/ddd-qce/core/domain/aggregate"
    "github.com/ddd-qce/core/domain/event"
)

type Order struct {
    aggregate.AggregateRoot
    Items  []OrderItem
    Status OrderStatus
    Total  float64
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

### 2. 实现 EventApplier（推荐方式）

```go
func (o *Order) When(evt event.DomainEvent) {
    switch e := evt.(type) {
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

func NewOrder(orderID string, items []OrderItem) *Order {
    order := &Order{
        AggregateRoot: *aggregate.NewAggregateRootWithApplier(orderID, order),
        Items:         items,
        Status:        OrderStatusPending,
    }

    var total float64
    for _, item := range items {
        total += item.Price * float64(item.Quantity)
    }
    order.Total = total

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
        OrderID:   o.ID,
        OldStatus: oldStatus,
    })

    return nil
}
```

### 4. 获取与提交事件

```go
// 获取未提交的领域事件
events := order.UncommittedEvents()

// 标记事件已提交
order.MarkEventsAsCommitted()

// 从历史事件重建聚合
order.LoadFromHistory(events)
```

### 5. 纯事件收集器（无 When 回调）

```go
// NewEventCollector 仅收集事件，不触发 When 回调
collector := aggregate.NewEventCollector("order-123")
collector.Apply(someEvent)
```

---

## 四、Repository 使用指南

### 1. 标准仓储接口

```go
type Repository[T any] interface {
    Save(ctx context.Context, aggregate T) error
    FindByID(ctx context.Context, id string) (T, error)
    Delete(ctx context.Context, id string) error
}
```

### 2. PostgreSQL 仓储（带乐观锁）

```go
package repository

import (
    "database/sql"
    "github.com/ddd-qce/core/infra/repository/pg"
)

type OrderRepository struct {
    *pg.PgRepository[*Order]
}

func NewOrderRepository(db *sql.DB) *OrderRepository {
    return &OrderRepository{
        PgRepository: pg.NewRepository[*Order](db, pg.JSONSerializer[*Order]{}),
    }
}
```

### 3. 乐观锁错误处理

```go
import "github.com/ddd-qce/core/infra/repository/pg"

err := repo.Save(ctx, order)
if err != nil {
    var lockErr *pg.OptimisticLockError
    if errors.As(err, &lockErr) {
        // 处理并发冲突
        fmt.Printf("乐观锁冲突: aggregate=%s expected_version=%d\n",
            lockErr.AggregateID, lockErr.ExpectedVersion)
    }
    return err
}
```

### 4. 事件溯源仓储接口

```go
type EventSourcingRepository[T any] interface {
    Save(ctx context.Context, aggregate T) error
    Load(ctx context.Context, id string) (T, error)
}
```

### 5. PostgreSQL 事件溯源仓储（带快照）

```go
import (
    "database/sql"
    cqevent "github.com/ddd-qce/core/cqrs/event"
    "github.com/ddd-qce/core/cqrs/event/pg"
    "github.com/ddd-qce/core/infra/repository/pg"
)

eventStore := pgevent.NewEventStore[event.DomainEvent](db)

repo := pg.NewEventSourcedRepository[*Order](
    db,
    (*cqevent.EventStore[event.DomainEvent])(&eventStore), // 需要适配
    func(id string) *Order {
        return &Order{AggregateRoot: *aggregate.NewAggregateRootWithApplier(id, &Order{})}
    },
    pg.WithSnapshotEvery[*Order](10),      // 每 10 个事件保存一次快照
    pg.WithSerializer[*Order](pg.JSONSerializer[*Order]{}),
)
```

---

## 五、Command 使用指南

### 1. 定义 Command 结构

```go
package command

import "github.com/ddd-qce/core/cqrs/command"

type CreateUserCommand struct {
    command.BaseCommand
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
    "github.com/ddd-qce/core/cqrs/command"
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
    "github.com/ddd-qce/core/aspect"
    commandmemory "github.com/ddd-qce/core/cqrs/command/memory"
)

func main() {
    ctx := context.Background()

    chain := aspect.NewAspectChain()
    bus := commandmemory.NewCommandBus(chain)

    // 注册 Handler
    commandmemory.RegisterCommand[CreateUserCommand, string](bus, &CreateUserHandler{userRepo: repo})

    // 执行 Command（注意：bus 在 ctx 前面）
    userID, err := commandmemory.Dispatch[CreateUserCommand, string](
        bus, ctx, CreateUserCommand{
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
    command.BaseCommand
    UserID string
}

type DeleteUserHandler struct{}

func (h *DeleteUserHandler) Handle(ctx context.Context, cmd DeleteUserCommand) (struct{}, error) {
    return struct{}{}, nil
}

// 执行
_, err := commandmemory.Dispatch[DeleteUserCommand, struct{}](bus, ctx, cmd)
```

---

## 六、Query 使用指南

### 1. 定义 Query 结构

```go
package query

import "github.com/ddd-qce/core/cqrs/query"

type GetUserQuery struct {
    query.BaseQuery
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
    "github.com/ddd-qce/core/aspect"
    querymemory "github.com/ddd-qce/core/cqrs/query/memory"
)

func main() {
    ctx := context.Background()

    chain := aspect.NewAspectChain()
    bus := querymemory.NewQueryBus(chain)

    // 注册 Handler
    querymemory.RegisterQuery[GetUserQuery, *GetUserResult](bus, &GetUserHandler{userRepo: repo})

    // 执行 Query（注意：bus 在 ctx 前面）
    result, err := querymemory.Dispatch[GetUserQuery, *GetUserResult](
        bus, ctx, GetUserQuery{UserID: "user-123"},
    )
}
```

---

## 七、Event 使用指南

### 1. 定义 Event 结构

```go
package event

import (
    "time"
    "github.com/ddd-qce/core/domain/event"
)

type UserCreatedEvent struct {
    UserID    string
    Name      string
    Email     string
    CreatedAt time.Time
}

func (e UserCreatedEvent) AggregateID() string   { return e.UserID }
func (e UserCreatedEvent) EventType() string     { return "UserCreated" }
func (e UserCreatedEvent) OccurredAt() time.Time { return e.CreatedAt }
```

### 2. 定义 Handler

```go
package event

import "context"

type SendWelcomeEmailHandler struct {
    emailService EmailService
}

func (h *SendWelcomeEmailHandler) Handle(ctx context.Context, evt UserCreatedEvent) error {
    return h.emailService.SendWelcome(evt.Email, evt.Name)
}

type UpdateSearchIndexHandler struct {
    searchClient SearchClient
}

func (h *UpdateSearchIndexHandler) Handle(ctx context.Context, evt UserCreatedEvent) error {
    return h.searchClient.IndexUser(evt.UserID, evt.Name, evt.Email)
}
```

### 3. 注册与发布（EventBus + RegisterHandler/Dispatch）

```go
import (
    "context"
    "github.com/ddd-qce/core/aspect"
    eventmemory "github.com/ddd-qce/core/cqrs/event/memory"
)

func main() {
    ctx := context.Background()

    chain := aspect.NewAspectChain()
    // EventBus 是非泛型的，一个实例处理所有事件类型
    bus := eventmemory.NewEventBus(eventmemory.WithBusAspectChain(chain))

    // 订阅 Handler（同一事件可多个订阅者）
    eventmemory.RegisterHandler[UserCreatedEvent](bus, &SendWelcomeEmailHandler{emailService: svc})
    eventmemory.RegisterHandler[UserCreatedEvent](bus, &UpdateSearchIndexHandler{searchClient: client})

    // 发布事件
    err := eventmemory.Dispatch[UserCreatedEvent](bus, ctx, UserCreatedEvent{
        UserID:    "user-123",
        Name:      "Alice",
        Email:     "alice@example.com",
        CreatedAt: time.Now(),
    })
}
```

### 4. 多类型事件（同一个 EventBus）

```go
// 一个 EventBus 实例处理所有事件类型
bus := eventmemory.NewEventBus(eventmemory.WithBusAspectChain(chain))

// 订阅不同类型的事件
eventmemory.RegisterHandler[UserCreatedEvent](bus, &SendWelcomeEmailHandler{})
eventmemory.RegisterHandler[OrderPlacedEvent](bus, &UpdateInventoryHandler{})

// 发布不同类型的事件
eventmemory.Dispatch[UserCreatedEvent](bus, ctx, UserCreatedEvent{...})
eventmemory.Dispatch[OrderPlacedEvent](bus, ctx, OrderPlacedEvent{...})
```

### 5. EventStore 使用

```go
import eventmemory "github.com/ddd-qce/core/cqrs/event/memory"

func main() {
    // 创建事件存储
    store := eventmemory.NewEventStore[UserCreatedEvent]()

    // 追加事件（4 参数：ctx, aggregateID, expectedVersion, events）
    store.Append(ctx, "user-123", 0, []UserCreatedEvent{
        {UserID: "user-123", Name: "Alice", Email: "alice@example.com", CreatedAt: time.Now()},
    })

    // 加载事件（用于事件溯源重建聚合）
    loadedEvents, err := store.Load(ctx, "user-123", 0)
}
```

### 6. PostgreSQL EventStore

```go
import pgevent "github.com/ddd-qce/core/cqrs/event/pg"

store := pgevent.NewEventStore[event.DomainEvent](db)

// 带工厂函数（用于反序列化优化）
store := pgevent.NewEventStoreWithFactory[event.DomainEvent](db, func() event.DomainEvent {
    return &MyDomainEvent{}
})
```

---

## 八、Aspect 系统

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
}
```

### 2. TransactionAspect（事务管理）

```go
import (
    "github.com/ddd-qce/core/aspect/builtin"
    "github.com/ddd-qce/core/aspect"
)

func main() {
    // 使用 Backend 提供的 TransactionManager
    backend := infra.NewMemoryBackend()

    txAspect := &builtin.TransactionAspect{TxManager: backend.TransactionManager}

    chain := aspect.NewAspectChain()
    chain.RegisterCommandAspect(txAspect)

    // Command 执行自动包裹在事务中
    // Handler 成功 → Commit
    // Handler 失败 → Rollback
    // 支持嵌套事务（Savepoint 语义）
}
```

### 3. LoggingAspect（日志记录）

```go
import "github.com/ddd-qce/core/aspect/builtin"

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
import "github.com/ddd-qce/core/aspect/builtin"

func main() {
    metricsAspect := &builtin.MetricsAspect{Recorder: &MyMetricsRecorder{}}

    chain := aspect.NewAspectChain()
    chain.RegisterCommandAspect(metricsAspect)
    chain.RegisterQueryAspect(metricsAspect)
    chain.RegisterEventAspect(metricsAspect)
}
```

### 5. PersistenceAspect（消息持久化）

```go
import "github.com/ddd-qce/core/aspect/builtin"

func main() {
    // 使用 NopMessageStore 跳过持久化
    nopStore := builtin.NewNopMessageStore()

    persistenceAspect := &builtin.PersistenceAspect{Store: nopStore}

    chain := aspect.NewAspectChain()
    chain.RegisterCommandAspect(persistenceAspect)
}
```

### 6. 自定义 Aspect

```go
import (
    "context"
    "time"

    "github.com/ddd-qce/core/cqrs/aspect"
)

type MyAuthAspect struct{}

func (a *MyAuthAspect) Name() string  { return "Auth" }
func (a *MyAuthAspect) Order() int    { return 5 } // 在 Tracing 之后，Logging 之前

// 实现 CommandAspect
func (a *MyAuthAspect) BeforeCommand(ctx context.Context, cmd any) (context.Context, error) {
    if !hasPermission(ctx, cmd) {
        return ctx, errors.New("permission denied")
    }
    return ctx, nil
}

func (a *MyAuthAspect) AfterCommand(ctx context.Context, cmd any, result any, err error, duration time.Duration) error {
    auditLog(ctx, cmd, err)
    return nil
}

// 注册
chain.RegisterCommandAspect(&MyAuthAspect{})
```

---

## 九、Job 系统

### 1. 提交异步任务

```go
import (
    "context"
    "github.com/ddd-qce/core/job/core"
    jobmemory "github.com/ddd-qce/core/job/memory"
    commandmemory "github.com/ddd-qce/core/cqrs/command/memory"
)

func main() {
    ctx := context.Background()

    chain := aspect.NewAspectChain()
    cBus := commandmemory.NewCommandBus(chain)

    jobStore := jobmemory.NewJobStore()
    // JobManager 需要 CommandExecutor 来执行任务
    jobManager := jobmemory.NewJobManager(jobStore, cBus)

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

## 十、链路追踪

### 1. 跨 Command → Event → Command 传播

```go
import (
    "context"
    "github.com/ddd-qce/core/trace"
    "github.com/ddd-qce/core/aspect/builtin"
    commandmemory "github.com/ddd-qce/core/cqrs/command/memory"
    eventmemory "github.com/ddd-qce/core/cqrs/event/memory"
)

func main() {
    ctx := context.Background()
    traceStore := trace.NewInMemoryTraceStore()

    chain := aspect.NewAspectChain()
    chain.RegisterCommandAspect(&builtin.TracingAspect{Store: traceStore})
    chain.RegisterEventAspect(&builtin.TracingAspect{Store: traceStore})

    cBus := commandmemory.NewCommandBus(chain)
    eBus := eventmemory.NewEventBus(eventmemory.WithBusAspectChain(chain))

    // 执行 Command 1
    commandmemory.Dispatch[CreateUserCommand, string](cBus, ctx, cmd)
    // ↓ 自动创建 Span (TraceID: xxx, SpanID: aaa)

    // Command 1 内部发布 Event
    eBus.Publish(ctx, event)
    // ↓ 自动创建 Span (TraceID: xxx, SpanID: bbb, ParentID: aaa)

    // Event Handler 内部执行 Command 2
    commandmemory.Dispatch[SendEmailCommand, struct{}](cBus, ctx, emailCmd)
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
        Type:         "command",
        Status:       "error",
        NameContains: "CreateUser",
        StartTime:    time.Now().Add(-1 * time.Hour),
        EndTime:      time.Now(),
    }
    traceIDs, err := traceStore.ListTraces(ctx, filter)
}
```

### 4. Span 结构

```go
type Span struct {
    ID        string        // Span 唯一标识
    TraceID   string        // 调用链唯一标识
    ParentID  string        // 父 Span ID
    Type      string        // command / query / event
    Name      string        // 操作名称
    Status    string        // success / error
    Error     string        // 错误信息
    StartedAt time.Time     // 开始时间
    Duration  time.Duration // 耗时
}
```

---

## 十一、Backend 基础设施配置

### 1. 内存后端（开发/测试）

```go
import "github.com/ddd-qce/core/infra"

backend := infra.NewMemoryBackend()

// backend.TransactionManager  — 内存事务管理器（支持嵌套）
// backend.JobStore            — nil（需手动设置）
// backend.TraceStore          — nil（需手动设置）
// backend.MessageStore        — nil（需手动设置）
```

### 2. PostgreSQL 后端（生产）

```go
import (
    "database/sql"
    "github.com/ddd-qce/core/pgx"
)

db, _ := sql.Open("pgx", "postgres://...")
backend := pgx.NewBackend(db)

// backend.TransactionManager  — PgTransactionManager（Savepoint 嵌套事务）
// backend.JobStore            — PgJobStore
// backend.TraceStore          — PgTraceStore
// backend.MessageStore        — PgMessageStore
// backend.Migrate             — pg.Migrate 函数
```

### 3. 数据库迁移

```go
// 使用 Backend 的 Migrate 函数
if backend.Migrate != nil {
    err := backend.Migrate(db)
}

// 或直接调用
err := pg.Migrate(db)
```

### 4. 嵌套事务

```go
// PgTransactionManager 自动支持嵌套事务
// 外层: BEGIN
//   内层: SAVEPOINT sp_1
//   内层 Commit: RELEASE SAVEPOINT sp_1
//   内层 Rollback: ROLLBACK TO SAVEPOINT sp_1 (标记 aborted)
// 外层 Commit: COMMIT (检查 aborted 标记)
// 外层 Rollback: ROLLBACK

txMgr := pg.NewTransactionManager(db)
ctx, _ = txMgr.Begin(ctx)     // BEGIN
  ctx2, _ = txMgr.Begin(ctx)  // SAVEPOINT sp_1
  txMgr.Commit(ctx2)          // RELEASE SAVEPOINT sp_1
txMgr.Commit(ctx)              // COMMIT
```

### 5. 事务内查询

```go
import "github.com/ddd-qce/core/pg"

// GetQuerier 自动从 ctx 获取当前事务，若无事务则使用 db
querier := pg.GetQuerier(ctx, db)
querier.QueryContext(ctx, "SELECT ...", args...)
```

---

## 十二、完整集成示例

```go
package main

import (
    "context"
    "log"
    "time"

    "github.com/ddd-qce/core/aspect"
    "github.com/ddd-qce/core/aspect/builtin"
    commandmemory "github.com/ddd-qce/core/cqrs/command/memory"
    eventmemory "github.com/ddd-qce/core/cqrs/event/memory"
    querymemory "github.com/ddd-qce/core/cqrs/query/memory"
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
    chain.RegisterCommandAspect(&builtin.LoggingAspect{Logger: &SimpleLogger{}})
    chain.RegisterCommandAspect(&builtin.MetricsAspect{Recorder: &SimpleMetricsRecorder{}})
    chain.RegisterQueryAspect(&builtin.TracingAspect{Store: traceStore})
    chain.RegisterQueryAspect(&builtin.LoggingAspect{Logger: &SimpleLogger{}})
    chain.RegisterEventAspect(&builtin.TracingAspect{Store: traceStore})
    chain.RegisterEventAspect(&builtin.LoggingAspect{Logger: &SimpleLogger{}})

    // 3. 创建 Bus
    qBus := querymemory.NewQueryBus(chain)
    cBus := commandmemory.NewCommandBus(chain)
    eBus := eventmemory.NewEventBus(eventmemory.WithBusAspectChain(chain))

    // 4. 创建 Job 管理器
    jobStore := jobmemory.NewJobStore()
    jobManager := jobmemory.NewJobManager(jobStore, cBus)

    // 5. 注册 Handler
    // commandmemory.RegisterCommand[CreateUserCommand, string](cBus, &CreateUserHandler{...})
    // querymemory.RegisterQuery[GetUserQuery, *GetUserResult](qBus, &GetUserHandler{...})
    // eventmemory.RegisterHandler[UserCreatedEvent](eBus, &SendWelcomeEmailHandler{...})

    // 6. 执行
    // commandmemory.Dispatch[CreateUserCommand, string](cBus, ctx, cmd)
    // querymemory.Dispatch[GetUserQuery, *GetUserResult](qBus, ctx, q)
    // eventmemory.Dispatch[UserCreatedEvent](eBus, ctx, evt)
    // jobManager.Submit(ctx, cmd)

    // 7. 查看追踪
    // traceStore.GetTrace(ctx, traceID)
}
```

---

## 十三、最佳实践

### 1. Command 命名规范

- 使用动词开头：`CreateUser`、`UpdateOrder`、`CancelPayment`
- 包含完整意图：避免模糊名称如 `Process`、`Handle`
- 一个 Command 对应一个 Handler（1:1）

### 2. Query 命名规范

- 使用 `Get`、`List`、`Find` 开头：`GetUser`、`ListOrders`、`FindInactiveUsers`
- Query 应该是只读的，不修改状态
- 一个 Query 对应一个 Handler（1:1）

### 3. Event 命名规范

- 使用过去时态：`UserCreated`、`OrderCancelled`、`PaymentCompleted`
- Event 表示已发生的事实，不可拒绝
- 一个 Event 可以有多个订阅者（1:N）

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
chain.RegisterCommandAspect(&builtin.TracingAspect{...})      // Order: 0
chain.RegisterCommandAspect(&builtin.TransactionAspect{...})  // Order: 10
chain.RegisterCommandAspect(&MyAuthAspect{})                  // Order: 5-20 (自定义)
chain.RegisterCommandAspect(&builtin.LoggingAspect{...})      // Order: 50
chain.RegisterCommandAspect(&builtin.MetricsAspect{...})      // Order: 100
chain.RegisterCommandAspect(&builtin.PersistenceAspect{...})  // Order: 200
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

// 乐观锁冲突
var lockErr *pg.OptimisticLockError
if errors.As(err, &lockErr) {
    // 重试或返回冲突提示
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

### 8. Backend 选择

```go
// 开发/测试：使用内存后端
backend := infra.NewMemoryBackend()

// 生产：使用 PostgreSQL 后端
backend := pgx.NewBackend(db)

// 切换后端不需要修改业务代码
// Backend 统一了 TransactionManager、JobStore、TraceStore、MessageStore 接口
```

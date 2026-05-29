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
// 自动生成 UUID hex 格式 ID（32 字符，无连字符）
product := &Product{
    Entity: *entity.NewEntityWithID(),
    Name:   name,
    Price:  price,
}

// 自定义 ID 直接传入
product := &Product{
    Entity: *entity.NewEntity(myCustomGen()),
    Name:   name,
    Price:  price,
}
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
    "github.com/ddd-qce/core/cqrs/event"
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
func (o *Order) When(evt domainevent.Event) error {
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

func NewOrder(ctx context.Context, orderID string, items []OrderItem) *Order {
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

    order.Apply(ctx, OrderCreatedEvent{
        AggregateID: orderID,
        Items:       items,
        Total:       total,
    })

    return order
}
```

### 3. 聚合根业务方法

```go
func (o *Order) Confirm(ctx context.Context) error {
    if o.Status != OrderStatusPending {
        return fmt.Errorf("order can only be confirmed from pending status")
    }

    o.Status = OrderStatusConfirmed

    o.Apply(ctx, OrderConfirmedEvent{
        AggregateID: o.ID(),
    })

    return nil
}

func (o *Order) Cancel(ctx context.Context) error {
    if o.Status == OrderStatusShipped {
        return fmt.Errorf("cannot cancel shipped order")
    }

    oldStatus := o.Status
    o.Status = OrderStatusCancelled

    o.Apply(ctx, OrderCancelledEvent{
        AggregateID: o.ID(),
        OldStatus:   oldStatus,
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
        PgRepository: pg.NewRepository[*Order](db),
    }
}
```

### 3. 乐观锁错误处理

```go
import (
    "github.com/ddd-qce/core/infra/repository"
    "github.com/ddd-qce/core/infra/repository/pg"
)

err := repo.Save(ctx, order)
if err != nil {
    var lockErr *repository.OptimisticLockError
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
    pgevent "github.com/ddd-qce/core/cqrs/event/impl/pg"
    "github.com/ddd-qce/core/infra/repository/pg"
)

eventStore, err := pgevent.NewEventSourceStore[cqevent.Event](db,
    pgevent.WithFactory[cqevent.Event](func() cqevent.Event {
        return &OrderPlacedEvent{}
    }),
)

repo := pg.NewEventSourcedRepository[*Order](
    db,
    eventStore,
    func(id string) *Order {
        o := &Order{}
        o.AggregateRoot = *aggregate.NewAggregateRootWithApplier(id, o)
        return o
    },
    pg.WithSnapshotEvery[*Order](10),
    pg.WithSerializer[*Order](repository.JSONSerializer[*Order]{}),
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
    "github.com/ddd-qce/core/cqrs/command"
    commandmemory "github.com/ddd-qce/core/cqrs/command/impl/memory"
)

func main() {
    ctx := context.Background()

    chain := aspect.NewAspectChain()
    bus := commandmemory.NewCommandBus(commandmemory.WithCommandBusAspectChain(chain))

    // 注册 Handler（通过 CommandBus 接口方法）
    bus.RegisterHandler(&CreateUserHandler{userRepo: repo})

    // 执行 Command（通过接口级 Dispatch，ctx 在前）
    userID, err := command.Dispatch[CreateUserCommand, string](
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
    command.BaseCommand
    UserID string
}

type DeleteUserHandler struct{}

func (h *DeleteUserHandler) Handle(ctx context.Context, cmd DeleteUserCommand) (struct{}, error) {
    return struct{}{}, nil
}

// 执行
_, err := command.Dispatch[DeleteUserCommand, struct{}](ctx, bus, cmd)
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
    "github.com/ddd-qce/core/cqrs/query"
    querymemory "github.com/ddd-qce/core/cqrs/query/impl/memory"
)

func main() {
    ctx := context.Background()

    chain := aspect.NewAspectChain()
    bus := querymemory.NewQueryBus(querymemory.WithQueryBusAspectChain(chain))

    // 注册 Handler（通过 QueryBus 接口方法）
    bus.RegisterHandler(&GetUserHandler{userRepo: repo})

    // 执行 Query（通过接口级 Dispatch，ctx 在前）
    result, err := query.Dispatch[GetUserQuery, *GetUserResult](
        ctx, bus, GetUserQuery{UserID: "user-123"},
    )
}
```

---

## 七、Event 使用指南

### 1. 定义 Event 结构

```go
package event

import (
    "github.com/ddd-qce/core/cqrs/event"
)

type UserCreatedEvent struct {
    event.BaseEvent
    Name  string
    Email string
}

func NewUserCreatedEvent(userID, name, email string) UserCreatedEvent {
    return UserCreatedEvent{
        BaseEvent: event.NewDomainEvent(userID),
        Name:      name,
        Email:     email,
    }
}
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
    "github.com/ddd-qce/core/cqrs/event"
    eventmemory "github.com/ddd-qce/core/cqrs/event/impl/memory"
)

func main() {
    ctx := context.Background()

    chain := aspect.NewAspectChain()
    bus := eventmemory.NewEventBus(eventmemory.WithBusAspectChain(chain))

    // 订阅 Handler（通过 EventBus 接口方法）
    bus.SubscribeHandler(&SendWelcomeEmailHandler{emailService: svc})
    bus.SubscribeHandler(&UpdateSearchIndexHandler{searchClient: client})

    // 发布事件（通过接口级 Dispatch，ctx 在前）
    err := event.Dispatch[UserCreatedEvent](ctx, bus, NewUserCreatedEvent("user-123", "Alice", "alice@example.com"))
}
```

### 4. 多类型事件（同一个 EventBus）

```go
// 一个 EventBus 实例处理所有事件类型
bus := eventmemory.NewEventBus(eventmemory.WithBusAspectChain(chain))

// 订阅不同类型的事件
bus.SubscribeHandler(&SendWelcomeEmailHandler{})
bus.SubscribeHandler(&UpdateInventoryHandler{})

// 发布不同类型的事件
event.Dispatch[UserCreatedEvent](ctx, bus, UserCreatedEvent{...})
event.Dispatch[OrderPlacedEvent](ctx, bus, OrderPlacedEvent{...})
```

### 5. EventSourceStore 使用

```go
import eventmemory "github.com/ddd-qce/core/cqrs/event/impl/memory"

func main() {
    // 创建事件存储
    store := eventmemory.NewEventSourceStore[UserCreatedEvent]()

    // 追加事件（4 参数：ctx, aggregateID, expectedVersion, events）
    store.Append(ctx, "user-123", 0, []UserCreatedEvent{
        NewUserCreatedEvent("user-123", "Alice", "alice@example.com"),
    })

    // 加载事件（用于事件溯源重建聚合）
    loadedEvents, err := store.Load(ctx, "user-123", 0)
}
```

### 6. PostgreSQL EventSourceStore

```go
import pgevent "github.com/ddd-qce/core/cqrs/event/impl/pg"

store, err := pgevent.NewEventSourceStore[MyEvent](db)

// 带工厂函数（用于反序列化优化，接口类型 T 必须使用）
store, err := pgevent.NewEventSourceStore[event.Event](db,
    pgevent.WithFactory[event.Event](func() event.Event {
        return &MyEvent{}
    }),
)
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
    // 使用 InMemoryMessageStore 进行内存持久化
    memStore := builtin.NewInMemoryMessageStore()

    persistenceAspect := &builtin.PersistenceAspect{Store: memStore}

    chain := aspect.NewAspectChain()
    chain.RegisterCommandAspect(persistenceAspect)
}
```

### 6. 自定义 Aspect

```go
import (
    "context"
    "time"

    "github.com/ddd-qce/core/aspect"
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
    commandmemory "github.com/ddd-qce/core/cqrs/command/impl/memory"
)

func main() {
    ctx := context.Background()

    chain := aspect.NewAspectChain()
    cBus := commandmemory.NewCommandBus(commandmemory.WithCommandBusAspectChain(chain))

    jobStore := jobmemory.NewJobStore()
    // JobManager 需要 CommandBus 来执行任务
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
    "github.com/ddd-qce/core/aspect"
    "github.com/ddd-qce/core/aspect/builtin"
    "github.com/ddd-qce/core/cqrs/command"
    commandmemory "github.com/ddd-qce/core/cqrs/command/impl/memory"
    eventmemory "github.com/ddd-qce/core/cqrs/event/impl/memory"
    "github.com/ddd-qce/core/trace"
)

func main() {
    ctx := context.Background()
    traceStore := trace.NewInMemoryTraceStore()

    chain := aspect.NewAspectChain()
    chain.RegisterCommandAspect(&builtin.TracingAspect{Store: traceStore})
    chain.RegisterEventAspect(&builtin.TracingAspect{Store: traceStore})

    cBus := commandmemory.NewCommandBus(commandmemory.WithCommandBusAspectChain(chain))
    eBus := eventmemory.NewEventBus(eventmemory.WithBusAspectChain(chain))

    // 执行 Command 1
    command.Dispatch[CreateUserCommand, string](ctx, cBus, cmd)
    // ↓ 自动创建 Span (TraceID: xxx, SpanID: aaa)

    // Command 1 内部发布 Event
    eBus.Publish(ctx, event)
    // ↓ 自动创建 Span (TraceID: xxx, SpanID: bbb, ParentID: aaa)

    // Event Handler 内部执行 Command 2
    command.Dispatch[SendEmailCommand, struct{}](ctx, cBus, emailCmd)
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
    "github.com/ddd-qce/core/infra"
)

db, _ := sql.Open("pgx", "postgres://...")
backend := infra.NewPgBackend(db)

// backend.TransactionManager  — PgTransactionManager（Savepoint 嵌套事务）
// backend.JobStore            — PgJobStore
// backend.TraceStore          — PgTraceStore
// backend.MessageStore        — PgMessageStore
// backend.Migrator          — pg.Migrator（执行数据库迁移）
```

### 3. 环境变量配置切换

框架支持通过环境变量一键切换存储后端，无需修改代码：

| 环境变量 | 默认值 | 说明 |
|----------|--------|------|
| `DDD_STORE_TYPE` | `postgresql` | 存储后端类型：`postgresql` 或 `memory` |
| `DDD_POSTGRES_URI` | 无 | PostgreSQL 连接地址，`postgresql` 模式下必填 |

```bash
# PostgreSQL 模式（默认）
DDD_POSTGRES_URI="postgres://user:pass@localhost:5432/mydb" ./myapp

# Memory 模式（开发/测试）
DDD_STORE_TYPE=memory ./myapp
```

PostgreSQL 是唯一的生产运行模式。Memory 模式用于设计验证——确保应用层只依赖接口、不依赖具体实现。详见 [架构设计文档](architecture.md#七存储模式设计哲学)。

在应用代码中通过 Provider 模式组装：

```go
// infrastructure/provider.go
func NewProvider(cfg *Config) (*StoreComponents, error) {
    switch cfg.StoreType {
    case "memory":
        return &StoreComponents{
            Backend:    infra.NewMemoryBackend(),
            EventStore: eventmemory.NewEventSourceStore[cqevent.Event](),
            OrderRepo:  application.NewOrderRepository(),
        }, nil
    case "postgresql":
        db, _ := sql.Open("pgx", cfg.PostgresURI)
        eventStore, _ := pgevent.NewEventSourceStore[cqevent.Event](db,
            pgevent.WithFactory[cqevent.Event](func() cqevent.Event { return &OrderPlacedEvent{} }),
        )
        return &StoreComponents{
            Backend:    infra.NewPgBackend(db),
            EventStore: eventStore,
            OrderRepo:  application.NewOrderRepository(),
            DB:         db,
        }, nil
    }
}
```

### 4. 数据库迁移

```go
// 使用 Backend 的 Migrator 执行迁移
if backend.Migrator != nil {
    err := backend.Migrator.Migrate(ctx)
}

// 或直接调用
err := pg.Migrate(db)
```

### 5. 嵌套事务

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

### 6. 事务内查询

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
    "github.com/ddd-qce/core/cqrs/command"
    commandmemory "github.com/ddd-qce/core/cqrs/command/impl/memory"
    "github.com/ddd-qce/core/cqrs/event"
    eventmemory "github.com/ddd-qce/core/cqrs/event/impl/memory"
    "github.com/ddd-qce/core/cqrs/query"
    querymemory "github.com/ddd-qce/core/cqrs/query/impl/memory"
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
    qBus := querymemory.NewQueryBus(querymemory.WithQueryBusAspectChain(chain))
    cBus := commandmemory.NewCommandBus(commandmemory.WithCommandBusAspectChain(chain))
    eBus := eventmemory.NewEventBus(eventmemory.WithBusAspectChain(chain))

    // 4. 创建 Job 管理器
    jobStore := jobmemory.NewJobStore()
    jobManager := jobmemory.NewJobManager(jobStore, cBus)

    // 5. 注册 Handler
    // cBus.RegisterHandler(&CreateUserHandler{...})
    // qBus.RegisterHandler(&GetUserHandler{...})
    // eBus.SubscribeHandler(&SendWelcomeEmailHandler{...})

    // 6. 执行
    // command.Dispatch[CreateUserCommand, string](ctx, cBus, cmd)
    // query.Dispatch[GetUserQuery, *GetUserResult](ctx, qBus, q)
    // event.Dispatch[UserCreatedEvent](ctx, eBus, evt)
    // jobManager.Submit(ctx, cmd)

    // 7. 查看追踪
    // traceStore.GetTrace(ctx, traceID)
}
```

---

## 十三、最佳实践

### 0. Entity 嵌入规范

所有 Entity/AggregateRoot 嵌入**统一使用值类型**，禁止指针嵌入：

```go
// ✅ 正确：值类型嵌入
type Order struct {
    aggregate.AggregateRoot
    Items []OrderItem
}

type Product struct {
    entity.Entity
    Name string
}

type Document struct {
    entity.AuditableEntity
    Title string
}

// ❌ 错误：指针嵌入（运行时行为不同，nil 方法调用会 panic）
type Order struct {
    *aggregate.AggregateRoot
    Items []OrderItem
}
```

初始化时对构造函数返回值解引用：

```go
order := &Order{
    AggregateRoot: *aggregate.NewAggregateRootWithApplier(id, order),
}
```

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
var lockErr *repository.OptimisticLockError
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

### 8. 存储后端选择

```go
// 生产：PostgreSQL 模式（默认）
// 设置 DDD_POSTGRES_URI 环境变量即可
DDD_POSTGRES_URI="postgres://..." ./myapp

// 开发/测试：Memory 模式
DDD_STORE_TYPE=memory ./myapp

// 切换后端不需要修改业务代码
// 应用层只依赖接口，存储层由 Provider 组装
```

PostgreSQL 是唯一的生产运行模式。Memory 模式的价值在于验证架构设计——确保依赖倒置、接口隔离等原则得到遵守。

---

## 十二、DDD Lint 规则

ddd-qce 提供基于 `go/analysis` 框架的静态分析工具，自动扫描项目代码是否满足 DDD 驱动开发规范。消费项目引入 ddd-qce 后，通过 golangci-lint 或独立 CLI 即可启用。

### 目录约定

所有领域位于 `ddd/` 目录下，每个领域是一个直接子目录：

```
ddd/
├── order/
│   ├── command/     # 公开 — 其他领域可 import
│   ├── query/       # 公开
│   ├── event/       # 公开
│   ├── domain/      # 内部 — 仅本领域可 import
│   ├── service/     # 内部
│   ├── repository/  # 内部
│   └── wire/        # 基础设施 — 唯一可 import 实现包的地方
└── inventory/
    ├── command/
    ├── query/
    ├── event/
    ├── domain/
    ├── service/
    ├── repository/
    └── wire/
```

| 包路径 | 可见性 | 说明 |
|--------|--------|------|
| `ddd/{domain}/command` | 公开 | 其他领域可 import，定义 Command、Result |
| `ddd/{domain}/query` | 公开 | 其他领域可 import，定义 Query、Result |
| `ddd/{domain}/event` | 公开 | 其他领域可 import，定义 Event |
| `ddd/{domain}/domain` | 内部 | 仅本领域可 import，包含实体、值对象、聚合根 |
| `ddd/{domain}/service` | 内部 | 仅本领域可 import，包含领域服务 |
| `ddd/{domain}/repository` | 内部 | 仅本领域可 import，包含仓储实现 |
| `ddd/{domain}/wire` | 基础设施 | 唯一可 import `cqrs/impl/*` 的地方 |

### 规则 1: dddcrossdomain — 跨领域内部包引用检查

**目的**: 保护限界上下文边界，防止领域间隐式耦合。

**检查逻辑**:
- 如果文件属于 `ddd/{domainA}/` 且 import 了 `ddd/{domainB}/domain/`、`ddd/{domainB}/service/` 或 `ddd/{domainB}/repository/`（domainA ≠ domainB），报错
- 同领域内互相引用不报错
- 引用公开包（command/query/event）不报错

**违规示例**:

```go
// ddd/inventory/command/handler.go
import "myproject/ddd/order/domain"  // ❌ 跨领域引用内部包
import "myproject/ddd/order/event"    // ✅ 引用公开包
import "myproject/ddd/inventory/domain" // ✅ 同领域引用
```

**诊断消息**:

```
ddd/inventory/command/handler.go:5:2: dddcrossdomain: package "myproject/ddd/order/domain" is internal to domain "order", import from domain "inventory" is forbidden; use command/query/event for cross-domain communication
```

### 规则 2: dddpublicleak — 公开类型泄露检查

**目的**: 防止内部领域模型通过公开 API 泄露到其他领域，保持限界上下文边界清晰。

**检查逻辑**:
- 在 `ddd/{domain}/command/`、`ddd/{domain}/query/`、`ddd/{domain}/event/` 包中
- 扫描导出 struct 的字段和导出 func 的参数/返回值
- 如果引用了**其他领域**的 `ddd/*/domain/` 类型，报错
- 同领域内的引用不报错（command handler 引用本领域 domain 是正常的）

**违规示例**:

```go
// ddd/order/command/commands.go
import inventorydomain "myproject/ddd/inventory/domain"

type GetInventoryResult struct {
    Inv inventorydomain.Inventory // ❌ 跨领域泄露内部类型
}

type PlaceOrderResult struct {
    OrderID string     // ✅ 标量值
    Status  string
}
```

**诊断消息**:

```
ddd/order/command/commands.go:8:2: dddpublicleak: type in "GetInventoryResult" references internal domain package from another domain; use scalar fields and map domain objects to Result types in handler
```

### 规则 3: dddimplimport — 实现包引用检查（依赖倒置）

**目的**: 确保应用层只依赖接口，实现包只在 wire 层引用。

**检查逻辑**:
- 在 `ddd/` 目录下，排除 `ddd/*/wire/` 包
- 如果 import 路径匹配 `*/cqrs/impl/*`，报错
- `ddd/*/wire/` 包中的 import 不报错

**违规示例**:

```go
// ddd/order/command/handler.go
import "github.com/ddd-qce/core/cqrs/impl/memory" // ❌ 非 wire 层引用实现包

// ddd/order/wire/wire.go
import "github.com/ddd-qce/core/cqrs/impl/memory" // ✅ wire 层允许
```

**诊断消息**:

```
ddd/order/command/handler.go:4:2: dddimplimport: import of implementation package "github.com/ddd-qce/core/cqrs/impl/memory" is forbidden outside wire layer; use interface packages (cqrs/command, cqrs/query, cqrs/event) instead
```

### 安装与使用

#### 方式 1: 独立 CLI

```bash
go install github.com/ddd-qce/core/lint/cmd/ddd-lint@latest
ddd-lint ./...
```

#### 方式 2: golangci-lint custom 配置

在项目 `.golangci.yml` 中添加：

```yaml
linters-settings:
  custom:
    dddcrossdomain:
      type: module
      description: "Check cross-domain internal package imports"
    dddpublicleak:
      type: module
      description: "Check domain type leaks in public packages"
    dddimplimport:
      type: module
      description: "Check CQRS impl package imports outside wire layer"
```

### 跨领域交互正确方式

当领域 A 需要与领域 B 交互时：

| 场景 | 正确方式 | 错误方式 |
|------|---------|---------|
| A 需要 B 执行操作 | A 通过 CommandBus 发送 B 的 Command | A 直接 import B 的 domain 包 |
| A 需要查询 B 的数据 | A 通过 QueryBus 发送 B 的 Query | A 直接 import B 的 repository |
| A 需要监听 B 的事件 | A 的 wire 层订阅 B 的 Event | A 直接 import B 的 domain 事件 |
| A 需要公开数据给 B | A 在 command/query/event 包中定义 Result | A 在公开包中暴露 domain 实体 |

### wire 包说明

`ddd/{domain}/wire/` 是 Composition Root（组合根），职责单一：

- **只做组装和注册** — 实例化 Handler 并注册到 Bus
- **不含业务逻辑** — 所有业务判断在 domain 或 command/query 层
- **是唯一引用实现包的地方** — `cqrs/impl/memory`、`cqrs/impl/pg` 等
- **可包含跨领域事件 Handler** — 监听其他领域公开事件并分发本领域 Command

当领域内聚合增多时，wire 可按聚合拆文件（如 `wire/order_wire.go`、`wire/refund_wire.go`）。

# AI Domain 代码生成规则

> 本文档定义使用 AI 生成基于 ddd-qce 框架的 domain 代码时必须遵守的规则。使用 ddd-qce 的项目按需引用本文档（如在 AGENTS.md / CLAUDE.md 中添加链接）。

---

## 一、目录结构约定（强制）

每个领域（Bounded Context）必须按以下目录结构组织：

```
/{domain}/
├── command/          # ✅ 公开 — Command + Result 定义 + Handler
├── query/            # ✅ 公开 — Query + Result 定义 + Handler
├── event/            # ✅ 公开 — Event 定义 + Handler
├── domain/           # ❌ 内部 — AggregateRoot + Entity + ValueObject
├── service/          # ❌ 内部 — Domain Service（可选）
└── repository/       # ❌ 内部 — Repository 实现
```

### 包可见性规则

| 包路径 | 可见性 | 说明 |
|--------|--------|------|
| `/{domain}/command` | ✅ 公开 | 其他领域可 import，定义 Command、Result 结构和 Handler |
| `/{domain}/query` | ✅ 公开 | 其他领域可 import，定义 Query、Result 结构和 Handler |
| `/{domain}/event` | ✅ 公开 | 其他领域可 import，定义 Event 结构和 Handler |
| `/{domain}/domain` | ❌ 内部 | 仅领域内部使用，包含实体、值对象、聚合根 |
| `/{domain}/service` | ❌ 内部 | 仅领域内部使用，包含领域服务 |
| `/{domain}/repository` | ❌ 内部 | 仅领域内部使用，包含仓储实现 |

---

## 二、聚合根生成规则

### 2.1 构造器

聚合根必须提供两个构造器：

```go
// 业务构造器 — 创建新聚合时使用
func NewOrder(ctx context.Context, id, userID string, items []*OrderItem) (*Order, error) {
    o := &Order{
        UserID:    userID,
        Items:     items,
        Status:    OrderStatusPending,
        CreatedAt: time.Now(),
    }
    ar, err := aggregate.NewAggregateRoot(id)
    if err != nil {
        return nil, err
    }
    o.AggregateRoot = *ar
    if err := o.validate(); err != nil {
        return nil, err
    }
    if err := o.Apply(ctx, &OrderCreatedEvent{
        BaseEvent: event.WithCorrelation(ctx, o.ID()),
        UserID:    o.UserID,
    }); err != nil {
        return nil, err
    }
    return o, nil
}

// 回溯构造器 — 事件溯源 Load 时使用
func NewOrderForReplay(id string) *Order {
    o := &Order{}
    ar, err := aggregate.NewAggregateRoot(id)
    if err != nil {
        panic(err)
    }
    o.AggregateRoot = *ar
    return o
}
```

**关键约束**：
- 使用 `aggregate.NewAggregateRoot(id)` 构造
- 回溯构造器中不需要 `ctx` 参数，因为不从业务方法调用
- 业务构造器中必须通过 `Apply(ctx, event)` 发布创建事件

### 2.2 AggregateRef 接口实现

聚合根必须实现 `AggregateRef` 接口的 `When(evt event.Event) error` 方法，并添加 `Apply` 和 `LoadFromHistory` 委托方法：

```go
func (o *Order) When(evt event.Event) error {
    switch e := evt.(type) {
    case *OrderCreatedEvent:
        o.UserID = e.UserID
        o.Status = OrderStatusPending
        o.CreatedAt = e.OccurredAt()
    case *OrderConfirmedEvent:
        o.Status = OrderStatusConfirmed
    case *OrderCancelledEvent:
        o.Status = OrderStatusCancelled
    default:
        return fmt.Errorf("order: unhandled event type %T", evt)
    }
    return nil
}

func (o *Order) Apply(ctx context.Context, evt event.Event) error {
    return aggregate.ApplyChange(o, ctx, evt)
}

func (o *Order) LoadFromHistory(events []event.Event) error {
    return aggregate.LoadFromHistory(o, events)
}
```

**关键约束**：
- 必须使用 `type switch` 匹配事件类型
- 未匹配的事件类型必须返回 error，不能静默忽略
- 事件回溯时修改状态，不发布新事件
- `Apply` 委托到 `aggregate.ApplyChange(o, ctx, evt)`
- `LoadFromHistory` 委托到 `aggregate.LoadFromHistory(o, events)`

### 2.3 业务方法

业务方法通过 `Apply(ctx, event)` 发布事件，不直接修改状态：

```go
func (o *Order) Confirm(ctx context.Context) error {
    if o.Status != OrderStatusPending {
        return fmt.Errorf("order can only be confirmed from pending status")
    }
    return o.Apply(ctx, &OrderConfirmedEvent{
        BaseEvent: event.WithCorrelation(ctx, o.ID()),
    })
}
```

**关键约束**：
- 业务方法先做前置条件校验，再 `Apply` 发布事件
- `When` 方法中修改状态，业务方法中不直接修改状态
- 事件必须使用 `event.WithCorrelation(ctx, o.ID())` 传播链路上下文

### 2.4 JSON 序列化

聚合根必须实现 `MarshalJSON` / `UnmarshalJSON` + 对应的 JSON 结构体：

```go
type OrderJSON struct {
    aggregate.AggregateRootJSON
    UserID    string        `json:"userId"`
    Status    OrderStatus   `json:"status"`
    Items     []*OrderItem  `json:"items"`
    CreatedAt time.Time     `json:"createdAt"`
}

func (o *Order) MarshalJSON() ([]byte, error) {
    return json.Marshal(OrderJSON{
        AggregateRootJSON: o.AggregateRoot.ToJSON(),
        UserID:            o.UserID,
        Status:            o.Status,
        Items:             o.Items,
        CreatedAt:         o.CreatedAt,
    })
}

func (o *Order) UnmarshalJSON(data []byte) error {
    var aux OrderJSON
    if err := json.Unmarshal(data, &aux); err != nil {
        return err
    }
    o.AggregateRoot.FromJSON(aux.AggregateRootJSON)
    o.UserID = aux.UserID
    o.Status = aux.Status
    o.Items = aux.Items
    o.CreatedAt = aux.CreatedAt
    return nil
}
```

**关键约束**：
- 必须定义对应的 `XxxJSON` 结构体，嵌入 `aggregate.AggregateRootJSON`
- `UnmarshalJSON` 中无需调用 `RestoreApplier`（已移除，`When` 是类型自身方法）
- 嵌套实体也需要实现 `MarshalJSON` / `UnmarshalJSON`

---

## 三、领域事件定义规则

```go
type OrderCreatedEvent struct {
    event.BaseEvent
    UserID string
    TotalAmount float64
}

type OrderConfirmedEvent struct {
    event.BaseEvent
}

type OrderCancelledEvent struct {
    event.BaseEvent
    Reason string
}
```

**关键约束**：
- 每个事件嵌入 `event.BaseEvent`
- 创建事件时使用 `event.WithCorrelation(ctx, aggregateID)` 或 `event.NewDomainEvent(aggregateID)`
- 事件命名：`XxxCreatedEvent` / `XxxConfirmedEvent` / `XxxCancelledEvent`（过去时态）
- 事件应为值对象语义：创建后不可修改字段

---

## 四、Command / Query 定义规则

### 4.1 Command

```go
type CreateOrderCommand struct {
    command.BaseCommand
    UserID string
    Items  []ItemInput
}

type CreateOrderResult struct {
    OrderID     string
    TotalAmount float64
}

type CreateOrderHandler struct {
    repo     OrderRepositoryAdapter
    eventBus cqrsevent.EventBus
}

func (h *CreateOrderHandler) Handle(ctx context.Context, cmd *CreateOrderCommand) (*CreateOrderResult, error) {
    // 业务逻辑...
}

var _ command.CommandHandler[*CreateOrderCommand, *CreateOrderResult] = (*CreateOrderHandler)(nil)
```

### 4.2 Query

```go
type GetOrderQuery struct {
    query.BaseQuery
    OrderID string
}

type GetOrderResult struct {
    OrderID     string
    UserID      string
    Status      string
    TotalAmount float64
}

type GetOrderHandler struct {
    repo OrderRepositoryAdapter
}

func (h *GetOrderHandler) Handle(ctx context.Context, q *GetOrderQuery) (*GetOrderResult, error) {
    // 查询逻辑...
}

var _ query.QueryHandler[*GetOrderQuery, *GetOrderResult] = (*GetOrderHandler)(nil)
```

### 4.3 Result 定义规则

Result 是 Handler 的返回值类型，属于应用层公开契约，与 Command/Query 同包同文件定义。

#### 定位

- Result 是调用方获取操作结果的数据载体，通过 `Dispatch[T, R]` 返回
- Result 对其他领域公开可见，可被跨领域 import
- Result 中**禁止**引用 `domain` 包的实体类型，避免泄露内部模型

#### 命名规范

| 源类型 | Result 命名 | 示例 |
|--------|-------------|------|
| `XxxYyyCommand` | `XxxYyyResult` | `PlaceOrderCommand` → `PlaceOrderResult` |
| `GetXxxQuery` | `GetXxxResult` | `GetOrderQuery` → `GetOrderResult` |
| `ListXxxsQuery` | `ListXxxsResult` | `ListOrdersQuery` → `ListOrdersResult` |

#### Command Result vs Query Result

**Command Result** — 通常轻量，返回操作标识和状态：

```go
type PlaceOrderResult struct {
    OrderID     string
    TotalAmount float64
}

type ConfirmPaymentResult struct {
    Success bool
}
```

**Query Result** — 作为读模型，可包含丰富的业务数据，支持嵌套结构体：

```go
type OrderItem struct {
    ProductID   string
    ProductName string
    Price       float64
    Quantity    int
}

type GetOrderResult struct {
    OrderID     string
    UserID      string
    Status      string
    TotalAmount float64
    Items       []OrderItem
    CreatedAt   string
}
```

**List 类 Query** 的 Result 可复用单个 Query 的 Result 类型：

```go
type ListOrdersResult struct {
    Orders []GetOrderResult
}
```

#### Domain → Result 映射

Handler 中需将领域对象映射为 Result，**逐字段赋值**，不直接返回领域实体：

```go
func (h *GetOrderHandler) Handle(ctx context.Context, q *GetOrderQuery) (*GetOrderResult, error) {
    order, err := h.repo.FindByID(ctx, q.OrderID)
    if err != nil {
        return nil, err
    }
    items := make([]OrderItem, len(order.Items))
    for i, item := range order.Items {
        items[i] = OrderItem{
            ProductID:   item.ProductID,
            ProductName: item.ProductName,
            Price:       item.Price,
            Quantity:    item.Quantity,
        }
    }
    return &GetOrderResult{
        OrderID:     order.ID(),
        UserID:      order.UserID,
        Status:      string(order.Status),
        TotalAmount: order.TotalAmount,
        Items:       items,
        CreatedAt:   order.CreatedAt.Format(time.RFC3339),
    }, nil
}
```

**关键约束**：
- Command 嵌入 `command.BaseCommand`，Query 嵌入 `query.BaseQuery`
- Handler 签名：`Handle(ctx context.Context, cmd T) (R, error)`
- **必须**在文件末尾添加编译期接口检查：`var _ XxxHandler[T,R] = (*Handler)(nil)`
- Handler 通过 `Dispatch[T,R](ctx, bus, cmd)` 调用，不直接调用 Handle 方法
- Result 与 Command/Query 同包同文件定义，是公开类型
- Result 统一使用指针返回 `*XxxResult`，便于错误时返回 `nil`
- Result 字段只包含标量值和同包内定义的嵌套结构体，**禁止**引用 `domain` 包的实体类型
- `Dispatch[T, R]` 调用时 R 必须与 Handler 的 Result 类型一致

---

## 五、Event Handler 定义规则

```go
type OrderCreatedNotificationHandler struct{}

func (h *OrderCreatedNotificationHandler) Handle(ctx context.Context, evt *OrderCreatedEvent) error {
    // 通知逻辑（发邮件、写日志等）
    return nil
}
```

**关键约束**：
- Event Handler 不返回结果，只返回 error
- 一个事件可以有多个 Handler（1:N），它们并行执行
- Event Handler 不应抛出 panic，框架会 recover 并转为 error

---

## 六、Repository 规则

### 6.1 应用层定义 Repository 适配器接口

```go
type OrderRepositoryAdapter interface {
    Save(ctx context.Context, order *domain.Order) error
    FindByID(ctx context.Context, id string) (*domain.Order, error)
    Delete(ctx context.Context, id string) error
    FindAll() []*domain.Order
}
```

### 6.2 快照模式 Repository

使用框架提供的 `memory.NewRepository[T]` 或 `pg.NewRepository[T]`。

### 6.3 事件溯源模式 Repository

```go
type OrderEventSourcedRepository struct {
    eventStore event.EventSourceStore[event.Event]
    snapshotRepo OrderRepositoryAdapter
}

func (r *OrderEventSourcedRepository) Save(ctx context.Context, order *domain.Order) error {
    uncommitted := order.UncommittedEvents()
    if len(uncommitted) > 0 {
        if err := r.eventStore.Append(ctx, order.ID(), order.Version()-len(uncommitted), uncommitted); err != nil {
            return err
        }
        order.MarkEventsAsCommitted()
    }
    return r.snapshotRepo.Save(ctx, order)
}

func (r *OrderEventSourcedRepository) Load(ctx context.Context, id string) (*domain.Order, error) {
    events, err := r.eventStore.Load(ctx, id, 0)
    if err != nil {
        return nil, err
    }
    if len(events) == 0 {
        return nil, fmt.Errorf("order %s not found", id)
    }
    order := domain.NewOrderForReplay(id)
    if err := order.LoadFromHistory(events); err != nil {
        return nil, err
    }
    return order, nil
}
```

**关键约束**：
- Save 时先 Append 事件到 EventStore，再 MarkEventsAsCommitted
- Load 时使用 `NewXxxForReplay(id)` + `LoadFromHistory(events)`
- 事件溯源的 expectedVersion = `aggregate.Version() - len(uncommittedEvents)`

---

## 七、Wire 注册规则

在 `infrastructure/wire.go` 中注册所有 Handler：

```go
func Setup(buses AppBuses, backend *infra.Backend) {
    // Command Handlers
    cmdBus := buses.CommandBus
    cmdBus.RegisterHandler(&application.CreateOrderHandler{Repo: orderRepo, EventBus: eventBus})

    // Query Handlers
    queryBus := buses.QueryBus
    queryBus.RegisterHandler(&application.GetOrderHandler{Repo: orderRepo})

    // Event Subscriptions
    eventBus := buses.EventBus
    eventBus.SubscribeHandler(&application.OrderCreatedNotificationHandler{})
}
```

**关键约束**：
- Wire 层是唯一 import 实现包（`cqrs/*/memory` 或 `cqrs/*/pg`）的地方
- 应用层只 import 接口包（`cqrs/command`、`cqrs/query`、`cqrs/event`）

---

## 八、禁止事项

| 编号 | 禁止操作 | 原因 | 正确方式 |
|------|----------|------|----------|
| 1 | 跨领域 import 内部包 | 破坏限界上下文边界 | 通过 Command / Query / Event 交互 |
| 2 | Handler 中直接操作其他领域的 Repository | 隐式耦合，无法独立部署 | 通过 CommandBus / EventBus 交互 |
| 3 | 跳过 `Apply()` 直接修改聚合状态 | 绕过事件溯源，状态不可回溯 | 所有状态变更通过 `Apply(ctx, event)` |
| 4 | 忽略 `event.WithCorrelation()` | 链路追踪断裂，无法跨领域追踪 | 创建事件时必须使用 `WithCorrelation(ctx, id)` |
| 5 | 在 `When()` 方法中发布新事件 | 导致无限递归或状态不一致 | `When()` 只修改状态，业务方法中发布事件 |
| 6 | Handler 直接 import `cqrs/*/memory` 或 `cqrs/*/pg` | 违反依赖倒置，无法切换存储后端 | 通过接口包 `cqrs/command` 等 + Dispatch 调用 |
| 7 | 聚合根使用 `NewEntity(id)` 时传入空字符串 | 运行时 panic | 始终确保 ID 非空 |
| 8 | Result 中引用 `domain` 包的实体类型 | 泄露内部模型，破坏限界上下文边界 | Result 只包含标量和同包嵌套结构体，在 Handler 中逐字段映射 |

---

## 九、完整文件清单

创建一个新的聚合时，需生成以下文件：

| 文件 | 包 | 说明 |
|------|-----|------|
| `domain/{name}.go` | `domain` | 聚合根 + 嵌套实体 + 状态常量 + MarshalJSON/UnmarshalJSON + When + Apply/LoadFromHistory 委托 + 业务方法 |
| `domain/{name}_events.go` | `domain` | 领域事件结构体定义 |
| `domain/{name}_test.go` | `domain` | 基础测试（Create / Confirm / Cancel / When 回溯） |
| `application/{name}_commands.go` | `application` | Command / Query / Result / 嵌套结构体 / Input 定义 |
| `application/{name}_cmd_handler.go` | `application` | Command Handler 实现 |
| `application/{name}_query_handler.go` | `application` | Query Handler 实现 + Domain→Result 映射 |
| `application/{name}_event_handler.go` | `application` | Event Handler 实现 |
| `application/{name}_repository.go` | `application` | RepositoryAdapter 接口 + InMemory / EventSourced 实现 |

---

## 十、参考实现

完整示例请参考 `exampleapp/` 目录，其中包含：

- `domain/` — Inventory 聚合根、事件定义
- `application/` — Command/Query/Event Handler、Repository
- `infrastructure/` — Wire 组装、Provider、配置
- `interfaces/http/` — HTTP 接口层

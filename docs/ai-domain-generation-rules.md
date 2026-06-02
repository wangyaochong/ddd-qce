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

### 2.0 Typed ID 定义

每个跨聚合/跨限界上下文被引用的实体，在 `domain` 包中定义 typed ID：

```go
type OrderID string

func (id OrderID) String() string { return string(id) }
func NewOrderID(s string) OrderID { return OrderID(s) }

type UserID string

func (id UserID) String() string { return string(id) }
func NewUserID(s string) UserID  { return UserID(s) }
```

**规则**：
- 仅聚合根和被跨聚合引用的实体需要 typed ID，聚合内部子实体不需要
- Typed ID 定义在 `domain` 包内，紧邻实体定义
- Event 中 ID 字段保持 `string` 类型（避免循环依赖），在 `When` 方法中转换为 typed ID
- 与框架核心（Entity、AggregateRoot、Repository 等）交互时使用 `id.String()` 转换

### 2.1 构造器

聚合根只需一个构造器：

```go
// 业务构造器 — 创建新聚合时使用
func NewOrder(ctx context.Context, id OrderID, userID UserID, items []*OrderItem) (*Order, error) {
    o := &Order{
        UserID:    userID,
        Items:     items,
        Status:    OrderStatusPending,
        CreatedAt: time.Now(),
    }
    ar, err := aggregate.NewAggregateRoot(id.String())
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
```

**关键约束**：
- 使用 `aggregate.NewAggregateRoot(id)` 构造
- 只保留一个构造器，不再需要 `NewXxxForReplay`
- 事件溯源回溯时：先调用 `NewOrder(ctx, id, ...)` 创建空壳，再调用 `order.LoadFromHistory(events)` 加载历史
- `AggregateRoot` 初始 version 为 0，`LoadFromHistory` 后自动设置

### 2.2 AggregateRef 接口实现

聚合根必须实现 `AggregateRef` 接口的 `When(evt domainevent.Event) error` 方法，并添加 `Apply` 和 `LoadFromHistory` 委托方法：

```go
func (o *Order) When(evt domainevent.Event) error {
	switch e := evt.(type) {
	case *OrderCreatedEvent:
		o.UserID = UserID(e.UserID)
		o.Status = OrderStatusPending
		o.CreatedAt = e.OccurredAt()
	case *OrderConfirmedEvent:
		o.Status = OrderStatusConfirmed
	default:
		return fmt.Errorf("order: unhandled event type %T", evt)
	}
	return nil
}

func (o *Order) Apply(ctx context.Context, evt domainevent.Event) error {
	return aggregate.ApplyChange(o, ctx, evt)
}

func (o *Order) LoadFromHistory(events []domainevent.Event) error {
	return aggregate.LoadFromHistory(o, events)
}
```

**关键约束**：
- 必须使用 `type switch` 匹配事件类型
- 未匹配的事件类型必须返回 error，不能静默忽略
- 事件回溯时修改状态，不发布新事件
- `Apply` 委托到 `aggregate.ApplyChange(o, ctx, evt)`
- `LoadFromHistory` 委托到 `aggregate.LoadFromHistory(o, events)`
- `When`/`Apply`/`LoadFromHistory` 的参数类型为 `domainevent.Event`（`github.com/ddd-qce/core/domain/event`）
- 导入别名约定：`domainevent "github.com/ddd-qce/core/domain/event"`，`cqrsevent "github.com/ddd-qce/core/cqrs/event"`
- `domainevent.Event` 接口包含 `AggregateID()`、`OccurredAt()`、`CorrelationID()`、`CausationID()` 四个方法
- `When` 中可直接通过类型断言到具体事件类型后调用 `e.OccurredAt()`

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

聚合根和实体字段须加 `json` tag。`AggregateRoot` 和 `Entity` 已内置反射自动 JSON 序列化，用户无需手写任何 JSON 委托代码：

```go
type Order struct {
    aggregate.AggregateRoot
    UserID       UserID       `json:"userId"`
    Status       OrderStatus  `json:"status"`
    Items        []*OrderItem `json:"items"`
    TotalAmount  float64      `json:"totalAmount"`
    CreatedAt    time.Time    `json:"createdAt"`
}
// 无需手写 MarshalJSON/UnmarshalJSON

type OrderItem struct {
    entity.Entity
    ProductName string  `json:"productName"`
    Price       float64 `json:"price"`
    Quantity    int     `json:"quantity"`
}
// 无需手写 MarshalJSON/UnmarshalJSON
```

**关键约束**：
- 聚合根和嵌套实体的 exported 字段必须添加 `json` tag
- `AggregateRoot` 和 `Entity` 自带 `MarshalJSON`/`UnmarshalJSON` 实现，自动处理嵌入字段
- 如果用户需要自定义序列化，直接在自己的类型上实现 `json.Marshaler`
- Typed ID（如 `UserID`）不需要转换为 `string`，Go 标准 json 包可直接序列化基于 `string` 的自定义类型

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
    UserID UserID
    Items  []ItemInput
}

type CreateOrderResult struct {
    OrderID     OrderID
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
    OrderID OrderID
}

type GetOrderResult struct {
    OrderID     OrderID
    UserID      UserID
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
    OrderID     OrderID
    TotalAmount float64
}

type ConfirmPaymentResult struct {
    Success bool
}
```

**Query Result** — 作为读模型，可包含丰富的业务数据，支持嵌套结构体：

```go
type OrderItem struct {
    ProductID   ProductID
    ProductName string
    Price       float64
    Quantity    int
}

type GetOrderResult struct {
    OrderID     OrderID
    UserID      UserID
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
    order, err := h.repo.FindByID(ctx, q.OrderID.String())
    if err != nil {
        return nil, err
    }
    items := make([]OrderItem, len(order.Items))
    for i, item := range order.Items {
        items[i] = OrderItem{
            ProductID:   ProductID(item.ProductID),
            ProductName: item.ProductName,
            Price:       item.Price,
            Quantity:    item.Quantity,
        }
    }
    return &GetOrderResult{
        OrderID:     OrderID(order.ID()),
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
- Result 字段只包含标量值、typed ID 和同包内定义的嵌套结构体，**禁止**引用 `domain` 包的实体类型
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
    eventStore cqrsevent.AggregateEventStore[domainevent.Event]
    eventBus   cqrsevent.EventBus
    orderRepo  OrderRepositoryAdapter
}

func (r *OrderEventSourcedRepository) Save(ctx context.Context, order *domain.Order) error {
    uncommitted := order.UncommittedEvents()
    if len(uncommitted) > 0 {
        if err := r.eventStore.Append(ctx, order.ID(), order.Version()-len(uncommitted), uncommitted); err != nil {
            return err
        }
        order.MarkEventsAsCommitted()
        for _, evt := range uncommitted {
            if err := r.eventBus.Publish(ctx, evt); err != nil {
                return fmt.Errorf("publish event %T: %w", evt, err)
            }
        }
    }
    return r.orderRepo.Save(ctx, order)
}

func (r *OrderEventSourcedRepository) Load(ctx context.Context, id string) (*domain.Order, error) {
    events, err := r.eventStore.Load(ctx, id, 0)
    if err != nil {
        return nil, err
    }
    if len(events) == 0 {
        return nil, fmt.Errorf("order %s not found", id)
    }
    order := domain.NewOrderForReplay(domain.OrderID(id))
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
| 9 | Event 中使用 typed ID 类型 | 导致 domain 与 event 包循环依赖 | Event 中 ID 字段保持 `string`，在 `When` / Handler 中转换为 typed ID |
| 10 | 传 nil 给 `CommandNameOf` / `QueryNameOf` / `EventTypeOf` | 运行时 panic，生产环境不可恢复 | 确保传入的对象非 nil；如需安全获取类型名可先判空再调用 |
| 10 | 对 `domain/event.Event` 做类型断言转 `cqrs/event.Event` | 事件接口已统一，不再需要类型断言 | 直接传递 `domain/event.Event`，EventBus 和 EventStore 均接受 `domain/event.Event` |

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

## 十、DDD Lint 自动检查

ddd-qce 内置 DDD Lint 静态分析规则，可在 CI 中自动检测上述禁止事项。引入 ddd-qce 的项目应配置 golangci-lint custom 或独立运行 `ddd-lint`。

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
    └── ...
```

### Lint 规则与禁止事项对应关系

| Lint 规则 | 对应禁止事项 | 检查内容 |
|-----------|-------------|---------|
| `dddcrossdomain` | 禁止事项 #1, #2 | 跨领域 import 内部包（domain/service/repository） |
| `dddpublicleak` | 禁止事项 #8 | 公开包（command/query/event）中引用其他领域 domain 类型 |
| `dddimplimport` | 禁止事项 #6 | 非 wire 层 import `cqrs/impl/*` 实现包 |
| `dddeventimmutable` | 新增 | 禁止事件构造后修改 `BaseEvent` 字段（`AggregateID`/`OccurredAt`/`CorrelationID`/`CausationID`） |

### 集成方式

在项目 `.golangci.yml` 中添加：

```yaml
linters-settings:
  custom:
    dddcrossdomain:
      type: module
      description: "Check cross-domain internal package imports"
    ddddpublicleak:
      type: module
      description: "Check domain type leaks in public packages"
    dddimplimport:
      type: module
      description: "Check CQRS impl package imports outside wire layer"
```

或独立运行：

```bash
go install github.com/ddd-qce/core/lint/cmd/ddd-lint@latest
ddd-lint ./...
```

详细使用说明请参考[实战指南](guide.md#十二ddd-lint-规则)。

---

## 十一、参考实现

完整示例请参考 `exampleapp/` 目录，其中包含：

- `domain/` — Inventory 聚合根、事件定义
- `application/` — Command/Query/Event Handler、Repository
- `infrastructure/` — Wire 组装、Provider、配置
- `interfaces/http/` — HTTP 接口层

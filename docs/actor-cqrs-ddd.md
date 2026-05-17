# Actor 模型 + CQRS + DDD 组合架构详解

这三个技术是现代分布式系统设计的黄金三角，它们各自解决不同层面的问题，组合在一起能构建出高并发、高可靠、易维护、业务表达力强的后端系统。特别适合处理复杂业务逻辑、长耗时任务和高并发场景。

## 一、三个概念的核心本质

### 1.1 DDD（领域驱动设计）：业务建模的方法论

**核心思想**：以业务领域为中心，而不是以技术为中心来设计系统。

**解决的问题**：
- 业务逻辑与技术代码混杂，难以维护
- 开发人员与业务人员沟通障碍
- 系统架构随业务复杂度增加而失控

**核心概念**：

| 概念 | 说明 |
|------|------|
| 聚合根 (Aggregate Root) | 业务对象的边界，是事务一致性的基本单位 |
| 实体 (Entity) | 有唯一标识的业务对象 |
| 值对象 (Value Object) | 无唯一标识，通过属性值相等来判断 |
| 领域事件 (Domain Event) | 领域中发生的重要事情 |
| 限界上下文 (Bounded Context) | 业务领域的边界划分 |

**简单理解**：DDD 教你如何把复杂的业务需求翻译成清晰的代码结构，让代码 "说业务语言"。

### 1.2 CQRS（命令查询职责分离）：架构模式

**核心思想**：将写操作 (Command) 和读操作 (Query) 完全分离，使用不同的模型处理。

**解决的问题**：
- 传统 CRUD 架构中读写模型冲突
- 复杂查询难以用领域模型表达
- 读写性能无法独立优化

**核心概念**：

| 概念 | 说明 |
|------|------|
| Command | 改变系统状态的操作（创建、更新、删除） |
| Query | 不改变系统状态的操作（查询） |
| 写模型 | 处理 Command，遵循 DDD 领域规则 |
| 读模型 | 处理 Query，针对查询优化 |
| 事件总线 | 写模型生成领域事件，读模型订阅事件更新自己 |

**简单理解**：CQRS 让你用最合适的方式做最合适的事 —— 写操作严格遵循业务规则，读操作怎么快怎么来。

### 1.3 Actor 模型：并发执行模型

**核心思想**：将系统分解为独立的、通过消息通信的 Actor，每个 Actor 有自己的状态和行为，一次只处理一个消息。

**解决的问题**：
- 传统共享内存并发模型的锁竞争和死锁问题
- 长耗时任务阻塞请求线程
- 系统故障恢复困难
- 分布式系统的状态管理

**核心概念**：

| 概念 | 说明 |
|------|------|
| Actor | 基本计算单元，有自己的状态和邮箱 |
| 消息 | Actor 之间通信的唯一方式 |
| 邮箱 | 存储待处理消息的队列 |
| 行为 | Actor 处理消息的逻辑 |
| 监督 | 父 Actor 监督子 Actor 的故障 |

**简单理解**：Actor 模型就像现实世界中的人 —— 每个人独立工作，通过发消息沟通，不会互相干扰，一个人出问题不会影响其他人。

---

## 二、为什么这三个技术总是被放在一起？

它们是完美互补的关系，各自解决不同层面的问题，组合在一起形成一个完整的系统设计方案：

```
DDD 提供业务建模方法 → 告诉我们"做什么"
CQRS 提供架构分离模式 → 告诉我们"怎么做"
Actor模型提供并发执行模型 → 告诉我们"怎么高效地做"
```

### 2.1 DDD 与 CQRS 的协同

- DDD 的聚合根天然适合作为 CQRS 的写模型
- CQRS 解决了 DDD 中复杂查询难以用领域模型表达的问题
- DDD 的领域事件是 CQRS 中读写模型同步的桥梁

### 2.2 CQRS 与 Actor 模型的协同

- CQRS 的 Command 天然适合作为 Actor 的消息
- Actor 模型解决了 CQRS 中 Command 处理的并发问题
- Actor 的邮箱天然实现了 Command 的排队执行
- Actor 的状态隔离保证了聚合根的事务一致性

### 2.3 DDD 与 Actor 模型的协同

- DDD 的聚合根可以直接映射为 Actor
- Actor 的单线程处理保证了聚合根的不变量不会被并发破坏
- Actor 的监督机制提供了领域对象的故障恢复能力
- Actor 的位置透明性让 DDD 的限界上下文可以轻松分布到不同节点

---

## 三、组合架构的完整工作流程

以 "用户下单" 这个常见业务场景为例：

### 3.1 整体架构图

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   API Gateway   │    │  Command Service │    │   Query Service │
│ (接收HTTP请求)  │───>│ (处理写操作)    │    │ (处理读操作)    │
└─────────────────┘    └─────────────────┘    └─────────────────┘
                              │                        ▲
                              ▼                        │
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│  Order Actor    │    │  Event Bus      │    │  Read Model     │
│ (聚合根)        │───>│ (领域事件总线)  │───>│ (数据库/缓存)   │
└─────────────────┘    └─────────────────┘    └─────────────────┘
```

### 3.2 详细执行步骤

1. **用户发送下单请求**到 API Gateway
2. **API Gateway** 将请求转换为 `CreateOrderCommand`
3. **Command Service** 将 Command 发送给对应的 `Order Actor`
4. **Order Actor** 从邮箱中取出 Command，单线程处理：
   - 验证业务规则（库存是否充足、用户是否有足够余额）
   - 更新自己的状态（订单状态从 "新建" 变为 "已确认"）
   - 生成 `OrderCreatedEvent` 领域事件
5. **Order Actor** 将 `OrderCreatedEvent` 发布到 Event Bus
6. **多个订阅者**处理这个事件：
   - `Inventory Actor`：扣减库存
   - `Payment Actor`：处理支付
   - `Read Model Updater`：更新读数据库中的订单信息
7. **用户通过 Query Service** 查询订单状态，直接从读数据库获取结果

---

## 四、这个组合架构的核心优势

### 4.1 业务表达力强

- 代码结构与业务结构高度一致
- 业务规则集中在聚合根 (Actor) 中，不会分散
- 开发人员与业务人员使用相同的语言沟通

### 4.2 高并发性能

- 每个 Actor 独立运行，充分利用多核 CPU
- 没有锁竞争和死锁问题
- 读写分离，读写性能可以独立优化
- 长耗时任务在 Actor 中异步处理，不阻塞请求线程

### 4.3 高可靠性

- Actor 状态隔离，一个 Actor 崩溃不会影响其他 Actor
- 监督机制可以自动重启失败的 Actor
- 事件溯源可以重建任何时间点的系统状态
- 消息持久化保证不丢失

### 4.4 可扩展性好

- 可以轻松将不同的 Actor 部署到不同的节点
- 读模型可以水平扩展
- 可以根据业务负载动态调整 Actor 数量

### 4.5 特别适合长耗时任务

- 任务在 Actor 中异步执行，不阻塞用户请求
- Actor 状态持久化，系统重启后自动恢复
- 可以通过消息随时查询任务进度
- 可以通过消息暂停、恢复、取消任务
- 分阶段执行可以通过 Actor 的状态机实现

---

## 五、Go 语言中的实现方式

Go 语言的 `goroutine` 和 `channel` 天然适合实现 Actor 模型，不需要复杂的框架。

### 5.1 基础 Actor 接口

```go
package actor

import (
    "context"
    "log"
)

// Actor 基础接口
type Actor interface {
    Receive(ctx context.Context, msg any) error
}

// BaseActor 基础实现
type BaseActor struct {
    ID      string
    mailbox chan any
    ctx     context.Context
    cancel  context.CancelFunc
}

// NewBaseActor 创建 Actor
func NewBaseActor(id string) *BaseActor {
    ctx, cancel := context.WithCancel(context.Background())
    return &BaseActor{
        ID:      id,
        mailbox: make(chan any, 100),
        ctx:     ctx,
        cancel:  cancel,
    }
}

// Start 启动 Actor
func (a *BaseActor) Start(actor Actor) {
    go func() {
        for {
            select {
            case msg := <-a.mailbox:
                if err := actor.Receive(a.ctx, msg); err != nil {
                    log.Printf("Actor %s error: %v", a.ID, err)
                }
            case <-a.ctx.Done():
                return
            }
        }
    }()
}

// Send 发送消息
func (a *BaseActor) Send(msg any) {
    select {
    case a.mailbox <- msg:
    default:
        log.Printf("Actor %s mailbox is full", a.ID)
    }
}

// Stop 停止 Actor
func (a *BaseActor) Stop() {
    a.cancel()
}
```

### 5.2 订单聚合根 Actor 实现

```go
package order

import (
    "context"
    "time"

    "github.com/yourproject/actor"
    "github.com/yourproject/event"
)

type OrderStatus string

const (
    OrderStatusNew       OrderStatus = "new"
    OrderStatusConfirmed OrderStatus = "confirmed"
    OrderStatusPaid      OrderStatus = "paid"
    OrderStatusShipped   OrderStatus = "shipped"
    OrderStatusCompleted OrderStatus = "completed"
    OrderStatusCancelled OrderStatus = "cancelled"
)

var (
    ErrInvalidOrderStatus     = errors.New("invalid order status")
    ErrCannotCancelShippedOrder = errors.New("cannot cancel shipped order")
)

type Order struct {
    *actor.BaseActor
    ID         string
    UserID     string
    Items      []OrderItem
    TotalPrice float64
    Status     OrderStatus
    CreatedAt  time.Time
    eventBus   event.Bus
}

func NewOrder(id string, userID string, items []OrderItem, totalPrice float64, eventBus event.Bus) *Order {
    return &Order{
        BaseActor:  actor.NewBaseActor(id),
        ID:         id,
        UserID:     userID,
        Items:      items,
        TotalPrice: totalPrice,
        Status:     OrderStatusNew,
        CreatedAt:  time.Now(),
        eventBus:   eventBus,
    }
}

func (o *Order) Receive(ctx context.Context, msg any) error {
    switch m := msg.(type) {
    case ConfirmOrderCommand:
        return o.handleConfirmOrder(ctx, m)
    case PayOrderCommand:
        return o.handlePayOrder(ctx, m)
    case CancelOrderCommand:
        return o.handleCancelOrder(ctx, m)
    default:
        return nil
    }
}

func (o *Order) handleConfirmOrder(ctx context.Context, cmd ConfirmOrderCommand) error {
    if o.Status != OrderStatusNew {
        return ErrInvalidOrderStatus
    }

    o.Status = OrderStatusConfirmed

    o.eventBus.Publish(ctx, OrderConfirmedEvent{
        OrderID: o.ID,
        UserID:  o.UserID,
    })

    return nil
}

func (o *Order) handlePayOrder(ctx context.Context, cmd PayOrderCommand) error {
    if o.Status != OrderStatusConfirmed {
        return ErrInvalidOrderStatus
    }

    o.Status = OrderStatusPaid

    o.eventBus.Publish(ctx, OrderPaidEvent{
        OrderID:    o.ID,
        UserID:     o.UserID,
        PaymentID:  cmd.PaymentID,
        PaidAmount: cmd.Amount,
    })

    return nil
}

func (o *Order) handleCancelOrder(ctx context.Context, cmd CancelOrderCommand) error {
    if o.Status == OrderStatusShipped || o.Status == OrderStatusCompleted {
        return ErrCannotCancelShippedOrder
    }

    o.Status = OrderStatusCancelled

    o.eventBus.Publish(ctx, OrderCancelledEvent{
        OrderID: o.ID,
        UserID:  o.UserID,
        Reason:  cmd.Reason,
    })

    return nil
}
```

### 5.3 Command Bus 集成

```go
package command

import (
    "context"
    "fmt"
    "reflect"
    "sync"

    "github.com/yourproject/actor"
)

type Bus interface {
    Execute(ctx context.Context, cmd any) error
    RegisterActor(actorID string, actor actor.Actor)
}

type InMemoryBus struct {
    actors map[string]actor.Actor
    mu     sync.RWMutex
}

func NewInMemoryBus() *InMemoryBus {
    return &InMemoryBus{
        actors: make(map[string]actor.Actor),
    }
}

func (b *InMemoryBus) Execute(ctx context.Context, cmd any) error {
    actorID, err := extractActorID(cmd)
    if err != nil {
        return err
    }

    b.mu.RLock()
    actor, exists := b.actors[actorID]
    b.mu.RUnlock()

    if !exists {
        return fmt.Errorf("actor %s not found", actorID)
    }

    actor.Send(cmd)
    return nil
}

func (b *InMemoryBus) RegisterActor(actorID string, actor actor.Actor) {
    b.mu.Lock()
    defer b.mu.Unlock()
    b.actors[actorID] = actor
}

func extractActorID(cmd any) (string, error) {
    val := reflect.ValueOf(cmd)
    if val.Kind() == reflect.Ptr {
        val = val.Elem()
    }
    idField := val.FieldByName("ID")
    if !idField.IsValid() {
        return "", fmt.Errorf("command %T has no ID field", cmd)
    }
    return idField.String(), nil
}
```

---

## 六、与 DDD-QCE 框架的结合

DDD-QCE 框架的 Command/Event/Query 总线系统与 Actor 模型天然契合：

### 6.1 架构映射

| DDD-QCE 组件 | Actor 模型对应 | 说明 |
|-------------|---------------|------|
| Command | Actor 消息 | Command 发送到 Actor 邮箱 |
| CommandHandler | Actor.Receive() | Actor 单线程处理 Command |
| EventBus | Actor 间事件广播 | 领域事件发布到多个 Actor |
| Aggregate Root | Actor 实例 | 每个聚合根是一个 Actor |
| Bounded Context | Actor 组 | 同一领域的 Actor 集合 |

### 6.2 结合示例

```go
package main

import (
    "context"

    "github.com/ddd-qce/core/aspect"
    "github.com/ddd-qce/core/aspect/builtin"
    commandmemory "github.com/ddd-qce/core/command/memory"
    eventmemory "github.com/ddd-qce/core/event/memory"
    "github.com/ddd-qce/core/trace"
)

// OrderActor 订单聚合根 Actor
type OrderActor struct {
    *actor.BaseActor
    order    *Order
    eventBus *eventmemory.EventBus
}

func (a *OrderActor) Receive(ctx context.Context, msg any) error {
    switch cmd := msg.(type) {
    case CreateOrderCommand:
        return a.handleCreateOrder(ctx, cmd)
    case ConfirmOrderCommand:
        return a.handleConfirmOrder(ctx, cmd)
    }
    return nil
}

func (a *OrderActor) handleCreateOrder(ctx context.Context, cmd CreateOrderCommand) error {
    // 业务规则验证
    order := NewOrder(cmd.UserID, cmd.Items)
    a.order = order

    // 发布领域事件
    a.eventBus.Publish(ctx, OrderCreatedEvent{
        OrderID: order.ID,
        UserID:  order.UserID,
    })

    return nil
}

func main() {
    ctx := context.Background()

    // 1. 创建 DDD-QCE 基础设施
    traceStore := trace.NewInMemoryTraceStore()
    chain := aspect.NewAspectChain()
    chain.RegisterCommandAspect(&builtin.TracingAspect{Store: traceStore})
    chain.RegisterEventAspect(&builtin.TracingAspect{Store: traceStore})

    cBus := commandmemory.NewCommandBus(chain)
    eBus := eventmemory.NewEventBus(chain)

    // 2. 创建 Actor
    orderActor := &OrderActor{
        BaseActor: actor.NewBaseActor("order-123"),
        eventBus:  eBus,
    }
    orderActor.Start(orderActor)

    // 3. 注册到 Command Bus
    commandmemory.RegisterCommand(cBus, &CreateOrderHandler{actor: orderActor})

    // 4. 执行 Command → Actor 接收消息 → 发布 Event
    cBus.Dispatch[CreateOrderCommand, string](ctx, CreateOrderCommand{
        UserID: "user-456",
        Items:  []OrderItem{{ProductID: "prod-1", Quantity: 2}},
    })
}
```

### 6.3 优势叠加

| 维度 | DDD-QCE 提供 | Actor 模型提供 | 组合效果 |
|------|-------------|---------------|---------|
| 业务建模 | Command/Event/Query 接口 | 聚合根 = Actor | 业务逻辑清晰且并发安全 |
| 并发处理 | Bus 调度 | 邮箱排队 + 单线程 | 无锁并发 + 顺序保证 |
| 故障恢复 | Job 重试机制 | Actor 监督机制 | 多层次容错 |
| 链路追踪 | Trace 上下文传播 | Actor 间消息追踪 | 完整调用链可视化 |
| 部署灵活 | 接口与实现分离 | Actor 位置透明 | 单体/微服务自由切换 |

---

## 七、适用场景与不适用场景

### 特别适合的场景

| 场景 | 说明 |
|------|------|
| 复杂业务逻辑系统 | 电商、金融、物流等 |
| 高并发系统 | 每秒处理数千甚至数万请求 |
| 长耗时任务系统 | 数据导出、报表生成、批量处理等 |
| 需要高可靠性的系统 | 不能丢失数据或状态 |
| 分布式系统 | 需要跨多个服务协作的系统 |

### 不适合的场景

| 场景 | 说明 |
|------|------|
| 简单 CRUD 系统 | 没有复杂业务逻辑的管理后台 |
| 极低延迟要求的系统 | 亚毫秒级响应的系统 |
| 非常小的项目 | 只有几个接口的微型服务 |
| 团队技术能力不足 | 团队对这三个概念不熟悉 |

---

## 八、Go 语言生态中的相关框架

如果不想自己实现所有基础组件，可以使用以下框架：

### Actor 模型框架

| 框架 | 说明 |
|------|------|
| [Proto.Actor](https://proto.actor/) | Go 语言中最成熟的 Actor 框架，支持分布式 |
| [Go-Actor](https://github.com/Anthology/go-actor) | 轻量级 Actor 框架 |
| [Ergo](https://github.com/ergo-services/ergo) | 受 Erlang 启发的 Actor 框架 |

### CQRS/DDD 框架

| 框架 | 说明 |
|------|------|
| [Watermill](https://watermill.io/) | Go 语言的消息处理库，非常适合实现 CQRS |
| [Go-CQRS](https://github.com/mojocn/cqrs) | 轻量级 CQRS 框架 |
| [EventStore](https://www.eventstore.com/) | 事件存储数据库 |

### 全栈框架

| 框架 | 说明 |
|------|------|
| [Axon-Go](https://github.com/hetiandev/axon-go) | Java Axon Framework 的 Go 版本，完整支持 DDD+CQRS+Event Sourcing |

---

## 九、总结

Actor 模型 + CQRS + DDD 是一个强大而优雅的组合架构：

- **DDD** 让你的代码反映业务本质
- **CQRS** 让你的架构清晰分离
- **Actor 模型** 让你的系统高效可靠

对于天级超长任务场景，这个组合是目前最好的解决方案。它不仅能解决任务的持久化、故障恢复、进度追踪等技术问题，还能让你的业务逻辑保持清晰和可维护。

在小型项目中，不需要使用复杂的框架，只需要用 Go 的 `goroutine` 和 `channel` 实现一个简单的 Actor 模型，再结合 CQRS 的思想，就能获得大部分好处。

DDD-QCE 框架提供了这个组合架构所需的全部基础设施，你可以直接使用框架的 Command/Event/Query 总线，结合自己实现的 Actor 模型，构建出高并发、高可靠的业务系统。

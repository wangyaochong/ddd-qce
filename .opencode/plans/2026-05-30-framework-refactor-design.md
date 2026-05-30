# DDD-QCE 框架全面重构设计

> 日期：2026-05-30
> 策略：B+C 结合 — 一次性 Breaking Change + Lint 规则完善
> 原则：只保留最优秀最清晰的代码，不向后兼容，不 fallback

---

## 一、正确性/安全性修复（P0）

### 1.1 Job 字段私有化

**问题**：`Job` 结构体的 `ID`, `Command`, `CommandType`, `CreatedAt`, `Timeout`, `RetryCount`, `MaxRetries` 为公开字段，外部可直接修改绕过状态机。

**方案**：
- 所有字段改为小写私有
- 添加 getter 方法：`ID()`, `Command()`, `CommandType()`, `CreatedAt()`, `Timeout()`, `RetryCount()`, `MaxRetries()`
- `RestoreJobState` 添加状态转换校验（只允许合法转换：Pending→Running, Running→Completed/Failed/Cancelled, Failed→Pending(retry)）
- `Snapshot()` 对 `Command` 做深拷贝（当前浅拷贝共享引用）

**影响文件**：`job/core/job.go`, `job/memory/job_manager.go`, `job/memory/job_store.go`, `job/pg/job_store.go`, `job/core/jobtest/`, 所有 job 测试文件, exampleapp

### 1.2 EventStore shallowCopy 安全修复

**问题**：当 `T` 是接口类型时，memory EventStore 静默降级为 shallowCopy，允许外部修改内部状态。PG 实现则要求 `WithFactory`。行为不一致。

**方案**：
- 接口类型 `T` 必须提供 `WithFactory` 选项，否则 `NewEventSourceStore` 返回 error
- 删除 `shallowCopy` 模式和相关代码
- memory/PG 实现行为统一

**影响文件**：`cqrs/impl/memory/event_store.go`, `cqrs/impl/memory/event_store_test.go`

### 1.3 PG isUniqueViolation 修复

**问题**：`isUniqueViolation` 使用自定义 `SQLState()` 接口，可能不匹配实际驱动错误类型，导致乐观锁检测静默失败。

**方案**：
- 使用 `*pgconn.PgError`（项目已依赖 pgx）检测唯一约束冲突
- 删除 `SQLState()` 接口 fallback

**影响文件**：`cqrs/impl/pg/event_store.go`, `infra/repository/pg/repository.go`

### 1.4 GetQuerier 数据竞争修复

**问题**：`GetQuerier` 读取 `state.tx` 不加锁，与 `Begin`/`Commit`/`Rollback` 存在数据竞争。

**方案**：
- `GetQuerier` 在读取 `state.tx` 前加 `state.mu.Lock()/Unlock()`

**影响文件**：`pg/transaction.go`

### 1.5 BaseEvent 可变性收紧

**问题**：`BaseEvent` 有 `SetCorrelation` 和 `Restore` 方法，允许外部修改事件状态。事件应为不可变。

**方案**：
- 删除 `SetCorrelation` 方法
- 删除 `Restore` 方法
- `ApplyCorrelation` 通过 `domainevent.CorrelationSetter` 接口直接设置私有字段（在 `domainevent` 包内实现，利用包内可见性）
- `RestoreBaseEvent` 同理，通过 `domainevent.Restorer` 接口在包内实现
- `BaseEvent` 的 `correlationID`/`causationID` 字段改为包内可写（小写），通过 `domainevent` 包内的函数设置

**影响文件**：`cqrs/event/event.go`, `domain/domainevent/event.go`, `cqrs/impl/pg/event_store.go`, `infra/repository/pg/repository.go`

### 1.6 EventStore.Append key 一致性

**问题**：`Append` 方法用 `evt.AggregateID()` 作为 map key，但用 `aggregateID` 参数做版本检查。两者不一致可能导致数据损坏。

**方案**：
- 统一使用 `aggregateID` 参数作为 map key
- 添加校验：`aggregateID != evt.AggregateID()` 时返回 error

**影响文件**：`cqrs/impl/memory/event_store.go`

### 1.7 ValueObject 不可变性

**问题**：`UnmarshalJSON` 后 `validate` 函数丢失，后续 `Validate()` 使用零值检查而非原始验证器。

**方案**：
- 添加 `RegisterValidator[T](name string, validate func(T) error)` 全局注册表
- `MarshalJSON` 写入 validator 名称，`UnmarshalJSON` 从注册表恢复
- 未注册的 validator 在反序列化时使用零值检查

**影响文件**：`domain/valueobject/valueobject.go`

---

## 二、设计一致性重构（P1）

### 2.1 CQRS 命名统一

| 旧 API | 新 API | 说明 |
|--------|--------|------|
| `EventTypeOf` | `EventNameOf` | 与 `CommandNameOf`/`QueryNameOf` 统一 |
| `WithBusAspectChain` | 删除 | 只保留 `WithEventBusAspectChain` |
| `Dispatch[T]` for events | 删除 | EventBus 只保留 `Publish(ctx, evt)` |
| `RegisterEvent[T]` | `RegisterHandler[T]` | 与 Command/Query 一致 |

保留的语义差异（1:1 vs 1:N）：
- `SubscribeHandler` — 保留（事件 1:N 语义）
- `SubscribedTypes` — 保留（与 SubscribeHandler 一致）
- `Publish` — 保留（与 RegisterHandler/Execute 语义不同）

### 2.2 CommandBus + QueryBus 代码去重

**问题**：`command_bus.go` 和 `query_bus.go` 结构几乎完全相同，大量重复代码。

**方案**：
- 新建 `cqrs/impl/memory/bus_core.go`：
  ```go
  type invokerFunc func(ctx context.Context, msg any) (any, error)

  type messageBus struct {
      handlers  map[reflect.Type]any
      invokers  map[reflect.Type]invokerFunc
      chain     *aspect.AspectChain
      mu        sync.RWMutex
      closed    atomic.Bool
      inFlight  sync.WaitGroup
      stopCh    chan struct{}
  }
  ```
- `messageBus` 提供：`registerHandler`, `registeredTypes`, `shutdown`, `execute` 通用实现
- `CommandBus` 和 `QueryBus` 嵌入 `messageBus`，只保留各自特有的 aspect chain 调用方法
- `invokerFunc` 签名统一为 `func(ctx context.Context, msg any) (any, error)`（修复当前 payload 在 ctx 前面的 Go 约定违反）
- `makeCommandInvoker`/`makeQueryInvoker` 合并为 `makeInvoker`
- `typeAssert` 提取到 `bus_helpers.go` 共享
- `ErrBusClosed` 移到 `bus_core.go`

**影响文件**：`cqrs/impl/memory/command_bus.go`, `cqrs/impl/memory/query_bus.go`, 新建 `bus_core.go`

### 2.3 Span.Type/Status 改为类型常量

```go
type SpanType string
const (
    SpanTypeCommand SpanType = "command"
    SpanTypeQuery   SpanType = "query"
    SpanTypeEvent   SpanType = "event"
)

type SpanStatus string
const (
    SpanStatusSuccess SpanStatus = "success"
    SpanStatusError   SpanStatus = "error"
)
```

**影响文件**：`trace/span.go`, `trace/store.go`, `trace/pg/trace_store.go`, `aspect/builtin/tracing.go`, `observability/`

### 2.4 EventSourceStore 接口拆分

**问题**：`EventSourceStore[T]` 混合了聚合事件存储（Append/Load）和全局事件日志（LoadAll）两个职责，违反 ISP。

**方案**：彻底拆分为两个独立接口，删除组合接口：

```go
// 聚合事件存储 — 事件溯源仓储使用
type AggregateEventStore[T domainevent.Event] interface {
    Append(ctx context.Context, aggregateID string, expectedVersion int, events []T) error
    Load(ctx context.Context, aggregateID string, afterVersion int) ([]T, error)
}

// 全局事件日志 — 投影/订阅使用
type GlobalEventStore[T domainevent.Event] interface {
    LoadAll(ctx context.Context, afterPosition int64) ([]GlobalEvent[T], error)
}
```

- memory/PG 实现同时实现两个接口
- `EventSourceStore` 组合接口删除
- `repository.EventSourcingRepository` 依赖 `AggregateEventStore`
- 投影/订阅消费者依赖 `GlobalEventStore`

**影响文件**：`cqrs/event/event.go`, `cqrs/impl/memory/event_store.go`, `cqrs/impl/pg/event_store.go`, `domain/repository/repository.go`, `infra/repository/`, exampleapp

### 2.5 统一 Bus 生命周期接口

```go
type CommandBus interface {
    Execute(ctx context.Context, cmd any) (any, error)
    RegisterHandler(handler any) error
    RegisteredTypes() []string
    Shutdown(ctx context.Context) error  // 新增
}

type QueryBus interface {
    Execute(ctx context.Context, query any) (any, error)
    RegisterHandler(handler any) error
    RegisteredTypes() []string
    Shutdown(ctx context.Context) error  // 新增
}

type EventBus interface {
    SubscribeHandler(handler any) error
    Publish(ctx context.Context, evt domainevent.Event) error
    SubscribedTypes() []string
    Shutdown(ctx context.Context) error  // 新增
}
```

**影响文件**：`cqrs/command/command.go`, `cqrs/query/query.go`, `cqrs/event/bus.go`

### 2.6 TraceStore 接口添加 Close

```go
type TraceStore interface {
    RecordSpan(ctx context.Context, span Span) error
    GetTrace(ctx context.Context, traceID string) ([]Span, error)
    ListTraces(ctx context.Context, filter TraceFilter) ([]Span, error)
    Close() error  // 新增
}
```

`PgTraceStore` 也实现 `Close()`。

**影响文件**：`trace/store.go`, `trace/pg/trace_store.go`

### 2.7 TransactionManager 接口化

```go
type TransactionManager interface {
    Begin(ctx context.Context) (context.Context, error)
    Commit(ctx context.Context) error
    Rollback(ctx context.Context) error
}
```

- `PgTransactionManager` 实现此接口
- `InMemoryTransactionManager` 实现此接口
- `TransactionAspect` 依赖接口而非具体类型
- `Backend.TransactionManager` 字段类型改为接口

**影响文件**：`pg/transaction.go`, `infra/memory_backend.go`, `infra/backend.go`, `aspect/builtin/transaction.go`

### 2.8 其他一致性修复

- `deepCopy` 在 `infra/repository/memory/` 中提取为共享函数
- `ddd_inventory_products` 表从核心 migration 移除（属于 exampleapp 领域）
- `fieldcount` 的 `maxFields` 改为 analyzer flag（`-max-fields` 参数，默认 5）
- `lint/doc.go` 补充 `dddfieldcount` 文档
- `infra/bus.go` 的 `BusFactory` 改为接口（当前是 struct + 函数字段）
- `infra/provider.go` 的 `sql.Open("pgx", ...)` 中 driver name 提取为常量
- `pg/migrate.go` 的表名列表提取为包级 `[]string` 变量，`DropAll`/`TruncateAll`/`Migrate` 共用
- `job/pg/job_store.go` 的 `Get`/`List` 中 scan 逻辑提取为 `scanJob` 辅助函数
- `trace/pg/trace_store.go` 的 `ListTraces` 中 `NameContains` 做 LIKE 通配符转义
- `observability/message_reader.go` 中 `InMemoryMessageStore` 重命名为 `ObservableMessageStore`（避免与 `builtin.InMemoryMessageStore` 混淆）

---

## 三、易用性优化（P2）

### 3.1 聚合构造简化

**问题**：当前需要 `NewOrder` + `NewOrderForReplay` 两个构造器。

**方案**：统一为单一构造器

```go
// 用户代码 — 只需一个构造器
func NewOrder(id OrderID) *Order {
    o := &Order{}
    ar, _ := aggregate.NewAggregateRoot(id.String())
    o.AggregateRoot = *ar
    return o
}

// 业务创建 — 构造后 Apply 事件
order := NewOrder(orderID)
order.Apply(ctx, &OrderCreatedEvent{...})

// 事件溯源回溯 — 构造后 LoadFromHistory
order := NewOrder(orderID)
order.LoadFromHistory(events)
```

- 删除 `NewXxxForReplay` 模式
- `AggregateRoot` 初始 version 为 0，`LoadFromHistory` 后自动设置
- 更新 `docs/ai-domain-generation-rules.md` 中的构造器规则

**影响文件**：`domain/aggregate/aggregate.go`, exampleapp, docs

### 3.2 JSON 完全自动委托

**问题**：每个聚合/实体都需要手写 `MarshalJSON`/`UnmarshalJSON` 委托代码。

**方案**：
- `AggregateRoot` 自带 `MarshalJSON`/`UnmarshalJSON` 实现（利用反射自动处理嵌入字段）
- `Entity` 自带 `MarshalJSON`/`UnmarshalJSON` 实现
- `AuditableEntity`/`SoftDeletableEntity` 同理
- 删除 `MarshalAggregate`/`UnmarshalAggregate`/`MarshalEntity`/`UnmarshalEntity` 辅助函数
- 删除 `AggregateRootJSON`/`EntityJSON`/`AuditableEntityJSON`/`SoftDeletableEntityJSON` 中间结构体
- 删除 `ToJSON`/`FromJSON` 方法
- 用户自定义类型不再需要手写任何 JSON 委托代码
- 如果用户需要自定义序列化，直接在自己的类型上实现 `json.Marshaler`

**影响文件**：`domain/aggregate/aggregate.go`, `domain/aggregate/json_helper.go`(删除), `domain/entity/entity.go`, `domain/entity/auditable.go`, `domain/entity/soft_deletable.go`, `domain/entity/json_helper.go`(删除), exampleapp

### 3.3 Wire 顺序校验

**方案**：
- `WithAutoBuses` 检查 `AppContext.Backend` 是否已设置，未设置返回明确 error
- `WithCommandHandlers`/`WithQueryHandlers`/`WithEventSubscriptions` 检查对应 Bus 是否已设置
- 添加 `WithPreset[T]()` 快捷函数，组合常用配置（Backend + Buses + DefaultAspects）
- `WithConfigFile` 修复：当前加载的 config 被丢弃，应存入 AppContext

**影响文件**：`app/wire_bus.go`, `app/wire_handler.go`, `app/wire_config.go`, `app/wire_backend.go`

### 3.4 Result 简化

```go
// 框架提供通用 Result 类型
type SimpleResult struct {
    Success bool
}

type IDResult struct {
    ID string
}
```

Command Handler 可直接使用 `*SimpleResult` 或 `*IDResult`，不必为每个 Command 定义 Result 类型。

**影响文件**：`cqrs/command/command.go`

### 3.5 TypeRegistry 自动注册

```go
// 从 Handler 自动提取类型信息
func (r *TypeRegistry) RegisterFromHandler(handler any) error
```

通过反射从 Handler 的 `Handle` 方法签名自动提取 Command/Query/Event 类型和 Result 类型。

**影响文件**：`job/core/job.go`, `observability/type_prototype.go`

---

## 四、新增 Lint 规则

### 4.1 `dddnaming` — 命名一致性检查

检查 CQRS 三层接口的命名是否一致：
- 事件类型名函数应使用 `EventNameOf`（不是 `EventTypeOf`）
- Command/Query/Event 命名规范：过去时态事件、XxxCommand/XxxQuery 后缀
- Handler 命名规范：XxxHandler 后缀
- Result 命名规范：XxxResult 后缀

### 4.2 `dddencapsulation` — 字段封装检查

检查聚合根和实体的字段是否全部私有：
- 嵌入 `AggregateRoot`/`Entity` 的结构体，其业务字段应为小写（通过 getter 访问）
- 允许嵌入的基础类型（`AggregateRoot`, `Entity`, `BaseEvent` 等）为公开
- 检测公开的可变字段（slice、map、pointer）导致的封装破坏
- Job 结构体的业务字段应为私有

### 4.3 `dddwireorder` — Wire 依赖顺序检查

检查 `app.Wire()` 调用中 wire 函数的顺序：
- `WithAutoBuses` 必须在 `WithBackend`/`WithAutoBackend` 之后
- `WithCommandHandlers`/`WithQueryHandlers`/`WithEventSubscriptions` 必须在对应 Bus 之后
- `WithDefaultAspects` 应在 `WithBackend` 之后

### 4.4 `dddbusdup` — Bus 重复代码检测

检查用户项目中是否存在重复的 Bus 实现模式：
- 如果项目中有自定义 Bus 实现，检查是否可以使用框架提供的 `memory.CommandBus`/`memory.QueryBus`/`memory.EventBus`
- 检测 `RegisterHandler` + `Execute` + `Shutdown` 的重复实现模式

---

## 五、变更影响总览

| 变更 | 影响文件数 | Breaking | 风险 |
|------|-----------|----------|------|
| Job 字段私有化 | ~15 | 是 | 中 |
| EventStore shallowCopy 修复 | ~5 | 是 | 低 |
| PG isUniqueViolation | ~2 | 否 | 低 |
| GetQuerier 加锁 | ~1 | 否 | 低 |
| BaseEvent 可变性 | ~10 | 是 | 中 |
| EventStore.Append key | ~1 | 否 | 低 |
| ValueObject 不可变性 | ~2 | 是 | 低 |
| CQRS 命名统一 | ~30 | 是 | 高 |
| Bus 代码去重 | ~8 | 否 | 中 |
| Span 类型常量 | ~10 | 是 | 低 |
| EventSourceStore 拆分 | ~10 | 是 | 中 |
| Bus 生命周期接口 | ~5 | 是 | 低 |
| TraceStore Close | ~3 | 是 | 低 |
| TransactionManager 接口化 | ~5 | 是 | 低 |
| 聚合构造简化 | ~10 | 是 | 中 |
| JSON 自动委托 | ~8 | 是 | 中 |
| Wire 顺序校验 | ~5 | 否 | 低 |
| Result 简化 | ~3 | 否 | 低 |
| TypeRegistry 自动注册 | ~3 | 否 | 低 |
| 新增 Lint 规则 | ~4 新文件 | 否 | 低 |

---

## 六、文档更新

- `docs/ai-domain-generation-rules.md`：更新构造器规则（单一构造器）、删除 JSON 委托要求、更新 EventSourceStore 接口名
- `docs/guide.md`：更新所有 API 引用（EventNameOf、RegisterHandler、SubscribeHandler 保留等）
- `docs/architecture.md`：更新接口定义
- `README.md`：更新快速开始示例
- `AGENTS.md`：无需变更

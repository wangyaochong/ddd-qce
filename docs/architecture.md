# 架构设计文档

## 一、核心收益：AI 时代的架构治理

```
人类编码时代                    AI 生成时代
─────────────────               ─────────────────
靠代码审查保证规范              靠架构规则自动拦截
靠经验识别跨领域调用            靠静态分析生成调用图
靠文档维护边界                  靠编译/扫描强制边界
```

**→ 规则越明确，AI 越不会"越界"**
**→ 调用链越清晰，AI 越能正确生成**

### 为什么需要严格架构？

在 AI 辅助编码时代，代码生成速度大幅提升，但 AI 缺乏对业务边界的理解能力。如果没有明确的架构约束：

1. **领域污染**: AI 倾向于走捷径，直接引用其他领域的内部实现
2. **边界模糊**: 跨领域调用变成隐式依赖，难以追踪和维护
3. **部署耦合**: 单体思维导致领域无法独立部署
4. **测试困难**: 隐式依赖导致单元测试需要大量 Mock

通过 **Command / Event / Query** 三种显式接口，我们实现了：

- ✅ **编译期拦截**: 禁止引入其他领域的内部包
- ✅ **调用图清晰**: 所有跨领域交互一目了然
- ✅ **部署无关**: 无论单体还是微服务，接口不变
- ✅ **AI 友好**: 规则明确，AI 生成代码不会越界

---

## 二、严格规则定义

### 核心规则

**所有跨领域交互，无论部署方式，必须通过 Command / Event / Query 三种显式接口。**

```
领域 A (user)                    领域 B (order)
┌─────────────────┐              ┌─────────────────┐
│                 │   Command    │                 │
│  CommandBus     │─────────────►│  CommandHandler │
│                 │              │                 │
│  EventBus       │◄─────────────│  EventPublisher │
│                 │    Event     │                 │
│  QueryBus       │─────────────►│  QueryHandler   │
│                 │    Query     │                 │
└─────────────────┘              └─────────────────┘
         │                              │
         └────── 禁止直接引用 ───────────┘
         (no import order/service, order/domain, order/repository)
```

### 三种接口的职责

| 接口 | 方向 | 语义 | 返回值 | CQRS 关系 | 使用场景 |
|------|------|------|--------|-----------|----------|
| **Command** | 单向调用 | "请执行某个操作" | 可选 | 1:1 Handler | 创建订单、取消订单、更新状态 |
| **Query** | 单向调用 | "请查询某些数据" | 必须 | 1:1 Handler | 获取用户信息、查询订单列表 |
| **Event** | 发布订阅 | "某件事已发生" | 无 | 1:N Handler | 用户已创建、订单已支付 |

### 交互模式

#### 1. Command 模式（同步调用）

```
领域 A ──Command──► 领域 B
                    │
                    ▼
              CommandHandler 执行
                    │
                    ▼
              返回结果 (可选)
```

#### 2. Query 模式（同步查询）

```
领域 A ───Query───► 领域 B
                    │
                    ▼
              QueryHandler 执行
                    │
                    ▼
              返回查询结果
```

#### 3. Event 模式（异步解耦）

```
领域 B ──Event──► EventBus ──► 领域 A Handler
                          ──► 领域 C Handler
                          ──► 领域 D Handler
```

---

## 三、目录约定（强制）

```
/project
├── /internal
│   ├── /user                    # 用户领域
│   │   ├── /command             # ✅ 对外暴露
│   │   │   ├── create_user.go
│   │   │   └── handler.go
│   │   ├── /query               # ✅ 对外暴露
│   │   │   ├── get_user.go
│   │   │   └── handler.go
│   │   ├── /event               # ✅ 对外暴露
│   │   │   ├── user_created.go
│   │   │   └── publisher.go
│   │   ├── /domain              # ❌ 仅内部使用
│   │   ├── /service             # ❌ 仅内部使用
│   │   └── /repository          # ❌ 仅内部使用
│   │
│   ├── /order                   # 订单领域
│   │   ├── /command             # ✅ 对外暴露
│   │   ├── /query               # ✅ 对外暴露
│   │   ├── /event               # ✅ 对外暴露
│   │   ├── /domain              # ❌ 内部
│   │   └── ...
│   │
│   └── /shared                  # 共享基础设施（可选）
│       └── /bus                 # Bus 实现
│
├── /cmd                         # 入口
└── go.mod
```

### 包可见性规则

| 包路径 | 可见性 | 说明 |
|--------|--------|------|
| `/{domain}/command` | ✅ 公开 | 其他领域可引入，定义 Command 结构和 Handler |
| `/{domain}/query` | ✅ 公开 | 其他领域可引入，定义 Query 结构和 Handler |
| `/{domain}/event` | ✅ 公开 | 其他领域可引入，定义 Event 结构和 Handler |
| `/{domain}/domain` | ❌ 私有 | 仅领域内部使用，包含实体、值对象、聚合根 |
| `/{domain}/service` | ❌ 私有 | 仅领域内部使用，包含领域服务 |
| `/{domain}/repository` | ❌ 私有 | 仅领域内部使用，包含仓储实现 |

---

## 四、跨领域调用示例

### ✅ 合法方式

```go
// user 领域需要创建订单时

package usercommand

import (
    "context"

    // ✅ 允许：只引入 order 的 command 包
    ordercmd "myproject/internal/order/command"

    commandmemory "github.com/ddd-qce/core/cqrs/command/memory"
)

type CreateUserHandler struct {
    // ✅ 通过 Bus 发送 Command，不直接依赖实现
    orderCommandBus *commandmemory.CommandBus
}

func (h *CreateUserHandler) Handle(ctx context.Context, cmd CreateUserCommand) (string, error) {
    // 创建用户...

    // ✅ 合法：发送 Command 到 order 领域
    _, err := commandmemory.Dispatch[ordercmd.CreateOrderCommand, string](
        h.orderCommandBus, ctx, ordercmd.CreateOrderCommand{
            UserID: user.ID,
            Items:  cartItems,
        },
    )

    return user.ID, err
}
```

### ❌ 非法方式

```go
package usercommand

import (
    "context"

    // ❌ 禁止：引入 order 的内部包
    "myproject/internal/order/domain"
    "myproject/internal/order/service"
    "myproject/internal/order/repository"
)

type CreateUserHandler struct {
    // ❌ 禁止：直接依赖 order 领域的服务
    orderService *order.service.OrderService

    // ❌ 禁止：直接依赖 order 领域的仓储
    orderRepo *order.repository.OrderRepository
}

func (h *CreateUserHandler) Handle(ctx context.Context, cmd CreateUserCommand) error {
    // ❌ 禁止：直接调用 order 领域的方法
    order, err := h.orderService.CreateOrder(ctx, domain.NewOrder(...))

    // ❌ 禁止：直接操作 order 领域的仓储
    h.orderRepo.Save(ctx, order)

    return err
}
```

---

## 五、架构收益总结

### 对 AI 生成的影响

| 维度 | 无架构约束 | 有架构约束 |
|------|-----------|-----------|
| 代码生成准确率 | 低（容易越界） | 高（规则明确） |
| 跨领域调用 | 隐式、难追踪 | 显式、可分析 |
| 边界维护 | 靠人工审查 | 靠编译拦截 |
| 部署灵活性 | 耦合严重 | 独立部署 |
| 测试复杂度 | 高（大量 Mock） | 低（接口隔离） |

### 静态分析能力

基于严格的目录约定和接口约束，可以实现：

1. **依赖图生成**: 自动分析领域间的 Command/Event/Query 调用关系
2. **违规检测**: 扫描 import 语句，检测是否引入了其他领域的内部包
3. **调用链追踪**: 结合 Trace 系统，记录完整的跨领域调用链
4. **架构文档**: 从代码自动生成领域交互图

### 部署无关性

```
单体部署                    微服务部署
─────────                  ─────────
CommandBus → 函数调用       CommandBus → HTTP/gRPC
EventBus → 内存广播         EventBus → Kafka/RabbitMQ
QueryBus → 函数调用         QueryBus → HTTP/gRPC
```

**接口不变，部署方式可自由切换。**

---

## 六、DDD-QCE 框架实现

本框架 (`github.com/ddd-qce/core`) 提供了实现上述架构所需的全部基础设施。核心模块仅依赖 `github.com/google/uuid`，PostgreSQL 驱动仅在集成测试模块 (`it/`) 中引入。

### 领域层

| 组件 | 包路径 | 说明 |
|------|--------|------|
| Entity | `domain/entity` | 实体基类，提供 id (私有)、GetID()、Equals()、IsEmpty()、Validate()、MarshalJSON/UnmarshalJSON |
| AuditableEntity | `domain/entity` | 审计实体，嵌入 Entity + createdAt/updatedAt (私有) + CreatedAt()/UpdatedAt() + Touch() + FromData 构造器 |
| SoftDeletableEntity | `domain/entity` | 软删除实体，嵌入 AuditableEntity + deletedAt (私有) + DeletedAt() + SoftDelete()/Restore()/IsDeleted() + FromData 构造器 |
| IDGenerator | `domain/entity` | ID 生成器类型，DefaultIDGenerator() (UUID hex)，NewEntityWithID() |
| AggregateRoot | `domain/aggregate` | 聚合根，嵌入 Entity + Version + 事件收集/回溯 + EventApplier |
| ValueObject[T] | `domain/valueobject` | 泛型值对象，New[T]() / MustNew[T]() / Value() / Equals() / Validate() / DeepEquals() |
| DomainEvent | `domain/event` | 领域事件接口，AggregateID() / EventType() / OccurredAt() |
| EventHandler[T] | `domain/event` | 泛型事件处理器接口，Handle(ctx, T) error |
| EventStore[T] | `domain/event` | 事件存储接口，Append(ctx, []T) / Load(ctx, aggregateID, afterVersion) |
| Repository[T] | `domain/repository` | 仓储接口，Save / FindByID / Delete |
| EventSourcingRepository[T] | `domain/repository` | 事件溯源仓储接口，Save / Load |

### CQRS 层

| 组件 | 包路径 | 说明 |
|------|--------|------|
| Command | `cqrs/command` | 命令接口 + BaseCommand 结构体 |
| CommandHandler[T,R] | `cqrs/command` | 命令处理器接口 |
| CommandBus | `cqrs/command/memory` | 内存命令总线，RegisterCommand[T,R](bus, handler) / Dispatch[T,R](ctx, bus, cmd) |
| Query | `cqrs/query` | 查询接口 + BaseQuery 结构体 |
| QueryHandler[T,R] | `cqrs/query` | 查询处理器接口 |
| QueryBus | `cqrs/query/memory` | 内存查询总线，RegisterQuery[T,R](bus, handler) / Dispatch[T,R](ctx, bus, q) |
| EventBus | `cqrs/event` | 事件总线接口，Publish(ctx, evt) error |
| EventStore[T] | `cqrs/event` | 事件存储接口（re-export from domain/event），Append(ctx, aggregateID, expectedVersion, events) / Load(...) |
| EventBus (实现) | `cqrs/event/memory` | 非泛型内存事件总线，RegisterHandler[T] / Dispatch[T] 类型安全辅助 |
| TypedEventBus[T] | `cqrs/event/memory` | 单类型事件总线包装（Deprecated，推荐用 RegisterHandler[T]/Dispatch[T]） |
| EventStore[T] | `cqrs/event/memory` | 内存事件存储，实现 domain/event.EventStore[T] |
| EventStore[T] | `cqrs/event/pg` | PostgreSQL 事件存储，实现 domain/event.EventStore[T] |

### 切面层

| 组件 | 包路径 | 说明 |
|------|--------|------|
| AspectChain | `aspect` | 切面链，洋葱模型执行，RegisterCommandAspect/QueryAspect/EventAspect |
| CommandAspect | `cqrs/aspect` | 命令切面接口，BeforeCommand / AfterCommand |
| QueryAspect | `cqrs/aspect` | 查询切面接口，BeforeQuery / AfterQuery |
| EventAspect | `cqrs/aspect` | 事件切面接口，BeforePublish / AfterPublish |
| TracingAspect | `aspect/builtin` | 链路追踪切面（Order: 0） |
| TransactionAspect | `aspect/builtin` | 事务管理切面（Order: 10） |
| LoggingAspect | `aspect/builtin` | 日志记录切面（Order: 50） |
| MetricsAspect | `aspect/builtin` | 指标采集切面（Order: 100） |
| PersistenceAspect | `aspect/builtin` | 消息持久化切面（Order: 200） |
| MessageStore | `aspect/builtin` | 消息存储接口，RecordCommand/Query/Event/EventHandler |
| NopMessageStore | `aspect/builtin` | 空操作消息存储 |
| PgMessageStore | `aspect/builtin/pg` | PostgreSQL 消息存储 |

### Job 系统

| 组件 | 包路径 | 说明 |
|------|--------|------|
| Job | `job/core` | 任务模型，ID/Status/Result/Error/Timeout/RetryCount |
| JobStore | `job/core` | 任务存储接口，Create/Get/Update/List/Delete |
| JobManager | `job/core` | 任务管理接口，Submit/GetStatus/Cancel/Retry/Wait/ListByStatus |
| JobOption | `job/core` | 任务选项，WithTimeout / WithMaxRetries |
| InMemoryJobStore | `job/memory` | 内存任务存储 |
| JobManager (实现) | `job/memory` | 内存任务管理器，需要 CommandExecutor |
| PgJobStore | `job/pg` | PostgreSQL 任务存储 + RecordJobExecution |

### 链路追踪

| 组件 | 包路径 | 说明 |
|------|--------|------|
| Span | `trace` | Span 结构体，ID/TraceID/ParentID/Type/Name/Status/Error/Duration |
| TraceStore | `trace` | 追踪存储接口，RecordSpan/GetTrace/ListTraces |
| InMemoryTraceStore | `trace` | 内存追踪存储 |
| 上下文传播 | `trace` | WithTrace/GetTraceID/GetSpanID/WithParentSpan/GetParentSpanID |
| PgTraceStore | `trace/pg` | PostgreSQL 追踪存储 |

### 基础设施层

| 组件 | 包路径 | 说明 |
|------|--------|------|
| Backend | `infra` | 统一后端结构体，TransactionManager + JobStore + TraceStore + MessageStore + Migrate |
| TransactionManager | `infra` | 事务管理器接口，Begin/Commit/Rollback |
| MemoryTransactionManager | `infra` | 内存事务管理器，支持嵌套（depth + aborted） |
| NewMemoryBackend() | `infra` | 创建内存后端（全部内存实现） |
| PgRepository[T] | `infra/repository/pg` | PostgreSQL 仓储实现，自动乐观锁（OptimisticLockError） |
| PgEventSourcedRepository[T] | `infra/repository/pg` | PostgreSQL 事件溯源仓储，快照 + 乐观锁 |
| SnapshotSerializer[T] | `infra/repository/pg` | 快照序列化接口，Serialize/Deserialize |
| JSONSerializer[T] | `infra/repository/pg` | JSON 快照序列化实现 |
| OptimisticLockError | `infra/repository/pg` | 乐观锁冲突错误 |
| PgTransactionManager | `pg` | PostgreSQL 事务管理器，Savepoint 嵌套事务 |
| DBTX / GetQuerier | `pg` | 事务内查询器，自动从 ctx 获取当前 tx |
| Migrate / DropAll | `pg` | 数据库迁移 |
| NewBackend(db) | `pgx` | 创建 PostgreSQL 后端（组装全部 PG 实现） |

### 内置切面优先级

| 切面 | 优先级 (Order) | 说明 |
|------|--------|------|
| TracingAspect | 0 | 链路追踪，记录 Span |
| TransactionAspect | 10 | 事务管理（嵌套事务 + Savepoint） |
| LoggingAspect | 50 | 日志记录 |
| MetricsAspect | 100 | 指标采集 |
| PersistenceAspect | 200 | 消息持久化到 MessageStore |

### 关键设计决策

| 决策 | 原因 |
|------|------|
| Entity ID 类型为 `string` | 简单灵活，通过 IDGenerator 抽象生成逻辑 |
| AggregateRoot 嵌入 Entity | Go embedding 使 `a.ID` 仍可直接访问，零破坏性 |
| EventBus 非泛型统一 | 1 个实例处理所有事件类型，RegisterHandler[T]/Dispatch[T] 保留类型安全 |
| Dispatch 参数顺序：ctx 在 bus 前 | ctx 是请求上下文，bus 是总线实例，遵循 Go 惯例 |
| AppendOnlyStore 4 参数签名 | Append(ctx, aggregateID, expectedVersion, events) 支持乐观锁 |
| pgx 依赖隔离到 it/ 模块 | `go mod tidy` 忽略 build tags，只有独立模块才能真正移除依赖 |
| Backend 全局统一配置 | 所有基础设施组件共享同一后端，避免配置碎片化 |
| Savepoint 嵌套事务 | inner Commit = RELEASE SAVEPOINT，inner Rollback = ROLLBACK TO SAVEPOINT + aborted 标记 |

详细使用指南请查看 [实战指南](guide.md)。

想了解如何将本框架与 Actor 模型结合，请查看 [Actor + CQRS + DDD 组合架构](actor-cqrs-ddd.md)。

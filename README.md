# DDD-QCE

DDD-QCE 是一个基于 **CQRS + Event Sourcing + AOP** 的 Go 领域驱动开发框架，专为 AI 生成代码时代设计。

## 核心特性

- **CQRS 分离**: Command（写操作）与 Query（读操作）完全分离，通过独立的 Bus 调度
- **Event Sourcing**: 领域事件驱动，支持事件发布/订阅与事件存储
- **AOP 切面**: 支持 Tracing、Transaction、Logging、Metrics、Persistence 等切面，洋葱模型执行
- **链路追踪**: 跨 Command → Event → Query 的完整调用链追踪
- **异步 Job**: 支持超时、重试、取消的后台任务系统
- **泛型安全**: 基于 Go 1.18+ 泛型，类型安全的 Handler 注册与调度
- **可插拔持久化**: 统一 Backend 抽象，内存实现开箱即用，PostgreSQL 生产就绪
- **乐观锁**: 聚合保存自动版本检查，防止并发冲突
- **嵌套事务**: 基于 SAVEPOINT 的嵌套事务支持
- **实体增强**: AuditableEntity（审计）、SoftDeletableEntity（软删除）、IDGenerator（ID 生成）、ValueObject（值对象）

## 快速开始

### 安装

```bash
go get github.com/ddd-qce/core
```

### 5 分钟示例

```go
package main

import (
    "context"
    "fmt"
    "time"

    "github.com/ddd-qce/core/aspect"
    "github.com/ddd-qce/core/aspect/builtin"
    commandmemory "github.com/ddd-qce/core/cqrs/command/memory"
    "github.com/ddd-qce/core/cqrs/event"
    eventmemory "github.com/ddd-qce/core/cqrs/event/memory"
    querymemory "github.com/ddd-qce/core/cqrs/query/memory"
    "github.com/ddd-qce/core/trace"
)

// 1. 定义 Command（嵌入 BaseCommand）
type CreateUserCommand struct {
    command.BaseCommand
    Name  string
    Email string
}

// 2. 定义 Handler
type CreateUserHandler struct{}

func (h *CreateUserHandler) Handle(ctx context.Context, cmd CreateUserCommand) (string, error) {
    return "user-123", nil
}

// 3. 定义 Query（嵌入 BaseQuery）
type GetUserQuery struct {
    query.BaseQuery
    UserID string
}

type GetUserHandler struct{}

func (h *GetUserHandler) Handle(ctx context.Context, q GetUserQuery) (string, error) {
    return "User: " + q.UserID, nil
}

// 4. 定义 Event（实现 DomainEvent 接口）
type UserCreatedEvent struct {
    UserID    string
    Name      string
    CreatedAt time.Time
}

func (e UserCreatedEvent) AggregateID() string   { return e.UserID }
func (e UserCreatedEvent) EventType() string     { return "UserCreated" }
func (e UserCreatedEvent) OccurredAt() time.Time { return e.CreatedAt }

type UserCreatedHandler struct{}

func (h *UserCreatedHandler) Handle(ctx context.Context, evt UserCreatedEvent) error {
    fmt.Println("User created:", evt.Name)
    return nil
}

func main() {
    ctx := context.Background()

    // 创建切面链
    chain := aspect.NewAspectChain()
    chain.RegisterCommandAspect(&builtin.TracingAspect{Store: trace.NewInMemoryTraceStore()})
    chain.RegisterCommandAspect(&builtin.LoggingAspect{Logger: &SimpleLogger{}})

    // 创建 Bus
    qBus := querymemory.NewQueryBus(chain)
    cBus := commandmemory.NewCommandBus(chain)
    eBus := eventmemory.NewEventBus(eventmemory.WithBusAspectChain(chain))

    // 注册 Handler
    commandmemory.RegisterCommand[CreateUserCommand, string](cBus, &CreateUserHandler{})
    querymemory.RegisterQuery[GetUserQuery, string](qBus, &GetUserHandler{})
    eventmemory.RegisterHandler[UserCreatedEvent](eBus, &UserCreatedHandler{})

    // 执行
    commandmemory.Dispatch[CreateUserCommand, string](cBus, ctx, CreateUserCommand{Name: "Alice", Email: "alice@example.com"})
    querymemory.Dispatch[GetUserQuery, string](qBus, ctx, GetUserQuery{UserID: "user-123"})
    eBus.Publish(ctx, UserCreatedEvent{UserID: "user-123", Name: "Alice", CreatedAt: time.Now()})
}
```

### 运行示例

```bash
# 克隆仓库
git clone https://github.com/ddd-qce/core.git
cd core

# 运行完整示例（展示 CQRS、Event、Job、Trace 等全部功能）
go run example/main.go
```

## 核心概念

| 概念 | 说明 |
|------|------|
| **AggregateRoot** | 聚合根基类，嵌入 Entity，提供版本管理、领域事件收集/清除、事件回溯 |
| **Entity** | 实体基类，提供 ID 标识、相等性判断、验证；扩展：AuditableEntity（审计字段）、SoftDeletableEntity（软删除） |
| **ValueObject** | 值对象泛型 `ValueObject[T comparable]`，通过值相等判断，内置验证 |
| **Repository** | 仓储接口，标准 CRUD 操作，泛型支持任意聚合类型 |
| **EventSourcingRepository** | 事件溯源仓储接口，通过事件流加载/保存聚合，支持快照 |
| **Command** | 写操作指令，通过 CommandBus 分发到 CommandHandler，可返回结果 |
| **Query** | 读操作指令，通过 QueryBus 分发到 QueryHandler，返回查询结果 |
| **Event** | 领域事件，通过 EventBus 发布，支持多订阅者，最终一致性 |
| **Aspect** | 切面拦截器，支持 Before/After 钩子，洋葱模型执行 |
| **Job** | 异步后台任务，支持超时、重试、取消、状态追踪 |
| **Trace** | 链路追踪，跨 Command/Event/Query 传播 TraceID 与 SpanID |
| **Backend** | 统一基础设施抽象，包含 TransactionManager、JobStore、TraceStore、MessageStore |

## 架构原则

> **规则越明确，AI 越不会"越界"**
> **调用链越清晰，AI 越能正确生成**

所有跨领域交互，无论部署方式，**必须**通过 Command / Event / Query 三种显式接口。

禁止跨领域直接引用内部包（domain/service/repository）。

详细架构规范请查看 [架构设计文档](docs/architecture.md)。

## 文档导航

- [架构设计文档](docs/architecture.md) - AI 时代架构治理、严格边界规则、目录约定
- [实战指南](docs/guide.md) - Command/Query/Event/Aspect/Job/Trace 完整使用指南
- [Actor + CQRS + DDD 组合架构](docs/actor-cqrs-ddd.md) - 黄金三角架构详解、Actor 模型实现、长耗时任务场景

## 项目结构

```
/core
├── /domain                      # 领域层
│   ├── /aggregate               # AggregateRoot（嵌入 Entity，版本管理，事件收集）
│   ├── /entity                  # Entity / AuditableEntity / SoftDeletableEntity / IDGenerator
│   ├── /valueobject             # ValueObject[T comparable] 值对象
│   ├── /event                   # DomainEvent 接口 / EventHandler[T] / EventStore[T]
│   └── /repository              # Repository[T] / EventSourcingRepository[T] 接口
│
├── /infra                       # 基础设施层
│   ├── backend.go               # Backend 结构体 + TransactionManager 接口
│   ├── memory_backend.go        # NewMemoryBackend() + MemoryTransactionManager
│   └── /repository/pg           # PgRepository[T] / PgEventSourcedRepository[T]（乐观锁 + 快照）
│
├── /cqrs                        # CQRS 层
│   ├── /aspect                  # Aspect / CommandAspect / QueryAspect / EventAspect 接口
│   ├── /command                 # Command / CommandHandler[T,R] / CommandExecutor 接口
│   │   └── /memory              # 内存 CommandBus（RegisterCommand / Dispatch）
│   ├── /query                   # Query / QueryHandler[T,R] 接口
│   │   └── /memory              # 内存 QueryBus（RegisterQuery / Dispatch）
│   └── /event                   # EventBus / TypedEventBus[T] / AppendOnlyStore[T] 接口
│       ├── /memory              # 内存 EventBus / RegisterHandler[T] / Dispatch[T] / DomainEventStore / EventStore[T]
│       └── /pg                  # PostgreSQL EventStore[T]
│
├── /aspect                      # 切面链实现
│   ├── chain.go                 # AspectChain 洋葱模型执行
│   └── /builtin                 # 内置切面
│       ├── TracingAspect        # 链路追踪（Order: 0）
│       ├── TransactionAspect    # 事务管理（Order: 10）
│       ├── LoggingAspect        # 日志记录（Order: 50）
│       ├── MetricsAspect        # 指标采集（Order: 100）
│       ├── PersistenceAspect    # 消息持久化（Order: 200）
│       └── /pg                  # PgMessageStore
│
├── /job                         # 异步 Job 系统
│   ├── /core                    # Job / JobStore / JobManager 接口 + JobOption
│   ├── /memory                  # 内存 InMemoryJobStore + JobManager
│   └── /pg                      # PostgreSQL PgJobStore
│
├── /trace                       # 链路追踪
│   ├── trace.go                 # Trace 上下文传播（WithTrace / GetTraceID / GetSpanID）
│   ├── span.go                  # Span 定义 + TraceFilter
│   ├── store.go                 # TraceStore 接口 + InMemoryTraceStore
│   └── /pg                      # PostgreSQL PgTraceStore
│
├── /pg                          # PostgreSQL 基础设施
│   ├── transaction.go           # PgTransactionManager（Savepoint 嵌套事务）+ DBTX + GetQuerier
│   └── migrate.go               # Migrate() / DropAll()
│
├── /pgx                         # PostgreSQL Backend 预设
│   └── backend.go               # NewBackend(db) 组装全部 PG 实现
│
├── /infra                       # 基础设施抽象
│   ├── backend.go               # Backend 结构体 + TransactionManager 接口
│   ├── memory_backend.go        # NewMemoryBackend() + MemoryTransactionManager
│   └── /repository/pg           # PgRepository / PgEventSourcedRepository（乐观锁 + 快照）
│
├── /example                     # 独立示例模块（module github.com/ddd-qce/example）
├── /exampleapp                  # 示例应用模块（module github.com/ddd-qce/exampleapp）
├── /it                          # 集成测试模块（module github.com/ddd-qce/it，含 pgx 依赖）
└── /docs                        # 文档
    ├── architecture.md
    ├── guide.md
    └── actor-cqrs-ddd.md
```

## License

MIT

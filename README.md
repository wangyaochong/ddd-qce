# DDD-QCE

DDD-QCE 是一个基于 **CQRS + Event Sourcing + AOP** 的 Go 领域驱动开发框架，专为 AI 生成代码时代设计。

## 核心特性

- **CQRS 分离**: Command（写操作）与 Query（读操作）完全分离，通过独立的 Bus 调度
- **Event Sourcing**: 领域事件驱动，支持事件发布/订阅与事件存储
- **AOP 切面**: 支持 Tracing、Transaction、Logging、Metrics 等切面，洋葱模型执行
- **链路追踪**: 跨 Command → Event → Query 的完整调用链追踪
- **异步 Job**: 支持超时、重试、取消的后台任务系统
- **泛型安全**: 基于 Go 1.18+ 泛型，类型安全的 Handler 注册与调度

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

    "github.com/ddd-qce/core/aspect"
    "github.com/ddd-qce/core/aspect/builtin"
    commandmemory "github.com/ddd-qce/core/cqrs/command/memory"
    eventmemory "github.com/ddd-qce/core/cqrs/event/memory"
    querymemory "github.com/ddd-qce/core/cqrs/query/memory"
)

// 1. 定义 Command
type CreateUserCommand struct {
    Name  string
    Email string
}

// 2. 定义 Handler
type CreateUserHandler struct{}

func (h *CreateUserHandler) Handle(ctx context.Context, cmd CreateUserCommand) (string, error) {
    return "user-123", nil
}

// 3. 定义 Query
type GetUserQuery struct {
    UserID string
}

type GetUserHandler struct{}

func (h *GetUserHandler) Handle(ctx context.Context, query GetUserQuery) (string, error) {
    return "User: " + query.UserID, nil
}

// 4. 定义 Event
type UserCreatedEvent struct {
    UserID string
    Name   string
}

func (e UserCreatedEvent) AggregateID() string    { return e.UserID }
func (e UserCreatedEvent) EventType() string      { return "UserCreated" }
func (e UserCreatedEvent) OccurredAt() time.Time  { return time.Now() }

type UserCreatedHandler struct{}

func (h *UserCreatedHandler) Handle(ctx context.Context, event UserCreatedEvent) error {
    fmt.Println("User created:", event.Name)
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
    eBus := eventmemory.NewEventBus(chain)

    // 注册 Handler
    commandmemory.RegisterCommand(cBus, &CreateUserHandler{})
    querymemory.RegisterQuery(qBus, &GetUserHandler{})
    eBus.Subscribe(&UserCreatedHandler{})

    // 执行
    cBus.Dispatch[CreateUserCommand, string](ctx, CreateUserCommand{Name: "Alice", Email: "alice@example.com"})
    querymemory.Ask[GetUserQuery, string](ctx, qBus, GetUserQuery{UserID: "user-123"})
    eBus.Publish[UserCreatedEvent](ctx, UserCreatedEvent{UserID: "user-123", Name: "Alice"})
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
| **Aggregate Root** | 聚合根基类，提供 ID、版本管理、领域事件收集/清除、事件回溯 |
| **Entity** | 实体基类，提供 ID 标识与相等性判断 |
| **Repository** | 仓储接口，标准 CRUD 操作，泛型支持任意聚合类型 |
| **EventSourcingRepository** | 事件溯源仓储接口，通过事件流加载/保存聚合 |
| **Command** | 写操作指令，通过 CommandBus 分发到 CommandHandler，可返回结果 |
| **Query** | 读操作指令，通过 QueryBus 分发到 QueryHandler，返回查询结果 |
| **Event** | 领域事件，通过 EventBus 发布，支持多订阅者，最终一致性 |
| **Aspect** | 切面拦截器，支持 Before/After 钩子，洋葱模型执行 |
| **Job** | 异步后台任务，支持超时、重试、取消、状态追踪 |
| **Trace** | 链路追踪，跨 Command/Event/Query 传播 TraceID 与 SpanID |

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
/project
├── /domain                  # 领域层
│   ├── /aggregate           # AggregateRoot 聚合根基类
│   ├── /entity              # Entity 实体基类
│   ├── /event               # DomainEvent 接口 + EventTypeOf
│   └── /repository          # Repository / EventSourcingRepository 接口
│
├── /cqrs                    # CQRS 层
│   ├── /aspect              # Aspect 接口定义
│   ├── /command             # CommandHandler 接口
│   │   └── /memory          # 内存实现
│   ├── /query               # QueryHandler 接口
│   │   └── /memory          # 内存实现
│   └── /event               # EventHandler / EventStore 接口
│       └── /memory          # 内存实现 + EventStore
│
├── /aspect                  # 切面链实现
│   ├── chain.go             # AspectChain 洋葱模型执行
│   └── /builtin             # 内置切面 (Tracing/Transaction/Logging/Metrics)
│
├── /job                     # 异步 Job 系统
│   ├── /core                # Job / JobStore / JobManager 接口
│   └── /memory              # 内存实现
│
├── /trace                   # 链路追踪
│   ├── trace.go             # Trace 上下文传播
│   ├── span.go              # Span 定义
│   └── store.go             # TraceStore 接口
│
├── /adapter                 # 持久化适配器（待实现）
│   ├── /sql
│   ├── /redis
│   └── /mongodb
│
├── /error                   # 统一错误类型（待实现）
│
├── /example                 # 独立示例模块（module github.com/ddd-qce/example）
└── /docs                    # 文档
    ├── architecture.md
    └── guide.md
```

## License

MIT

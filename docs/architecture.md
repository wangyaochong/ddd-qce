# 架构设计文档

## 一、核心收益：AI 时代的架构治理

```
人类编码时代                    AI 生成时代
─────────────────              ─────────────────
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

| 接口 | 方向 | 语义 | 返回值 | 使用场景 |
|------|------|------|--------|----------|
| **Command** | 单向调用 | "请执行某个操作" | 可选 | 创建订单、取消订单、更新状态 |
| **Query** | 单向调用 | "请查询某些数据" | 必须 | 获取用户信息、查询订单列表 |
| **Event** | 发布订阅 | "某件事已发生" | 无 | 用户已创建、订单已支付 |

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
)

type CreateUserHandler struct {
    // ✅ 通过 Bus 发送 Command，不直接依赖实现
    orderCommandBus CommandBus
}

func (h *CreateUserHandler) Handle(ctx context.Context, cmd CreateUserCommand) error {
    // 创建用户...
    
    // ✅ 合法：发送 Command 到 order 领域
    err := h.orderCommandBus.Dispatch(ctx, &ordercmd.CreateOrderCommand{
        UserID: user.ID,
        Items:  cartItems,
    })
    
    return err
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

本框架 (`github.com/ddd-qce/core`) 提供了实现上述架构所需的全部基础设施：

### 核心组件

| 组件 | 包路径 | 说明 |
|------|--------|------|
| CommandHandler | `core/command.go` | 泛型命令处理器接口 |
| QueryHandler | `core/query.go` | 泛型查询处理器接口 |
| DomainEvent | `core/event.go` | 领域事件接口 |
| EventHandler | `core/event.go` | 泛型事件处理器接口 |
| EventStore | `core/event.go` | 事件存储接口 |
| AspectChain | `aspect/chain.go` | 切面链（洋葱模型） |
| CommandBus | `command/memory/` | 内存命令总线 |
| QueryBus | `query/memory/` | 内存查询总线 |
| EventBus | `event/memory/` | 内存事件总线 |
| JobManager | `job/memory/` | 异步任务管理器 |
| TraceStore | `trace/store.go` | 链路追踪存储 |

### 内置切面

| 切面 | 优先级 | 说明 |
|------|--------|------|
| TracingAspect | 0 | 链路追踪，记录 Span |
| TransactionAspect | 10 | 事务管理（仅 Command） |
| LoggingAspect | 50 | 日志记录 |
| MetricsAspect | 100 | 指标采集 |

详细使用指南请查看 [实战指南](guide.md)。

想了解如何将本框架与 Actor 模型结合，请查看 [Actor + CQRS + DDD 组合架构](actor-cqrs-ddd.md)。

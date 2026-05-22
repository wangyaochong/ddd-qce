# DDD-QCE 框架全面改进设计

## 概述

对 DDD-QCE 框架进行系统性改进，分三个阶段推进：代码缺陷修复 → API 改进 → 新功能。

## 阶段 1：代码缺陷修复

无 breaking change，可直接合并。

### 1.1 EventStore.Load 移除深拷贝

**文件**: `cqrs/event/memory/event_store.go`

**问题**: `Load` 方法用 `reflect.New(reflect.TypeOf(e).Elem())` 做深拷贝，T 为值类型时 `.Elem()` panic。

**改动**: 直接返回原始 slice 元素，不做拷贝。与 `InMemoryTraceStore.GetTrace` 行为一致。在方法文档中说明返回值为只读引用，调用方不应修改返回的事件对象。

### 1.2 JobManager 竞态修复

**文件**: `job/memory/job_manager.go`, `job/memory/job_store.go`

**问题 A**: `Cancel` 方法在 Get→检查→Update 之间存在 TOCTOU 竞态，高并发下可能覆盖已完成的 job 状态。

**问题 B**: `executeJob` 使用 `Submit` 传入的 ctx 作为基 context，调用方 ctx 取消后 store 更新失败。

**改动**:

A. `executeJob` 的 store 操作使用独立 context：
```go
func (m *JobManager) executeJob(ctx context.Context, job *jobcore.Job) {
    storeCtx := context.Background()
    // ctx 仅用于超时和取消信号
    timeoutCtx, cancel := context.WithTimeout(ctx, job.Timeout)
    defer cancel()
    // ... 执行命令用 timeoutCtx，store 操作用 storeCtx
}
```

B. `InMemoryJobStore.Update` 增加状态保护：如果 store 中的 job 已处于终态（completed/failed/cancelled），拒绝更新并返回 `ErrJobAlreadyCompleted`。`Cancel` 方法在收到此错误时视为成功（job 已结束）。

### 1.3 EventBus 收集全部错误

**文件**: `cqrs/event/memory/event_bus.go`, 新增 `error/multi_error.go`

**问题**: `Publish` 只返回第一个 handler 错误，后续 handler 错误被丢弃。

**改动**:

阶段 1 先创建 `error/multi_error.go`（阶段 3 扩展该包增加 DomainError 等类型）：

```go
package error

type MultiError struct {
    Errors []error
}

func NewMultiError(errs ...error) *MultiError
func (e *MultiError) Error() string  // 格式化为多行错误
func (e *MultiError) Unwrap() []error // 支持 errors.Is/As
```

`EventBus.Publish` 收集所有 handler 错误：
- 0 个错误：返回 nil
- 1 个错误：返回该 error
- 多个错误：返回 `*MultiError`

每个 handler 的错误不影响其他 handler 执行（保持现有行为）。

### 1.4 TraceFilter 同一 span 匹配

**文件**: `trace/store.go`

**问题**: `matchesFilter` 对每个过滤字段独立匹配不同 span（OR 语义），导致 `Type=command AND Status=error` 匹配"有 command span 且有 error span"而非"有 command 且 error 的 span"。

**改动**: `matchesFilter` 遍历每个 span，检查单个 span 是否满足所有非空过滤条件。只要有一个 span 全部满足，trace 就匹配。

```go
func matchesFilter(spans []*Span, filter TraceFilter) bool {
    for _, span := range spans {
        if spanMatchesFilter(span, filter) {
            return true
        }
    }
    return false
}

func spanMatchesFilter(span *Span, filter TraceFilter) bool {
    if filter.TraceID != "" && span.TraceID != filter.TraceID {
        return false
    }
    if filter.Type != "" && span.Type != filter.Type {
        return false
    }
    if filter.Status != "" && span.Status != filter.Status {
        return false
    }
    if !filter.StartTime.IsZero() && span.StartedAt.Before(filter.StartTime) {
        return false
    }
    if !filter.EndTime.IsZero() && span.StartedAt.After(filter.EndTime) {
        return false
    }
    if filter.NameContains != "" && !strings.Contains(span.Name, filter.NameContains) {
        return false
    }
    return true
}
```

### 1.5 接口重复统一

**文件**: `cqrs/event/event.go`

**问题**: `domain/event.EventHandler[T]` 与 `cqrs/event.Handler[T]` 结构相同但类型不同，`EventStore` 同理。

**改动**: `cqrs/event` 包改为 type alias re-export：

```go
package event

import (
    domainevent "github.com/ddd-qce/core/domain/event"
)

type Handler[T domainevent.DomainEvent] = domainevent.EventHandler[T]
type Store[T domainevent.DomainEvent] = domainevent.EventStore[T]
```

type alias (`=`) 完全兼容现有代码，两种 import 路径可互换。

---

## 阶段 2：API 改进

部分 breaking changes，但框架尚在早期（3 次 commit），影响可控。

### 2.1 Register 错误信息改进

**文件**: `cqrs/command/memory/command_bus.go`, `cqrs/query/memory/query_bus.go`

**改动**: 保持 panic 策略（尽早发现配置错误），但改进 panic 信息：

```go
panic(fmt.Sprintf(
    "handler already registered for command type %T (existing: %T, new: %T)",
    zero, existing, handler,
))
```

### 2.2 JobManager.Wait 改用 channel 通知

**文件**: `job/memory/job_store.go`, `job/memory/job_manager.go`

**问题**: `Wait` 用 100ms 轮询，CPU 浪费 + 延迟高。

**改动**:

`InMemoryJobStore` 增加 completion channel：

```go
type InMemoryJobStore struct {
    mu             sync.RWMutex
    jobs           map[string]*jobcore.Job
    completionChans map[string]chan struct{}
}
```

- Job 首次创建时初始化 `completionChans[id] = make(chan struct{})`
- `Update` 时检测 job 是否进入终态（completed/failed/cancelled），若是则 close channel
- `Wait` 改为：

```go
func (m *JobManager) Wait(ctx context.Context, jobID string, timeout time.Duration) (*jobcore.Job, error) {
    ch := m.store.CompletionChan(jobID)
    if ch == nil {
        return nil, ErrJobNotFound
    }

    ctx, cancel := context.WithTimeout(ctx, timeout)
    defer cancel()

    select {
    case <-ch:
        return m.store.Get(ctx, jobID)
    case <-ctx.Done():
        return nil, ctx.Err()
    }
}
```

### 2.3 TransactionAspect 可选 Event 事务

**文件**: `aspect/builtin/transaction.go`

**改动**: 增加 `EnableEventTx bool` 字段：

```go
type TransactionAspect struct {
    TxManager     TransactionManager
    EnableEventTx bool // 默认 false
}
```

- `EnableEventTx = false`（默认）：BeforePublish/AfterPublish 为 no-op，保持现有行为
- `EnableEventTx = true`：Event handler 也包裹在事务中，适用于 outbox pattern

### 2.4 Span.Duration JSON 输出改进

**文件**: `trace/span.go`

**问题**: `duration` 字段输出纳秒整数，不直观。

**改动**: 自定义 `MarshalJSON`：

```go
func (s *Span) MarshalJSON() ([]byte, error) {
    type Alias Span
    return json.Marshal(&struct {
        *Alias
        DurationMs float64 `json:"duration_ms"`
    }{
        Alias:      (*Alias)(s),
        DurationMs: float64(s.Duration) / float64(time.Millisecond),
    })
}
```

输出示例：`{"id":"...", "duration":1500000000, "duration_ms":1500.0, ...}`

### 2.5 invokeHandler 编译期保障

**文件**: `cqrs/command/memory/command_bus.go`

**问题**: `Execute` 路径用 `MethodByName("Handle")` 反射调用，签名不匹配时运行时才报错。

**改动**: 在 `handlers` map 旁边增加 `dispatchFuncs` map，`RegisterCommand` 时同时注册一个类型安全的闭包：

```go
type CommandBus struct {
    handlers      map[reflect.Type]any
    dispatchFuncs map[reflect.Type]func(context.Context, any) (any, error)
    chain         *aspect.AspectChain
    mu            sync.RWMutex
}
```

`RegisterCommand` 注册时：

```go
func RegisterCommand[T any, R any](bus *CommandBus, handler command.CommandHandler[T, R]) {
    t := reflect.TypeOf((*T)(nil)).Elem()
    // ... panic if duplicate
    bus.handlers[t] = handler
    bus.dispatchFuncs[t] = func(ctx context.Context, cmd any) (any, error) {
        return handler.Handle(ctx, cmd.(T))
    }
}
```

`Execute` 和 `Dispatch` 统一使用 `dispatchFuncs`，消除 `invokeHandler` 反射调用。

---

## 阶段 3：新功能

纯新增，不影响现有 API。

### 3.1 统一错误类型体系

**新文件**: `error/`

```
/error
├── domain_error.go    # DomainError 业务错误
├── multi_error.go     # MultiError 多错误聚合
└── errors.go          # 通用错误变量
```

**DomainError**:

```go
package error

type DomainError struct {
    Code    string
    Message string
    Cause   error
}

func NewDomainError(code, msg string) *DomainError
func NewDomainErrorWithCause(code, msg string, cause error) *DomainError
func (e *DomainError) Error() string  // "[CODE] message"
func (e *DomainError) Unwrap() error
```

**MultiError**:

```go
type MultiError struct {
    Errors []error
}

func NewMultiError(errs ...error) *MultiError
func (e *MultiError) Error() string
func (e *MultiError) Unwrap() []error
```

**通用错误变量**:

```go
var (
    ErrNotFound              = NewDomainError("NOT_FOUND", "resource not found")
    ErrAlreadyExists         = NewDomainError("ALREADY_EXISTS", "resource already exists")
    ErrInvalidState          = NewDomainError("INVALID_STATE", "invalid state for operation")
    ErrPermissionDenied      = NewDomainError("PERMISSION_DENIED", "permission denied")
    ErrHandlerAlreadyRegistered = errors.New("handler already registered for this type")
    ErrJobAlreadyCompleted   = errors.New("job has already reached a terminal state")
    ErrJobNotFound           = errors.New("job not found")
)
```

**与框架集成**:
- `TracingAspect` 区分 `DomainError` 和系统 error：`DomainError` 标记 Span.Status 为 `"business_error"`，系统 error 标记为 `"error"`
- `LoggingAspect` 对 `DomainError` 用 Info 级别，系统 error 用 Error 级别
- `MetricsAspect` 区分 business_error 和 system_error 指标

### 3.2 PostgreSQL 适配器

**新文件**: `adapter/sql/`

```
/adapter/sql
├── options.go              # SQLOptions 配置
├── transaction.go          # TransactionManager SQL 实现
├── serializer.go           # EventSerializer 接口 + JSON 默认实现
├── postgres/
│   ├── event_store.go      # EventStore[T] PostgreSQL 实现
│   ├── job_store.go        # JobStore PostgreSQL 实现
│   ├── trace_store.go      # TraceStore PostgreSQL 实现
│   ├── snapshot_store.go   # 聚合根快照存储
│   └── migrations/
│       └── 001_init.sql    # 建表 DDL
```

**SQLOptions**:

```go
type SQLOptions struct {
    DB            *sql.DB
    TablePrefix   string // 默认 "ddd_"
    SnapshotEvery int    // 每 N 个事件做快照，0 不做
}
```

**EventSerializer**:

```go
type EventSerializer[T event.DomainEvent] interface {
    Serialize(event T) ([]byte, error)
    Deserialize(eventType string, data []byte) (T, error)
}

type JSONSerializer[T event.DomainEvent] struct {
    registry map[string]func() T // eventType -> 工厂函数
}

func NewJSONSerializer[T event.DomainEvent]() *JSONSerializer[T]
func (s *JSONSerializer[T]) Register(eventType string, factory func() T)
```

**EventStore 建表 DDL**:

```sql
CREATE TABLE ddd_domain_events (
    aggregate_id VARCHAR(255) NOT NULL,
    version      BIGINT NOT NULL,
    event_type   VARCHAR(255) NOT NULL,
    payload      JSONB NOT NULL,
    occurred_at  TIMESTAMP WITH TIME ZONE NOT NULL,
    trace_id     VARCHAR(255),
    PRIMARY KEY (aggregate_id, version)
);

CREATE INDEX idx_events_aggregate ON ddd_domain_events (aggregate_id, version);
```

**Append**: `INSERT INTO ddd_domain_events ...` 带版本号乐观锁检查。

**Load**: `SELECT ... WHERE aggregate_id = $1 AND version > $2 ORDER BY version`。

**快照**:

```sql
CREATE TABLE ddd_snapshots (
    aggregate_id VARCHAR(255) PRIMARY KEY,
    version      BIGINT NOT NULL,
    payload      JSONB NOT NULL,
    created_at   TIMESTAMP WITH TIME ZONE NOT NULL
);
```

- `SnapshotEvery = 100` 时，每 100 个事件保存一次快照
- Load 时先加载最新快照，再加载快照之后的事件

**JobStore 建表 DDL**:

```sql
CREATE TABLE ddd_jobs (
    id              VARCHAR(255) PRIMARY KEY,
    command_type    VARCHAR(255) NOT NULL,
    command_payload JSONB NOT NULL,
    status          VARCHAR(50) NOT NULL DEFAULT 'pending',
    result          JSONB,
    error           TEXT,
    retry_count     INT NOT NULL DEFAULT 0,
    max_retries     INT NOT NULL DEFAULT 0,
    timeout_ns      BIGINT NOT NULL DEFAULT 0,
    created_at      TIMESTAMP WITH TIME ZONE NOT NULL,
    started_at      TIMESTAMP WITH TIME ZONE,
    completed_at    TIMESTAMP WITH TIME ZONE
);

CREATE INDEX idx_jobs_status ON ddd_jobs (status);
```

- 分布式 job 领取：`SELECT ... WHERE status = 'pending' FOR UPDATE SKIP LOCKED LIMIT 1`
- 状态转换乐观锁：`UPDATE ddd_jobs SET status = $1 WHERE id = $2 AND status = $3`

**TraceStore 建表 DDL**:

```sql
CREATE TABLE ddd_spans (
    id          VARCHAR(255) PRIMARY KEY,
    trace_id    VARCHAR(255) NOT NULL,
    parent_id   VARCHAR(255),
    type        VARCHAR(50) NOT NULL,
    name        VARCHAR(255) NOT NULL,
    status      VARCHAR(50) NOT NULL,
    error       TEXT,
    started_at  TIMESTAMP WITH TIME ZONE NOT NULL,
    duration_ns BIGINT NOT NULL
);

CREATE INDEX idx_spans_trace ON ddd_spans (trace_id);
CREATE INDEX idx_spans_type_status ON ddd_spans (type, status);
CREATE INDEX idx_spans_name ON ddd_spans USING gin (name gin_trgm_ops);
```

- `RecordSpan`: `INSERT INTO ddd_spans ...`
- `GetTrace`: `SELECT ... WHERE trace_id = $1 ORDER BY started_at`
- `ListTraces`: SQL WHERE 子句映射 filter，用 `EXISTS` 子查询确保同一 span 满足所有条件

**TransactionManager SQL 实现**:

```go
type SQLTransactionManager struct {
    db *sql.DB
}

func (m *SQLTransactionManager) Begin(ctx context.Context) (context.Context, error)
func (m *SQLTransactionManager) Commit(ctx context.Context) error
func (m *SQLTransactionManager) Rollback(ctx context.Context) error
```

事务存储在 context 中，与 `TransactionAspect` 配合使用。

---

## 实施顺序

| 阶段 | 内容 | 预计改动量 |
|------|------|-----------|
| 1 | 5 个缺陷修复 | ~200 行 |
| 2 | 5 个 API 改进 | ~300 行 |
| 3a | 统一错误类型 | ~150 行 |
| 3b | PostgreSQL 适配器 | ~800 行 |

每个阶段完成后运行全量测试，确认无回归后进入下一阶段。

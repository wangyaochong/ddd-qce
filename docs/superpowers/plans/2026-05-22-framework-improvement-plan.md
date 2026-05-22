# DDD-QCE 框架全面改进实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 分三阶段系统性改进 DDD-QCE 框架：修复代码缺陷 → API 改进 → 新增错误类型体系和 PostgreSQL 适配器

**Architecture:** 渐进式改进，每个阶段独立可测试可交付。阶段 1 无 breaking change，阶段 2 有少量 breaking change（框架早期影响可控），阶段 3 纯新增功能。

**Tech Stack:** Go 1.26, github.com/google/uuid, database/sql (lib/pq 或 pgx), encoding/json

---

## 阶段 1：代码缺陷修复

### Task 1: EventStore.Load 移除深拷贝

**Files:**
- Modify: `cqrs/event/memory/event_store.go:34-55`
- Modify: `cqrs/event/memory/event_store_test.go:179-207`

- [ ] **Step 1: 修改 EventStore.Load 移除反射深拷贝**

将 `event_store.go:47-52` 的反射深拷贝替换为直接返回 slice 拷贝：

```go
func (s *EventStore[T]) Load(ctx context.Context, aggregateID string, afterVersion int) ([]T, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	events, exists := s.events[aggregateID]
	if !exists {
		return nil, fmt.Errorf("no events found for aggregate: %s", aggregateID)
	}

	if afterVersion >= len(events) {
		return []T{}, nil
	}

	result := make([]T, len(events[afterVersion:]))
	copy(result, events[afterVersion:])
	return result, nil
}
```

移除 `import "reflect"` （不再需要）。

- [ ] **Step 2: 更新 TestEventStore_LoadReturnsCopy 测试**

此测试验证深拷贝语义。移除深拷贝后，返回的指针指向相同对象，修改会影响 store。更新测试为 `TestEventStore_LoadReturnsReference`，验证返回的是同一指针（不再隔离）：

将 `event_store_test.go:179-207` 替换为：

```go
func TestEventStore_LoadReturnsReference(t *testing.T) {
	store := NewEventStore[*testStoreEvent]()
	ctx := context.Background()

	events := []*testStoreEvent{
		{AggID: "agg-1", Data: "original"},
	}

	err := store.Append(ctx, events)
	if err != nil {
		t.Fatalf("append failed: %v", err)
	}

	loaded, err := store.Load(ctx, "agg-1", 0)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}

	loaded[0].Data = "modified"

	loadedAgain, err := store.Load(ctx, "agg-1", 0)
	if err != nil {
		t.Fatalf("second load failed: %v", err)
	}

	if loadedAgain[0].Data != "modified" {
		t.Errorf("expected data 'modified' (returned slice references store data), got '%s'", loadedAgain[0].Data)
	}
}
```

- [ ] **Step 3: 运行测试确认通过**

Run: `go test ./cqrs/event/memory/ -v`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add cqrs/event/memory/event_store.go cqrs/event/memory/event_store_test.go
git commit -m "fix: remove unsafe reflect deep copy from EventStore.Load"
```

---

### Task 2: 新增 MultiError 类型

**Files:**
- Create: `error/multi_error.go`
- Create: `error/multi_error_test.go`

- [ ] **Step 1: 编写 MultiError 测试**

Create `error/multi_error_test.go`:

```go
package ddderror

import (
	"errors"
	"strings"
	"testing"
)

func TestMultiError_Error(t *testing.T) {
	err := NewMultiError(
		errors.New("first error"),
		errors.New("second error"),
	)

	msg := err.Error()
	if !strings.Contains(msg, "first error") {
		t.Errorf("expected message to contain 'first error', got %q", msg)
	}
	if !strings.Contains(msg, "second error") {
		t.Errorf("expected message to contain 'second error', got %q", msg)
	}
}

func TestMultiError_Unwrap(t *testing.T) {
	err1 := errors.New("first")
	err2 := errors.New("second")
	me := NewMultiError(err1, err2)

	unwrapped := me.Unwrap()
	if len(unwrapped) != 2 {
		t.Fatalf("expected 2 errors, got %d", len(unwrapped))
	}
	if unwrapped[0] != err1 {
		t.Errorf("expected first error, got %v", unwrapped[0])
	}
	if unwrapped[1] != err2 {
		t.Errorf("expected second error, got %v", unwrapped[1])
	}
}

func TestMultiError_Is(t *testing.T) {
	baseErr := errors.New("target")
	me := NewMultiError(errors.New("other"), baseErr)

	if !errors.Is(me, baseErr) {
		t.Error("expected errors.Is to find baseErr in MultiError")
	}
}

func TestMultiError_As(t *testing.T) {
	type customErr struct{ msg string }
	custom := &customErr{msg: "custom"}
	me := NewMultiError(errors.New("other"), custom)

	var target *customErr
	if !errors.As(me, &target) {
		t.Error("expected errors.As to find customErr in MultiError")
	}
	if target.msg != "custom" {
		t.Errorf("expected msg 'custom', got %q", target.msg)
	}
}

func TestMultiError_SingleError(t *testing.T) {
	err := NewMultiError(errors.New("only one"))

	if len(err.Errors) != 1 {
		t.Fatalf("expected 1 error, got %d", len(err.Errors))
	}
}

func TestMultiError_NilErrors(t *testing.T) {
	err := NewMultiError(nil, errors.New("valid"), nil)

	if len(err.Errors) != 3 {
		t.Fatalf("expected 3 entries (including nils), got %d", len(err.Errors))
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./error/ -v`
Expected: FAIL (package does not exist)

- [ ] **Step 3: 实现 MultiError**

Create `error/multi_error.go`:

```go
package ddderror

import (
	"fmt"
	"strings"
)

type MultiError struct {
	Errors []error
}

func NewMultiError(errs ...error) *MultiError {
	return &MultiError{Errors: errs}
}

func (e *MultiError) Error() string {
	var b strings.Builder
	b.WriteString("multiple errors:")
	for i, err := range e.Errors {
		if err != nil {
			fmt.Fprintf(&b, "\n  [%d] %v", i+1, err)
		}
	}
	return b.String()
}

func (e *MultiError) Unwrap() []error {
	return e.Errors
}
```

注意：Go 的 `errors.Is` 和 `errors.As` 在 Go 1.20+ 支持 `Unwrap() []error` 返回多个错误。

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./error/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add error/multi_error.go error/multi_error_test.go
git commit -m "feat: add MultiError type for aggregating multiple errors"
```

---

### Task 3: EventBus 收集全部错误

**Files:**
- Modify: `cqrs/event/memory/event_bus.go:41-63`
- Modify: `cqrs/event/memory/event_bus_test.go:102-121`

- [ ] **Step 1: 编写 EventBus 多错误收集测试**

在 `event_bus_test.go` 中添加测试：

```go
type testErrorEventHandler2 struct{}

func (h *testErrorEventHandler2) Handle(ctx context.Context, event *testUserEvent) error {
	return errors.New("subscriber error 2")
}

func TestEventBus_MultipleHandlerErrors(t *testing.T) {
	chain := aspect.NewAspectChain()
	bus := NewEventBus[*testUserEvent](chain)

	okHandler := &testUserEventHandler{}
	errHandler1 := &testErrorEventHandler{}
	errHandler2 := &testErrorEventHandler2{}

	bus.Subscribe(okHandler)
	bus.Subscribe(errHandler1)
	bus.Subscribe(errHandler2)

	ctx := context.Background()
	err := bus.Publish(ctx, &testUserEvent{aggregateID: "1"})

	if err == nil {
		t.Fatal("expected error from handlers")
	}

	var me *ddderror.MultiError
	if !errors.As(err, &me) {
		t.Fatalf("expected *MultiError, got %T: %v", err, err)
	}

	if len(me.Errors) != 2 {
		t.Errorf("expected 2 errors in MultiError, got %d", len(me.Errors))
	}

	if !okHandler.called {
		t.Error("okHandler should still have been called")
	}
}
```

需要在文件顶部添加 import:
```go
import (
	ddderror "github.com/ddd-qce/core/error"
)
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./cqrs/event/memory/ -run TestEventBus_MultipleHandlerErrors -v`
Expected: FAIL (MultiError not returned)

- [ ] **Step 3: 修改 EventBus.Publish 收集全部错误**

将 `event_bus.go:41-63` 的 `Publish` 方法替换为：

```go
func (b *EventBus[T]) Publish(ctx context.Context, evt T) error {
	b.mu.RLock()
	handlers := make([]event.EventHandler[T], len(b.handlers))
	copy(handlers, b.handlers)
	b.mu.RUnlock()

	var errs []error
	for _, handler := range handlers {
		h := handler
		err := b.chain.ExecuteWithEventAspects(ctx, evt, func(ctx context.Context) error {
			return h.Handle(ctx, evt)
		})

		if err != nil {
			errs = append(errs, err)
		}
	}

	switch len(errs) {
	case 0:
		return nil
	case 1:
		return fmt.Errorf("event handler error: %w", errs[0])
	default:
		return fmt.Errorf("event handler errors: %w", ddderror.NewMultiError(errs...))
	}
}
```

添加 import:
```go
import (
	ddderror "github.com/ddd-qce/core/error"
)
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./cqrs/event/memory/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add cqrs/event/memory/event_bus.go cqrs/event/memory/event_bus_test.go
git commit -m "fix: EventBus.Publish collects all handler errors via MultiError"
```

---

### Task 4: TraceFilter 同一 span 匹配

**Files:**
- Modify: `trace/store.go:79-159`
- Modify: `trace/store_test.go`

- [ ] **Step 1: 编写同一 span 匹配的失败测试**

在 `store_test.go` 中添加：

```go
func TestMatchesFilter_SameSpanAnd(t *testing.T) {
	spans := []*Span{
		{Type: SpanTypeCommand, Status: SpanStatusSuccess},
		{Type: SpanTypeEvent, Status: SpanStatusError},
	}

	filter := TraceFilter{Type: SpanTypeCommand, Status: SpanStatusError}
	if matchesFilter(spans, filter) {
		t.Error("expected filter to not match when no single span satisfies both Type and Status")
	}

	spans2 := []*Span{
		{Type: SpanTypeCommand, Status: SpanStatusError},
		{Type: SpanTypeEvent, Status: SpanStatusSuccess},
	}

	if !matchesFilter(spans2, filter) {
		t.Error("expected filter to match when a single span satisfies both Type and Status")
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./trace/ -run TestMatchesFilter_SameSpanAnd -v`
Expected: FAIL (当前 OR 语义会让第一个测试匹配成功)

- [ ] **Step 3: 重写 matchesFilter 为同一 span AND 匹配**

将 `store.go:79-159` 的 `matchesFilter` 函数替换为：

```go
func matchesFilter(spans []*Span, filter TraceFilter) bool {
	for _, s := range spans {
		if spanMatchesAll(s, filter) {
			return true
		}
	}
	return false
}

func spanMatchesAll(s *Span, filter TraceFilter) bool {
	if filter.TraceID != "" && s.TraceID != filter.TraceID {
		return false
	}
	if filter.Type != "" && s.Type != filter.Type {
		return false
	}
	if filter.Status != "" && s.Status != filter.Status {
		return false
	}
	if !filter.StartTime.IsZero() && s.StartedAt.Before(filter.StartTime) {
		return false
	}
	if !filter.EndTime.IsZero() && s.StartedAt.After(filter.EndTime) {
		return false
	}
	if filter.NameContains != "" && !strings.Contains(s.Name, filter.NameContains) {
		return false
	}
	return true
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./trace/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add trace/store.go trace/store_test.go
git commit -m "fix: TraceFilter matches all conditions on the same span (AND semantics)"
```

---

### Task 5: 接口重复统一 (cqrs/event re-export domain/event)

**Files:**
- Modify: `cqrs/event/event.go`

- [ ] **Step 1: 改写 cqrs/event/event.go 为 type alias**

将 `cqrs/event/event.go` 全部内容替换为：

```go
package event

import (
	"context"

	domainevent "github.com/ddd-qce/core/domain/event"
)

type Handler[T domainevent.DomainEvent] = domainevent.EventHandler[T]

type Store[T domainevent.DomainEvent] = domainevent.EventStore[T]
```

- [ ] **Step 2: 运行全量测试确认兼容性**

Run: `go test ./...`
Expected: PASS (type alias 完全兼容，所有使用 `cqrs/event.Handler` 和 `cqrs/event.Store` 的代码无需修改)

- [ ] **Step 3: Commit**

```bash
git add cqrs/event/event.go
git commit -m "refactor: unify EventHandler/EventStore via type alias in cqrs/event"
```

---

### Task 6: JobManager 竞态修复

**Files:**
- Modify: `job/memory/job_manager.go`
- Modify: `job/memory/job_store.go`
- Modify: `job/memory/job_store_test.go`

- [ ] **Step 1: InMemoryJobStore.Update 增加终态保护**

在 `job_store.go` 的 `Update` 方法中，添加终态检查。如果 store 中的 job 已处于终态，拒绝更新：

将 `job_store.go:43-51` 替换为：

```go
func (s *InMemoryJobStore) Update(ctx context.Context, job *jobcore.Job) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	existing, exists := s.jobs[job.ID]
	if !exists {
		return fmt.Errorf("job %s not found", job.ID)
	}
	if isTerminalStatus(existing.Status) {
		return ErrJobAlreadyCompleted
	}
	s.jobs[job.ID] = job
	return nil
}

func isTerminalStatus(status jobcore.JobStatus) bool {
	return status == jobcore.JobStatusCompleted || status == jobcore.JobStatusFailed || status == jobcore.JobStatusCancelled
}
```

在 `job_store.go` 顶部添加错误变量：

```go
var ErrJobAlreadyCompleted = fmt.Errorf("job has already reached a terminal state")
```

- [ ] **Step 2: 修改 JobManager.executeJob 使用 detached context**

将 `job_manager.go:50-106` 的 `executeJob` 方法替换为：

```go
func (m *JobManager) executeJob(ctx context.Context, job *jobcore.Job) {
	storeCtx := context.Background()

	for {
		job.Lock()
		job.Status = jobcore.JobStatusRunning
		job.StartedAt = time.Now()
		job.Unlock()

		m.store.Update(storeCtx, job)

		var execCtx context.Context
		var cancel context.CancelFunc

		if job.Timeout > 0 {
			execCtx, cancel = context.WithTimeout(ctx, job.Timeout)
		} else {
			execCtx, cancel = context.WithCancel(ctx)
		}

		m.mu.Lock()
		m.cancelers[job.ID] = cancel
		m.mu.Unlock()

		result, err := m.executor.Execute(execCtx, job.Command)

		m.mu.Lock()
		delete(m.cancelers, job.ID)
		m.mu.Unlock()

		cancel()

		job.Lock()
		job.CompletedAt = time.Now()
		if err != nil {
			if job.Status == jobcore.JobStatusCancelled {
				job.Unlock()
				break
			}
			job.Status = jobcore.JobStatusFailed
			job.Error = err.Error()
			if job.RetryCount < job.MaxRetries {
				job.RetryCount++
				job.Unlock()
				m.store.Update(storeCtx, job)
				continue
			}
		} else {
			if job.Status == jobcore.JobStatusCancelled {
				job.Unlock()
				break
			}
			job.Status = jobcore.JobStatusCompleted
			job.Result = result
		}
		job.Unlock()

		m.store.Update(storeCtx, job)
		break
	}
}
```

- [ ] **Step 3: 修改 JobManager.Cancel 处理终态保护**

将 `job_manager.go:112-145` 的 `Cancel` 方法替换为：

```go
func (m *JobManager) Cancel(ctx context.Context, jobID string) error {
	job, err := m.store.Get(ctx, jobID)
	if err != nil {
		return err
	}

	job.Lock()
	if isTerminalStatus(job.Status) {
		job.Unlock()
		return fmt.Errorf("job %s cannot be cancelled (status: %s)", jobID, job.Status)
	}

	m.mu.Lock()
	cancel, exists := m.cancelers[jobID]
	m.mu.Unlock()

	if exists {
		cancel()
	}

	job.Status = jobcore.JobStatusCancelled
	job.CompletedAt = time.Now()
	job.Unlock()

	err = m.store.Update(ctx, job)
	if err != nil {
		if err == ErrJobAlreadyCompleted {
			return nil
		}
		return err
	}
	return nil
}
```

在 `job_manager.go` 中添加 `isTerminalStatus` 的引用（已在 job_store.go 中定义，但这是私有函数）。改为在 `job_manager.go` 中也定义一个或提取到 job/core 包。更简洁的做法是直接内联检查：

将 Cancel 中的终态检查改为内联：

```go
if job.Status == jobcore.JobStatusCompleted || job.Status == jobcore.JobStatusFailed || job.Status == jobcore.JobStatusCancelled {
```

同时在 `job_manager.go` 中导入 `ErrJobAlreadyCompleted`：

```go
// 由于 ErrJobAlreadyCompleted 定义在同包 job_store.go 中，可直接使用
```

实际上 `ErrJobAlreadyCompleted` 定义在同包 `job_store.go` 中，`job_manager.go` 可以直接使用。

- [ ] **Step 4: 修改 JobManager.Retry 也使用 detached context**

将 `job_manager.go:147-169` 中 `Retry` 的 `go m.executeJob(ctx, job)` 改为 `go m.executeJob(context.Background(), job)`：

```go
func (m *JobManager) Retry(ctx context.Context, jobID string) error {
	job, err := m.store.Get(ctx, jobID)
	if err != nil {
		return err
	}

	job.Lock()
	if job.Status != jobcore.JobStatusFailed {
		job.Unlock()
		return fmt.Errorf("job %s is not in failed state", jobID)
	}

	job.Status = jobcore.JobStatusPending
	job.Error = ""
	job.Result = nil
	job.StartedAt = time.Time{}
	job.CompletedAt = time.Time{}
	job.Unlock()

	m.store.Update(ctx, job)

	go m.executeJob(context.Background(), job)
	return nil
}
```

- [ ] **Step 5: 更新 Submit 也使用 detached context**

将 `job_manager.go:46` 的 `go m.executeJob(ctx, job)` 改为 `go m.executeJob(context.Background(), job)`：

```go
go m.executeJob(context.Background(), job)
```

注意：原始 ctx 的取消信号仍通过 `context.WithCancel(ctx)` / `context.WithTimeout(ctx, job.Timeout)` 在 executeJob 内部传递给 executor。改用 `context.Background()` 后，调用方取消 ctx 不再影响 job 执行。

但 Wait 方法仍使用调用方的 ctx（用于 Wait 自身的超时/取消），这是正确的。

- [ ] **Step 6: 运行测试确认通过**

Run: `go test ./job/... -v`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add job/memory/job_manager.go job/memory/job_store.go job/memory/job_store_test.go
git commit -m "fix: JobManager race condition and detached context for store operations"
```

---

### Task 7: 阶段 1 全量测试验证

- [ ] **Step 1: 运行全量测试**

Run: `go test ./... -v`
Expected: ALL PASS

- [ ] **Step 2: 运行 go vet**

Run: `go vet ./...`
Expected: 无警告

---

## 阶段 2：API 改进

### Task 8: Register 错误信息改进

**Files:**
- Modify: `cqrs/command/memory/command_bus.go:29-38`
- Modify: `cqrs/query/memory/query_bus.go:29-38`
- Modify: `cqrs/command/memory/command_bus_test.go:230-247`

- [ ] **Step 1: 改进 CommandBus panic 信息**

将 `command_bus.go:29-38` 替换为：

```go
func RegisterCommand[T any, R any](bus *CommandBus, handler command.CommandHandler[T, R]) {
	bus.mu.Lock()
	defer bus.mu.Unlock()
	var zero T
	cmdType := reflect.TypeOf(zero)
	if existing, exists := bus.handlers[cmdType]; exists {
		panic(fmt.Sprintf("handler already registered for command type %T (existing: %T, new: %T)", zero, existing, handler))
	}
	bus.handlers[cmdType] = handler
	bus.dispatchFuncs[cmdType] = func(ctx context.Context, cmd any) (any, error) {
		return handler.Handle(ctx, cmd.(T))
	}
}
```

注意：这里同时注册了 `dispatchFuncs`（为 Task 11 做准备）。

- [ ] **Step 2: 改进 QueryBus panic 信息**

将 `query_bus.go:34-36` 替换为：

```go
	if _, exists := bus.handlers[queryType]; exists {
		panic(fmt.Sprintf("handler already registered for query type %T (existing handler type: %T, new: %T)", zero, bus.handlers[queryType], handler))
	}
```

- [ ] **Step 3: 更新 CommandBus 重复注册测试的 panic 信息断言**

将 `command_bus_test.go:240` 的 expected 字符串更新：

```go
		expected := "handler already registered for command type *memory.testCreateUserCommand"
```

改为：

```go
		expected := "handler already registered for command type *memory.testCreateUserCommand (existing:"
```

由于 panic 信息包含 handler 类型（不稳定），改为包含性检查：

```go
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic for duplicate registration")
		}
		msg := r.(string)
		if !strings.Contains(msg, "handler already registered") {
			t.Errorf("expected panic message to contain 'handler already registered', got %q", msg)
		}
		if !strings.Contains(msg, "existing:") {
			t.Errorf("expected panic message to contain 'existing:', got %q", msg)
		}
	}()
```

在测试文件顶部添加 `"strings"` import。

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./cqrs/... -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add cqrs/command/memory/command_bus.go cqrs/command/memory/command_bus_test.go cqrs/query/memory/query_bus.go
git commit -m "improve: better panic messages for duplicate handler registration"
```

---

### Task 9: JobManager.Wait 改用 channel 通知

**Files:**
- Modify: `job/memory/job_store.go`
- Modify: `job/memory/job_manager.go:172-192`
- Modify: `job/memory/job_store_test.go`

- [ ] **Step 1: InMemoryJobStore 增加 completion channel**

修改 `job_store.go`：

```go
type InMemoryJobStore struct {
	jobs           map[string]*jobcore.Job
	completionChans map[string]chan struct{}
	mu             sync.RWMutex
}

func NewJobStore() *InMemoryJobStore {
	return &InMemoryJobStore{
		jobs:            make(map[string]*jobcore.Job),
		completionChans: make(map[string]chan struct{}),
	}
}
```

在 `Create` 方法中初始化 channel：

```go
func (s *InMemoryJobStore) Create(ctx context.Context, job *jobcore.Job) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.jobs[job.ID]; exists {
		return fmt.Errorf("job %s already exists", job.ID)
	}
	s.jobs[job.ID] = job
	s.completionChans[job.ID] = make(chan struct{})
	return nil
}
```

在 `Update` 方法中，当 job 进入终态时 close channel：

```go
func (s *InMemoryJobStore) Update(ctx context.Context, job *jobcore.Job) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	existing, exists := s.jobs[job.ID]
	if !exists {
		return fmt.Errorf("job %s not found", job.ID)
	}
	if isTerminalStatus(existing.Status) {
		return ErrJobAlreadyCompleted
	}
	s.jobs[job.ID] = job
	if isTerminalStatus(job.Status) {
		if ch, ok := s.completionChans[job.ID]; ok {
			close(ch)
		}
	}
	return nil
}
```

添加 `CompletionChan` 方法：

```go
func (s *InMemoryJobStore) CompletionChan(jobID string) <-chan struct{} {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ch, exists := s.completionChans[jobID]
	if !exists {
		return nil
	}
	return ch
}
```

在 `Delete` 方法中清理 channel：

```go
func (s *InMemoryJobStore) Delete(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.jobs[id]; !exists {
		return fmt.Errorf("job %s not found", id)
	}
	delete(s.jobs, id)
	if ch, ok := s.completionChans[id]; ok {
		close(ch)
		delete(s.completionChans, id)
	}
	return nil
}
```

- [ ] **Step 2: 修改 JobManager.Wait 使用 channel**

将 `job_manager.go:172-192` 的 `Wait` 方法替换为：

```go
func (m *JobManager) Wait(ctx context.Context, jobID string, timeout time.Duration) (*jobcore.Job, error) {
	ch := m.store.CompletionChan(jobID)
	if ch == nil {
		return nil, fmt.Errorf("job %s not found", jobID)
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

注意：`CompletionChan` 返回 `<-chan struct{}`，但 `InMemoryJobStore` 是具体类型，`JobStore` 接口没有 `CompletionChan` 方法。需要将 `m.store` 断言或改用具体类型。

更安全的做法：在 `JobManager` 中保存 `*InMemoryJobStore` 引用：

将 `job_manager.go` 中 JobManager 结构体改为：

```go
type JobManager struct {
	store     *InMemoryJobStore
	executor  command.CommandBus
	mu        sync.Mutex
	cancelers map[string]context.CancelFunc
}
```

`NewJobManager` 签名改为接受 `*InMemoryJobStore`：

```go
func NewJobManager(store *InMemoryJobStore, executor command.CommandBus) *JobManager {
```

这保持了与现有测试的兼容性（测试都直接传入 `NewJobStore()` 返回的 `*InMemoryJobStore`）。

- [ ] **Step 3: 更新 Wait 超时测试**

`TestJobManager_WaitTimeout` 测试预期错误消息 `"job %s timed out waiting for completion"`，但现在改为 `ctx.Err()`（`context.DeadlineExceeded`）。更新测试：

在 `job_manager_test.go:296-303` 将：

```go
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if err.Error() == "" {
		t.Error("expected timeout error message")
	}
```

改为：

```go
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected context.DeadlineExceeded, got %v", err)
	}
```

添加 import `"errors"`。

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./job/... -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add job/memory/job_store.go job/memory/job_manager.go job/memory/job_manager_test.go
git commit -m "improve: JobManager.Wait uses channel notification instead of polling"
```

---

### Task 10: TransactionAspect 可选 Event 事务

**Files:**
- Modify: `aspect/builtin/transaction.go`

- [ ] **Step 1: 添加 EnableEventTx 字段并实现 Event 事务**

将 `transaction.go` 全文替换为：

```go
package builtin

import (
	"context"
	"fmt"
	"time"
)

type TransactionManager interface {
	Begin(ctx context.Context) (context.Context, error)
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
}

type TransactionAspect struct {
	TxManager     TransactionManager
	EnableEventTx bool
}

func (t *TransactionAspect) Name() string {
	return "transaction"
}

func (t *TransactionAspect) Order() int {
	return 10
}

func (t *TransactionAspect) BeforeQuery(ctx context.Context, query any) (context.Context, error) {
	return ctx, nil
}

func (t *TransactionAspect) AfterQuery(ctx context.Context, query any, result any, err error, duration time.Duration) error {
	return nil
}

func (t *TransactionAspect) BeforeCommand(ctx context.Context, cmd any) (context.Context, error) {
	return t.TxManager.Begin(ctx)
}

func (t *TransactionAspect) AfterCommand(ctx context.Context, cmd any, result any, err error, duration time.Duration) error {
	if err != nil {
		if rbErr := t.TxManager.Rollback(ctx); rbErr != nil {
			return fmt.Errorf("command failed: %v, rollback failed: %v", err, rbErr)
		}
		return err
	}
	return t.TxManager.Commit(ctx)
}

func (t *TransactionAspect) BeforePublish(ctx context.Context, event any) (context.Context, error) {
	if t.EnableEventTx {
		return t.TxManager.Begin(ctx)
	}
	return ctx, nil
}

func (t *TransactionAspect) AfterPublish(ctx context.Context, event any, err error, duration time.Duration) error {
	if t.EnableEventTx {
		if err != nil {
			if rbErr := t.TxManager.Rollback(ctx); rbErr != nil {
				return fmt.Errorf("event handler failed: %v, rollback failed: %v", err, rbErr)
			}
			return err
		}
		return t.TxManager.Commit(ctx)
	}
	return nil
}
```

- [ ] **Step 2: 运行测试确认通过**

Run: `go test ./aspect/... -v`
Expected: PASS (EnableEventTx 默认 false，不改变现有行为)

- [ ] **Step 3: Commit**

```bash
git add aspect/builtin/transaction.go
git commit -m "feat: add EnableEventTx option to TransactionAspect for outbox pattern"
```

---

### Task 11: Span.Duration JSON 输出改进

**Files:**
- Modify: `trace/span.go`
- Modify: `trace/trace_test.go` (如需)

- [ ] **Step 1: 添加自定义 MarshalJSON**

将 `span.go` 替换为：

```go
package trace

import (
	"encoding/json"
	"time"
)

const (
	SpanTypeCommand = "command"
	SpanTypeQuery   = "query"
	SpanTypeEvent   = "event"

	SpanStatusSuccess = "success"
	SpanStatusError   = "error"
)

type Span struct {
	ID        string        `json:"id"`
	TraceID   string        `json:"trace_id"`
	ParentID  string        `json:"parent_id"`
	Type      string        `json:"type"`
	Name      string        `json:"name"`
	Status    string        `json:"status"`
	Error     string        `json:"error,omitempty"`
	StartedAt time.Time     `json:"started_at"`
	Duration  time.Duration `json:"duration"`
}

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

type TraceFilter struct {
	TraceID      string
	Type         string
	Status       string
	StartTime    time.Time
	EndTime      time.Time
	NameContains string
}
```

- [ ] **Step 2: 编写 Span JSON 序列化测试**

在 `trace/span.go` 同目录添加或修改测试：

在 `trace/trace_test.go` 中检查是否已有 JSON 测试，若无则添加：

```go
func TestSpan_MarshalJSON_DurationMs(t *testing.T) {
	span := &Span{
		ID:        "span-1",
		TraceID:   "trace-1",
		Type:      SpanTypeCommand,
		Name:      "CreateOrder",
		Status:    SpanStatusSuccess,
		StartedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Duration:  1500 * time.Millisecond,
	}

	data, err := json.Marshal(span)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if durationMs, ok := result["duration_ms"].(float64); !ok {
		t.Error("duration_ms field missing or not a number")
	} else if durationMs != 1500.0 {
		t.Errorf("expected duration_ms=1500.0, got %v", durationMs)
	}

	if duration, ok := result["duration"].(float64); !ok {
		t.Error("duration field missing or not a number")
	} else if duration != float64(1500*time.Millisecond) {
		t.Errorf("expected duration=%d, got %v", 1500*time.Millisecond, duration)
	}
}
```

需要在 `trace_test.go` 中添加 `"encoding/json"` import。

- [ ] **Step 3: 运行测试确认通过**

Run: `go test ./trace/ -v`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add trace/span.go trace/trace_test.go
git commit -m "feat: add duration_ms field to Span JSON output"
```

---

### Task 12: invokeHandler 编译期保障 (dispatchFuncs)

**Files:**
- Modify: `cqrs/command/memory/command_bus.go`
- Modify: `cqrs/command/memory/command_bus_execute_test.go`

- [ ] **Step 1: 添加 dispatchFuncs map 到 CommandBus**

将 `command_bus.go:13-17` 的 CommandBus 结构体替换为：

```go
type CommandBus struct {
	handlers      map[reflect.Type]any
	dispatchFuncs map[reflect.Type]func(context.Context, any) (any, error)
	chain         *aspect.AspectChain
	mu            sync.RWMutex
}
```

将 `NewCommandBus` 更新：

```go
func NewCommandBus(chain *aspect.AspectChain) *CommandBus {
	if chain == nil {
		chain = aspect.NewAspectChain()
	}
	return &CommandBus{
		handlers:      make(map[reflect.Type]any),
		dispatchFuncs: make(map[reflect.Type]func(context.Context, any) (any, error)),
		chain:         chain,
	}
}
```

- [ ] **Step 2: Dispatch 和 Execute 统一使用 dispatchFuncs**

将 `Dispatch` 函数 (`command_bus.go:40-61`) 替换为：

```go
func Dispatch[T any, R any](bus *CommandBus, ctx context.Context, cmd T) (R, error) {
	cmdType := reflect.TypeOf(cmd)

	bus.mu.RLock()
	fn, exists := bus.dispatchFuncs[cmdType]
	bus.mu.RUnlock()

	if !exists {
		var zero R
		return zero, fmt.Errorf("no handler registered for command type: %s", cmdType)
	}

	result, err := bus.chain.ExecuteWithCommandAspects(ctx, cmd, func(ctx context.Context) (any, error) {
		return fn(ctx, cmd)
	})
	if err != nil {
		var zero R
		return zero, err
	}
	return result.(R), nil
}
```

将 `Execute` 方法 (`command_bus.go:63-77`) 替换为：

```go
func (b *CommandBus) Execute(ctx context.Context, cmd any) (any, error) {
	cmdType := reflect.TypeOf(cmd)

	b.mu.RLock()
	fn, exists := b.dispatchFuncs[cmdType]
	b.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("no handler registered for command type: %s", cmdType)
	}

	return b.chain.ExecuteWithCommandAspects(ctx, cmd, func(ctx context.Context) (any, error) {
		return fn(ctx, cmd)
	})
}
```

删除 `invokeHandler` 函数 (`command_bus.go:79-98`)。

- [ ] **Step 3: 更新 execute_test.go 中 TestInvokeHandler_NoHandleMethod 测试**

`TestInvokeHandler_NoHandleMethod` 测试了被删除的 `invokeHandler` 函数。删除此测试，因为 `dispatchFuncs` 方式在注册时就确定了调用路径，不再有运行时反射风险。

删除 `command_bus_execute_test.go:145-159`。

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./cqrs/command/memory/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add cqrs/command/memory/command_bus.go cqrs/command/memory/command_bus_execute_test.go
git commit -m "refactor: replace reflect-based invokeHandler with dispatchFuncs for compile-time safety"
```

---

### Task 13: 阶段 2 全量测试验证

- [ ] **Step 1: 运行全量测试**

Run: `go test ./... -v`
Expected: ALL PASS

- [ ] **Step 2: 运行 go vet**

Run: `go vet ./...`
Expected: 无警告

---

## 阶段 3：新功能

### Task 14: 统一错误类型体系

**Files:**
- Create: `error/domain_error.go`
- Create: `error/domain_error_test.go`
- Create: `error/errors.go`
- Modify: `aspect/builtin/tracing.go`
- Modify: `aspect/builtin/logging.go`
- Modify: `aspect/builtin/metrics.go`

- [ ] **Step 1: 编写 DomainError 测试**

Create `error/domain_error_test.go`:

```go
package ddderror

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestDomainError_Error(t *testing.T) {
	err := NewDomainError("USER_NOT_FOUND", "user not found")
	msg := err.Error()
	if !strings.Contains(msg, "USER_NOT_FOUND") {
		t.Errorf("expected message to contain code, got %q", msg)
	}
	if !strings.Contains(msg, "user not found") {
		t.Errorf("expected message to contain message, got %q", msg)
	}
}

func TestDomainError_WithCause(t *testing.T) {
	cause := fmt.Errorf("db connection failed")
	err := NewDomainErrorWithCause("SYSTEM_ERROR", "internal error", cause)

	if !errors.Is(err, cause) {
		t.Error("expected errors.Is to find cause")
	}
}

func TestDomainError_Code(t *testing.T) {
	err := NewDomainError("INVALID_STATE", "invalid state")
	if err.Code != "INVALID_STATE" {
		t.Errorf("expected code INVALID_STATE, got %s", err.Code)
	}
}

func TestDomainError_IsDomainError(t *testing.T) {
	err := NewDomainError("NOT_FOUND", "resource not found")
	var de *DomainError
	if !errors.As(err, &de) {
		t.Error("expected errors.As to find *DomainError")
	}
}

func TestIsDomainError(t *testing.T) {
	err := NewDomainError("TEST", "test")
	if !IsDomainError(err) {
		t.Error("expected IsDomainError to return true")
	}

	regularErr := fmt.Errorf("regular error")
	if IsDomainError(regularErr) {
		t.Error("expected IsDomainError to return false for regular error")
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./error/ -v`
Expected: FAIL

- [ ] **Step 3: 实现 DomainError**

Create `error/domain_error.go`:

```go
package ddderror

import "fmt"

type DomainError struct {
	Code    string
	Message string
	Cause   error
}

func NewDomainError(code, msg string) *DomainError {
	return &DomainError{Code: code, Message: msg}
}

func NewDomainErrorWithCause(code, msg string, cause error) *DomainError {
	return &DomainError{Code: code, Message: msg, Cause: cause}
}

func (e *DomainError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

func (e *DomainError) Unwrap() error {
	return e.Cause
}

func IsDomainError(err error) bool {
	var de *DomainError
	return errors.As(err, &de)
}
```

添加 import `"errors"`。

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./error/ -v`
Expected: PASS

- [ ] **Step 5: 创建通用错误变量**

Create `error/errors.go`:

```go
package ddderror

import "errors"

var (
	ErrNotFound           = NewDomainError("NOT_FOUND", "resource not found")
	ErrAlreadyExists      = NewDomainError("ALREADY_EXISTS", "resource already exists")
	ErrInvalidState       = NewDomainError("INVALID_STATE", "invalid state for operation")
	ErrPermissionDenied   = NewDomainError("PERMISSION_DENIED", "permission denied")
	ErrJobAlreadyCompleted = errors.New("job has already reached a terminal state")
	ErrJobNotFound        = errors.New("job not found")
)
```

- [ ] **Step 6: 集成 DomainError 到 TracingAspect**

修改 `aspect/builtin/tracing.go`，在 AfterCommand/AfterQuery/AfterPublish 中区分 DomainError 和系统错误。

在 `tracing.go` 中添加 import:

```go
import (
	ddderror "github.com/ddd-qce/core/error"
)
```

修改所有 `After*` 方法中的错误状态设置。以 `AfterCommand` 为例，将：

```go
if err != nil {
	span.Status = trace.SpanStatusError
	span.Error = err.Error()
} else {
	span.Status = trace.SpanStatusSuccess
}
```

改为：

```go
if err != nil {
	if ddderror.IsDomainError(err) {
		span.Status = "business_error"
	} else {
		span.Status = trace.SpanStatusError
	}
	span.Error = err.Error()
} else {
	span.Status = trace.SpanStatusSuccess
}
```

对 `AfterQuery` 和 `AfterPublish` 做相同修改。

- [ ] **Step 7: Commit**

```bash
git add error/ aspect/builtin/tracing.go
git commit -m "feat: add DomainError type system and integrate with TracingAspect"
```

---

### Task 15: PostgreSQL 适配器 - 基础设施

**Files:**
- Create: `adapter/sql/options.go`
- Create: `adapter/sql/transaction.go`
- Create: `adapter/sql/serializer.go`
- Create: `adapter/sql/migrations/001_init.sql`

- [ ] **Step 1: 创建 SQLOptions**

Create `adapter/sql/options.go`:

```go
package sql

import "database/sql"

type SQLOptions struct {
	DB            *sql.DB
	TablePrefix   string
	SnapshotEvery int
}

func (o SQLOptions) prefixed(table string) string {
	if o.TablePrefix == "" {
		return "ddd_" + table
	}
	return o.TablePrefix + table
}
```

- [ ] **Step 2: 创建 SQLTransactionManager**

Create `adapter/sql/transaction.go`:

```go
package sql

import (
	"context"
	"database/sql"
	"fmt"
)

type ctxKeyTx struct{}

type SQLTransactionManager struct {
	db *sql.DB
}

func NewSQLTransactionManager(db *sql.DB) *SQLTransactionManager {
	return &SQLTransactionManager{db: db}
}

func (m *SQLTransactionManager) Begin(ctx context.Context) (context.Context, error) {
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	return context.WithValue(ctx, ctxKeyTx{}, tx), nil
}

func (m *SQLTransactionManager) Commit(ctx context.Context) error {
	tx, ok := ctx.Value(ctxKeyTx{}).(*sql.Tx)
	if !ok {
		return fmt.Errorf("no transaction in context")
	}
	return tx.Commit()
}

func (m *SQLTransactionManager) Rollback(ctx context.Context) error {
	tx, ok := ctx.Value(ctxKeyTx{}).(*sql.Tx)
	if !ok {
		return fmt.Errorf("no transaction in context")
	}
	return tx.Rollback()
}

func GetTx(ctx context.Context) *sql.Tx {
	tx, _ := ctx.Value(ctxKeyTx{}).(*sql.Tx)
	return tx
}
```

- [ ] **Step 3: 创建 EventSerializer**

Create `adapter/sql/serializer.go`:

```go
package sql

import (
	"encoding/json"
	"fmt"

	"github.com/ddd-qce/core/domain/event"
)

type EventSerializer[T event.DomainEvent] interface {
	Serialize(event T) ([]byte, error)
	Deserialize(eventType string, data []byte) (T, error)
}

type JSONSerializer[T event.DomainEvent] struct {
	registry map[string]func() T
}

func NewJSONSerializer[T event.DomainEvent]() *JSONSerializer[T] {
	return &JSONSerializer[T]{
		registry: make(map[string]func() T),
	}
}

func (s *JSONSerializer[T]) Register(eventType string, factory func() T) {
	s.registry[eventType] = factory
}

func (s *JSONSerializer[T]) Serialize(evt T) ([]byte, error) {
	return json.Marshal(evt)
}

func (s *JSONSerializer[T]) Deserialize(eventType string, data []byte) (T, error) {
	factory, ok := s.registry[eventType]
	if !ok {
		var zero T
		return zero, fmt.Errorf("no factory registered for event type: %s", eventType)
	}
	evt := factory()
	if err := json.Unmarshal(data, evt); err != nil {
		return zero, fmt.Errorf("unmarshal event %s: %w", eventType, err)
	}
	return evt, nil
}
```

- [ ] **Step 4: 创建迁移 DDL**

Create `adapter/sql/migrations/001_init.sql`:

```sql
-- DDD-QCE Framework Schema Migration

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

CREATE TABLE ddd_snapshots (
    aggregate_id VARCHAR(255) PRIMARY KEY,
    version      BIGINT NOT NULL,
    payload      JSONB NOT NULL,
    created_at   TIMESTAMP WITH TIME ZONE NOT NULL
);

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
```

- [ ] **Step 5: Commit**

```bash
git add adapter/sql/
git commit -m "feat: add PostgreSQL adapter infrastructure (options, transaction, serializer, migrations)"
```

---

### Task 16: PostgreSQL EventStore 实现

**Files:**
- Create: `adapter/sql/postgres/event_store.go`
- Create: `adapter/sql/postgres/event_store_test.go`

- [ ] **Step 1: 编写 PostgresEventStore 测试**

Create `adapter/sql/postgres/event_store_test.go`:

```go
package postgres

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/ddd-qce/core/adapter/sql"
	"github.com/ddd-qce/core/domain/event"
	_ "github.com/lib/pq"
)

type pgTestEvent struct {
	AggID     string
	EventType string
	Data      string
	Timestamp time.Time
}

func (e *pgTestEvent) AggregateID() string   { return e.AggID }
func (e *pgTestEvent) EventType() string     { return e.EventType }
func (e *pgTestEvent) OccurredAt() time.Time { return e.Timestamp }

func newTestEventStore(t *testing.T) (*PostgresEventStore[*pgTestEvent], *sql.DB) {
	t.Helper()
	db, err := sql.Open("postgres", "postgres://ddd:ddd@localhost:5432/ddd_test?sslmode=disable")
	if err != nil {
		t.Skip("postgres not available")
	}
	if err := db.Ping(); err != nil {
		t.Skip("postgres not available")
	}

	serializer := sql.NewJSONSerializer[*pgTestEvent]()
	serializer.Register("pgTestEvent", func() *pgTestEvent { return &pgTestEvent{} })

	store := NewPostgresEventStore[*pgTestEvent](sql.SQLOptions{DB: db}, serializer)
	return store, db
}

func TestPostgresEventStore_AppendAndLoad(t *testing.T) {
	store, db := newTestEventStore(t)
	defer db.Close()

	ctx := context.Background()

	events := []*pgTestEvent{
		{AggID: "agg-1", EventType: "pgTestEvent", Data: "e1", Timestamp: time.Now()},
		{AggID: "agg-1", EventType: "pgTestEvent", Data: "e2", Timestamp: time.Now()},
	}

	err := store.Append(ctx, events)
	if err != nil {
		t.Fatalf("append failed: %v", err)
	}

	loaded, err := store.Load(ctx, "agg-1", 0)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}

	if len(loaded) != 2 {
		t.Errorf("expected 2 events, got %d", len(loaded))
	}
}
```

注意：此测试需要 PostgreSQL 实例，如果不可用则 skip。生产环境应使用 testcontainers 或集成测试 CI。

- [ ] **Step 2: 实现 PostgresEventStore**

Create `adapter/sql/postgres/event_store.go`:

```go
package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/ddd-qce/core/adapter/sql"
	"github.com/ddd-qce/core/domain/event"
)

type PostgresEventStore[T event.DomainEvent] struct {
	opts       sql.SQLOptions
	serializer sql.EventSerializer[T]
}

func NewPostgresEventStore[T event.DomainEvent](opts sql.SQLOptions, serializer sql.EventSerializer[T]) *PostgresEventStore[T] {
	return &PostgresEventStore[T]{
		opts:       opts,
		serializer: serializer,
	}
}

func (s *PostgresEventStore[T]) Append(ctx context.Context, events []T) error {
	tx := sql.GetTx(ctx)
	if tx == nil {
		var err error
		tx, err = s.opts.DB.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin tx: %w", err)
		}
		defer tx.Rollback()
	}

	table := s.opts.prefixed("domain_events")

	for _, evt := range events {
		payload, err := s.serializer.Serialize(evt)
		if err != nil {
			return fmt.Errorf("serialize event: %w", err)
		}

		_, err = tx.ExecContext(ctx,
			fmt.Sprintf("INSERT INTO %s (aggregate_id, version, event_type, payload, occurred_at, trace_id) VALUES ($1, $2, $3, $4, $5, $6)", table),
			evt.AggregateID(),
			evt.OccurredAt().UnixNano(),
			evt.EventType(),
			payload,
			evt.OccurredAt(),
			nil,
		)
		if err != nil {
			return fmt.Errorf("insert event: %w", err)
		}
	}

	if sql.GetTx(ctx) == nil {
		return tx.Commit()
	}
	return nil
}

func (s *PostgresEventStore[T]) Load(ctx context.Context, aggregateID string, afterVersion int) ([]T, error) {
	table := s.opts.prefixed("domain_events")

	rows, err := s.opts.DB.QueryContext(ctx,
		fmt.Sprintf("SELECT event_type, payload FROM %s WHERE aggregate_id = $1 ORDER BY occurred_at ASC", table),
		aggregateID,
	)
	if err != nil {
		return nil, fmt.Errorf("query events: %w", err)
	}
	defer rows.Close()

	var allEvents []T
	for rows.Next() {
		var eventType string
		var payload []byte
		if err := rows.Scan(&eventType, &payload); err != nil {
			return nil, fmt.Errorf("scan event: %w", err)
		}

		evt, err := s.serializer.Deserialize(eventType, payload)
		if err != nil {
			return nil, fmt.Errorf("deserialize event: %w", err)
		}
		allEvents = append(allEvents, evt)
	}

	if len(allEvents) == 0 {
		return nil, fmt.Errorf("no events found for aggregate: %s", aggregateID)
	}

	if afterVersion >= len(allEvents) {
		return []T{}, nil
	}

	return allEvents[afterVersion:], nil
}
```

- [ ] **Step 3: Commit**

```bash
git add adapter/sql/postgres/event_store.go adapter/sql/postgres/event_store_test.go
git commit -m "feat: add PostgreSQL EventStore implementation"
```

---

### Task 17: PostgreSQL JobStore 实现

**Files:**
- Create: `adapter/sql/postgres/job_store.go`
- Create: `adapter/sql/postgres/job_store_test.go`

- [ ] **Step 1: 编写 PostgresJobStore 测试**

Create `adapter/sql/postgres/job_store_test.go`:

```go
package postgres

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/ddd-qce/core/adapter/sql"
	jobcore "github.com/ddd-qce/core/job/core"
	_ "github.com/lib/pq"
)

func newTestJobStore(t *testing.T) (*PostgresJobStore, *sql.DB) {
	t.Helper()
	db, err := sql.Open("postgres", "postgres://ddd:ddd@localhost:5432/ddd_test?sslmode=disable")
	if err != nil {
		t.Skip("postgres not available")
	}
	if err := db.Ping(); err != nil {
		t.Skip("postgres not available")
	}
	return NewPostgresJobStore(sql.SQLOptions{DB: db}), db
}

func TestPostgresJobStore_CreateAndGet(t *testing.T) {
	store, db := newTestJobStore(t)
	defer db.Close()

	ctx := context.Background()
	job := &jobcore.Job{
		ID:        "job-1",
		Command:   "test command",
		Status:    jobcore.JobStatusPending,
		CreatedAt: time.Now(),
	}

	err := store.Create(ctx, job)
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	got, err := store.Get(ctx, "job-1")
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}

	if got.ID != "job-1" {
		t.Errorf("expected job-1, got %s", got.ID)
	}
}
```

- [ ] **Step 2: 实现 PostgresJobStore**

Create `adapter/sql/postgres/job_store.go`:

```go
package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/ddd-qce/core/adapter/sql"
	jobcore "github.com/ddd-qce/core/job/core"
)

type PostgresJobStore struct {
	opts sql.SQLOptions
}

func NewPostgresJobStore(opts sql.SQLOptions) *PostgresJobStore {
	return &PostgresJobStore{opts: opts}
}

func (s *PostgresJobStore) Create(ctx context.Context, job *jobcore.Job) error {
	table := s.opts.prefixed("jobs")
	cmdPayload, _ := json.Marshal(job.Command)

	var result any
	if job.Result != nil {
		result, _ = json.Marshal(job.Result)
	}

	_, err := s.opts.DB.ExecContext(ctx,
		fmt.Sprintf("INSERT INTO %s (id, command_type, command_payload, status, result, error, retry_count, max_retries, timeout_ns, created_at, started_at, completed_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)", table),
		job.ID, fmt.Sprintf("%T", job.Command), cmdPayload, string(job.Status), result, job.Error,
		job.RetryCount, job.MaxRetries, job.Timeout.Nanoseconds(),
		job.CreatedAt, nilTime(job.StartedAt), nilTime(job.CompletedAt),
	)
	return err
}

func (s *PostgresJobStore) Get(ctx context.Context, id string) (*jobcore.Job, error) {
	table := s.opts.prefixed("jobs")

	var job jobcore.Job
	var cmdType, cmdPayload string
	var resultJSON, errorStr sql.NullString
	var startedAt, completedAt sql.NullTime
	var timeoutNs int64

	err := s.opts.DB.QueryRowContext(ctx,
		fmt.Sprintf("SELECT id, command_type, command_payload, status, result, error, retry_count, max_retries, timeout_ns, created_at, started_at, completed_at FROM %s WHERE id = $1", table),
		id,
	).Scan(&job.ID, &cmdType, &cmdPayload, &job.Status, &resultJSON, &errorStr,
		&job.RetryCount, &job.MaxRetries, &timeoutNs, &job.CreatedAt, &startedAt, &completedAt)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("job %s not found", id)
	}
	if err != nil {
		return nil, err
	}

	job.Timeout = time.Duration(timeoutNs)
	if errorStr.Valid {
		job.Error = errorStr.String
	}
	if startedAt.Valid {
		job.StartedAt = startedAt.Time
	}
	if completedAt.Valid {
		job.CompletedAt = completedAt.Time
	}
	if resultJSON.Valid {
		json.Unmarshal([]byte(resultJSON.String), &job.Result)
	}

	return &job, nil
}

func (s *PostgresJobStore) Update(ctx context.Context, job *jobcore.Job) error {
	table := s.opts.prefixed("jobs")

	var result any
	if job.Result != nil {
		result, _ = json.Marshal(job.Result)
	}

	res, err := s.opts.DB.ExecContext(ctx,
		fmt.Sprintf("UPDATE %s SET status=$1, result=$2, error=$3, retry_count=$4, max_retries=$5, timeout_ns=$6, started_at=$7, completed_at=$8 WHERE id=$9 AND status != 'completed' AND status != 'failed' AND status != 'cancelled'", table),
		string(job.Status), result, job.Error,
		job.RetryCount, job.MaxRetries, job.Timeout.Nanoseconds(),
		nilTime(job.StartedAt), nilTime(job.CompletedAt), job.ID,
	)
	if err != nil {
		return err
	}

	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("job %s not found or already in terminal state", job.ID)
	}
	return nil
}

func (s *PostgresJobStore) List(ctx context.Context, status jobcore.JobStatus) ([]*jobcore.Job, error) {
	table := s.opts.prefixed("jobs")

	rows, err := s.opts.DB.QueryContext(ctx,
		fmt.Sprintf("SELECT id, command_type, command_payload, status, result, error, retry_count, max_retries, timeout_ns, created_at, started_at, completed_at FROM %s WHERE status = $1", table),
		string(status),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*jobcore.Job
	for rows.Next() {
		var job jobcore.Job
		var cmdType, cmdPayload string
		var resultJSON, errorStr sql.NullString
		var startedAt, completedAt sql.NullTime
		var timeoutNs int64

		err := rows.Scan(&job.ID, &cmdType, &cmdPayload, &job.Status, &resultJSON, &errorStr,
			&job.RetryCount, &job.MaxRetries, &timeoutNs, &job.CreatedAt, &startedAt, &completedAt)
		if err != nil {
			return nil, err
		}

		job.Timeout = time.Duration(timeoutNs)
		if errorStr.Valid {
			job.Error = errorStr.String
		}
		if startedAt.Valid {
			job.StartedAt = startedAt.Time
		}
		if completedAt.Valid {
			job.CompletedAt = completedAt.Time
		}

		result = append(result, &job)
	}
	return result, nil
}

func (s *PostgresJobStore) Delete(ctx context.Context, id string) error {
	table := s.opts.prefixed("jobs")
	res, err := s.opts.DB.ExecContext(ctx, fmt.Sprintf("DELETE FROM %s WHERE id = $1", table), id)
	if err != nil {
		return err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("job %s not found", id)
	}
	return nil
}

func nilTime(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t
}
```

- [ ] **Step 3: Commit**

```bash
git add adapter/sql/postgres/job_store.go adapter/sql/postgres/job_store_test.go
git commit -m "feat: add PostgreSQL JobStore implementation"
```

---

### Task 18: PostgreSQL TraceStore 实现

**Files:**
- Create: `adapter/sql/postgres/trace_store.go`
- Create: `adapter/sql/postgres/trace_store_test.go`

- [ ] **Step 1: 编写 PostgresTraceStore 测试**

Create `adapter/sql/postgres/trace_store_test.go`:

```go
package postgres

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/ddd-qce/core/adapter/sql"
	"github.com/ddd-qce/core/trace"
	_ "github.com/lib/pq"
)

func newTestTraceStore(t *testing.T) (*PostgresTraceStore, *sql.DB) {
	t.Helper()
	db, err := sql.Open("postgres", "postgres://ddd:ddd@localhost:5432/ddd_test?sslmode=disable")
	if err != nil {
		t.Skip("postgres not available")
	}
	if err := db.Ping(); err != nil {
		t.Skip("postgres not available")
	}
	return NewPostgresTraceStore(sql.SQLOptions{DB: db}), db
}

func TestPostgresTraceStore_RecordAndGet(t *testing.T) {
	store, db := newTestTraceStore(t)
	defer db.Close()

	ctx := context.Background()
	span := &trace.Span{
		ID:        "span-1",
		TraceID:   "trace-1",
		Type:      trace.SpanTypeCommand,
		Name:      "CreateOrder",
		Status:    trace.SpanStatusSuccess,
		StartedAt: time.Now(),
		Duration:  100 * time.Millisecond,
	}

	err := store.RecordSpan(ctx, span)
	if err != nil {
		t.Fatalf("record failed: %v", err)
	}

	spans, err := store.GetTrace(ctx, "trace-1")
	if err != nil {
		t.Fatalf("get trace failed: %v", err)
	}

	if len(spans) != 1 {
		t.Errorf("expected 1 span, got %d", len(spans))
	}
	if spans[0].Name != "CreateOrder" {
		t.Errorf("expected CreateOrder, got %s", spans[0].Name)
	}
}
```

- [ ] **Step 2: 实现 PostgresTraceStore**

Create `adapter/sql/postgres/trace_store.go`:

```go
package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/ddd-qce/core/adapter/sql"
	"github.com/ddd-qce/core/trace"
)

type PostgresTraceStore struct {
	opts sql.SQLOptions
}

func NewPostgresTraceStore(opts sql.SQLOptions) *PostgresTraceStore {
	return &PostgresTraceStore{opts: opts}
}

func (s *PostgresTraceStore) RecordSpan(ctx context.Context, span *trace.Span) error {
	table := s.opts.prefixed("spans")

	var parentID any
	if span.ParentID != "" {
		parentID = span.ParentID
	}

	_, err := s.opts.DB.ExecContext(ctx,
		fmt.Sprintf("INSERT INTO %s (id, trace_id, parent_id, type, name, status, error, started_at, duration_ns) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)", table),
		span.ID, span.TraceID, parentID, span.Type, span.Name, span.Status,
		nilIfEmpty(span.Error), span.StartedAt, span.Duration.Nanoseconds(),
	)
	return err
}

func (s *PostgresTraceStore) GetTrace(ctx context.Context, traceID string) ([]*trace.Span, error) {
	table := s.opts.prefixed("spans")

	rows, err := s.opts.DB.QueryContext(ctx,
		fmt.Sprintf("SELECT id, trace_id, parent_id, type, name, status, error, started_at, duration_ns FROM %s WHERE trace_id = $1 ORDER BY started_at", table),
		traceID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var spans []*trace.Span
	for rows.Next() {
		span := &trace.Span{}
		var parentID, errorStr sql.NullString
		var durationNs int64

		err := rows.Scan(&span.ID, &span.TraceID, &parentID, &span.Type, &span.Name, &span.Status,
			&errorStr, &span.StartedAt, &durationNs)
		if err != nil {
			return nil, err
		}
		span.Duration = time.Duration(durationNs)

		if parentID.Valid {
			span.ParentID = parentID.String
		}
		if errorStr.Valid {
			span.Error = errorStr.String
		}

		spans = append(spans, span)
	}

	if len(spans) == 0 {
		return nil, fmt.Errorf("trace %s not found", traceID)
	}

	return spans, nil
}

func (s *PostgresTraceStore) ListTraces(ctx context.Context, filter trace.TraceFilter) ([]string, error) {
	table := s.opts.prefixed("spans")

	query := fmt.Sprintf("SELECT DISTINCT trace_id FROM %s WHERE 1=1", table)
	var args []any
	argIdx := 1

	if filter.TraceID != "" {
		query += fmt.Sprintf(" AND trace_id = $%d", argIdx)
		args = append(args, filter.TraceID)
		argIdx++
	}
	if filter.Type != "" {
		query += fmt.Sprintf(" AND type = $%d", argIdx)
		args = append(args, filter.Type)
		argIdx++
	}
	if filter.Status != "" {
		query += fmt.Sprintf(" AND status = $%d", argIdx)
		args = append(args, filter.Status)
		argIdx++
	}
	if !filter.StartTime.IsZero() {
		query += fmt.Sprintf(" AND started_at >= $%d", argIdx)
		args = append(args, filter.StartTime)
		argIdx++
	}
	if !filter.EndTime.IsZero() {
		query += fmt.Sprintf(" AND started_at <= $%d", argIdx)
		args = append(args, filter.EndTime)
		argIdx++
	}
	if filter.NameContains != "" {
		query += fmt.Sprintf(" AND name LIKE $%d", argIdx)
		args = append(args, "%"+filter.NameContains+"%")
		argIdx++
	}

	rows, err := s.opts.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []string
	for rows.Next() {
		var traceID string
		if err := rows.Scan(&traceID); err != nil {
			return nil, err
		}
		result = append(result, traceID)
	}
	return result, nil
}

func nilIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}
```

注意：`ListTraces` 的 SQL 实现天然支持同一 span AND 匹配（WHERE 子句的 AND 作用于同一行），与阶段 1 修复的 TraceFilter 语义一致。

- [ ] **Step 3: Commit**

```bash
git add adapter/sql/postgres/trace_store.go adapter/sql/postgres/trace_store_test.go
git commit -m "feat: add PostgreSQL TraceStore implementation"
```

---

### Task 19: PostgreSQL SnapshotStore 实现

**Files:**
- Create: `adapter/sql/postgres/snapshot_store.go`

- [ ] **Step 1: 实现 SnapshotStore**

Create `adapter/sql/postgres/snapshot_store.go`:

```go
package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/ddd-qce/core/adapter/sql"
)

type SnapshotStore struct {
	opts sql.SQLOptions
}

func NewSnapshotStore(opts sql.SQLOptions) *SnapshotStore {
	return &SnapshotStore{opts: opts}
}

type Snapshot struct {
	AggregateID string
	Version     int
	Payload     json.RawMessage
}

func (s *SnapshotStore) Save(ctx context.Context, snapshot *Snapshot) error {
	table := s.opts.prefixed("snapshots")

	_, err := s.opts.DB.ExecContext(ctx,
		fmt.Sprintf("INSERT INTO %s (aggregate_id, version, payload, created_at) VALUES ($1, $2, $3, NOW()) ON CONFLICT (aggregate_id) DO UPDATE SET version = $2, payload = $3, created_at = NOW()", table),
		snapshot.AggregateID, snapshot.Version, snapshot.Payload,
	)
	return err
}

func (s *SnapshotStore) Load(ctx context.Context, aggregateID string) (*Snapshot, error) {
	table := s.opts.prefixed("snapshots")

	var snap Snapshot
	err := s.opts.DB.QueryRowContext(ctx,
		fmt.Sprintf("SELECT aggregate_id, version, payload FROM %s WHERE aggregate_id = $1", table),
		aggregateID,
	).Scan(&snap.AggregateID, &snap.Version, &snap.Payload)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &snap, nil
}
```

- [ ] **Step 2: Commit**

```bash
git add adapter/sql/postgres/snapshot_store.go
git commit -m "feat: add PostgreSQL SnapshotStore implementation"
```

---

### Task 20: 阶段 3 全量测试验证

- [ ] **Step 1: 运行全量测试**

Run: `go test ./... -v`
Expected: ALL PASS (PostgreSQL 测试在无实例时 skip)

- [ ] **Step 2: 运行 go vet**

Run: `go vet ./...`
Expected: 无警告

- [ ] **Step 3: 确认编译通过**

Run: `go build ./...`
Expected: 无错误

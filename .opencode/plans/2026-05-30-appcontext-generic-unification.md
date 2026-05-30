# AppContext[T] 统一应用组装模式 实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 废弃 `app.NewApp(opts...)` 选项式 API，统一为 `app.AppContext[T]` 泛型模式，让框架自身和示例项目使用同一种组装方式。

**Architecture:** `AppContext[T]` 泛型结构体，框架核心字段（CmdBus/QueryBus/EventBus/Chain/Backend）直接挂在结构体上，用户自定义字段通过泛型参数 `T` 挂在 `Custom` 字段上，保持类型安全。提供细粒度 `WireFunc[T]` 支持可组合的组装。同步删除旧的 `app.NewApp` 及所有 AppOption。

**Tech Stack:** Go 1.22+ 泛型

---

## 文件变更清单

| 文件 | 操作 | 职责 |
|------|------|------|
| `app/app.go` | 重写 | `AppContext[T]`、`WireFunc[T]`、`NewAppContext`、生命周期方法 |
| `app/signal.go` | 修改 | `WaitForSignal` 适配泛型 |
| `app/lifecycle.go` | 不变 | `Lifecycle`、`LifecycleFunc` 不变 |
| `app/wire_backend.go` | 新增 | `WithAutoBackend`、`WithBackend` WireFunc |
| `app/wire_bus.go` | 新增 | `WithBuses`、`WithAutoBuses`、`WithCommandBus`、`WithQueryBus`、`WithEventBus` WireFunc |
| `app/wire_aspect.go` | 新增 | `WithAspectChain`、`WithAspect`、`WithCommandAspect`、`WithDefaultAspects` WireFunc |
| `app/wire_handler.go` | 新增 | `WithCommandHandlers`、`WithQueryHandlers`、`WithEventSubscriptions` WireFunc |
| `app/wire_config.go` | 新增 | `WithConfigFile`、`WithLogger`、`WithMetrics` WireFunc |
| `app/app_test.go` | 重写 | 测试新的泛型 API |
| `exampleapp/infrastructure/types.go` | 新增 | `AppCustom` 类型定义、`AppContext` 类型别名 |
| `exampleapp/infrastructure/wire.go` | 重写 | 迁移到 `AppContext[T]` 模式 |
| `exampleapp/main.go` | 修改 | 确认 import 路径兼容 |
| `exampleapp/interfaces/http/server.go` | 修改 | 适配 `AppContext` 字段访问变化 |
| `exampleapp/interfaces/http/handlers.go` | 修改 | 适配 `AppContext` 字段访问变化 |
| `exampleapp/interfaces/http/e2e_test.go` | 修改 | 适配 `AppContext` 字段访问变化 |
| `exampleapp/interfaces/http/http_test.go` | 修改 | 适配 `AppContext` 字段访问变化 |
| `exampleapp/integration/integration_test.go` | 修改 | 适配 `AppContext` 字段访问变化 |
| `README.md` | 修改 | 更新 API 示例 |

---

### Task 1: 重写 `app/app.go` — AppContext[T] 核心结构

**Files:**
- Rewrite: `app/app.go`
- Modify: `app/signal.go`

- [ ] **Step 1: 重写 app/app.go**

删除所有 `AppOption`、`optionPhase`、`ConfigOption`、`InitOption`、`NewApp`、`ensureChain`、以及所有 `With*` 函数。替换为：

```go
package app

import (
	"context"
	"fmt"

	"github.com/ddd-qce/core/aspect"
	"github.com/ddd-qce/core/cqrs/command"
	"github.com/ddd-qce/core/cqrs/event"
	"github.com/ddd-qce/core/cqrs/query"
	"github.com/ddd-qce/core/infra"
)

type WireFunc[T any] func(ctx context.Context, app *AppContext[T]) error

type AppContext[T any] struct {
	CmdBus   command.CommandBus
	QueryBus query.QueryBus
	EventBus event.EventBus
	Chain    *aspect.AspectChain
	Backend  *infra.Backend
	Custom   T

	lifecycles []Lifecycle
	cleanup    []func() error
}

func NewAppContext[T any](custom T) *AppContext[T] {
	return &AppContext[T]{Custom: custom}
}

func (a *AppContext[T]) Wire(ctx context.Context, wires ...WireFunc[T]) error {
	for _, wire := range wires {
		if err := wire(ctx, a); err != nil {
			return err
		}
	}
	return nil
}

func (a *AppContext[T]) RegisterLifecycle(l Lifecycle) {
	a.lifecycles = append(a.lifecycles, l)
}

func (a *AppContext[T]) OnClose(fn func() error) {
	a.cleanup = append(a.cleanup, fn)
}

func (a *AppContext[T]) Close(ctx context.Context) error {
	var errs []error
	for _, l := range a.lifecycles {
		if err := l.Shutdown(ctx); err != nil {
			errs = append(errs, err)
		}
		if ctx.Err() != nil {
			break
		}
	}
	for _, fn := range a.cleanup {
		if err := fn(); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("shutdown errors: %v", errs)
	}
	return nil
}
```

- [ ] **Step 2: 修改 app/signal.go 适配泛型**

```go
package app

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func (a *AppContext[T]) WaitForSignal(timeout time.Duration) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	sig := <-sigCh
	log.Printf("received %v, shutting down...", sig)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if err := a.Close(ctx); err != nil {
		log.Printf("shutdown error: %v", err)
	}
}
```

- [ ] **Step 3: 运行编译确认核心包结构正确**

Run: `go vet ./app/...`
Expected: 编译失败（缺少 WireFunc 预设函数，后续 Task 补充）

---

### Task 2: 新增 WireFunc 预设函数

**Files:**
- Create: `app/wire_backend.go`
- Create: `app/wire_bus.go`
- Create: `app/wire_aspect.go`
- Create: `app/wire_handler.go`
- Create: `app/wire_config.go`

- [ ] **Step 1: 创建 app/wire_backend.go**

```go
package app

import (
	"context"
	"fmt"

	"github.com/ddd-qce/core/config"
	"github.com/ddd-qce/core/infra"
)

func WithAutoBackend[T any]() WireFunc[T] {
	return func(ctx context.Context, a *AppContext[T]) error {
		storeCfg := config.ResolveStoreConfig()
		backend, err := infra.NewBackendFromConfig(storeCfg)
		if err != nil {
			return fmt.Errorf("auto backend: %w", err)
		}
		a.Backend = backend
		a.OnClose(backend.Close)
		return nil
	}
}

func WithBackend[T any](backend *infra.Backend) WireFunc[T] {
	return func(ctx context.Context, a *AppContext[T]) error {
		a.Backend = backend
		return nil
	}
}
```

- [ ] **Step 2: 创建 app/wire_bus.go**

```go
package app

import (
	"context"
	"errors"

	"github.com/ddd-qce/core/aspect"
	"github.com/ddd-qce/core/cqrs/command"
	"github.com/ddd-qce/core/cqrs/event"
	"github.com/ddd-qce/core/cqrs/query"
)

func WithBuses[T any](cmdBus command.CommandBus, queryBus query.QueryBus, eventBus event.EventBus) WireFunc[T] {
	return func(ctx context.Context, a *AppContext[T]) error {
		a.CmdBus = cmdBus
		a.QueryBus = queryBus
		a.EventBus = eventBus
		return nil
	}
}

func WithCommandBus[T any](cmdBus command.CommandBus) WireFunc[T] {
	return func(ctx context.Context, a *AppContext[T]) error {
		a.CmdBus = cmdBus
		return nil
	}
}

func WithQueryBus[T any](queryBus query.QueryBus) WireFunc[T] {
	return func(ctx context.Context, a *AppContext[T]) error {
		a.QueryBus = queryBus
		return nil
	}
}

func WithEventBus[T any](eventBus event.EventBus) WireFunc[T] {
	return func(ctx context.Context, a *AppContext[T]) error {
		a.EventBus = eventBus
		return nil
	}
}

func WithAutoBuses[T any]() WireFunc[T] {
	return func(ctx context.Context, a *AppContext[T]) error {
		if a.Backend == nil || a.Backend.BusFactory == nil {
			return errors.New("WithAutoBuses requires Backend with BusFactory")
		}
		a.Chain = ensureChain(a.Chain)
		a.CmdBus = a.Backend.BusFactory.NewCommandBus(a.Chain)
		a.QueryBus = a.Backend.BusFactory.NewQueryBus(a.Chain)
		a.EventBus = a.Backend.BusFactory.NewEventBus(a.Chain)
		return nil
	}
}

func ensureChain(chain *aspect.AspectChain) *aspect.AspectChain {
	if chain == nil {
		return aspect.NewAspectChain()
	}
	return chain
}
```

- [ ] **Step 3: 创建 app/wire_aspect.go**

```go
package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/ddd-qce/core/aspect"
	"github.com/ddd-qce/core/aspect/builtin"
	"github.com/ddd-qce/core/config"
)

func WithAspectChain[T any](chain *aspect.AspectChain) WireFunc[T] {
	return func(ctx context.Context, a *AppContext[T]) error {
		a.Chain = chain
		return nil
	}
}

func WithAspect[T any](asp aspect.Aspect) WireFunc[T] {
	return func(ctx context.Context, a *AppContext[T]) error {
		a.Chain = ensureChain(a.Chain)
		a.Chain.RegisterAspect(asp)
		return nil
	}
}

func WithCommandAspect[T any](asp aspect.CommandAspect) WireFunc[T] {
	return func(ctx context.Context, a *AppContext[T]) error {
		a.Chain = ensureChain(a.Chain)
		a.Chain.RegisterCommandAspect(asp)
		return nil
	}
}

func WithDefaultAspects[T any]() WireFunc[T] {
	return func(ctx context.Context, a *AppContext[T]) error {
		a.Chain = ensureChain(a.Chain)

		cfg, ok := config.ConfigFromContext(ctx)
		if !ok {
			cfg = config.DefaultConfig()
		}

		if cfg.Aspect.EnableLogging {
			a.Chain.RegisterAspect(builtin.NewLoggingAspect(builtin.NewStdLogger()))
		}
		if cfg.Aspect.EnableTracing {
			if a.Backend == nil || a.Backend.TraceStore == nil {
				return errors.New("tracing enabled but Backend.TraceStore is nil: use WithAutoBackend or WithBackend")
			}
			a.Chain.RegisterAspect(builtin.NewTracingAspect(a.Backend.TraceStore))
		}
		if cfg.Aspect.EnableMetrics {
			a.Chain.RegisterAspect(builtin.NewMetricsAspect(builtin.NewInMemoryMetricsRecorder()))
		}
		if cfg.Aspect.EnableTransaction {
			if a.Backend == nil || a.Backend.TransactionManager == nil {
				return errors.New("transaction enabled but Backend.TransactionManager is nil: use WithAutoBackend or WithBackend")
			}
			ta, err := builtin.NewTransactionAspect(a.Backend.TransactionManager)
			if err != nil {
				return fmt.Errorf("create transaction aspect: %w", err)
			}
			a.Chain.RegisterCommandAspect(ta)
		}

		return nil
	}
}
```

注意：`WithDefaultAspects` 改为从 context 读取 Config（而非 App 字段），因为泛型模式下 `AppContext[T]` 不再有固定 `Config` 字段。简单场景通过 `config.ContextWithConfig(ctx, cfg)` 传入。

- [ ] **Step 4: 创建 app/wire_handler.go**

```go
package app

import (
	"context"
	"fmt"
)

func WithCommandHandlers[T any](handlers ...any) WireFunc[T] {
	return func(ctx context.Context, a *AppContext[T]) error {
		if a.CmdBus == nil {
			return fmt.Errorf("WithCommandHandlers requires CmdBus: use WithCommandBus or WithAutoBuses first")
		}
		for _, h := range handlers {
			if err := a.CmdBus.RegisterHandler(h); err != nil {
				return fmt.Errorf("register command handler %T: %w", h, err)
			}
		}
		return nil
	}
}

func WithQueryHandlers[T any](handlers ...any) WireFunc[T] {
	return func(ctx context.Context, a *AppContext[T]) error {
		if a.QueryBus == nil {
			return fmt.Errorf("WithQueryHandlers requires QueryBus: use WithQueryBus or WithAutoBuses first")
		}
		for _, h := range handlers {
			if err := a.QueryBus.RegisterHandler(h); err != nil {
				return fmt.Errorf("register query handler %T: %w", h, err)
			}
		}
		return nil
	}
}

func WithEventSubscriptions[T any](subs ...any) WireFunc[T] {
	return func(ctx context.Context, a *AppContext[T]) error {
		if a.EventBus == nil {
			return fmt.Errorf("WithEventSubscriptions requires EventBus: use WithEventBus or WithAutoBuses first")
		}
		for _, s := range subs {
			if err := a.EventBus.SubscribeHandler(s); err != nil {
				return fmt.Errorf("subscribe event handler %T: %w", s, err)
			}
		}
		return nil
	}
}
```

- [ ] **Step 5: 创建 app/wire_config.go**

```go
package app

import (
	"context"
	"fmt"

	"github.com/ddd-qce/core/aspect/builtin"
	"github.com/ddd-qce/core/config"
)

func WithConfigFile[T any](path string) WireFunc[T] {
	return func(ctx context.Context, a *AppContext[T]) error {
		loader := config.NewConfigLoader()
		_, err := loader.Load(path)
		if err != nil {
			return fmt.Errorf("load config from %s: %w", path, err)
		}
		return nil
	}
}

func WithLogger[T any](logger builtin.Logger) WireFunc[T] {
	return func(ctx context.Context, a *AppContext[T]) error {
		a.Chain = ensureChain(a.Chain)
		a.Chain.RegisterAspect(builtin.NewLoggingAspect(logger))
		return nil
	}
}

func WithMetrics[T any](recorder builtin.MetricsRecorder) WireFunc[T] {
	return func(ctx context.Context, a *AppContext[T]) error {
		a.Chain = ensureChain(a.Chain)
		a.Chain.RegisterAspect(builtin.NewMetricsAspect(recorder))
		return nil
	}
}
```

- [ ] **Step 6: 运行编译确认**

Run: `go build ./app/...`
Expected: PASS

---

### Task 3: 重写 `app/app_test.go`

**Files:**
- Rewrite: `app/app_test.go`

- [ ] **Step 1: 重写测试文件**

测试需要覆盖：
1. `NewAppContext` 创建
2. `Wire` 执行 WireFunc
3. `Wire` 错误传播
4. `RegisterLifecycle` + `Close`
5. `OnClose` + `Close`
6. `Close` 生命周期错误聚合
7. `Close` 取消 context 提前终止
8. `WithBuses` / `WithCommandBus` / `WithQueryBus` / `WithEventBus`
9. `WithAutoBuses` 需要 Backend
10. `WithAutoBackend` + `WithAutoBuses` + `WithCommandHandlers` 端到端
11. `WithAspectChain` / `WithAspect` / `WithCommandAspect`
12. `WithDefaultAspects` 错误场景
13. `WithCommandHandlers` / `WithQueryHandlers` / `WithEventSubscriptions` 无 Bus 错误

```go
package app

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/ddd-qce/core/aspect"
	"github.com/ddd-qce/core/aspect/builtin"
	"github.com/ddd-qce/core/config"
	"github.com/ddd-qce/core/cqrs/command"
	"github.com/ddd-qce/core/cqrs/event"
	"github.com/ddd-qce/core/cqrs/query"
	"github.com/ddd-qce/core/infra"
)

type testCustom struct {
	Label string
}

type testCommand struct{ command.BaseCommand }
type testCommandHandler struct{}

func (h *testCommandHandler) Handle(ctx context.Context, cmd testCommand) (string, error) {
	return "cmd-result", nil
}

type testQuery struct{ query.BaseQuery }
type testQueryHandler struct{}

func (h *testQueryHandler) Handle(ctx context.Context, q testQuery) (string, error) {
	return "query-result", nil
}

type testEvent struct{ event.BaseEvent }
type testEventHandler struct {
	called bool
}

func (h *testEventHandler) Handle(ctx context.Context, evt testEvent) error {
	h.called = true
	return nil
}

func testBusFactory() *infra.BusFactory {
	return infra.NewMemoryBusFactory()
}

func TestNewAppContext(t *testing.T) {
	app := NewAppContext(testCustom{Label: "test"})
	if app.Custom.Label != "test" {
		t.Errorf("Custom.Label = %q, want %q", app.Custom.Label, "test")
	}
}

func TestWire_Success(t *testing.T) {
	app := NewAppContext(testCustom{})
	err := app.Wire(context.Background(),
		func(ctx context.Context, a *AppContext[testCustom]) error {
			a.Custom.Label = "wired"
			return nil
		},
	)
	if err != nil {
		t.Fatalf("Wire: %v", err)
	}
	if app.Custom.Label != "wired" {
		t.Errorf("Custom.Label = %q, want %q", app.Custom.Label, "wired")
	}
}

func TestWire_Error(t *testing.T) {
	app := NewAppContext(testCustom{})
	err := app.Wire(context.Background(),
		func(ctx context.Context, a *AppContext[testCustom]) error {
			return errors.New("wire-failed")
		},
	)
	if err == nil {
		t.Fatal("expected error from Wire")
	}
	if !strings.Contains(err.Error(), "wire-failed") {
		t.Errorf("error = %v, want wire-failed", err)
	}
}

func TestClose_NoLifecycles(t *testing.T) {
	app := NewAppContext(testCustom{})
	if err := app.Close(context.Background()); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestClose_WithLifecycles(t *testing.T) {
	app := NewAppContext(testCustom{})
	called := false
	app.RegisterLifecycle(LifecycleFunc(func(ctx context.Context) error {
		called = true
		return nil
	}))
	if err := app.Close(context.Background()); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if !called {
		t.Error("lifecycle Shutdown was not called")
	}
}

func TestClose_CleanupFunctions(t *testing.T) {
	app := NewAppContext(testCustom{})
	cleanupCalled := false
	app.OnClose(func() error {
		cleanupCalled = true
		return nil
	})
	if err := app.Close(context.Background()); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if !cleanupCalled {
		t.Error("cleanup function was not called")
	}
}

func TestClose_LifecycleError(t *testing.T) {
	app := NewAppContext(testCustom{})
	app.RegisterLifecycle(LifecycleFunc(func(ctx context.Context) error {
		return errors.New("shutdown-err-1")
	}))
	app.RegisterLifecycle(LifecycleFunc(func(ctx context.Context) error {
		return errors.New("shutdown-err-2")
	}))
	err := app.Close(context.Background())
	if err == nil {
		t.Fatal("expected error from Close with failing lifecycles")
	}
}

func TestClose_CancelledContext(t *testing.T) {
	app := NewAppContext(testCustom{})
	var secondCalled bool
	app.RegisterLifecycle(LifecycleFunc(func(ctx context.Context) error {
		return nil
	}))
	app.RegisterLifecycle(LifecycleFunc(func(ctx context.Context) error {
		secondCalled = true
		return nil
	}))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_ = app.Close(ctx)
	if secondCalled {
		t.Error("second lifecycle should not be called when context is already cancelled")
	}
}

func TestWithBuses(t *testing.T) {
	factory := testBusFactory()
	cmdBus := factory.NewCommandBus(aspect.NewAspectChain())
	queryBus := factory.NewQueryBus(aspect.NewAspectChain())
	eventBus := factory.NewEventBus(aspect.NewAspectChain())

	app := NewAppContext(testCustom{})
	err := app.Wire(context.Background(),
		WithBuses[testCustom](cmdBus, queryBus, eventBus),
	)
	if err != nil {
		t.Fatalf("Wire WithBuses: %v", err)
	}
	if app.CmdBus != cmdBus {
		t.Error("CmdBus not set correctly via WithBuses")
	}
	if app.QueryBus != queryBus {
		t.Error("QueryBus not set correctly via WithBuses")
	}
	if app.EventBus != eventBus {
		t.Error("EventBus not set correctly via WithBuses")
	}
}

func TestWithCommandBus(t *testing.T) {
	cmdBus := testBusFactory().NewCommandBus(nil)
	app := NewAppContext(testCustom{})
	err := app.Wire(context.Background(),
		WithCommandBus[testCustom](cmdBus),
	)
	if err != nil {
		t.Fatalf("Wire WithCommandBus: %v", err)
	}
	if app.CmdBus != cmdBus {
		t.Error("CmdBus not set correctly via WithCommandBus")
	}
}

func TestWithQueryBus(t *testing.T) {
	queryBus := testBusFactory().NewQueryBus(nil)
	app := NewAppContext(testCustom{})
	err := app.Wire(context.Background(),
		WithQueryBus[testCustom](queryBus),
	)
	if err != nil {
		t.Fatalf("Wire WithQueryBus: %v", err)
	}
	if app.QueryBus != queryBus {
		t.Error("QueryBus not set correctly via WithQueryBus")
	}
}

func TestWithEventBus(t *testing.T) {
	eventBus := testBusFactory().NewEventBus(nil)
	app := NewAppContext(testCustom{})
	err := app.Wire(context.Background(),
		WithEventBus[testCustom](eventBus),
	)
	if err != nil {
		t.Fatalf("Wire WithEventBus: %v", err)
	}
	if app.EventBus != eventBus {
		t.Error("EventBus not set correctly via WithEventBus")
	}
}

func TestWithAutoBuses_RequiresBackend(t *testing.T) {
	app := NewAppContext(testCustom{})
	err := app.Wire(context.Background(),
		WithAutoBuses[testCustom](),
	)
	if err == nil {
		t.Fatal("expected error when WithAutoBuses is called without Backend")
	}
}

func TestWithAutoBuses_WithBackend(t *testing.T) {
	backend := infra.NewMemoryBackend()
	app := NewAppContext(testCustom{})
	err := app.Wire(context.Background(),
		WithBackend[testCustom](backend),
		WithAutoBuses[testCustom](),
	)
	if err != nil {
		t.Fatalf("Wire: %v", err)
	}
	if app.CmdBus == nil {
		t.Error("CmdBus should be created by WithAutoBuses")
	}
	if app.QueryBus == nil {
		t.Error("QueryBus should be created by WithAutoBuses")
	}
	if app.EventBus == nil {
		t.Error("EventBus should be created by WithAutoBuses")
	}
}

func TestWithCommandHandlers_RequiresCmdBus(t *testing.T) {
	app := NewAppContext(testCustom{})
	err := app.Wire(context.Background(),
		WithCommandHandlers[testCustom](&testCommandHandler{}),
	)
	if err == nil {
		t.Fatal("expected error when WithCommandHandlers is called without CmdBus")
	}
}

func TestWithCommandHandlers_WithBus(t *testing.T) {
	factory := testBusFactory()
	cmdBus := factory.NewCommandBus(aspect.NewAspectChain())
	app := NewAppContext(testCustom{})
	err := app.Wire(context.Background(),
		WithCommandBus[testCustom](cmdBus),
		WithCommandHandlers[testCustom](&testCommandHandler{}),
	)
	if err != nil {
		t.Fatalf("Wire: %v", err)
	}
	result, err := command.Dispatch[testCommand, string](context.Background(), app.CmdBus, testCommand{})
	if err != nil {
		t.Fatalf("dispatch command: %v", err)
	}
	if result != "cmd-result" {
		t.Errorf("command result = %q, want %q", result, "cmd-result")
	}
}

func TestWithQueryHandlers_RequiresQueryBus(t *testing.T) {
	app := NewAppContext(testCustom{})
	err := app.Wire(context.Background(),
		WithQueryHandlers[testCustom](&testQueryHandler{}),
	)
	if err == nil {
		t.Fatal("expected error when WithQueryHandlers is called without QueryBus")
	}
}

func TestWithQueryHandlers_WithBus(t *testing.T) {
	factory := testBusFactory()
	queryBus := factory.NewQueryBus(aspect.NewAspectChain())
	app := NewAppContext(testCustom{})
	err := app.Wire(context.Background(),
		WithQueryBus[testCustom](queryBus),
		WithQueryHandlers[testCustom](&testQueryHandler{}),
	)
	if err != nil {
		t.Fatalf("Wire: %v", err)
	}
	result, err := query.Dispatch[testQuery, string](context.Background(), app.QueryBus, testQuery{})
	if err != nil {
		t.Fatalf("dispatch query: %v", err)
	}
	if result != "query-result" {
		t.Errorf("query result = %q, want %q", result, "query-result")
	}
}

func TestWithEventSubscriptions_RequiresEventBus(t *testing.T) {
	app := NewAppContext(testCustom{})
	err := app.Wire(context.Background(),
		WithEventSubscriptions[testCustom](&testEventHandler{}),
	)
	if err == nil {
		t.Fatal("expected error when WithEventSubscriptions is called without EventBus")
	}
}

func TestWithEventSubscriptions_WithBus(t *testing.T) {
	factory := testBusFactory()
	eventBus := factory.NewEventBus(aspect.NewAspectChain())
	handler := &testEventHandler{}
	app := NewAppContext(testCustom{})
	err := app.Wire(context.Background(),
		WithEventBus[testCustom](eventBus),
		WithEventSubscriptions[testCustom](handler),
	)
	if err != nil {
		t.Fatalf("Wire: %v", err)
	}
	evt := testEvent{event.NewDomainEvent("test-aggregate")}
	if err := event.Dispatch(context.Background(), app.EventBus, evt); err != nil {
		t.Fatalf("dispatch event: %v", err)
	}
	if !handler.called {
		t.Error("event handler was not called")
	}
}

func TestWithAspectChain(t *testing.T) {
	chain := aspect.NewAspectChain()
	app := NewAppContext(testCustom{})
	err := app.Wire(context.Background(),
		WithAspectChain[testCustom](chain),
	)
	if err != nil {
		t.Fatalf("Wire: %v", err)
	}
	if app.Chain != chain {
		t.Error("Chain not set correctly via WithAspectChain")
	}
}

func TestWithAspect(t *testing.T) {
	app := NewAppContext(testCustom{})
	logger := builtin.NewStdLogger()
	err := app.Wire(context.Background(),
		WithAspect[testCustom](builtin.NewLoggingAspect(logger)),
	)
	if err != nil {
		t.Fatalf("Wire: %v", err)
	}
	if app.Chain == nil {
		t.Error("Chain should be created by WithAspect")
	}
}

func TestWithDefaultAspects_NilBackend_ReturnsError(t *testing.T) {
	app := NewAppContext(testCustom{})
	err := app.Wire(context.Background(),
		WithDefaultAspects[testCustom](),
	)
	if err == nil {
		t.Fatal("expected error when WithDefaultAspects is called without Backend")
	}
}

func TestWithDefaultAspects_TracingDisabled_NoTraceStoreRequired(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Aspect.EnableTracing = false
	cfg.Aspect.EnableTransaction = false
	ctx := config.ContextWithConfig(context.Background(), cfg)

	app := NewAppContext(testCustom{})
	err := app.Wire(ctx,
		WithDefaultAspects[testCustom](),
	)
	if err != nil {
		t.Fatalf("WithDefaultAspects with tracing and transaction disabled should not error, got: %v", err)
	}
	if app.Chain == nil {
		t.Error("expected AspectChain to be created")
	}
}

func TestEndToEnd_AutoBackendWithBusesAndHandlers(t *testing.T) {
	backend := infra.NewMemoryBackend()
	app := NewAppContext(testCustom{})
	err := app.Wire(context.Background(),
		WithBackend[testCustom](backend),
		WithAutoBuses[testCustom](),
		WithCommandHandlers[testCustom](&testCommandHandler{}),
		WithQueryHandlers[testCustom](&testQueryHandler{}),
	)
	if err != nil {
		t.Fatalf("Wire: %v", err)
	}
	if app.CmdBus == nil {
		t.Fatal("CmdBus should be created")
	}
	if app.QueryBus == nil {
		t.Fatal("QueryBus should be created")
	}

	cmdResult, err := command.Dispatch[testCommand, string](context.Background(), app.CmdBus, testCommand{})
	if err != nil {
		t.Fatalf("dispatch command: %v", err)
	}
	if cmdResult != "cmd-result" {
		t.Errorf("command result = %q, want %q", cmdResult, "cmd-result")
	}

	queryResult, err := query.Dispatch[testQuery, string](context.Background(), app.QueryBus, testQuery{})
	if err != nil {
		t.Fatalf("dispatch query: %v", err)
	}
	if queryResult != "query-result" {
		t.Errorf("query result = %q, want %q", queryResult, "query-result")
	}
}

func TestLifecycleFunc(t *testing.T) {
	called := false
	fn := LifecycleFunc(func(ctx context.Context) error {
		called = true
		return nil
	})
	if err := fn.Shutdown(context.Background()); err != nil {
		t.Fatalf("LifecycleFunc.Shutdown: %v", err)
	}
	if !called {
		t.Error("LifecycleFunc did not call the underlying function")
	}
}
```

- [ ] **Step 2: 运行测试确认通过**

Run: `go test ./app/... -v`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add app/
git commit -m "refactor: replace app.NewApp with AppContext[T] + WireFunc[T]"
```

---

### Task 4: 迁移 exampleapp — 新增 types.go + 重写 wire.go

**Files:**
- Create: `exampleapp/infrastructure/types.go`
- Rewrite: `exampleapp/infrastructure/wire.go`

这是最关键的 Task。exampleapp 不再自己定义 `AppContext` 结构体和方法，而是使用框架的 `app.AppContext[T]`。

**字段迁移映射：**

| 旧 AppContext 字段 | 新位置 |
|---|---|
| `Config *Config` | `Custom.Config` |
| `Chain *aspect.AspectChain` | 框架核心 `Chain` |
| `CmdBus command.CommandBus` | 框架核心 `CmdBus` |
| `QueryBus query.QueryBus` | 框架核心 `QueryBus` |
| `EventBus cqrsevent.EventBus` | 框架核心 `EventBus` |
| `Backend *infra.Backend` | 框架核心 `Backend` |
| `JobManager *jobmemory.JobManager` | `Custom.JobManager` |
| `DDDViewer *observability.DDDViewer` | `Custom.DDDViewer` |
| `OrderRepo` | `Custom.OrderRepo` |
| `EventSourcedRepo` | `Custom.EventSourcedRepo` |
| `EventStore` | `Custom.EventStore` |
| `Inventory` | `Custom.Inventory` |
| `MetricsRecorder` | `Custom.MetricsRecorder` |
| `TxManager` | `Custom.TxManager` |
| `store *StoreComponents` | `Custom.Store` |
| `lifecycles []app.Lifecycle` | 框架管理 |
| `Close` | 框架提供 |
| `WaitForSignal` | 框架提供 |
| `RegisterLifecycle` | 框架提供 |
| `Store()` | `Custom.Store` |

- [ ] **Step 1: 创建 exampleapp/infrastructure/types.go**

```go
package infrastructure

import (
	"github.com/ddd-qce/core/app"
	cqrsevent "github.com/ddd-qce/core/cqrs/event"
	domainevent "github.com/ddd-qce/core/domain/event"
	jobmemory "github.com/ddd-qce/core/job/memory"
	"github.com/ddd-qce/core/observability"
	inventorydomain "github.com/ddd-qce/exampleapp/ddd/inventory/domain"
	orderrepo "github.com/ddd-qce/exampleapp/ddd/order/repository"
)

type AppCustom struct {
	Config           *Config
	Store            *StoreComponents
	OrderRepo        orderrepo.OrderRepositoryAdapter
	EventSourcedRepo *orderrepo.OrderEventSourcedRepository
	EventStore       cqrsevent.EventSourceStore[domainevent.Event]
	Inventory        *inventorydomain.Inventory
	JobManager       *jobmemory.JobManager
	DDDViewer        *observability.DDDViewer
	MetricsRecorder  *AppMetricsRecorder
	TxManager        *AppTransactionManager
}

type AppContext = app.AppContext[AppCustom]
```

- [ ] **Step 2: 重写 exampleapp/infrastructure/wire.go**

关键变化：
1. 不再定义 `AppContext` 结构体和方法
2. 使用 `app.NewAppContext(AppCustom{...})` 创建
3. 框架核心字段直接赋值（CmdBus/QueryBus/EventBus/Chain/Backend）
4. 自定义字段通过 `Custom` 访问
5. `Close`/`WaitForSignal`/`RegisterLifecycle` 由框架提供
6. `OnClose` 注册 DB cleanup
7. `Store()` 方法改为直接访问 `Custom.Store`

```go
package infrastructure

import (
	"context"
	"fmt"

	"github.com/ddd-qce/core/app"
	"github.com/ddd-qce/core/aspect"
	"github.com/ddd-qce/core/aspect/builtin"
	"github.com/ddd-qce/core/cqrs/command"
	domainevent "github.com/ddd-qce/core/domain/event"
	cqrsevent "github.com/ddd-qce/core/cqrs/event"
	"github.com/ddd-qce/core/cqrs/query"
	commandmemory "github.com/ddd-qce/core/cqrs/impl/memory"
	eventmemory "github.com/ddd-qce/core/cqrs/impl/memory"
	querymemory "github.com/ddd-qce/core/cqrs/impl/memory"
	jobcore "github.com/ddd-qce/core/job/core"
	jobmemory "github.com/ddd-qce/core/job/memory"
	"github.com/ddd-qce/core/observability"
	observabilitypg "github.com/ddd-qce/core/observability/pg"
	inventorydomain "github.com/ddd-qce/exampleapp/ddd/inventory/domain"
	inventorywire "github.com/ddd-qce/exampleapp/ddd/inventory/wire"
	orderrepo "github.com/ddd-qce/exampleapp/ddd/order/repository"
	orderwire "github.com/ddd-qce/exampleapp/ddd/order/wire"
)

func WireApp() *AppContext {
	app, err := WireAppWithConfig(LoadConfig())
	if err != nil {
		panic(err)
	}
	return app
}

func WireAppWithConfig(cfg *Config) (*AppContext, error) {
	store, err := NewProvider(cfg)
	if err != nil {
		return nil, err
	}
	recoveryEnabled := cfg.StoreType == StoreTypePostgreSQL
	return WireAppWithStore(store, cfg, recoveryEnabled)
}

func WireAppWithStore(store *StoreComponents, cfg *Config, recoveryEnabled bool) (*AppContext, error) {
	backend := store.Backend

	logger := NewAppLogger()
	metricsRecorder := NewAppMetricsRecorder()
	txManager := NewAppTransactionManager(backend.TransactionManager)

	chain := aspect.NewAspectChain()
	chain.RegisterAspect(builtin.NewTracingAspect(backend.TraceStore))
	chain.RegisterAspect(builtin.NewLoggingAspect(logger))

	statsCollector := observability.NewStatsCollector()
	composedMetrics := observability.ComposeMetrics(statsCollector, metricsRecorder)
	chain.RegisterAspect(builtin.NewMetricsAspect(composedMetrics))

	var msgStoreForReader builtin.MessageStore
	if store.DB != nil {
		chain.RegisterAspect(builtin.NewPersistenceAspect(backend.MessageStore))
		msgStoreForReader = backend.MessageStore
	} else {
		memMsgStore := observability.NewInMemoryMessageStore()
		chain.RegisterAspect(builtin.NewPersistenceAspect(memMsgStore))
		msgStoreForReader = memMsgStore
	}

	ta, err := builtin.NewTransactionAspect(txManager)
	if err != nil {
		return nil, fmt.Errorf("create transaction aspect: %w", err)
	}
	chain.RegisterCommandAspect(ta)

	cmdBus := commandmemory.NewCommandBus(commandmemory.WithCommandBusAspectChain(chain))
	queryBus := querymemory.NewQueryBus(querymemory.WithQueryBusAspectChain(chain))
	eventBus := eventmemory.NewEventBus(eventmemory.WithBusAspectChain(chain))

	var orderRepo orderrepo.OrderRepositoryAdapter
	var inventory *inventorydomain.Inventory

	if store.DB != nil {
		orderRepo = orderrepo.NewPgOrderRepository(store.DB)
		pgProductStore := inventorydomain.NewPgProductStore(store.DB)
		inventory = inventorydomain.NewInventory(pgProductStore)
		ctx := context.Background()
		if err := inventory.InitFromStore(ctx); err != nil {
			return nil, fmt.Errorf("init inventory from store: %w", err)
		}
		if len(inventory.GetAll()) == 0 {
			if err := inventorydomain.SeedProducts(ctx, pgProductStore, inventorydomain.DefaultProducts()); err != nil {
				return nil, fmt.Errorf("seed products: %w", err)
			}
			inventory.InitFromStore(ctx)
		}
	} else {
		orderRepo = store.OrderRepo
		inventory = inventorydomain.NewInventoryWithSeed(nil)
	}

	eventStore := store.EventStore

	eventSourcedRepo := orderrepo.NewOrderEventSourcedRepository(eventStore, eventBus, orderRepo)

	if err := orderwire.WireOrder(chain, cmdBus, queryBus, eventBus, eventSourcedRepo); err != nil {
		return nil, err
	}
	if err := inventorywire.WireInventory(chain, cmdBus, queryBus, eventBus, inventory); err != nil {
		return nil, err
	}

	jobManagerOpts := []jobmemory.JobManagerOption{
		jobmemory.WithStoreErrorHandler(func(ctx context.Context, storeErr *jobcore.StoreError) {
			logger.Error("job store error: %v", storeErr)
		}),
	}
	if recoveryEnabled {
		jobManagerOpts = append(jobManagerOpts, jobmemory.WithRecovery())
	}
	jobManager := jobmemory.NewJobManager(backend.JobStore, cmdBus, jobManagerOpts...)

	typeRegistry := observability.NewTypePrototypeRegistry()
	RegisterAppTypes(typeRegistry)

	var dddViewer *observability.DDDViewer
	dddViewerOpts := []observability.DDDViewerOption{
		observability.WithDDDViewerStatsCollector(statsCollector),
		observability.WithDDDViewerTraceStore(backend.TraceStore),
		observability.WithDDDViewerJobManager(jobManager),
		observability.WithDDDViewerTypeRegistry(typeRegistry),
		observability.WithDDDViewerBaseURL("http://localhost:8080"),
	}
	if store.DB != nil {
		dddViewerOpts = append(dddViewerOpts,
			observability.WithDDDViewerPgDB(store.DB),
			observability.WithDDDViewerSchemaReader(observabilitypg.NewSchemaReader(store.DB), "PostgreSQL"),
			observability.WithDDDViewerMessageReader(observabilitypg.NewMessageStoreReader(store.DB)),
		)
	} else {
		if ms, ok := msgStoreForReader.(observability.MessageStoreReader); ok {
			dddViewerOpts = append(dddViewerOpts, observability.WithDDDViewerMessageReader(ms))
		}
	}
	dddViewer = observability.NewDDDViewer(dddViewerOpts...)

	appCtx := app.NewAppContext(AppCustom{
		Config:           cfg,
		Store:            store,
		OrderRepo:        orderRepo,
		EventSourcedRepo: eventSourcedRepo,
		EventStore:       eventStore,
		Inventory:        inventory,
		JobManager:       jobManager,
		DDDViewer:        dddViewer,
		MetricsRecorder:  metricsRecorder,
		TxManager:        txManager,
	})
	appCtx.CmdBus = cmdBus
	appCtx.QueryBus = queryBus
	appCtx.EventBus = eventBus
	appCtx.Chain = chain
	appCtx.Backend = backend

	appCtx.RegisterLifecycle(eventBus)
	appCtx.RegisterLifecycle(cmdBus)
	appCtx.RegisterLifecycle(queryBus)
	appCtx.RegisterLifecycle(jobManager)
	appCtx.RegisterLifecycle(backend)

	if store.DB != nil {
		db := store.DB
		appCtx.OnClose(func() error { return db.Close() })
	}

	return appCtx, nil
}
```

- [ ] **Step 3: 运行编译确认**

Run: `go build github.com/ddd-qce/exampleapp/infrastructure/...`
Expected: 可能失败，因为 http handlers 和 tests 还没迁移

---

### Task 5: 迁移 exampleapp 接口层和测试

**Files:**
- Modify: `exampleapp/interfaces/http/server.go`
- Modify: `exampleapp/interfaces/http/handlers.go`
- Modify: `exampleapp/interfaces/http/e2e_test.go`
- Modify: `exampleapp/interfaces/http/http_test.go`
- Modify: `exampleapp/integration/integration_test.go`
- Modify: `exampleapp/main.go`
- Modify: `exampleapp/infrastructure/provider_contract_test.go` (如有引用)

**关键映射：所有通过 `h.app.X` 访问自定义字段的地方需要改为 `h.app.Custom.X`**

| 旧访问 | 新访问 |
|--------|--------|
| `h.app.Config.TestMode` | `h.app.Custom.Config.TestMode` |
| `h.app.Inventory` | `h.app.Custom.Inventory` |
| `h.app.OrderRepo` | `h.app.Custom.OrderRepo` |
| `h.app.EventStore` | `h.app.Custom.EventStore` |
| `h.app.JobManager` | `h.app.Custom.JobManager` |
| `h.app.DDDViewer` | `h.app.Custom.DDDViewer` |
| `h.app.Store().DB` | `h.app.Custom.Store.DB` |
| `h.app.CmdBus` | `h.app.CmdBus` (不变，框架核心字段) |
| `h.app.QueryBus` | `h.app.QueryBus` (不变) |
| `h.app.EventBus` | `h.app.EventBus` (不变) |
| `h.app.Backend` | `h.app.Backend` (不变) |

- [ ] **Step 1: 修改 server.go**

- `app.DDDViewer.RegisterRoutes(mux)` → `app.Custom.DDDViewer.RegisterRoutes(mux)`
- `app.Config.TestMode` → `app.Custom.Config.TestMode`

- [ ] **Step 2: 修改 handlers.go**

批量替换（使用 edit 工具的 replaceAll）：
- `h.app.Inventory` → `h.app.Custom.Inventory`
- `h.app.OrderRepo` → `h.app.Custom.OrderRepo`
- `h.app.EventStore` → `h.app.Custom.EventStore`
- `h.app.JobManager` → `h.app.Custom.JobManager`
- `h.app.DDDViewer` → `h.app.Custom.DDDViewer`
- `h.app.Config.TestMode` → `h.app.Custom.Config.TestMode`
- `h.app.Store().DB` → `h.app.Custom.Store.DB`

注意：CmdBus/QueryBus/EventBus/Backend 不变。

- [ ] **Step 3: 修改 e2e_test.go**

替换自定义字段访问：
- `app.DDDViewer` → `app.Custom.DDDViewer`
- `app.EventSourcedRepo` → `app.Custom.EventSourcedRepo`
- `app.OrderRepo` → `app.Custom.OrderRepo`
- `app.MetricsRecorder` → `app.Custom.MetricsRecorder`
- `app.EventStore` → `app.Custom.EventStore`

CmdBus/QueryBus/Backend/Close 不变。

- [ ] **Step 4: 修改 http_test.go**

同上映射。

- [ ] **Step 5: 修改 integration/integration_test.go**

同上映射。

- [ ] **Step 6: 修改 main.go**

`main.go` 只使用了 `appCtx.Close`、`appCtx.RegisterLifecycle`、`appCtx.WaitForSignal`，这些都是框架方法，无需改动。确认 `app.LifecycleFunc` 的 import 路径不变（`"github.com/ddd-qce/core/app"`）。

- [ ] **Step 7: 检查 provider_contract_test.go**

如有引用旧 AppContext 字段，按映射修改。

- [ ] **Step 8: 运行编译确认**

Run: `go build github.com/ddd-qce/exampleapp/...`
Expected: PASS

- [ ] **Step 9: 运行 exampleapp 测试**

Run: `go test github.com/ddd-qce/exampleapp/... -v -count=1`
Expected: PASS

- [ ] **Step 10: Commit**

```bash
git add exampleapp/
git commit -m "refactor: migrate exampleapp to AppContext[T]"
```

---

### Task 6: 更新 README.md

**Files:**
- Modify: `README.md`

- [ ] **Step 1: 更新 README 中的 API 示例**

将 `app.NewApp(app.WithAutoBackend())` 替换为新的 `AppContext[T]` API 示例。

- [ ] **Step 2: Commit**

```bash
git add README.md
git commit -m "docs: update README for AppContext[T] API"
```

---

### Task 7: 全量验证

- [ ] **Step 1: 运行 core 测试**

Run: `go test ./... -count=1`
Expected: PASS

- [ ] **Step 2: 运行 exampleapp 测试**

Run: `go test github.com/ddd-qce/exampleapp/... -count=1`
Expected: PASS

- [ ] **Step 3: 运行 lint**

Run: `golangci-lint run ./...`
Expected: PASS

- [ ] **Step 4: 运行 exampleapp lint**

Run: `cd exampleapp && golangci-lint run ./...`
Expected: PASS

---

## 自审 Checklist

1. **Spec 覆盖:** 废弃 app.NewApp ✅ | AppContext[T] 核心结构 ✅ | WireFunc 预设 ✅ | exampleapp 迁移 ✅ | README 更新 ✅
2. **Placeholder 扫描:** 无 TBD/TODO，所有代码完整
3. **类型一致性:** `AppContext[T]` 贯穿所有文件，`AppCustom` 在 types.go 定义，类型别名 `type AppContext = app.AppContext[AppCustom]` 统一使用

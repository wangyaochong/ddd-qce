# WithBuses 可插拔 Bus 注入 实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 新增 `WithBuses(cmdBus, queryBus, eventBus)` AppOption，允许用户注入自定义 Bus 实现，同时保留 `WithCommandHandlers` / `WithQueryHandlers` / `WithEventSubscriptions` 作为 memory 默认便捷方法。

**Architecture:** 在 `app/app.go` 中新增 `WithBuses` 函数，接受三个接口参数直接赋值给 App 字段。原有 `With*Handlers` 函数保持不变作为便捷入口。两者互斥——若同时使用，后者覆盖前者（与现有 `WithBackend` / `WithAutoBackend` 模式一致）。同时新增 `WithCommandBus` / `WithQueryBus` / `WithEventBus` 三个单独注入函数，提供更细粒度的控制。

**Tech Stack:** Go 1.22+, 现有 command.CommandBus / query.QueryBus / event.EventBus 接口

---

## 文件变更清单

| 文件 | 操作 | 职责 |
|------|------|------|
| `app/app.go` | 修改 | 新增 WithBuses、WithCommandBus、WithQueryBus、WithEventBus |
| `app/app_test.go` | 创建 | 测试新的 WithBuses 系列函数 |

---

### Task 1: 编写 WithBuses 及单独注入函数的失败测试

**Files:**
- Create: `app/app_test.go`

- [ ] **Step 1: 创建测试文件，编写 WithBuses 测试**

```go
package app

import (
	"context"
	"testing"

	"github.com/ddd-qce/core/cqrs/command"
	"github.com/ddd-qce/core/cqrs/event"
	"github.com/ddd-qce/core/cqrs/impl/memory"
	"github.com/ddd-qce/core/cqrs/query"
)

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

func TestWithBuses(t *testing.T) {
	cmdBus := memory.NewCommandBus()
	queryBus := memory.NewQueryBus()
	eventBus := memory.NewEventBus()

	app, err := NewApp(
		WithBuses(cmdBus, queryBus, eventBus),
	)
	if err != nil {
		t.Fatalf("NewApp with WithBuses: %v", err)
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
	cmdBus := memory.NewCommandBus()

	app, err := NewApp(
		WithCommandBus(cmdBus),
	)
	if err != nil {
		t.Fatalf("NewApp with WithCommandBus: %v", err)
	}

	if app.CmdBus != cmdBus {
		t.Error("CmdBus not set correctly via WithCommandBus")
	}
}

func TestWithQueryBus(t *testing.T) {
	queryBus := memory.NewQueryBus()

	app, err := NewApp(
		WithQueryBus(queryBus),
	)
	if err != nil {
		t.Fatalf("NewApp with WithQueryBus: %v", err)
	}

	if app.QueryBus != queryBus {
		t.Error("QueryBus not set correctly via WithQueryBus")
	}
}

func TestWithEventBus(t *testing.T) {
	eventBus := memory.NewEventBus()

	app, err := NewApp(
		WithEventBus(eventBus),
	)
	if err != nil {
		t.Fatalf("NewApp with WithEventBus: %v", err)
	}

	if app.EventBus != eventBus {
		t.Error("EventBus not set correctly via WithEventBus")
	}
}

func TestWithBuses_AllowsHandlerRegistration(t *testing.T) {
	cmdBus := memory.NewCommandBus()
	queryBus := memory.NewQueryBus()
	eventBus := memory.NewEventBus()

	if err := cmdBus.RegisterHandler(&testCommandHandler{}); err != nil {
		t.Fatalf("register command handler: %v", err)
	}
	if err := queryBus.RegisterHandler(&testQueryHandler{}); err != nil {
		t.Fatalf("register query handler: %v", err)
	}
	handler := &testEventHandler{}
	if err := eventBus.SubscribeHandler(handler); err != nil {
		t.Fatalf("subscribe event handler: %v", err)
	}

	app, err := NewApp(
		WithBuses(cmdBus, queryBus, eventBus),
	)
	if err != nil {
		t.Fatalf("NewApp: %v", err)
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

	if err := event.Dispatch(context.Background(), app.EventBus, testEvent{}); err != nil {
		t.Fatalf("dispatch event: %v", err)
	}
	if !handler.called {
		t.Error("event handler was not called")
	}
}

func TestWithBuses_NilBusUsesDefault(t *testing.T) {
	app, err := NewApp(
		WithBuses(nil, nil, nil),
	)
	if err != nil {
		t.Fatalf("NewApp with nil buses: %v", err)
	}

	if app.CmdBus != nil {
		t.Error("expected CmdBus to be nil when nil passed")
	}
	if app.QueryBus != nil {
		t.Error("expected QueryBus to be nil when nil passed")
	}
	if app.EventBus != nil {
		t.Error("expected EventBus to be nil when nil passed")
	}
}
```

- [ ] **Step 2: 运行测试确认编译失败**

Run: `go test ./app/ -run "TestWithBuses|TestWithCommandBus|TestWithQueryBus|TestWithEventBus" -v`
Expected: 编译失败 — `WithBuses`、`WithCommandBus`、`WithQueryBus`、`WithEventBus` 未定义

---

### Task 2: 实现 WithBuses 及单独注入函数

**Files:**
- Modify: `app/app.go` (在 `WithEventSubscriptions` 函数后追加新函数)

- [ ] **Step 1: 在 `app/app.go` 末尾（第 159 行之后）添加四个新函数**

```go
func WithBuses(cmdBus command.CommandBus, queryBus query.QueryBus, eventBus event.EventBus) AppOption {
	return func(a *App) error {
		a.CmdBus = cmdBus
		a.QueryBus = queryBus
		a.EventBus = eventBus
		return nil
	}
}

func WithCommandBus(cmdBus command.CommandBus) AppOption {
	return func(a *App) error {
		a.CmdBus = cmdBus
		return nil
	}
}

func WithQueryBus(queryBus query.QueryBus) AppOption {
	return func(a *App) error {
		a.QueryBus = queryBus
		return nil
	}
}

func WithEventBus(eventBus event.EventBus) AppOption {
	return func(a *App) error {
		a.EventBus = eventBus
		return nil
	}
}
```

- [ ] **Step 2: 运行新测试确认通过**

Run: `go test ./app/ -run "TestWithBuses|TestWithCommandBus|TestWithQueryBus|TestWithEventBus" -v`
Expected: 全部 PASS

- [ ] **Step 3: 运行全量测试确认无回归**

Run: `go test ./...`
Expected: 全部 PASS

- [ ] **Step 4: 提交**

```bash
git add app/app.go app/app_test.go
git commit -m "feat(app): add WithBuses/WithCommandBus/WithQueryBus/WithEventBus for pluggable bus injection"
```

---

### Task 3: 验证 exampleapp 兼容性

**Files:**
- Read-only check: `exampleapp/infrastructure/wire.go`

- [ ] **Step 1: 确认 exampleapp 不受影响**

`exampleapp/infrastructure/wire.go` 不使用 `app.NewApp`，而是手动构建 `AppContext`，因此不会受此变更影响。

Run: `go test ./exampleapp/...`
Expected: 全部 PASS

---

## 设计决策说明

1. **保留 `WithCommandHandlers` / `WithQueryHandlers` / `WithEventSubscriptions` 不变** — 开箱即用的便捷方法，简单场景直接用
2. **新增 `WithBuses` 一次性注入三个 Bus** — 适合需要全部替换的高级场景（如分布式 Bus）
3. **新增 `WithCommandBus` / `WithQueryBus` / `WithEventBus` 单独注入** — 适合只需替换某一个 Bus 的场景
4. **互斥策略** — 与 `WithBackend` / `WithAutoBackend` 一致，后设置的 option 覆盖前者，无需额外冲突检测
5. **nil 值语义** — 传入 nil 不会自动创建 memory Bus，App 字段保持 nil

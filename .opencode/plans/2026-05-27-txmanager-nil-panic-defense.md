# TxManager Nil Panic 双层防护 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 消除 `TransactionAspect` 在 `TxManager == nil` 时的 panic，采用构造层快速失败 + 方法层防御的双层防护策略。

**Architecture:** 第一层（构造层）在 `NewTransactionAspect(nil)` 时 panic 并给出明确引导信息；第二层（方法层）在 `BeforeCommand`/`AfterCommand` 入口检测 nil 并返回 error，防止通过 `&TransactionAspect{}` 直接构造时的运行时 panic。两层配合：构造层覆盖规范用法，方法层兜底绕过构造函数的场景。

**Tech Stack:** Go 1.x, standard library `fmt` / `strings`

---

## File Structure

| File | Action | Responsibility |
|------|--------|----------------|
| `aspect/builtin/transaction.go` | Modify | 添加双层 nil 防护 |
| `aspect/builtin/builtin_test.go` | Modify | 添加 nil 防护单元测试 |

`transaction_test.go`（integration build tag）和 `infrastructure_test.go` 已有构造模式不会受影响。`wire.go` / `app.go` 均通过 `NewTransactionAspect` 构造且传入非 nil 参数，无需改动。

---

### Task 1: 构造层 — NewTransactionAspect nil 参数 panic

**Files:**
- Modify: `aspect/builtin/transaction.go:39-41`
- Test: `aspect/builtin/builtin_test.go`

- [ ] **Step 1: Write the failing test**

在 `builtin_test.go` 末尾添加：

```go
func TestNewTransactionAspect_NilPanics(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic when TxManager is nil")
		}
		msg, ok := r.(string)
		if !ok {
			t.Fatalf("expected string panic, got %T: %v", r, r)
		}
		if !strings.Contains(msg, "TxManager") || !strings.Contains(msg, "NoOpTransactionManager") {
			t.Errorf("panic message should mention TxManager and NoOpTransactionManager, got: %s", msg)
		}
	}()
	NewTransactionAspect(nil)
}
```

同时确认文件顶部 import 包含 `"strings"`。

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./aspect/builtin/ -run TestNewTransactionAspect_NilPanics -v`
Expected: FAIL — 当前 `NewTransactionAspect(nil)` 不 panic

- [ ] **Step 3: Write minimal implementation**

修改 `aspect/builtin/transaction.go` 中 `NewTransactionAspect`：

```go
func NewTransactionAspect(txManager TransactionManager) *TransactionAspect {
	if txManager == nil {
		panic("transaction: TxManager must not be nil, use NoOpTransactionManager for non-transactional scenarios")
	}
	return &TransactionAspect{TxManager: txManager}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./aspect/builtin/ -run TestNewTransactionAspect_NilPanics -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add aspect/builtin/transaction.go aspect/builtin/builtin_test.go
git commit -m "fix: panic in NewTransactionAspect when TxManager is nil"
```

---

### Task 2: 方法层 — BeforeCommand / AfterCommand nil 防御

**Files:**
- Modify: `aspect/builtin/transaction.go:59-71`
- Test: `aspect/builtin/builtin_test.go`

- [ ] **Step 1: Write the failing test**

在 `builtin_test.go` 末尾添加：

```go
func TestTransactionAspect_BeforeCommand_NilTxManager(t *testing.T) {
	aspect := &TransactionAspect{}
	ctx := context.Background()
	_, err := aspect.BeforeCommand(ctx, &testCommand{})
	if err == nil {
		t.Fatal("expected error when TxManager is nil in BeforeCommand")
	}
	if !strings.Contains(err.Error(), "TxManager") {
		t.Errorf("error should mention TxManager, got: %v", err)
	}
}

func TestTransactionAspect_AfterCommand_NilTxManager(t *testing.T) {
	aspect := &TransactionAspect{}
	ctx := context.Background()
	err := aspect.AfterCommand(ctx, &testCommand{}, "result", nil, 100*time.Millisecond)
	if err == nil {
		t.Fatal("expected error when TxManager is nil in AfterCommand")
	}
	if !strings.Contains(err.Error(), "TxManager") {
		t.Errorf("error should mention TxManager, got: %v", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./aspect/builtin/ -run "TestTransactionAspect_BeforeCommand_NilTxManager|TestTransactionAspect_AfterCommand_NilTxManager" -v`
Expected: FAIL — 当前会 panic 而非返回 error

- [ ] **Step 3: Write minimal implementation**

修改 `aspect/builtin/transaction.go` 中 `BeforeCommand` 和 `AfterCommand`：

```go
func (t *TransactionAspect) BeforeCommand(ctx context.Context, cmd any) (context.Context, error) {
	if t.TxManager == nil {
		return ctx, fmt.Errorf("transaction: TxManager is nil, use NoOpTransactionManager for non-transactional scenarios")
	}
	return t.TxManager.Begin(ctx)
}

func (t *TransactionAspect) AfterCommand(ctx context.Context, cmd any, result any, err error, duration time.Duration) error {
	if t.TxManager == nil {
		return fmt.Errorf("transaction: TxManager is nil, use NoOpTransactionManager for non-transactional scenarios")
	}
	if err != nil {
		if rbErr := t.TxManager.Rollback(ctx); rbErr != nil {
			return fmt.Errorf("command failed: %v, rollback failed: %v", err, rbErr)
		}
		return err
	}
	return t.TxManager.Commit(ctx)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./aspect/builtin/ -run "TestTransactionAspect_BeforeCommand_NilTxManager|TestTransactionAspect_AfterCommand_NilTxManager" -v`
Expected: PASS

- [ ] **Step 5: Run full builtin test suite for regression**

Run: `go test ./aspect/builtin/ -v`
Expected: ALL PASS — 现有测试通过 `NewTransactionAspect` 或传入非 nil `TxManager`，不受 nil guard 影响

- [ ] **Step 6: Commit**

```bash
git add aspect/builtin/transaction.go aspect/builtin/builtin_test.go
git commit -m "fix: add nil TxManager guard in BeforeCommand/AfterCommand"
```

---

### Task 3: 验证 — 全量回归测试

**Files:** 无修改

- [ ] **Step 1: Run full project test suite**

Run: `go test ./...`
Expected: ALL PASS

- [ ] **Step 2: Verify integration-tagged tests compile correctly**

Run: `go test -tags=integration ./aspect/builtin/ -run TestTransactionAspect -v`
Expected: ALL PASS — integration 测试中 `&builtin.TransactionAspect{TxManager: txMgr}` 传入非 nil mock，不受影响

- [ ] **Step 3: Verify wire.go and app.go construction paths**

确认 `exampleapp/infrastructure/wire.go:76` 和 `app/app.go:92` 均通过 `NewTransactionAspect(txManager)` 构造，`txManager` 不为 nil（`NewAppTransactionManager()` / `NewNoOpTransactionManager()` 均返回非 nil），不受构造层 panic 影响。

---

## Self-Review

**1. Spec coverage:**
- ✅ 构造层 nil panic + 明确引导消息 → Task 1
- ✅ 方法层 nil guard + 返回 error → Task 2
- ✅ 回归验证 → Task 3

**2. Placeholder scan:**
- 无 TBD / TODO / placeholder — 所有代码和命令完整

**3. Type consistency:**
- `NewTransactionAspect` 签名不变 (`TransactionManager` interface → `*TransactionAspect`)
- `BeforeCommand` 返回 `(context.Context, error)` 不变
- `AfterCommand` 返回 `error` 不变
- 测试中 `&TransactionAspect{}` 零值构造触发方法层 guard，与设计一致

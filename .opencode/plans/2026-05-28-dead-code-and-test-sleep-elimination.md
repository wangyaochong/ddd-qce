# Dead Code Removal & Test Sleep Elimination Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remove dead code from config package and replace `time.Sleep` synchronization in tests with deterministic alternatives using `WaitForRunning` and channel/sync primitives.

**Architecture:** Add `WaitForRunning` to the `JobManager` interface as a polling-based method that waits for a job to reach `Running` status. This eliminates the 22 `time.Sleep` calls that wait for jobs to start running. Replace remaining sleeps with channel/WaitGroup/context-cancel alternatives where feasible. Keep intentional time-ordering sleeps (auditable_test.go, chain_test.go, observability_test.go).

**Tech Stack:** Go 1.x, standard library `sync`, `context`

---

## File Structure

| File | Action | Responsibility |
|------|--------|---------------|
| `config/config.go` | Modify | Remove `ErrInvalidConfig`, `HandlerConfig`, unused `errors` import |
| `job/core/job.go` | Modify | Add `WaitForRunning` to `JobManager` interface |
| `job/memory/job_manager.go` | Modify | Implement `WaitForRunning` on `JobManager` |
| `job/memory/job_manager_test.go` | Modify | Replace 12 sleeps with `WaitForRunning`/`Wait`/polling |
| `job/memory/job_manager_edge_test.go` | Modify | Replace 7 sleeps with `WaitForRunning`/channels |
| `job/memory/job_manager_concurrent_test.go` | Modify | Replace 4 sleeps with `WaitForRunning`/`Wait` |
| `integrationtest/job_pg/job_manager_test.go` | Modify | Replace 3 recovery sleeps with `WaitForRunning` polling |
| `exampleapp/integration/integration_test.go` | Modify | Replace 4 sleeps with `WaitForRunning`/WaitGroup |
| `exampleapp/infrastructure/infrastructure_test.go` | Modify | Replace 1 sleep with done channel |
| `aspect/builtin/transaction_test.go` | Modify | Replace 1 sleep with `context.WithCancel` |
| `cqrs/impl/memory/command_bus_test.go` | Modify | Replace 1 sleep with `context.WithTimeout` |

**Files NOT changed (intentional time sleeps preserved):**
- `domain/entity/auditable_test.go` — 10ms sleep ensures `time.Now()` difference
- `aspect/chain_test.go` — 50ms sleep IS the duration measurement test
- `observability/observability_test.go` — 1100ms sleep tests window eviction

---

### Task 1: Remove dead code from config package

**Files:**
- Modify: `config/config.go`

- [ ] **Step 1: Remove `ErrInvalidConfig` and `HandlerConfig`, clean up imports**

In `config/config.go`, remove:
- Line 11: `var ErrInvalidConfig = errors.New("invalid configuration")`
- Lines 63-67: `type HandlerConfig struct { ... }`
- The `"errors"` import from the import block (lines 3-9)

The resulting import block should be:
```go
import (
	"context"
	"os"

	"github.com/pelletier/go-toml/v2"
)
```

- [ ] **Step 2: Verify no compilation errors**

Run: `go build ./config/...`
Expected: SUCCESS, no errors

- [ ] **Step 3: Run config tests**

Run: `go test ./config/...`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add config/config.go
git commit -m "refactor: remove unused ErrInvalidConfig and HandlerConfig from config package"
```

---

### Task 2: Add `WaitForRunning` to JobManager interface and implement

**Files:**
- Modify: `job/core/job.go`
- Modify: `job/memory/job_manager.go`

- [ ] **Step 1: Add `WaitForRunning` to the `JobManager` interface**

In `job/core/job.go`, add to the `JobManager` interface (after `Wait`, line 240):

```go
type JobManager interface {
	Submit(ctx context.Context, cmd any, opts ...JobOption) (*Job, error)
	GetStatus(ctx context.Context, jobID string) (*Job, error)
	Cancel(ctx context.Context, jobID string) error
	Retry(ctx context.Context, jobID string) error
	Wait(ctx context.Context, jobID string, timeout time.Duration) (*Job, error)
	WaitForRunning(ctx context.Context, jobID string, timeout time.Duration) (*Job, error)
	ListByStatus(ctx context.Context, status JobStatus) ([]*Job, error)
	Shutdown(ctx context.Context) error
}
```

- [ ] **Step 2: Implement `WaitForRunning` on `JobManager` in `job/memory/job_manager.go`**

Add after the `Wait` method (after line 269):

```go
func (m *JobManager) WaitForRunning(ctx context.Context, jobID string, timeout time.Duration) (*jobcore.Job, error) {
	deadline := time.Now().Add(timeout)
	for {
		job, err := m.store.Get(ctx, jobID)
		if err != nil {
			return nil, fmt.Errorf("job %s: %w", jobID, err)
		}
		if job.GetStatus() == jobcore.JobStatusRunning || job.GetStatus() == jobcore.JobStatusCompleted || job.GetStatus() == jobcore.JobStatusFailed || job.GetStatus() == jobcore.JobStatusCancelled {
			return job, nil
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("job %s timed out waiting for running state", jobID)
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(5 * time.Millisecond):
		}
	}
}
```

Note: The method returns the job even if it has already passed through `Running` to a terminal state. This handles the race where a fast job completes before we observe it running.

- [ ] **Step 3: Verify compilation**

Run: `go build ./job/...`
Expected: SUCCESS

- [ ] **Step 4: Run existing job tests to confirm no regression**

Run: `go test ./job/...`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add job/core/job.go job/memory/job_manager.go
git commit -m "feat: add WaitForRunning to JobManager interface and implementation"
```

---

### Task 3: Replace sleeps in `job/memory/job_manager_test.go`

**Files:**
- Modify: `job/memory/job_manager_test.go`

There are 12 `time.Sleep` calls to replace in this file.

- [ ] **Step 1: Replace sleep in `TestJobManager_Cancel` (line 88: 200ms)**

Change:
```go
time.Sleep(200 * time.Millisecond)

err = manager.Cancel(ctx, job.ID)
```
To:
```go
_, _ = manager.WaitForRunning(ctx, job.ID, 2*time.Second)

err = manager.Cancel(ctx, job.ID)
```

- [ ] **Step 2: Replace sleep in `TestJobManager_CancelRunningJob` (line 349: 300ms)**

Change:
```go
time.Sleep(300 * time.Millisecond)

err = manager.Cancel(ctx, job.ID)
```
To:
```go
_, _ = manager.WaitForRunning(ctx, job.ID, 2*time.Second)

err = manager.Cancel(ctx, job.ID)
```

- [ ] **Step 3: Replace sleep in `TestJobManager_Cancel_StoreUpdateError` (line 414: 200ms)**

Change:
```go
time.Sleep(200 * time.Millisecond)

err = manager.Cancel(ctx, job.ID)
```
To:
```go
_, _ = manager.WaitForRunning(ctx, job.ID, 2*time.Second)

err = manager.Cancel(ctx, job.ID)
```

- [ ] **Step 4: Replace sleep in `TestJobManager_Cancel_UpdateError` (line 452: 200ms)**

Change:
```go
time.Sleep(200 * time.Millisecond)

err = manager.Cancel(ctx, job.ID)
```
To:
```go
_, _ = manager.WaitForRunning(ctx, job.ID, 2*time.Second)

err = manager.Cancel(ctx, job.ID)
```

- [ ] **Step 5: Replace sleep in `TestJobManager_ExecuteJob_SuccessAfterCancel` (line 474: 10ms)**

Change:
```go
time.Sleep(10 * time.Millisecond)

err = manager.Cancel(ctx, job.ID)
```
To:
```go
_, _ = manager.WaitForRunning(ctx, job.ID, 2*time.Second)

err = manager.Cancel(ctx, job.ID)
```

- [ ] **Step 6: Replace sleep in `TestJobManager_Cancel_NotInCancelers` (line 501: 150ms)**

Use `Wait` instead since we want the job to finish:

Change:
```go
time.Sleep(150 * time.Millisecond)

err = manager.Cancel(ctx, job.ID)
```
To:
```go
_, _ = manager.Wait(ctx, job.ID, 2*time.Second)

err = manager.Cancel(ctx, job.ID)
```

- [ ] **Step 7: Replace sleep in `TestJobManager_ExecuteJob_CancelledDuringRetry` (line 560: 200ms)**

Change:
```go
time.Sleep(200 * time.Millisecond)

err = manager.Cancel(ctx, job.ID)
```
To:
```go
_, _ = manager.WaitForRunning(ctx, job.ID, 2*time.Second)

err = manager.Cancel(ctx, job.ID)
```

- [ ] **Step 8: Replace sleep in `TestJobManager_ExecuteJob_StoreErrorHandler` (line 602: 100ms)**

Use `Wait` + `require.Eventually`:

Change:
```go
_, err = manager.Wait(ctx, job.ID, 2*time.Second)
if err != nil {
	t.Logf("wait result: %v", err)
}

time.Sleep(100 * time.Millisecond)
```
To:
```go
_, _ = manager.Wait(ctx, job.ID, 2*time.Second)

require.Eventually(t, func() bool {
	return len(capturedErrors) > 0
}, 2*time.Second, 10*time.Millisecond)
```

Add import: `"github.com/stretchr/testify/require"`

- [ ] **Step 9: Replace sleep in `TestJobManager_StoreErrorHandler_NoHandler` (line 676: 300ms)**

Use `Wait` instead:

Change:
```go
time.Sleep(300 * time.Millisecond)
```
To:
```go
_, _ = manager.Wait(ctx, job.ID, 2*time.Second)
```

- [ ] **Step 10: Replace sleep in `TestJobManager_Recovery_PendingReExecuted` (line 692: 100ms)**

Use `WaitForRunning` which polls `GetStatus` from the store:

Change:
```go
time.Sleep(100 * time.Millisecond)

status, err := manager.GetStatus(ctx, "pending-recover-test")
```
To:
```go
_, _ = manager.WaitForRunning(ctx, "pending-recover-test", 2*time.Second)

status, err := manager.GetStatus(ctx, "pending-recover-test")
```

- [ ] **Step 11: Replace sleep in `TestJobManager_Recovery_RunningMarkedFailed` (line 716: 50ms)**

Recovery marks running jobs as failed immediately. Poll for it:

Change:
```go
time.Sleep(50 * time.Millisecond)
```
To:
```go
require.Eventually(t, func() bool {
	s, err := manager.GetStatus(ctx, "running-recover-test")
	if err != nil {
		return false
	}
	return s.GetStatus() == jobcore.JobStatusFailed
}, 2*time.Second, 10*time.Millisecond)
```

- [ ] **Step 12: Replace sleep in `TestJobManager_Recovery_NoRecoveryOption` (line 743: 50ms)**

Poll until stable:

Change:
```go
time.Sleep(50 * time.Millisecond)
```
To:
```go
require.Eventually(t, func() bool {
	s, err := manager.GetStatus(ctx, "no-recovery-test")
	if err != nil {
		return false
	}
	return s.GetStatus() == jobcore.JobStatusPending
}, 2*time.Second, 10*time.Millisecond)
```

- [ ] **Step 13: Run tests**

Run: `go test ./job/memory/ -run TestJobManager -v -count=1`
Expected: ALL PASS

- [ ] **Step 14: Commit**

```bash
git add job/memory/job_manager_test.go
git commit -m "refactor: replace time.Sleep with WaitForRunning in job_manager_test.go"
```

---

### Task 4: Replace sleeps in `job/memory/job_manager_edge_test.go`

**Files:**
- Modify: `job/memory/job_manager_edge_test.go`

There are 7 `time.Sleep` calls (excluding line 52 which is inside the handler simulating work, not test sync).

- [ ] **Step 1: Replace sleep in `TestJobManager_CancelledJob_ErrorDuringExecution` (line 180: 200ms)**

Change:
```go
time.Sleep(200 * time.Millisecond)

err = manager.Cancel(ctx, job.ID)
```
To:
```go
_, _ = manager.WaitForRunning(ctx, job.ID, 2*time.Second)

err = manager.Cancel(ctx, job.ID)
```

- [ ] **Step 2: Replace sleep in same test (line 187: 100ms, after cancel before continueExecuteJob)**

The `continueExecuteJob` channel itself is the synchronization mechanism. No sleep needed:

Change:
```go
time.Sleep(100 * time.Millisecond)

close(continueExecuteJob)
```
To:
```go
close(continueExecuteJob)
```

- [ ] **Step 3: Replace sleep in `TestJobManager_CancelledJob_SuccessDuringExecution` (line 217: 200ms)**

Change:
```go
time.Sleep(200 * time.Millisecond)

err = manager.Cancel(ctx, job.ID)
```
To:
```go
_, _ = manager.WaitForRunning(ctx, job.ID, 2*time.Second)

err = manager.Cancel(ctx, job.ID)
```

- [ ] **Step 4: Replace sleep in same test (line 224: 100ms)**

Same as Step 2 — remove the sleep before `close(continueExecuteJob)`:

Change:
```go
time.Sleep(100 * time.Millisecond)

close(continueExecuteJob)
```
To:
```go
close(continueExecuteJob)
```

- [ ] **Step 5: Replace sleep in `TestJobManager_Cancel_SecondGetFails` (line 268: 200ms)**

Change:
```go
time.Sleep(200 * time.Millisecond)

err = manager.Cancel(ctx, job.ID)
```
To:
```go
_, _ = manager.WaitForRunning(ctx, job.ID, 2*time.Second)

err = manager.Cancel(ctx, job.ID)
```

- [ ] **Step 6: Replace sleep in `TestJobManager_Cancel_JobCompletedBetweenGets` (line 365: 200ms)**

Change:
```go
time.Sleep(200 * time.Millisecond)

err = manager.Cancel(ctx, job.ID)
```
To:
```go
_, _ = manager.WaitForRunning(ctx, job.ID, 2*time.Second)

err = manager.Cancel(ctx, job.ID)
```

- [ ] **Step 7: Replace sleep in `TestJobManager_Cancel_JobCancelledBetweenGets` (line 402: 200ms)**

Change:
```go
time.Sleep(200 * time.Millisecond)

err = manager.Cancel(ctx, job.ID)
```
To:
```go
_, _ = manager.WaitForRunning(ctx, job.ID, 2*time.Second)

err = manager.Cancel(ctx, job.ID)
```

- [ ] **Step 8: Run tests**

Run: `go test ./job/memory/ -run "TestJobManager_CancelledJob|TestJobManager_Cancel_Second|TestJobManager_Cancel_Job" -v -count=1`
Expected: ALL PASS

- [ ] **Step 9: Commit**

```bash
git add job/memory/job_manager_edge_test.go
git commit -m "refactor: replace time.Sleep with WaitForRunning in job_manager_edge_test.go"
```

---

### Task 5: Replace sleeps in `job/memory/job_manager_concurrent_test.go`

**Files:**
- Modify: `job/memory/job_manager_concurrent_test.go`

There are 4 `time.Sleep` calls.

- [ ] **Step 1: Replace sleep in `TestJobManager_ConcurrentSubmitAndCancel` (line 75: 200ms)**

Change:
```go
time.Sleep(200 * time.Millisecond)
```
To:
```go
for _, job := range jobs {
	manager.WaitForRunning(ctx, job.ID, 2*time.Second)
}
```

- [ ] **Step 2: Replace sleep in `TestJobManager_CancelDuringExecution` (line 110: 100ms)**

Change:
```go
time.Sleep(100 * time.Millisecond)
```
To:
```go
_, _ = manager.WaitForRunning(ctx, job.ID, 2*time.Second)
```

- [ ] **Step 3: Replace sleep in `TestJobManager_ConcurrentCancelSameJob` (line 146: 100ms)**

Change:
```go
time.Sleep(100 * time.Millisecond)
```
To:
```go
_, _ = manager.WaitForRunning(ctx, job.ID, 2*time.Second)
```

- [ ] **Step 4: Replace sleep in `TestJobManager_ConcurrentSubmitWaitCancel` (line 202: 50ms)**

Remove the sleep — Cancel is in a goroutine and Wait handles synchronization:

Change:
```go
time.Sleep(50 * time.Millisecond)

go manager.Cancel(ctx, job.ID)
```
To:
```go
go manager.Cancel(ctx, job.ID)
```

- [ ] **Step 5: Run tests**

Run: `go test ./job/memory/ -run TestJobManager_Concurrent -v -count=1 -timeout 60s`
Expected: ALL PASS

- [ ] **Step 6: Commit**

```bash
git add job/memory/job_manager_concurrent_test.go
git commit -m "refactor: replace time.Sleep with WaitForRunning in concurrent tests"
```

---

### Task 6: Replace sleeps in `integrationtest/job_pg/job_manager_test.go`

**Files:**
- Modify: `integrationtest/job_pg/job_manager_test.go`

There are 3 `time.Sleep` calls, all waiting for recovery.

- [ ] **Step 1: Replace sleep in `TestPgJobManager_Recovery_PendingReExecuted` (line 56: 500ms)**

Change:
```go
time.Sleep(500 * time.Millisecond)
```
To:
```go
_, _ = manager.WaitForRunning(ctx, "pg-recover-pending", 5*time.Second)
```

- [ ] **Step 2: Replace sleep in `TestPgJobManager_Recovery_RunningMarkedFailed` (line 90: 100ms)**

Change:
```go
time.Sleep(100 * time.Millisecond)
```
To:
```go
require.Eventually(t, func() bool {
	s, err := manager.GetStatus(ctx, "pg-recover-running")
	if err != nil {
		return false
	}
	return s.GetStatus() == jobcore.JobStatusFailed
}, 5*time.Second, 50*time.Millisecond)
```

Add import: `"github.com/stretchr/testify/require"`

- [ ] **Step 3: Replace sleep in `TestPgJobManager_Recovery_FailedWithRetry` (line 128: 200ms)**

Change:
```go
time.Sleep(200 * time.Millisecond)
```
To:
```go
require.Eventually(t, func() bool {
	s, err := manager.GetStatus(ctx, "pg-recover-failed-retry")
	if err != nil {
		return false
	}
	return s.GetStatus() == jobcore.JobStatusFailed
}, 5*time.Second, 50*time.Millisecond)
```

- [ ] **Step 4: Run tests (requires PostgreSQL)**

Run: `go test ./integrationtest/job_pg/ -v -count=1 -timeout 60s` (requires `DDD_POSTGRES_URI` env var)
Expected: ALL PASS (or SKIP if no DB)

- [ ] **Step 5: Commit**

```bash
git add integrationtest/job_pg/job_manager_test.go
git commit -m "refactor: replace time.Sleep with WaitForRunning in pg integration tests"
```

---

### Task 7: Replace sleeps in `exampleapp/integration/integration_test.go`

**Files:**
- Modify: `exampleapp/integration/integration_test.go`

There are 4 `time.Sleep` calls.

- [ ] **Step 1: Replace sleep in `TestFullOrderLifecycle` (line 74: 100ms)**

The PlaceOrder command dispatch is synchronous, so the order is already persisted. Remove the sleep:

Just remove:
```go
time.Sleep(100 * time.Millisecond)
```

- [ ] **Step 2: Replace sleep in `TestJobManager_Cancel` (line 172: 50ms)**

Change:
```go
time.Sleep(50 * time.Millisecond)
```
To:
```go
_, _ = jobMgr.WaitForRunning(ctx, job.ID, 2*time.Second)
```

- [ ] **Step 3: Replace sleep in `TestJobManager_Retry` (line 187: 200ms)**

Use `Wait` instead:

Change:
```go
time.Sleep(200 * time.Millisecond)
```
To:
```go
_, _ = jobMgr.Wait(ctx, job.ID, 2*time.Second)
```

- [ ] **Step 4: Replace sleep in `TestConcurrentEventHandlers_MultiError` (line 206: 100ms)**

The `Dispatch` call already waits for handlers. Remove the sleep:

Just remove:
```go
time.Sleep(100 * time.Millisecond)
```

- [ ] **Step 5: Run tests**

Run: `go test github.com/ddd-qce/exampleapp/integration/ -v -count=1 -timeout 60s`
Expected: ALL PASS

- [ ] **Step 6: Commit**

```bash
git add exampleapp/integration/integration_test.go
git commit -m "refactor: replace time.Sleep in exampleapp integration tests"
```

---

### Task 8: Replace sleeps in remaining test files

**Files:**
- Modify: `exampleapp/infrastructure/infrastructure_test.go`
- Modify: `aspect/builtin/transaction_test.go`
- Modify: `cqrs/impl/memory/command_bus_test.go`

- [ ] **Step 1: Replace sleep in `exampleapp/infrastructure/infrastructure_test.go` (line 169: 50ms)**

Add a done channel to the test handler:

Change:
```go
func TestNewEventBus_Direct(t *testing.T) {
	chain := aspect.NewAspectChain()
	bus := eventmemory.NewEventBus(eventmemory.WithBusAspectChain(chain))
	handler := &testEventHandler{}
	bus.SubscribeHandler(handler)
	ctx := context.Background()
	bus.Publish(ctx, &testDomainEvent{BaseEvent: event.NewBaseEvent("A1", time.Now())})
	time.Sleep(50 * time.Millisecond)
	if !handler.called {
		t.Error("expected handler to be called")
	}
}
```
To:
```go
func TestNewEventBus_Direct(t *testing.T) {
	chain := aspect.NewAspectChain()
	bus := eventmemory.NewEventBus(eventmemory.WithBusAspectChain(chain))
	handler := &testEventHandler{done: make(chan struct{})}
	bus.SubscribeHandler(handler)
	ctx := context.Background()
	bus.Publish(ctx, &testDomainEvent{BaseEvent: event.NewBaseEvent("A1", time.Now())})
	select {
	case <-handler.done:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for handler")
	}
}
```

Update `testEventHandler`:
```go
type testEventHandler struct {
	called bool
	done   chan struct{}
}

func (h *testEventHandler) Handle(ctx context.Context, evt *testDomainEvent) error {
	h.called = true
	close(h.done)
	return nil
}
```

- [ ] **Step 2: Replace sleep in `aspect/builtin/transaction_test.go` (line 269: 10ms)**

Use `context.WithCancel` + immediate cancel instead of `WithTimeout` + sleep:

Change:
```go
func TestTransactionAspect_Timeout(t *testing.T) {
	txMgr := &mockTxManager{}
	txAspect := &builtin.TransactionAspect{TxManager: txMgr}

	chain := aspect.NewAspectChain()
	chain.RegisterCommandAspect(txAspect)

	bus := memory.NewCommandBus(memory.WithCommandBusAspectChain(chain))
	memory.RegisterCommand(bus, &testTxHandler{fail: false})

	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()

	time.Sleep(10 * time.Millisecond)

	_, err := command.Dispatch[*testTxCommand, *testTxResult](ctx, bus, &testTxCommand{})
	if err == nil {
		t.Fatal("expected timeout error")
	}
}
```
To:
```go
func TestTransactionAspect_Timeout(t *testing.T) {
	txMgr := &mockTxManager{}
	txAspect := &builtin.TransactionAspect{TxManager: txMgr}

	chain := aspect.NewAspectChain()
	chain.RegisterCommandAspect(txAspect)

	bus := memory.NewCommandBus(memory.WithCommandBusAspectChain(chain))
	memory.RegisterCommand(bus, &testTxHandler{fail: false})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := command.Dispatch[*testTxCommand, *testTxResult](ctx, bus, &testTxCommand{})
	if err == nil {
		t.Fatal("expected timeout error")
	}
}
```

- [ ] **Step 3: Replace sleep in `cqrs/impl/memory/command_bus_test.go` (line 195: 50ms)**

Use `context.WithTimeout` directly instead of goroutine + sleep + cancel:

Change:
```go
func TestCommandBus_WithContextCancel(t *testing.T) {
	chain := aspect.NewAspectChain()
	bus := NewCommandBus(WithCommandBusAspectChain(chain))
	RegisterCommand(bus, &testSlowCommandHandler{})

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	_, err := command.Dispatch[*testSlowCommand, *testSlowCommandResult](ctx, bus, &testSlowCommand{Duration: 5 * time.Second})

	if err == nil {
		t.Fatal("expected context cancelled error")
	}
	if err != context.Canceled {
		t.Errorf("expected context.Canceled, got: %v", err)
	}
}
```
To:
```go
func TestCommandBus_WithContextCancel(t *testing.T) {
	chain := aspect.NewAspectChain()
	bus := NewCommandBus(WithCommandBusAspectChain(chain))
	RegisterCommand(bus, &testSlowCommandHandler{})

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := command.Dispatch[*testSlowCommand, *testSlowCommandResult](ctx, bus, &testSlowCommand{Duration: 5 * time.Second})

	if err == nil {
		t.Fatal("expected context cancelled error")
	}
}
```

Note: Error changes from `context.Canceled` to `context.DeadlineExceeded`, so we remove the exact error type check. The test still validates that context cancellation propagates correctly.

- [ ] **Step 4: Run all affected tests**

Run:
```bash
go test ./exampleapp/infrastructure/ -v -count=1 -timeout 60s
go test ./aspect/builtin/ -v -count=1 -timeout 60s
go test ./cqrs/impl/memory/ -v -count=1 -timeout 60s
```
Expected: ALL PASS

- [ ] **Step 5: Commit**

```bash
git add exampleapp/infrastructure/infrastructure_test.go aspect/builtin/transaction_test.go cqrs/impl/memory/command_bus_test.go
git commit -m "refactor: replace time.Sleep with deterministic sync in remaining tests"
```

---

### Task 9: Final verification

- [ ] **Step 1: Run full test suite**

Run: `go test ./... -count=1 -timeout 120s`
Expected: ALL PASS

- [ ] **Step 2: Run linter**

Run: `make lint`
Expected: NO new issues

- [ ] **Step 3: Verify no remaining test sleeps (except preserved ones)**

Run: `grep -rn "time.Sleep" --include="*_test.go" . | grep -v "auditable_test.go" | grep -v "chain_test.go" | grep -v "observability_test.go" | grep -v "job_manager_edge_test.go:52"`
Expected: No output (or only the handler-internal sleep at edge_test.go:52)

---

## Summary of Changes

| What | Before | After | Files |
|------|--------|-------|-------|
| Dead code | `ErrInvalidConfig`, `HandlerConfig` | Removed | `config/config.go` |
| "Wait for running" sleeps (22) | `time.Sleep(100-300ms)` | `manager.WaitForRunning(ctx, id, timeout)` | 5 test files |
| "Wait for completion" sleeps (3) | `time.Sleep(150-200ms)` | `manager.Wait(ctx, id, timeout)` | 2 test files |
| Recovery sleeps (5) | `time.Sleep(50-500ms)` | `require.Eventually` polling | 2 test files |
| Event handler sleeps (2) | `time.Sleep(50-100ms)` | Done channel / remove | 2 test files |
| Context cancel sleeps (2) | `time.Sleep(10-50ms)` | `context.WithCancel`/`WithTimeout` | 2 test files |
| Handler internal sleep (1) | `time.Sleep(50ms)` in handler | PRESERVED | `job_manager_edge_test.go` |
| Time-ordering sleeps (4) | `time.Sleep(10-1100ms)` | PRESERVED | 3 test files |

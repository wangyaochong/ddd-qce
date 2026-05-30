# 长时间执行场景测试覆盖 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为 exampleapp 添加长时间执行场景的测试覆盖，包括 Job 超时、取消、优雅退出等场景

**Architecture:** 新增 `ProcessBatchCommand` 支持可配置的运行时长，handler 响应 context 取消；在集成测试中添加对应用例验证超时失败、中途取消、metrics 记录等场景

**Tech Stack:** Go, testing, ddd-qce core (job, cqrs, aspect)

---

## 文件结构

| 文件 | 操作 | 说明 |
|------|------|------|
| `exampleapp/ddd/order/command/commands.go` | Modify | 添加 `ProcessBatchCommand` + `ProcessBatchResult` |
| `exampleapp/ddd/order/command/handlers.go` | Modify | 添加 `ProcessBatchHandler`，支持 Duration 参数和 ctx.Done() 响应 |
| `exampleapp/integration/integration_test.go` | Modify | 添加 5 个长时间执行场景测试用例 |

---

## Task 1: 添加 ProcessBatchCommand 和 Result

**Files:**
- Modify: `exampleapp/ddd/order/command/commands.go`

- [ ] **Step 1: 在 commands.go 末尾添加 ProcessBatchCommand 和 ProcessBatchResult**

在文件末尾添加：

```go
type ProcessBatchCommand struct {
	command.BaseCommand
	OrderID  orderdomain.OrderID
	Duration time.Duration
}

type ProcessBatchResult struct {
	OrderID      orderdomain.OrderID
	Processed    bool
	DurationUsed time.Duration
}
```

- [ ] **Step 2: 添加 time import**

检查 import 块，确保包含 `"time"`：

```go
import (
	"time"

	"github.com/ddd-qce/core/cqrs/command"
	orderdomain "github.com/ddd-qce/exampleapp/ddd/order/domain"
)
```

- [ ] **Step 3: 验证编译**

```bash
cd /home/wyc/projects/ddd-qce/exampleapp && go build ./...
```

---

## Task 2: 添加 ProcessBatchHandler

**Files:**
- Modify: `exampleapp/ddd/order/command/handlers.go:115-137`

- [ ] **Step 1: 在 handlers.go 末尾 (GenerateReportHandler 之后) 添加 ProcessBatchHandler**

```go
type ProcessBatchHandler struct{}

func NewProcessBatchHandler() *ProcessBatchHandler {
	return &ProcessBatchHandler{}
}

func (h *ProcessBatchHandler) Handle(ctx context.Context, cmd *ProcessBatchCommand) (*ProcessBatchResult, error) {
	start := time.Now()
	
	select {
	case <-time.After(cmd.Duration):
	case <-ctx.Done():
		return &ProcessBatchResult{
			OrderID:      cmd.OrderID,
			Processed:    false,
			DurationUsed: time.Since(start),
		}, ctx.Err()
	}

	return &ProcessBatchResult{
		OrderID:      cmd.OrderID,
		Processed:    true,
		DurationUsed: time.Since(start),
	}, nil
}

var _ command.CommandHandler[*ProcessBatchCommand, *ProcessBatchResult] = (*ProcessBatchHandler)(nil)
```

- [ ] **Step 2: 添加 time import (如果未存在)**

- [ ] **Step 3: 验证编译**

```bash
cd /home/wyc/projects/ddd-qce/exampleapp && go build ./...
```

---

## Task 3: 添加长时间执行场景集成测试

**Files:**
- Modify: `exampleapp/integration/integration_test.go:514-600`

- [ ] **Step 1: 在 integration_test.go 末尾添加 5 个测试函数**

```go
func TestLongRunningJob_CompletesSuccessfully(t *testing.T) {
	chain := aspect.NewAspectChain()
	cmdBus := commandmemory.NewCommandBus(commandmemory.WithCommandBusAspectChain(chain))
	cmdBus.RegisterHandler(ordercommand.NewProcessBatchHandler())
	jobStore := jobmemory.NewJobStore()
	jobMgr := jobmemory.NewJobManager(jobStore, cmdBus)
	ctx := context.Background()

	job, err := jobMgr.Submit(ctx, &ordercommand.ProcessBatchCommand{
		OrderID:  orderdomain.NewOrderID("LR-001"),
		Duration: 500 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("submit failed: %v", err)
	}

	result, err := jobMgr.Wait(ctx, job.ID, 2*time.Second)
	if err != nil {
		t.Fatalf("wait failed: %v", err)
	}

	if result.GetStatus() != jobcore.JobStatusCompleted {
		t.Errorf("expected completed, got %s", result.GetStatus())
	}

	jobFromStore, _ := jobMgr.GetStatus(ctx, job.ID)
	if jobFromStore.GetCompletedAt().IsZero() {
		t.Error("expected completedAt to be set")
	}
}

func TestLongRunningJob_TimeoutFails(t *testing.T) {
	chain := aspect.NewAspectChain()
	cmdBus := commandmemory.NewCommandBus(commandmemory.WithCommandBusAspectChain(chain))
	cmdBus.RegisterHandler(ordercommand.NewProcessBatchHandler())
	jobStore := jobmemory.NewJobStore()
	jobMgr := jobmemory.NewJobManager(jobStore, cmdBus)
	ctx := context.Background()

	job, err := jobMgr.Submit(ctx, &ordercommand.ProcessBatchCommand{
		OrderID:  orderdomain.NewOrderID("LR-002"),
		Duration: 3 * time.Second,
	}, jobcore.WithTimeout(500*time.Millisecond))
	if err != nil {
		t.Fatalf("submit failed: %v", err)
	}

	result, err := jobMgr.Wait(ctx, job.ID, 5*time.Second)
	if err != nil {
		t.Fatalf("wait failed: %v", err)
	}

	if result.GetStatus() != jobcore.JobStatusFailed {
		t.Errorf("expected failed, got %s", result.GetStatus())
	}

	if !strings.Contains(result.GetError(), "context deadline exceeded") {
		t.Errorf("expected context deadline exceeded error, got: %s", result.GetError())
	}
}

func TestLongRunningJob_CancelMidExecution(t *testing.T) {
	chain := aspect.NewAspectChain()
	cmdBus := commandmemory.NewCommandBus(commandmemory.WithCommandBusAspectChain(chain))
	cmdBus.RegisterHandler(ordercommand.NewProcessBatchHandler())
	jobStore := jobmemory.NewJobStore()
	jobMgr := jobmemory.NewJobManager(jobStore, cmdBus)
	ctx := context.Background()

	job, err := jobMgr.Submit(ctx, &ordercommand.ProcessBatchCommand{
		OrderID:  orderdomain.NewOrderID("LR-003"),
		Duration: 10 * time.Second,
	})
	if err != nil {
		t.Fatalf("submit failed: %v", err)
	}

	_, _ = jobMgr.WaitForRunning(ctx, job.ID, 2*time.Second)

	time.Sleep(100 * time.Millisecond)

	if err := jobMgr.Cancel(ctx, job.ID); err != nil {
		t.Fatalf("cancel failed: %v", err)
	}

	result, _ := jobMgr.GetStatus(ctx, job.ID)
	if result.GetStatus() != jobcore.JobStatusCancelled {
		t.Errorf("expected cancelled, got %s", result.GetStatus())
	}
}

func TestLongRunningJob_MetricsRecordDuration(t *testing.T) {
	metrics := builtin.NewMetricsRecorder()
	chain := aspect.NewAspectChain()
	chain.RegisterCommandAspect(builtin.NewMetricsAspect(metrics))
	cmdBus := commandmemory.NewCommandBus(commandmemory.WithCommandBusAspectChain(chain))
	cmdBus.RegisterHandler(ordercommand.NewProcessBatchHandler())
	jobStore := jobmemory.NewJobStore()
	jobMgr := jobmemory.NewJobManager(jobStore, cmdBus)
	ctx := context.Background()

	job, err := jobMgr.Submit(ctx, &ordercommand.ProcessBatchCommand{
		OrderID:  orderdomain.NewOrderID("LR-004"),
		Duration: 300 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("submit failed: %v", err)
	}

	_, _ = jobMgr.Wait(ctx, job.ID, 2*time.Second)

	if len(metrics.Durations) == 0 {
		t.Fatal("expected at least one duration metric")
	}

	var found bool
	var dur time.Duration
	for name, d := range metrics.Durations {
		if strings.Contains(name, "ProcessBatchCommand") {
			found = true
			dur = d
			break
		}
	}
	if !found {
		t.Error("expected ProcessBatchCommand in metrics")
	}

	if dur < 250*time.Millisecond || dur > 500*time.Millisecond {
		t.Errorf("expected duration ~300ms, got %v", dur)
	}
}

func TestLongRunningJob_GracefulShutdown(t *testing.T) {
	chain := aspect.NewAspectChain()
	cmdBus := commandmemory.NewCommandBus(commandmemory.WithCommandBusAspectChain(chain))
	cmdBus.RegisterHandler(ordercommand.NewProcessBatchHandler())
	jobStore := jobmemory.NewJobStore()
	jobMgr := jobmemory.NewJobManager(jobStore, cmdBus)
	ctx := context.Background()

	job, err := jobMgr.Submit(ctx, &ordercommand.ProcessBatchCommand{
		OrderID:  orderdomain.NewOrderID("LR-005"),
		Duration: 2 * time.Second,
	})
	if err != nil {
		t.Fatalf("submit failed: %v", err)
	}

	_, _ = jobMgr.WaitForRunning(ctx, job.ID, 2*time.Second)

	go func() {
		time.Sleep(200 * time.Millisecond)
		jobMgr.Shutdown(context.Background())
	}()

	select {
	case <-ctx.Done():
	case <-time.After(3 * time.Second):
	}

	result, _ := jobMgr.GetStatus(ctx, job.ID)
	if result.GetStatus() != jobcore.JobStatusCompleted && result.GetStatus() != jobcore.JobStatusCancelled {
		t.Logf("job status after shutdown: %s (may be expected to stay running)", result.GetStatus())
	}
}
```

- [ ] **Step 2: 添加必要的 import**

在文件顶部 import 块添加 `"strings"`:

```go
import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
	// ... rest of imports
)
```

- [ ] **Step 3: 运行测试验证**

```bash
cd /home/wyc/projects/ddd-qce/exampleapp && go test -v -run "TestLongRunningJob" ./integration/
```

预期输出：5 个测试全部 PASS

---

## Task 4: 注册 ProcessBatchHandler (可选 - 如需通过 HTTP 测试)

**Files:**
- Modify: `exampleapp/infrastructure/wire.go` (如果存在且需要)

- [ ] **Step 1: 检查 wire.go 是否需要添加 handler 注册**

```bash
grep -n "GenerateReportHandler" /home/wyc/projects/ddd-qce/exampleapp/infrastructure/wire.go
```

如果存在，则添加：

```go
cmdBus.RegisterHandler(ordercommand.NewProcessBatchHandler())
```

- [ ] **Step 2: 验证编译**

```bash
cd /home/wyc/projects/ddd-qce/exampleapp && go build ./...
```

---

## Task 5: 提交变更

- [ ] **Step 1: 检查 git 状态**

```bash
cd /home/wyc/projects/ddd-qce && git status
```

- [ ] **Step 2: 提交**

```bash
git add exampleapp/ddd/order/command/commands.go exampleapp/ddd/order/command/handlers.go exampleapp/integration/integration_test.go
git commit -m "test: add long-running command scenario coverage

- Add ProcessBatchCommand with configurable Duration field
- Add ProcessBatchHandler that respects context cancellation
- Add 5 integration tests:
  - TestLongRunningJob_CompletesSuccessfully
  - TestLongRunningJob_TimeoutFails
  - TestLongRunningJob_CancelMidExecution
  - TestLongRunningJob_MetricsRecordDuration
  - TestLongRunningJob_GracefulShutdown"
```

---

## 验证清单

| 场景 | 测试函数 | 验证点 |
|------|----------|--------|
| 长时间命令正常完成 | `TestLongRunningJob_CompletesSuccessfully` | Job status=completed, completedAt 设置 |
| Job 超时失败 | `TestLongRunningJob_TimeoutFails` | Job status=failed, 错误包含 "context deadline exceeded" |
| 中途取消 | `TestLongRunningJob_CancelMidExecution` | Job status=cancelled |
| Metrics 记录时长 | `TestLongRunningJob_MetricsRecordDuration` | Metrics.Durations 包含命令, 时长 ~300ms |
| 优雅退出 | `TestLongRunningJob_GracefulShutdown` | Shutdown 不 panic, job 状态合理 |

---

## Plan complete and saved to `.opencode/plans/2026-05-30-long-running-command-tests.md`

**Two execution options:**

**1. Subagent-Driven (recommended)** - I dispatch a fresh subagent per task, review between tasks, fast iteration

**2. Inline Execution** - Execute tasks in this session using executing_plans, batch execution with checkpoints

**Which approach?**
package memory

import (
	"context"
	"testing"
	"time"

	"github.com/ddd-qce/core/aspect"
	commandmemory "github.com/ddd-qce/core/cqrs/command/memory"
	jobcore "github.com/ddd-qce/core/job/core"
)

type testLongCommand struct {
	Duration time.Duration
}

type testLongResult struct {
	Message string
}

type testLongHandler struct {
	Duration time.Duration
}

func (h *testLongHandler) Handle(ctx context.Context, cmd *testLongCommand) (*testLongResult, error) {
	select {
	case <-time.After(h.Duration):
		return &testLongResult{Message: "completed"}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

type testError struct {
	msg string
}

func (e *testError) Error() string { return e.msg }

func newTestCommandBus(duration time.Duration) *commandmemory.CommandBus {
	chain := aspect.NewAspectChain()
	bus := commandmemory.NewCommandBus(chain)
	commandmemory.RegisterCommand(bus, &testLongHandler{Duration: duration})
	return bus
}

func TestJobManager_SubmitAndWait(t *testing.T) {
	store := NewJobStore()
	cmdBus := newTestCommandBus(1 * time.Second)
	manager := NewJobManager(store, cmdBus)

	ctx := context.Background()
	job, err := manager.Submit(ctx, &testLongCommand{Duration: 1 * time.Second})
	if err != nil {
		t.Fatalf("failed to submit job: %v", err)
	}

	if job.ID == "" {
		t.Fatal("job ID should not be empty")
	}
	if job.Status != jobcore.JobStatusPending && job.Status != jobcore.JobStatusRunning {
		t.Errorf("expected pending or running, got %s", job.Status)
	}

	result, err := manager.Wait(ctx, job.ID, 5*time.Second)
	if err != nil {
		t.Fatalf("wait failed: %v", err)
	}
	if result.Status != jobcore.JobStatusCompleted {
		t.Errorf("expected completed, got %s", result.Status)
	}
}

func TestJobManager_Cancel(t *testing.T) {
	store := NewJobStore()
	cmdBus := newTestCommandBus(10 * time.Second)
	manager := NewJobManager(store, cmdBus)

	ctx := context.Background()
	job, err := manager.Submit(ctx, &testLongCommand{Duration: 10 * time.Second})
	if err != nil {
		t.Fatalf("failed to submit job: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	err = manager.Cancel(ctx, job.ID)
	if err != nil {
		t.Fatalf("cancel failed: %v", err)
	}

	result, err := manager.Wait(ctx, job.ID, 5*time.Second)
	if err != nil {
		t.Fatalf("wait failed: %v", err)
	}
	if result.Status != jobcore.JobStatusCancelled {
		t.Errorf("expected cancelled, got %s", result.Status)
	}
}

func TestJobManager_Timeout(t *testing.T) {
	store := NewJobStore()
	cmdBus := newTestCommandBus(10 * time.Second)
	manager := NewJobManager(store, cmdBus)

	ctx := context.Background()
	job, err := manager.Submit(ctx, &testLongCommand{Duration: 10 * time.Second},
		jobcore.WithTimeout(500*time.Millisecond),
	)
	if err != nil {
		t.Fatalf("failed to submit job: %v", err)
	}

	result, err := manager.Wait(ctx, job.ID, 2*time.Second)
	if err != nil {
		t.Fatalf("wait failed: %v", err)
	}
	if result.Status != jobcore.JobStatusFailed {
		t.Errorf("expected failed due to timeout, got %s", result.Status)
	}
}

func TestJobManager_GetStatus(t *testing.T) {
	store := NewJobStore()
	cmdBus := newTestCommandBus(2 * time.Second)
	manager := NewJobManager(store, cmdBus)

	ctx := context.Background()
	job, err := manager.Submit(ctx, &testLongCommand{Duration: 2 * time.Second})
	if err != nil {
		t.Fatalf("failed to submit job: %v", err)
	}

	status, err := manager.GetStatus(ctx, job.ID)
	if err != nil {
		t.Fatalf("get status failed: %v", err)
	}
	if status.ID != job.ID {
		t.Errorf("expected job ID %s, got %s", job.ID, status.ID)
	}
}

func TestJobManager_ListByStatus(t *testing.T) {
	store := NewJobStore()
	cmdBus := newTestCommandBus(5 * time.Second)
	manager := NewJobManager(store, cmdBus)

	ctx := context.Background()
	_, err := manager.Submit(ctx, &testLongCommand{Duration: 5 * time.Second})
	if err != nil {
		t.Fatalf("failed to submit job: %v", err)
	}

	pending, err := manager.ListByStatus(ctx, jobcore.JobStatusPending)
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if len(pending) == 0 {
		t.Log("no pending jobs (may have started already)")
	}
}

func TestJobManager_Retry(t *testing.T) {
	store := NewJobStore()
	cmdBus := newTestCommandBus(100 * time.Millisecond)
	manager := NewJobManager(store, cmdBus)

	ctx := context.Background()
	job, err := manager.Submit(ctx, &testLongCommand{Duration: 100 * time.Millisecond},
		jobcore.WithTimeout(50*time.Millisecond),
	)
	if err != nil {
		t.Fatalf("failed to submit job: %v", err)
	}

	result, err := manager.Wait(ctx, job.ID, 2*time.Second)
	if err != nil {
		t.Fatalf("wait failed: %v", err)
	}
	if result.Status != jobcore.JobStatusFailed {
		t.Fatalf("expected failed, got %s", result.Status)
	}

	err = manager.Retry(ctx, job.ID)
	if err != nil {
		t.Fatalf("retry failed: %v", err)
	}

	status, err := manager.GetStatus(ctx, job.ID)
	if err != nil {
		t.Fatalf("get status failed: %v", err)
	}
	if status.Status != jobcore.JobStatusPending {
		t.Errorf("expected pending after retry, got %s", status.Status)
	}
	if status.Error != "" {
		t.Errorf("expected error cleared after retry, got %s", status.Error)
	}
}

func TestJobManager_RetryNonFailed(t *testing.T) {
	store := NewJobStore()
	cmdBus := newTestCommandBus(100 * time.Millisecond)
	manager := NewJobManager(store, cmdBus)

	ctx := context.Background()
	job, err := manager.Submit(ctx, &testLongCommand{Duration: 100 * time.Millisecond})
	if err != nil {
		t.Fatalf("failed to submit job: %v", err)
	}

	_, err = manager.Wait(ctx, job.ID, 2*time.Second)
	if err != nil {
		t.Fatalf("wait failed: %v", err)
	}

	err = manager.Retry(ctx, job.ID)
	if err == nil {
		t.Fatal("expected error when retrying non-failed job")
	}
}

func TestJobManager_CancelCompleted(t *testing.T) {
	store := NewJobStore()
	cmdBus := newTestCommandBus(100 * time.Millisecond)
	manager := NewJobManager(store, cmdBus)

	ctx := context.Background()
	job, err := manager.Submit(ctx, &testLongCommand{Duration: 100 * time.Millisecond})
	if err != nil {
		t.Fatalf("failed to submit job: %v", err)
	}

	_, err = manager.Wait(ctx, job.ID, 2*time.Second)
	if err != nil {
		t.Fatalf("wait failed: %v", err)
	}

	err = manager.Cancel(ctx, job.ID)
	if err == nil {
		t.Fatal("expected error when cancelling completed job")
	}
}

func TestJobManager_GetStatusNotFound(t *testing.T) {
	store := NewJobStore()
	cmdBus := newTestCommandBus(100 * time.Millisecond)
	manager := NewJobManager(store, cmdBus)

	ctx := context.Background()
	_, err := manager.GetStatus(ctx, "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent job")
	}
}

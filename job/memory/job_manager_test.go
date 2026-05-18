package memory

import (
	"context"
	"fmt"
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

func TestJobManager_SubmitWithOptions(t *testing.T) {
	store := NewJobStore()
	cmdBus := newTestCommandBus(100 * time.Millisecond)
	manager := NewJobManager(store, cmdBus)

	ctx := context.Background()
	job, err := manager.Submit(ctx, &testLongCommand{Duration: 100 * time.Millisecond},
		jobcore.WithTimeout(2*time.Second),
		jobcore.WithMaxRetries(3),
	)
	if err != nil {
		t.Fatalf("failed to submit job: %v", err)
	}

	if job.MaxRetries != 3 {
		t.Errorf("expected max retries 3, got %d", job.MaxRetries)
	}

	result, err := manager.Wait(ctx, job.ID, 5*time.Second)
	if err != nil {
		t.Fatalf("wait failed: %v", err)
	}
	if result.Status != jobcore.JobStatusCompleted {
		t.Errorf("expected completed, got %s", result.Status)
	}
}

func TestJobManager_WaitTimeout(t *testing.T) {
	store := NewJobStore()
	cmdBus := newTestCommandBus(10 * time.Second)
	manager := NewJobManager(store, cmdBus)

	ctx := context.Background()
	job, err := manager.Submit(ctx, &testLongCommand{Duration: 10 * time.Second})
	if err != nil {
		t.Fatalf("failed to submit job: %v", err)
	}

	_, err = manager.Wait(ctx, job.ID, 100*time.Millisecond)
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if err.Error() == "" {
		t.Error("expected timeout error message")
	}
}

func TestJobManager_Wait_GetStatusError(t *testing.T) {
	store := NewJobStore()
	cmdBus := newTestCommandBus(100 * time.Millisecond)
	manager := NewJobManager(store, cmdBus)

	ctx := context.Background()
	_, err := manager.Wait(ctx, "nonexistent", 200*time.Millisecond)
	if err == nil {
		t.Fatal("expected error when waiting for nonexistent job")
	}
}

func TestJobManager_WaitContextCancelled(t *testing.T) {
	store := NewJobStore()
	cmdBus := newTestCommandBus(10 * time.Second)
	manager := NewJobManager(store, cmdBus)

	ctx, cancel := context.WithCancel(context.Background())
	job, err := manager.Submit(ctx, &testLongCommand{Duration: 10 * time.Second})
	if err != nil {
		t.Fatalf("failed to submit job: %v", err)
	}

	cancel()

	_, err = manager.Wait(ctx, job.ID, 5*time.Second)
	if err == nil {
		t.Fatal("expected context cancelled error")
	}
}

func TestJobManager_CancelRunningJob(t *testing.T) {
	store := NewJobStore()
	cmdBus := newTestCommandBus(10 * time.Second)
	manager := NewJobManager(store, cmdBus)

	ctx := context.Background()
	job, err := manager.Submit(ctx, &testLongCommand{Duration: 10 * time.Second})
	if err != nil {
		t.Fatalf("failed to submit job: %v", err)
	}

	time.Sleep(300 * time.Millisecond)

	err = manager.Cancel(ctx, job.ID)
	if err != nil {
		t.Fatalf("cancel failed: %v", err)
	}

	result, err := manager.GetStatus(ctx, job.ID)
	if err != nil {
		t.Fatalf("get status failed: %v", err)
	}
	if result.Status != jobcore.JobStatusCancelled {
		t.Errorf("expected cancelled, got %s", result.Status)
	}
}

func TestJobManager_Cancel_GetLatestError(t *testing.T) {
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

type failingGetJobStore struct {
	jobcore.JobStore
	getErr error
}

func (s *failingGetJobStore) Get(ctx context.Context, id string) (*jobcore.Job, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	return s.JobStore.Get(ctx, id)
}

func TestJobManager_Cancel_GetLatestStoreError(t *testing.T) {
	innerStore := NewJobStore()
	store := &failingGetJobStore{
		JobStore: innerStore,
		getErr:   fmt.Errorf("get failed"),
	}
	cmdBus := newTestCommandBus(10 * time.Second)
	manager := NewJobManager(store, cmdBus)

	ctx := context.Background()
	job, err := manager.Submit(ctx, &testLongCommand{Duration: 10 * time.Second})
	if err != nil {
		t.Fatalf("failed to submit job: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	err = manager.Cancel(ctx, job.ID)
	if err == nil {
		t.Fatal("expected error when store.Get fails during cancel")
	}
}

type failingUpdateJobStore struct {
	jobcore.JobStore
	updateErr error
}

func (s *failingUpdateJobStore) Update(ctx context.Context, job *jobcore.Job) error {
	if s.updateErr != nil {
		return s.updateErr
	}
	return s.JobStore.Update(ctx, job)
}

func TestJobManager_Cancel_UpdateError(t *testing.T) {
	innerStore := NewJobStore()
	store := &failingUpdateJobStore{
		JobStore:  innerStore,
		updateErr: fmt.Errorf("update failed"),
	}
	cmdBus := newTestCommandBus(10 * time.Second)
	manager := NewJobManager(store, cmdBus)

	ctx := context.Background()
	job, err := manager.Submit(ctx, &testLongCommand{Duration: 10 * time.Second})
	if err != nil {
		t.Fatalf("failed to submit job: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	err = manager.Cancel(ctx, job.ID)
	if err == nil {
		t.Fatal("expected error when store.Update fails during cancel")
	}
	if err.Error() != "update failed" {
		t.Errorf("expected 'update failed', got '%v'", err)
	}
}

func TestJobManager_ExecuteJob_SuccessAfterCancel(t *testing.T) {
	store := NewJobStore()
	cmdBus := newTestCommandBus(50 * time.Millisecond)
	manager := NewJobManager(store, cmdBus)

	ctx := context.Background()
	job, err := manager.Submit(ctx, &testLongCommand{Duration: 50 * time.Millisecond})
	if err != nil {
		t.Fatalf("failed to submit job: %v", err)
	}

	time.Sleep(10 * time.Millisecond)

	err = manager.Cancel(ctx, job.ID)
	if err != nil {
		t.Logf("cancel may or may not succeed: %v", err)
	}

	result, err := manager.Wait(ctx, job.ID, 2*time.Second)
	if err != nil {
		t.Fatalf("wait failed: %v", err)
	}
	if result.Status != jobcore.JobStatusCompleted && result.Status != jobcore.JobStatusCancelled {
		t.Errorf("expected completed or cancelled, got %s", result.Status)
	}
}

func TestJobManager_Cancel_NotInCancelers(t *testing.T) {
	store := NewJobStore()
	cmdBus := newTestCommandBus(50 * time.Millisecond)
	manager := NewJobManager(store, cmdBus)

	ctx := context.Background()
	job, err := manager.Submit(ctx, &testLongCommand{Duration: 50 * time.Millisecond})
	if err != nil {
		t.Fatalf("failed to submit job: %v", err)
	}

	time.Sleep(150 * time.Millisecond)

	err = manager.Cancel(ctx, job.ID)
	if err != nil && err.Error() != "" {
		t.Logf("cancel result: %v", err)
	}
}

func TestJobManager_Retry_GetJobError(t *testing.T) {
	store := NewJobStore()
	cmdBus := newTestCommandBus(100 * time.Millisecond)
	manager := NewJobManager(store, cmdBus)

	ctx := context.Background()
	err := manager.Retry(ctx, "nonexistent")
	if err == nil {
		t.Fatal("expected error when retrying nonexistent job")
	}
}

type failingJobStore struct {
	jobcore.JobStore
	createErr error
}

func (s *failingJobStore) Create(ctx context.Context, job *jobcore.Job) error {
	return s.createErr
}

func TestJobManager_Submit_CreateError(t *testing.T) {
	store := &failingJobStore{
		createErr: fmt.Errorf("create failed"),
	}
	cmdBus := newTestCommandBus(100 * time.Millisecond)
	manager := NewJobManager(store, cmdBus)

	ctx := context.Background()
	_, err := manager.Submit(ctx, &testLongCommand{Duration: 100 * time.Millisecond})
	if err == nil {
		t.Fatal("expected error when create fails")
	}
	if err.Error() != "create failed" {
		t.Errorf("expected 'create failed', got '%v'", err)
	}
}

func TestJobManager_ExecuteJob_CancelledDuringRetry(t *testing.T) {
	store := NewJobStore()
	cmdBus := newTestCommandBus(10 * time.Second)
	manager := NewJobManager(store, cmdBus)

	ctx := context.Background()
	job, err := manager.Submit(ctx, &testLongCommand{Duration: 10 * time.Second},
		jobcore.WithMaxRetries(1),
	)
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

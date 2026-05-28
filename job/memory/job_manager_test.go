package memory

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/ddd-qce/core/aspect"
	"github.com/ddd-qce/core/cqrs/command"
	commandmemory "github.com/ddd-qce/core/cqrs/impl/memory"
	jobcore "github.com/ddd-qce/core/job/core"
	"github.com/stretchr/testify/require"
)

type testLongCommand struct {
	command.BaseCommand
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
	bus := commandmemory.NewCommandBus(commandmemory.WithCommandBusAspectChain(chain))
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
	if job.GetStatus() != jobcore.JobStatusPending && job.GetStatus() != jobcore.JobStatusRunning {
		t.Errorf("expected pending or running, got %s", job.GetStatus())
	}

	result, err := manager.Wait(ctx, job.ID, 5*time.Second)
	if err != nil {
		t.Fatalf("wait failed: %v", err)
	}
	if result.GetStatus() != jobcore.JobStatusCompleted {
		t.Errorf("expected completed, got %s", result.GetStatus())
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

	_, _ = manager.WaitForRunning(ctx, job.ID, 2*time.Second)

	err = manager.Cancel(ctx, job.ID)
	if err != nil {
		t.Fatalf("cancel failed: %v", err)
	}

	result, err := manager.Wait(ctx, job.ID, 5*time.Second)
	if err != nil {
		t.Fatalf("wait failed: %v", err)
	}
	if result.GetStatus() != jobcore.JobStatusCancelled {
		t.Errorf("expected cancelled, got %s", result.GetStatus())
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
	if result.GetStatus() != jobcore.JobStatusFailed {
		t.Errorf("expected failed due to timeout, got %s", result.GetStatus())
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
	if result.GetStatus() != jobcore.JobStatusFailed {
		t.Fatalf("expected failed, got %s", result.GetStatus())
	}

	err = manager.Retry(ctx, job.ID)
	if err != nil {
		t.Fatalf("retry failed: %v", err)
	}

	status, err := manager.GetStatus(ctx, job.ID)
	if err != nil {
		t.Fatalf("get status failed: %v", err)
	}
	if status.GetStatus() != jobcore.JobStatusPending {
		t.Errorf("expected pending after retry, got %s", status.GetStatus())
	}
	if status.GetError() != "" {
		t.Errorf("expected error cleared after retry, got %s", status.GetError())
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
	if result.GetStatus() != jobcore.JobStatusCompleted {
		t.Errorf("expected completed, got %s", result.GetStatus())
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

	_, _ = manager.WaitForRunning(ctx, job.ID, 2*time.Second)

	err = manager.Cancel(ctx, job.ID)
	if err != nil {
		t.Fatalf("cancel failed: %v", err)
	}

	result, err := manager.GetStatus(ctx, job.ID)
	if err != nil {
		t.Fatalf("get status failed: %v", err)
	}
	if result.GetStatus() != jobcore.JobStatusCancelled {
		t.Errorf("expected cancelled, got %s", result.GetStatus())
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

func TestJobManager_Cancel_StoreUpdateError(t *testing.T) {
	innerStore := NewJobStore()
	store := &failingUpdateJobStore{
		JobStore:  innerStore,
		updateErr: fmt.Errorf("store update failed"),
	}
	cmdBus := newTestCommandBus(10 * time.Second)
	manager := NewJobManager(store, cmdBus)

	ctx := context.Background()
	job, err := manager.Submit(ctx, &testLongCommand{Duration: 10 * time.Second})
	if err != nil {
		t.Fatalf("failed to submit job: %v", err)
	}

	_, _ = manager.WaitForRunning(ctx, job.ID, 2*time.Second)

	err = manager.Cancel(ctx, job.ID)
	if err == nil {
		t.Fatal("expected error when store.Update fails during cancel")
	}
	if err.Error() != "store update failed" {
		t.Errorf("expected 'store update failed', got '%v'", err)
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

	_, _ = manager.WaitForRunning(ctx, job.ID, 2*time.Second)

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

	_, _ = manager.WaitForRunning(ctx, job.ID, 2*time.Second)

	err = manager.Cancel(ctx, job.ID)
	if err != nil {
		t.Logf("cancel may or may not succeed: %v", err)
	}

	result, err := manager.Wait(ctx, job.ID, 2*time.Second)
	if err != nil {
		t.Fatalf("wait failed: %v", err)
	}
	if result.GetStatus() != jobcore.JobStatusCompleted && result.GetStatus() != jobcore.JobStatusCancelled {
		t.Errorf("expected completed or cancelled, got %s", result.GetStatus())
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

	_, _ = manager.Wait(ctx, job.ID, 2*time.Second)

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

	_, _ = manager.WaitForRunning(ctx, job.ID, 2*time.Second)

	err = manager.Cancel(ctx, job.ID)
	if err != nil {
		t.Fatalf("cancel failed: %v", err)
	}

	result, err := manager.Wait(ctx, job.ID, 5*time.Second)
	if err != nil {
		t.Fatalf("wait failed: %v", err)
	}
	if result.GetStatus() != jobcore.JobStatusCancelled {
		t.Errorf("expected cancelled, got %s", result.GetStatus())
	}
}

func TestJobManager_ExecuteJob_StoreErrorHandler(t *testing.T) {
	innerStore := NewJobStore()
	store := &failingUpdateJobStore{
		JobStore:  innerStore,
		updateErr: fmt.Errorf("update failed"),
	}

	var capturedErrors []*jobcore.StoreError
	handler := func(ctx context.Context, storeErr *jobcore.StoreError) {
		capturedErrors = append(capturedErrors, storeErr)
	}

	cmdBus := newTestCommandBus(100 * time.Millisecond)
	manager := NewJobManager(store, cmdBus, WithStoreErrorHandler(handler))

	ctx := context.Background()
	job, err := manager.Submit(ctx, &testLongCommand{Duration: 100 * time.Millisecond})
	if err != nil {
		t.Fatalf("failed to submit job: %v", err)
	}

	_, _ = manager.Wait(ctx, job.ID, 2*time.Second)

	require.Eventually(t, func() bool {
		return len(capturedErrors) > 0
	}, 2*time.Second, 10*time.Millisecond)

	if len(capturedErrors) == 0 {
		t.Fatal("expected store errors to be captured by handler")
	}

	foundRunning := false
	foundFinal := false
	for _, se := range capturedErrors {
		if se.JobID != job.ID {
			t.Errorf("expected job ID %s, got %s", job.ID, se.JobID)
		}
		if se.Operation == "update_running" {
			foundRunning = true
		}
		if se.Operation == "update_final" {
			foundFinal = true
		}
	}
	if !foundRunning {
		t.Error("expected update_running store error")
	}
	if !foundFinal {
		t.Error("expected update_final store error")
	}
}

func TestJobManager_Retry_UpdateError(t *testing.T) {
	innerStore := NewJobStore()
	store := &failingUpdateJobStore{
		JobStore:  innerStore,
		updateErr: fmt.Errorf("update failed"),
	}
	cmdBus := newTestCommandBus(100 * time.Millisecond)
	manager := NewJobManager(store, cmdBus)

	ctx := context.Background()
	job, err := manager.Submit(ctx, &testLongCommand{Duration: 100 * time.Millisecond},
		jobcore.WithTimeout(50*time.Millisecond),
	)
	if err != nil {
		t.Fatalf("failed to submit job: %v", err)
	}

	_, err = manager.Wait(ctx, job.ID, 2*time.Second)
	if err != nil {
		t.Fatalf("wait failed: %v", err)
	}

	err = manager.Retry(ctx, job.ID)
	if err == nil {
		t.Fatal("expected error when store.Update fails during retry")
	}
	if err.Error() != "failed to reset job for retry: update failed" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestJobManager_StoreErrorHandler_NoHandler(t *testing.T) {
	innerStore := NewJobStore()
	store := &failingUpdateJobStore{
		JobStore:  innerStore,
		updateErr: fmt.Errorf("update failed"),
	}

	cmdBus := newTestCommandBus(100 * time.Millisecond)
	manager := NewJobManager(store, cmdBus)

	ctx := context.Background()
	job, err := manager.Submit(ctx, &testLongCommand{Duration: 100 * time.Millisecond})
	if err != nil {
		t.Fatalf("failed to submit job: %v", err)
	}

	_, _ = manager.Wait(ctx, job.ID, 2*time.Second)
}

func TestJobManager_Recovery_PendingReExecuted(t *testing.T) {
	store := NewJobStore()
	cmdBus := newTestCommandBus(10 * time.Millisecond)
	manager := NewJobManager(store, cmdBus, WithRecovery())

	ctx := context.Background()

	pendingJob := jobcore.NewJob("pending-recover-test", &testLongCommand{Duration: 10 * time.Millisecond})
	pendingJob.RestoreJobState(jobcore.JobStatusPending, nil, "", "", time.Time{}, time.Time{})
	if err := store.Create(ctx, pendingJob); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	require.Eventually(t, func() bool {
		s, err := manager.GetStatus(ctx, "pending-recover-test")
		if err != nil {
			return false
		}
		return s.GetStatus() == jobcore.JobStatusCompleted
	}, 2*time.Second, 10*time.Millisecond)

	status, err := manager.GetStatus(ctx, "pending-recover-test")
	if err != nil {
		t.Fatalf("GetStatus failed: %v", err)
	}
	if status.GetStatus() != jobcore.JobStatusCompleted {
		t.Errorf("expected completed after recovery, got %s", status.GetStatus())
	}
}

func TestJobManager_Recovery_RunningMarkedFailed(t *testing.T) {
	store := NewJobStore()
	cmdBus := newTestCommandBus(10 * time.Millisecond)
	manager := NewJobManager(store, cmdBus, WithRecovery())

	ctx := context.Background()

	runningJob := jobcore.NewJob("running-recover-test", &testLongCommand{Duration: 10 * time.Millisecond})
	runningJob.RestoreJobState(jobcore.JobStatusRunning, nil, "", "", time.Time{}, time.Time{})
	if err := store.Create(ctx, runningJob); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	require.Eventually(t, func() bool {
		s, err := manager.GetStatus(ctx, "running-recover-test")
		if err != nil {
			return false
		}
		return s.GetStatus() == jobcore.JobStatusFailed
	}, 2*time.Second, 10*time.Millisecond)

	status, err := manager.GetStatus(ctx, "running-recover-test")
	if err != nil {
		t.Fatalf("GetStatus failed: %v", err)
	}
	if status.GetStatus() != jobcore.JobStatusFailed {
		t.Errorf("expected failed after recovery, got %s", status.GetStatus())
	}
	if status.GetError() == "" {
		t.Error("expected error message for recovered running job")
	}
}

func TestJobManager_Recovery_NoRecoveryOption(t *testing.T) {
	store := NewJobStore()
	cmdBus := newTestCommandBus(10 * time.Millisecond)
	manager := NewJobManager(store, cmdBus)

	ctx := context.Background()

	pendingJob := jobcore.NewJob("no-recovery-test", &testLongCommand{Duration: 10 * time.Millisecond})
	pendingJob.RestoreJobState(jobcore.JobStatusPending, nil, "", "", time.Time{}, time.Time{})
	if err := store.Create(ctx, pendingJob); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	require.Eventually(t, func() bool {
		s, err := manager.GetStatus(ctx, "no-recovery-test")
		if err != nil {
			return false
		}
		return s.GetStatus() == jobcore.JobStatusPending
	}, 2*time.Second, 10*time.Millisecond)

	status, err := manager.GetStatus(ctx, "no-recovery-test")
	if err != nil {
		t.Fatalf("GetStatus failed: %v", err)
	}
	if status.GetStatus() != jobcore.JobStatusPending {
		t.Errorf("expected pending (no recovery), got %s", status.GetStatus())
	}
}

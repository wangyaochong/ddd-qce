package memory

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/ddd-qce/core/aspect"
	commandmemory "github.com/ddd-qce/core/cqrs/command/memory"
	jobcore "github.com/ddd-qce/core/job/core"
)

type testCancellableCommand struct {
	Duration    time.Duration
	ShouldFail  bool
	BlockChan   chan struct{}
	FailAfterCancel bool
}

type testCancellableResult struct {
	Message string
}

type testCancellableHandler struct {
	Duration    time.Duration
	ShouldFail  bool
	BlockChan   chan struct{}
	FailAfterCancel bool
}

func (h *testCancellableHandler) Handle(ctx context.Context, cmd *testCancellableCommand) (*testCancellableResult, error) {
	if h.BlockChan != nil {
		select {
		case <-h.BlockChan:
		case <-ctx.Done():
			// Continue to next select to allow FailAfterCancel behavior
		}
	}

	select {
	case <-time.After(h.Duration):
		if h.ShouldFail {
			return nil, fmt.Errorf("handler failed")
		}
		return &testCancellableResult{Message: "completed"}, nil
	case <-ctx.Done():
		if h.FailAfterCancel {
			time.Sleep(50 * time.Millisecond)
			return nil, fmt.Errorf("failed after cancel")
		}
		return nil, ctx.Err()
	}
}

func newTestCancellableCommandBus(duration time.Duration, shouldFail bool, blockChan chan struct{}, failAfterCancel bool) *commandmemory.CommandBus {
	chain := aspect.NewAspectChain()
	bus := commandmemory.NewCommandBus(chain)
	commandmemory.RegisterCommand(bus, &testCancellableHandler{
		Duration:    duration,
		ShouldFail:  shouldFail,
		BlockChan:   blockChan,
		FailAfterCancel: failAfterCancel,
	})
	return bus
}

type testHookCommand struct {
	Duration time.Duration
}

type testHookHandler struct {
	Duration    time.Duration
	hook        func(ctx context.Context) error
	returnedChan chan struct{}
}

func (h *testHookHandler) Handle(ctx context.Context, cmd *testHookCommand) (*testCancellableResult, error) {
	select {
	case <-time.After(h.Duration):
		return &testCancellableResult{Message: "completed"}, nil
	case <-ctx.Done():
		if h.returnedChan != nil {
			close(h.returnedChan)
		}
		err := h.hook(ctx)
		if err != nil {
			return nil, err
		}
		return &testCancellableResult{Message: "completed"}, nil
	}
}

func newTestCancellableCommandBusWithHook(duration time.Duration, hook func(ctx context.Context) error, returnedChan chan struct{}) *commandmemory.CommandBus {
	chain := aspect.NewAspectChain()
	bus := commandmemory.NewCommandBus(chain)
	commandmemory.RegisterCommand(bus, &testHookHandler{
		Duration:    duration,
		hook:        hook,
		returnedChan: returnedChan,
	})
	return bus
}

type sameRefJobStore struct {
	jobs map[string]*jobcore.Job
	mu   sync.RWMutex
}

func newSameRefJobStore() *sameRefJobStore {
	return &sameRefJobStore{
		jobs: make(map[string]*jobcore.Job),
	}
}

func (s *sameRefJobStore) Create(ctx context.Context, job *jobcore.Job) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.jobs[job.ID] = job
	return nil
}

func (s *sameRefJobStore) Get(ctx context.Context, id string) (*jobcore.Job, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	job, exists := s.jobs[id]
	if !exists {
		return nil, fmt.Errorf("job %s not found", id)
	}
	return job, nil  // Returns same reference!
}

func (s *sameRefJobStore) Update(ctx context.Context, job *jobcore.Job) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.jobs[job.ID] = job
	return nil
}

func (s *sameRefJobStore) List(ctx context.Context, status jobcore.JobStatus) ([]*jobcore.Job, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []*jobcore.Job
	for _, job := range s.jobs {
		if job.Status == status {
			result = append(result, job)
		}
	}
	return result, nil
}

func (s *sameRefJobStore) Delete(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.jobs, id)
	return nil
}

func TestJobManager_CancelledJob_ErrorDuringExecution(t *testing.T) {
	store := newSameRefJobStore()
	handlerReturned := make(chan struct{})
	continueExecuteJob := make(chan struct{})

	cmdBus := newTestCancellableCommandBusWithHook(10*time.Second, func(ctx context.Context) error {
		<-continueExecuteJob
		return fmt.Errorf("failed after cancel")
	}, handlerReturned)
	manager := NewJobManager(store, cmdBus)

	ctx := context.Background()
	job, err := manager.Submit(ctx, &testHookCommand{Duration: 10 * time.Second})
	if err != nil {
		t.Fatalf("failed to submit job: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	err = manager.Cancel(ctx, job.ID)
	if err != nil {
		t.Fatalf("cancel failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	close(continueExecuteJob)

	result, err := manager.Wait(ctx, job.ID, 5*time.Second)
	if err != nil {
		t.Fatalf("wait failed: %v", err)
	}
	if result.Status != jobcore.JobStatusCancelled {
		t.Errorf("expected cancelled, got %s", result.Status)
	}
}

func TestJobManager_CancelledJob_SuccessDuringExecution(t *testing.T) {
	store := newSameRefJobStore()
	handlerReturned := make(chan struct{})
	continueExecuteJob := make(chan struct{})

	cmdBus := newTestCancellableCommandBusWithHook(10*time.Second, func(ctx context.Context) error {
		<-continueExecuteJob
		return nil
	}, handlerReturned)
	manager := NewJobManager(store, cmdBus)

	ctx := context.Background()
	job, err := manager.Submit(ctx, &testHookCommand{Duration: 10 * time.Second})
	if err != nil {
		t.Fatalf("failed to submit job: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	err = manager.Cancel(ctx, job.ID)
	if err != nil {
		t.Fatalf("cancel failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	close(continueExecuteJob)

	result, err := manager.Wait(ctx, job.ID, 5*time.Second)
	if err != nil {
		t.Fatalf("wait failed: %v", err)
	}
	if result.Status != jobcore.JobStatusCancelled {
		t.Errorf("expected cancelled, got %s", result.Status)
	}
}

type failingGetJobStoreV2 struct {
	jobcore.JobStore
	getErr    error
	getCount  int
	failOnGet int
}

func (s *failingGetJobStoreV2) Get(ctx context.Context, id string) (*jobcore.Job, error) {
	s.getCount++
	if s.getCount == s.failOnGet && s.getErr != nil {
		return nil, s.getErr
	}
	return s.JobStore.Get(ctx, id)
}

func TestJobManager_Cancel_SecondGetFails(t *testing.T) {
	innerStore := NewJobStore()
	store := &failingGetJobStoreV2{
		JobStore:  innerStore,
		getErr:    fmt.Errorf("second get failed"),
		failOnGet: 2,
	}
	cmdBus := newTestCancellableCommandBus(10 * time.Second, false, nil, false)
	manager := NewJobManager(store, cmdBus)

	ctx := context.Background()
	job, err := manager.Submit(ctx, &testCancellableCommand{Duration: 10 * time.Second})
	if err != nil {
		t.Fatalf("failed to submit job: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	err = manager.Cancel(ctx, job.ID)
	if err == nil {
		t.Fatal("expected error when second store.Get fails during cancel")
	}
	if err.Error() != "second get failed" {
		t.Errorf("expected 'second get failed', got '%v'", err)
	}
}

func TestJobManager_Cancel_AlreadyCompletedOnSecondGet(t *testing.T) {
	store := NewJobStore()
	cmdBus := newTestCancellableCommandBus(100 * time.Millisecond, false, nil, false)
	manager := NewJobManager(store, cmdBus)

	ctx := context.Background()
	job, err := manager.Submit(ctx, &testCancellableCommand{Duration: 100 * time.Millisecond})
	if err != nil {
		t.Fatalf("failed to submit job: %v", err)
	}

	result, err := manager.Wait(ctx, job.ID, 5*time.Second)
	if err != nil {
		t.Fatalf("wait failed: %v", err)
	}
	if result.Status != jobcore.JobStatusCompleted {
		t.Fatalf("expected completed, got %s", result.Status)
	}

	err = manager.Cancel(ctx, job.ID)
	if err == nil {
		t.Fatal("expected error when cancelling completed job")
	}
}

type interceptingJobStore struct {
	jobcore.JobStore
	mu                sync.Mutex
	updateIntercept   func(*jobcore.Job)
	getIntercept      func() (*jobcore.Job, error)
}

func (s *interceptingJobStore) Update(ctx context.Context, job *jobcore.Job) error {
	s.mu.Lock()
	intercept := s.updateIntercept
	s.mu.Unlock()
	if intercept != nil {
		intercept(job)
	}
	return s.JobStore.Update(ctx, job)
}

func (s *interceptingJobStore) Get(ctx context.Context, id string) (*jobcore.Job, error) {
	s.mu.Lock()
	intercept := s.getIntercept
	s.mu.Unlock()
	if intercept != nil {
		return intercept()
	}
	return s.JobStore.Get(ctx, id)
}

func TestJobManager_Cancel_JobCompletedBetweenGets(t *testing.T) {
	innerStore := NewJobStore()
	store := &interceptingJobStore{JobStore: innerStore}

	var jobID string
	getCount := 0

	store.getIntercept = func() (*jobcore.Job, error) {
		getCount++
		job, err := innerStore.Get(context.Background(), jobID)
		if err != nil {
			return nil, err
		}
		if getCount == 2 {
			job.Status = jobcore.JobStatusCompleted
		}
		return job, nil
	}

	cmdBus := newTestCancellableCommandBus(10*time.Second, false, nil, false)
	manager := NewJobManager(store, cmdBus)

	ctx := context.Background()
	job, err := manager.Submit(ctx, &testCancellableCommand{Duration: 10 * time.Second})
	if err != nil {
		t.Fatalf("failed to submit job: %v", err)
	}
	jobID = job.ID

	time.Sleep(200 * time.Millisecond)

	err = manager.Cancel(ctx, job.ID)
	if err != nil {
		t.Fatalf("cancel should succeed when job already completed: %v", err)
	}
}

func TestJobManager_Cancel_JobCancelledBetweenGets(t *testing.T) {
	innerStore := NewJobStore()
	store := &interceptingJobStore{JobStore: innerStore}

	var jobID string
	getCount := 0

	store.getIntercept = func() (*jobcore.Job, error) {
		getCount++
		job, err := innerStore.Get(context.Background(), jobID)
		if err != nil {
			return nil, err
		}
		if getCount == 2 {
			job.Status = jobcore.JobStatusCancelled
		}
		return job, nil
	}

	cmdBus := newTestCancellableCommandBus(10*time.Second, false, nil, false)
	manager := NewJobManager(store, cmdBus)

	ctx := context.Background()
	job, err := manager.Submit(ctx, &testCancellableCommand{Duration: 10 * time.Second})
	if err != nil {
		t.Fatalf("failed to submit job: %v", err)
	}
	jobID = job.ID

	time.Sleep(200 * time.Millisecond)

	err = manager.Cancel(ctx, job.ID)
	if err != nil {
		t.Fatalf("cancel should succeed when job already cancelled: %v", err)
	}
}

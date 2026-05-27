package memory

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/ddd-qce/core/aspect"
	"github.com/ddd-qce/core/cqrs/command"
	commandmemory "github.com/ddd-qce/core/cqrs/impl/memory"
	jobcore "github.com/ddd-qce/core/job/core"
)

type testConcurrentCommand struct {
	command.BaseCommand
	Duration time.Duration
}

type testConcurrentResult struct {
	Message string
}

type testConcurrentHandler struct {
	Duration time.Duration
}

func (h *testConcurrentHandler) Handle(ctx context.Context, cmd *testConcurrentCommand) (*testConcurrentResult, error) {
	select {
	case <-time.After(h.Duration):
		return &testConcurrentResult{Message: "completed"}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func newTestConcurrentCommandBus(duration time.Duration) *commandmemory.CommandBus {
	chain := aspect.NewAspectChain()
	bus := commandmemory.NewCommandBus(commandmemory.WithCommandBusAspectChain(chain))
	commandmemory.RegisterCommand(bus, &testConcurrentHandler{Duration: duration})
	return bus
}

func TestJobManager_ConcurrentSubmitAndCancel(t *testing.T) {
	store := NewJobStore()
	cmdBus := newTestConcurrentCommandBus(5 * time.Second)
	manager := NewJobManager(store, cmdBus)

	ctx := context.Background()
	var wg sync.WaitGroup
	submitted := make(chan *jobcore.Job, 50)

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			job, err := manager.Submit(ctx, &testConcurrentCommand{Duration: 10 * time.Second},
				jobcore.WithTimeout(30*time.Second))
			if err != nil {
				t.Errorf("submit %d failed: %v", id, err)
				return
			}
			submitted <- job
		}(i)
	}

	wg.Wait()
	close(submitted)

	var jobs []*jobcore.Job
	for job := range submitted {
		jobs = append(jobs, job)
	}

	time.Sleep(200 * time.Millisecond)

	cancelWg := sync.WaitGroup{}
	for _, job := range jobs {
		cancelWg.Add(1)
		go func(j *jobcore.Job) {
			defer cancelWg.Done()
			manager.Cancel(ctx, j.ID)
		}(job)
	}

	cancelWg.Wait()

	for _, job := range jobs {
		result, err := manager.Wait(ctx, job.ID, 10*time.Second)
		if err != nil {
			continue
		}
		if result.GetStatus() != jobcore.JobStatusCompleted && result.GetStatus() != jobcore.JobStatusCancelled && result.GetStatus() != jobcore.JobStatusFailed {
			t.Errorf("job %s has unexpected status: %s", job.ID, result.GetStatus())
		}
	}
}

func TestJobManager_CancelDuringExecution(t *testing.T) {
	store := NewJobStore()
	cmdBus := newTestConcurrentCommandBus(10 * time.Second)
	manager := NewJobManager(store, cmdBus)

	ctx := context.Background()
	job, err := manager.Submit(ctx, &testConcurrentCommand{Duration: 10 * time.Second})
	if err != nil {
		t.Fatalf("submit failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	var cancelErr error
	var cancelWg sync.WaitGroup
	cancelWg.Add(1)
	go func() {
		defer cancelWg.Done()
		cancelErr = manager.Cancel(ctx, job.ID)
	}()

	cancelWg.Wait()

	if cancelErr != nil {
		t.Fatalf("cancel failed: %v", cancelErr)
	}

	result, err := manager.Wait(ctx, job.ID, 5*time.Second)
	if err != nil {
		t.Fatalf("wait failed: %v", err)
	}
	if result.GetStatus() != jobcore.JobStatusCancelled {
		t.Errorf("expected cancelled, got %s", result.GetStatus())
	}
}

func TestJobManager_ConcurrentCancelSameJob(t *testing.T) {
	store := NewJobStore()
	cmdBus := newTestConcurrentCommandBus(10 * time.Second)
	manager := NewJobManager(store, cmdBus)

	ctx := context.Background()
	job, err := manager.Submit(ctx, &testConcurrentCommand{Duration: 10 * time.Second})
	if err != nil {
		t.Fatalf("submit failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	var wg sync.WaitGroup
	errors := make(chan error, 10)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := manager.Cancel(ctx, job.ID)
			if err != nil {
				errors <- err
			}
		}()
	}

	wg.Wait()
	close(errors)

	successCount := 0
	errorCount := 0
	for err := range errors {
		if err != nil {
			errorCount++
		} else {
			successCount++
		}
	}

	result, err := manager.Wait(ctx, job.ID, 5*time.Second)
	if err != nil {
		t.Fatalf("wait failed: %v", err)
	}
	if result.GetStatus() != jobcore.JobStatusCancelled && result.GetStatus() != jobcore.JobStatusFailed {
		t.Errorf("expected cancelled or failed, got %s", result.GetStatus())
	}
}

func TestJobManager_ConcurrentSubmitWaitCancel(t *testing.T) {
	store := NewJobStore()
	cmdBus := newTestConcurrentCommandBus(5 * time.Second)
	manager := NewJobManager(store, cmdBus)

	ctx := context.Background()
	var wg sync.WaitGroup

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			job, err := manager.Submit(ctx, &testConcurrentCommand{Duration: 5 * time.Second})
			if err != nil {
				return
			}

			time.Sleep(50 * time.Millisecond)

			go manager.Cancel(ctx, job.ID)

			_, _ = manager.Wait(ctx, job.ID, 3*time.Second)
		}(i)
	}

	wg.Wait()
}

func TestJobManager_RetryDuringCancel(t *testing.T) {
	store := NewJobStore()
	cmdBus := newTestConcurrentCommandBus(100 * time.Millisecond)
	manager := NewJobManager(store, cmdBus)

	ctx := context.Background()
	job, err := manager.Submit(ctx, &testConcurrentCommand{Duration: 100 * time.Millisecond},
		jobcore.WithTimeout(50*time.Millisecond),
		jobcore.WithMaxRetries(1),
	)
	if err != nil {
		t.Fatalf("submit failed: %v", err)
	}

	result, err := manager.Wait(ctx, job.ID, 5*time.Second)
	if err != nil {
		t.Fatalf("wait failed: %v", err)
	}

	if result.GetStatus() == jobcore.JobStatusFailed {
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			manager.Retry(ctx, job.ID)
		}()
		go func() {
			defer wg.Done()
			manager.Cancel(ctx, job.ID)
		}()
		wg.Wait()
	}
}

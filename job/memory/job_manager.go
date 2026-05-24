package memory

import (
	"context"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/ddd-qce/core/cqrs/command"
	ddderror "github.com/ddd-qce/core/error"
	jobcore "github.com/ddd-qce/core/job/core"
	"github.com/ddd-qce/core/trace"
)

type JobManager struct {
	store        jobcore.JobStore
	executor     command.CommandExecutor
	mu           sync.Mutex
	cancelers    map[string]context.CancelFunc
	jobs         map[string]*jobcore.Job
	onStoreError jobcore.StoreErrorHandler
	wg           sync.WaitGroup
	stopCh       chan struct{}
}

func NewJobManager(store jobcore.JobStore, executor command.CommandExecutor, opts ...JobManagerOption) *JobManager {
	m := &JobManager{
		store:     store,
		executor:  executor,
		cancelers: make(map[string]context.CancelFunc),
		jobs:      make(map[string]*jobcore.Job),
		stopCh:    make(chan struct{}),
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

type JobManagerOption func(*JobManager)

func WithStoreErrorHandler(handler jobcore.StoreErrorHandler) JobManagerOption {
	return func(m *JobManager) {
		m.onStoreError = handler
	}
}

func (m *JobManager) handleStoreError(ctx context.Context, jobID string, operation string, err error) {
	if err == nil || m.onStoreError == nil {
		return
	}
	m.onStoreError(ctx, &jobcore.StoreError{
		JobID:     jobID,
		Operation: operation,
		Err:       err,
	})
}

func (m *JobManager) Submit(ctx context.Context, cmd any, opts ...jobcore.JobOption) (*jobcore.Job, error) {
	jobID := uuid.New()
	job := jobcore.NewJob(hex.EncodeToString(jobID[:]), cmd, opts...)

	if err := m.store.Create(ctx, job); err != nil {
		return nil, err
	}

	m.mu.Lock()
	m.jobs[job.ID] = job
	m.mu.Unlock()

	m.wg.Add(1)
	go m.executeJob(job, trace.GetTraceID(ctx), trace.GetSpanID(ctx))
	return job, nil
}

func (m *JobManager) executeJob(job *jobcore.Job, parentTraceID, parentSpanID string) {
	defer m.wg.Done()

	bgCtx := context.Background()
	if parentTraceID != "" {
		spanID := trace.NewSpanID()
		bgCtx = trace.WithTrace(bgCtx, parentTraceID, spanID)
		bgCtx = trace.WithParentSpan(bgCtx, parentSpanID)
	}

	for {
		select {
		case <-m.stopCh:
			return
		default:
		}

		job.MarkRunning()

		if err := m.store.Update(bgCtx, job); err != nil {
			m.handleStoreError(bgCtx, job.ID, "update_running", err)
		}

		var execCtx context.Context
		var cancel context.CancelFunc

		if job.Timeout > 0 {
			execCtx, cancel = context.WithTimeout(bgCtx, job.Timeout)
		} else {
			execCtx, cancel = context.WithCancel(bgCtx)
		}

		m.mu.Lock()
		m.cancelers[job.ID] = cancel
		m.mu.Unlock()

		result, err := m.executor.Execute(execCtx, job.Command)
		cancel()

		m.mu.Lock()
		delete(m.cancelers, job.ID)
		m.mu.Unlock()

		if err != nil {
			cancelled, shouldRetry := job.TryFail(err.Error())
			if cancelled {
				job.MarkDone()
				break
			}
			if shouldRetry {
				if err := m.store.Update(bgCtx, job); err != nil {
					m.handleStoreError(bgCtx, job.ID, "update_retry", err)
				}
				continue
			}
		} else {
			if !job.TryComplete(result) {
				job.MarkDone()
				break
			}
		}

		if err := m.store.Update(bgCtx, job); err != nil {
			m.handleStoreError(bgCtx, job.ID, "update_final", err)
		}
		job.MarkDone()
		break
	}
}

func (m *JobManager) GetStatus(ctx context.Context, jobID string) (*jobcore.Job, error) {
	return m.store.Get(ctx, jobID)
}

func (m *JobManager) Cancel(ctx context.Context, jobID string) error {
	m.mu.Lock()
	liveJob, liveExists := m.jobs[jobID]
	m.mu.Unlock()

	if liveExists {
		if err := liveJob.TryCancel(); err != nil {
			return err
		}

		m.mu.Lock()
		cancel, exists := m.cancelers[jobID]
		m.mu.Unlock()

		if exists {
			cancel()
		}

		liveJob.MarkDone()

		if err := m.store.Update(ctx, liveJob); err != nil {
			return err
		}
		return nil
	}

	job, err := m.store.Get(ctx, jobID)
	if err != nil {
		return err
	}

	if err := job.TryCancel(); err != nil {
		return err
	}

	return m.store.Update(ctx, job)
}

func (m *JobManager) Retry(ctx context.Context, jobID string) error {
	m.mu.Lock()
	liveJob, liveExists := m.jobs[jobID]
	m.mu.Unlock()

	if liveExists {
		if err := liveJob.ResetForRetry(); err != nil {
			return err
		}

		liveJob.ResetDone()

		if err := m.store.Update(ctx, liveJob); err != nil {
			return fmt.Errorf("failed to reset job for retry: %w", err)
		}

		m.wg.Add(1)
		go m.executeJob(liveJob, trace.GetTraceID(ctx), trace.GetSpanID(ctx))
		return nil
	}

	job, err := m.store.Get(ctx, jobID)
	if err != nil {
		return err
	}

	if err := job.ResetForRetry(); err != nil {
		return err
	}

	job.ResetDone()

	if err := m.store.Update(ctx, job); err != nil {
		return fmt.Errorf("failed to reset job for retry: %w", err)
	}

	m.mu.Lock()
	m.jobs[job.ID] = job
	m.mu.Unlock()

	m.wg.Add(1)
	go m.executeJob(job, trace.GetTraceID(ctx), trace.GetSpanID(ctx))
	return nil
}

func (m *JobManager) Wait(ctx context.Context, jobID string, timeout time.Duration) (*jobcore.Job, error) {
	m.mu.Lock()
	job, exists := m.jobs[jobID]
	m.mu.Unlock()

	if !exists {
		return nil, fmt.Errorf("job %s: %w", jobID, ddderror.ErrJobNotFound)
	}

	select {
	case <-job.Done():
		return m.store.Get(ctx, jobID)
	case <-time.After(timeout):
		return nil, fmt.Errorf("job %s timed out waiting for completion", jobID)
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (m *JobManager) ListByStatus(ctx context.Context, status jobcore.JobStatus) ([]*jobcore.Job, error) {
	return m.store.List(ctx, status)
}

func (m *JobManager) Shutdown(ctx context.Context) error {
	close(m.stopCh)

	m.mu.Lock()
	for id, cancel := range m.cancelers {
		cancel()
		delete(m.cancelers, id)
	}
	m.mu.Unlock()

	done := make(chan struct{})
	go func() {
		m.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

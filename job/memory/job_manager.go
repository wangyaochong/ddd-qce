package memory

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/ddd-qce/core/cqrs/command"
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
}

func NewJobManager(store jobcore.JobStore, executor command.CommandExecutor, opts ...JobManagerOption) *JobManager {
	m := &JobManager{
		store:     store,
		executor:  executor,
		cancelers: make(map[string]context.CancelFunc),
		jobs:      make(map[string]*jobcore.Job),
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
	job := &jobcore.Job{
		ID:         uuid.New().String(),
		Command:    cmd,
		Status:     jobcore.JobStatusPending,
		CreatedAt:  time.Now(),
		MaxRetries: 0,
	}

	for _, opt := range opts {
		opt(job)
	}

	if err := m.store.Create(ctx, job); err != nil {
		return nil, err
	}

	m.mu.Lock()
	m.jobs[job.ID] = job
	m.mu.Unlock()

	go m.executeJob(job, trace.GetTraceID(ctx), trace.GetSpanID(ctx))
	return job, nil
}

func (m *JobManager) executeJob(job *jobcore.Job, parentTraceID, parentSpanID string) {
	bgCtx := context.Background()
	if parentTraceID != "" {
		spanID := trace.NewSpanID()
		bgCtx = trace.WithTrace(bgCtx, parentTraceID, spanID)
		bgCtx = trace.WithParentSpan(bgCtx, parentSpanID)
	}

	for {
		job.Lock()
		job.Status = jobcore.JobStatusRunning
		job.StartedAt = time.Now()
		job.Unlock()

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

		m.mu.Lock()
		delete(m.cancelers, job.ID)
		m.mu.Unlock()

		job.Lock()
		job.CompletedAt = time.Now()
		if err != nil {
			if job.Status == jobcore.JobStatusCancelled {
				job.Unlock()
				job.MarkDone()
				break
			}
			job.Status = jobcore.JobStatusFailed
			job.Error = err.Error()
			if job.RetryCount < job.MaxRetries {
				job.RetryCount++
				job.Unlock()
				if err := m.store.Update(bgCtx, job); err != nil {
					m.handleStoreError(bgCtx, job.ID, "update_retry", err)
				}
				continue
			}
		} else {
			if job.Status == jobcore.JobStatusCancelled {
				job.Unlock()
				job.MarkDone()
				break
			}
			job.Status = jobcore.JobStatusCompleted
			job.Result = result
		}
		job.Unlock()

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
		liveJob.Lock()
		if liveJob.Status == jobcore.JobStatusCompleted || liveJob.Status == jobcore.JobStatusCancelled {
			liveJob.Unlock()
			return fmt.Errorf("job %s cannot be cancelled (status: %s)", jobID, liveJob.Status)
		}

		m.mu.Lock()
		cancel, exists := m.cancelers[jobID]
		m.mu.Unlock()

		if exists {
			cancel()
		}

		liveJob.Status = jobcore.JobStatusCancelled
		liveJob.CompletedAt = time.Now()
		liveJob.Unlock()
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

	job.Lock()
	if job.Status == jobcore.JobStatusCompleted || job.Status == jobcore.JobStatusCancelled {
		job.Unlock()
		return fmt.Errorf("job %s cannot be cancelled (status: %s)", jobID, job.Status)
	}

	job.Status = jobcore.JobStatusCancelled
	job.CompletedAt = time.Now()
	job.Unlock()

	return m.store.Update(ctx, job)
}

func (m *JobManager) Retry(ctx context.Context, jobID string) error {
	m.mu.Lock()
	liveJob, liveExists := m.jobs[jobID]
	m.mu.Unlock()

	if liveExists {
		liveJob.Lock()
		if liveJob.Status != jobcore.JobStatusFailed {
			liveJob.Unlock()
			return fmt.Errorf("job %s is not in failed state", jobID)
		}

		liveJob.Status = jobcore.JobStatusPending
		liveJob.Error = ""
		liveJob.Result = nil
		liveJob.StartedAt = time.Time{}
		liveJob.CompletedAt = time.Time{}
		liveJob.Unlock()

		liveJob.ResetDone()

		if err := m.store.Update(ctx, liveJob); err != nil {
			return fmt.Errorf("failed to reset job for retry: %w", err)
		}

		go m.executeJob(liveJob, trace.GetTraceID(ctx), trace.GetSpanID(ctx))
		return nil
	}

	job, err := m.store.Get(ctx, jobID)
	if err != nil {
		return err
	}

	job.Lock()
	if job.Status != jobcore.JobStatusFailed {
		job.Unlock()
		return fmt.Errorf("job %s is not in failed state", jobID)
	}

	job.Status = jobcore.JobStatusPending
	job.Error = ""
	job.Result = nil
	job.StartedAt = time.Time{}
	job.CompletedAt = time.Time{}
	job.Unlock()

	job.ResetDone()

	if err := m.store.Update(ctx, job); err != nil {
		return fmt.Errorf("failed to reset job for retry: %w", err)
	}

	m.mu.Lock()
	m.jobs[job.ID] = job
	m.mu.Unlock()

	go m.executeJob(job, trace.GetTraceID(ctx), trace.GetSpanID(ctx))
	return nil
}

func (m *JobManager) Wait(ctx context.Context, jobID string, timeout time.Duration) (*jobcore.Job, error) {
	m.mu.Lock()
	job, exists := m.jobs[jobID]
	m.mu.Unlock()

	if !exists {
		return nil, fmt.Errorf("job %s not found", jobID)
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

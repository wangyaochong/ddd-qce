package core

import (
	"context"
	"sync"
	"time"
)

type JobStatus string

const (
	JobStatusPending   JobStatus = "pending"
	JobStatusRunning   JobStatus = "running"
	JobStatusCompleted JobStatus = "completed"
	JobStatusFailed    JobStatus = "failed"
	JobStatusCancelled JobStatus = "cancelled"
)

type Job struct {
	mu          sync.Mutex
	ID          string
	Command     any
	Status      JobStatus
	Result      any
	Error       string
	CreatedAt   time.Time
	StartedAt   time.Time
	CompletedAt time.Time
	Timeout     time.Duration
	RetryCount  int
	MaxRetries  int
}

func (j *Job) Lock()   { j.mu.Lock() }
func (j *Job) Unlock() { j.mu.Unlock() }

type JobStore interface {
	Create(ctx context.Context, job *Job) error
	Get(ctx context.Context, id string) (*Job, error)
	Update(ctx context.Context, job *Job) error
	List(ctx context.Context, status JobStatus) ([]*Job, error)
	Delete(ctx context.Context, id string) error
}

type JobOption func(*Job)

func WithTimeout(timeout time.Duration) JobOption {
	return func(j *Job) {
		j.Timeout = timeout
	}
}

func WithMaxRetries(maxRetries int) JobOption {
	return func(j *Job) {
		j.MaxRetries = maxRetries
	}
}

type JobManager interface {
	Submit(ctx context.Context, cmd any, opts ...JobOption) (*Job, error)
	GetStatus(ctx context.Context, jobID string) (*Job, error)
	Cancel(ctx context.Context, jobID string) error
	Retry(ctx context.Context, jobID string) error
	Wait(ctx context.Context, jobID string, timeout time.Duration) (*Job, error)
	ListByStatus(ctx context.Context, status JobStatus) ([]*Job, error)
}

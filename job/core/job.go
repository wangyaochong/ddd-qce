package core

import (
	"context"
	"fmt"
	"reflect"
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
	CommandType string
	Status      JobStatus
	Result      any
	ResultType  string
	Error       string
	CreatedAt   time.Time
	StartedAt   time.Time
	CompletedAt time.Time
	Timeout     time.Duration
	RetryCount  int
	MaxRetries  int
	done        chan struct{}
}

func (j *Job) Lock()   { j.mu.Lock() }
func (j *Job) Unlock() { j.mu.Unlock() }

func (j *Job) Done() <-chan struct{} {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.done == nil {
		j.done = make(chan struct{})
	}
	return j.done
}

func (j *Job) MarkDone() {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.done == nil {
		j.done = make(chan struct{})
	}
	select {
	case <-j.done:
	default:
		close(j.done)
	}
}

func (j *Job) ResetDone() {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.done = make(chan struct{})
}

func (j *Job) Snapshot() *Job {
	j.mu.Lock()
	defer j.mu.Unlock()
	return &Job{
		ID:          j.ID,
		Command:     j.Command,
		CommandType: j.CommandType,
		Status:      j.Status,
		Result:      j.Result,
		ResultType:  j.ResultType,
		Error:       j.Error,
		CreatedAt:   j.CreatedAt,
		StartedAt:   j.StartedAt,
		CompletedAt: j.CompletedAt,
		Timeout:     j.Timeout,
		RetryCount:  j.RetryCount,
		MaxRetries:  j.MaxRetries,
		done:        j.done,
	}
}

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

type StoreError struct {
	JobID     string
	Operation string
	Err       error
}

func (e *StoreError) Error() string {
	return fmt.Sprintf("store %s failed for job %s: %v", e.Operation, e.JobID, e.Err)
}

func (e *StoreError) Unwrap() error {
	return e.Err
}

type StoreErrorHandler func(ctx context.Context, storeErr *StoreError)

type JobManager interface {
	Submit(ctx context.Context, cmd any, opts ...JobOption) (*Job, error)
	GetStatus(ctx context.Context, jobID string) (*Job, error)
	Cancel(ctx context.Context, jobID string) error
	Retry(ctx context.Context, jobID string) error
	Wait(ctx context.Context, jobID string, timeout time.Duration) (*Job, error)
	ListByStatus(ctx context.Context, status JobStatus) ([]*Job, error)
}

type TypeRegistry struct {
	types map[string]reflect.Type
	mu    sync.RWMutex
}

func NewTypeRegistry() *TypeRegistry {
	return &TypeRegistry{types: make(map[string]reflect.Type)}
}

func (r *TypeRegistry) Register(sample any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	t := reflect.TypeOf(sample)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	r.types[t.String()] = t
}

func (r *TypeRegistry) NewInstance(typeName string) (any, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.types[typeName]
	if !ok {
		return nil, false
	}
	return reflect.New(t).Interface(), true
}

func TypeName(v any) string {
	if v == nil {
		return ""
	}
	t := reflect.TypeOf(v)
	if t.Kind() == reflect.Ptr {
		return t.Elem().String()
	}
	return t.String()
}

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
	status      JobStatus
	result      any
	resultType  string
	err         string
	CreatedAt   time.Time
	startedAt   time.Time
	completedAt time.Time
	Timeout     time.Duration
	RetryCount  int
	MaxRetries  int
	done        chan struct{}
}

func NewJob(id string, cmd any, opts ...JobOption) *Job {
	job := &Job{
		ID:        id,
		Command:   cmd,
		status:    JobStatusPending,
		CreatedAt: time.Now(),
	}
	for _, opt := range opts {
		opt(job)
	}
	return job
}

func (j *Job) GetStatus() JobStatus      { return j.status }
func (j *Job) GetResult() any            { return j.result }
func (j *Job) GetResultType() string     { return j.resultType }
func (j *Job) GetError() string          { return j.err }
func (j *Job) GetStartedAt() time.Time   { return j.startedAt }
func (j *Job) GetCompletedAt() time.Time { return j.completedAt }

// SetStatus sets the job status. Infrastructure-only: for constructing Job objects from persistence.
func (j *Job) SetStatus(s JobStatus) { j.status = s }

// SetResult sets the job result. Infrastructure-only: for constructing Job objects from persistence.
func (j *Job) SetResult(r any) { j.result = r }

// SetResultType sets the job result type. Infrastructure-only: for constructing Job objects from persistence.
func (j *Job) SetResultType(rt string) { j.resultType = rt }

// SetError sets the job error. Infrastructure-only: for constructing Job objects from persistence.
func (j *Job) SetError(e string) { j.err = e }

// SetStartedAt sets the job start time. Infrastructure-only: for constructing Job objects from persistence.
func (j *Job) SetStartedAt(t time.Time) { j.startedAt = t }

// SetCompletedAt sets the job completion time. Infrastructure-only: for constructing Job objects from persistence.
func (j *Job) SetCompletedAt(t time.Time) { j.completedAt = t }

// MarkRunning atomically sets the job status to running and records the start time.
func (j *Job) MarkRunning() {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.status = JobStatusRunning
	j.startedAt = time.Now()
}

// TryComplete atomically attempts to mark the job as completed with the given result.
// Returns false if the job was already cancelled.
func (j *Job) TryComplete(result any) bool {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.status == JobStatusCancelled {
		return false
	}
	j.completedAt = time.Now()
	j.status = JobStatusCompleted
	j.result = result
	return true
}

// TryFail atomically attempts to mark the job as failed with the given error message.
// Returns cancelled=true if the job was already cancelled.
// Returns shouldRetry=true if the job should be retried (retry count incremented).
func (j *Job) TryFail(errStr string) (cancelled bool, shouldRetry bool) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.completedAt = time.Now()
	if j.status == JobStatusCancelled {
		return true, false
	}
	j.status = JobStatusFailed
	j.err = errStr
	if j.RetryCount < j.MaxRetries {
		j.RetryCount++
		return false, true
	}
	return false, false
}

// TryCancel atomically attempts to mark the job as cancelled.
// Returns an error if the job is already completed or cancelled.
func (j *Job) TryCancel() error {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.status == JobStatusCompleted || j.status == JobStatusCancelled {
		return fmt.Errorf("job %s cannot be cancelled (status: %s)", j.ID, j.status)
	}
	j.status = JobStatusCancelled
	j.completedAt = time.Now()
	return nil
}

// ResetForRetry atomically resets the job for retry if it is in a failed state.
// Returns an error if the job is not in a failed state.
func (j *Job) ResetForRetry() error {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.status != JobStatusFailed {
		return fmt.Errorf("job %s is not in failed state", j.ID)
	}
	j.status = JobStatusPending
	j.err = ""
	j.result = nil
	j.startedAt = time.Time{}
	j.completedAt = time.Time{}
	return nil
}

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
		status:      j.status,
		result:      j.result,
		resultType:  j.resultType,
		err:         j.err,
		CreatedAt:   j.CreatedAt,
		startedAt:   j.startedAt,
		completedAt: j.completedAt,
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
	Shutdown(ctx context.Context) error
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

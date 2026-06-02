package core

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"time"
)

// JobStatus represents the lifecycle state of a background job.
type JobStatus string

const (
	// JobStatusPending indicates the job is queued awaiting execution.
	JobStatusPending JobStatus = "pending"
	// JobStatusRunning indicates the job is currently executing.
	JobStatusRunning JobStatus = "running"
	// JobStatusCompleted indicates the job finished successfully.
	JobStatusCompleted JobStatus = "completed"
	// JobStatusFailed indicates the job failed after exhausting retries.
	JobStatusFailed JobStatus = "failed"
	// JobStatusCancelled indicates the job was cancelled by the caller.
	JobStatusCancelled JobStatus = "cancelled"
)

// Job represents an asynchronous command execution with retry and cancellation support.
type Job struct {
	mu          sync.Mutex
	id          string
	command     any
	commandType string
	status      JobStatus
	result      any
	resultType  string
	err         string
	createdAt   time.Time
	startedAt   time.Time
	completedAt time.Time
	timeout     time.Duration
	retryCount  int
	maxRetries  int
	done        chan struct{}
}

// NewJob creates a Job with the given ID and command payload.
func NewJob(id string, cmd any, opts ...JobOption) *Job {
	job := &Job{
		id:        id,
		command:   cmd,
		status:    JobStatusPending,
		createdAt: time.Now(),
	}
	for _, opt := range opts {
		opt(job)
	}
	return job
}

func (j *Job) ID() string                { return j.id }
func (j *Job) Command() any              { return j.command }
func (j *Job) CommandType() string       { return j.commandType }
func (j *Job) CreatedAt() time.Time      { return j.createdAt }
func (j *Job) Timeout() time.Duration    { return j.timeout }
func (j *Job) RetryCount() int           { return j.retryCount }
func (j *Job) MaxRetries() int           { return j.maxRetries }
func (j *Job) GetStatus() JobStatus      { return j.status }
func (j *Job) GetResult() any            { return j.result }
func (j *Job) GetResultType() string     { return j.resultType }
func (j *Job) GetError() string          { return j.err }
func (j *Job) GetStartedAt() time.Time   { return j.startedAt }
func (j *Job) GetCompletedAt() time.Time { return j.completedAt }

var (
	// ErrJobNotFound is returned when a job cannot be located by ID.
	ErrJobNotFound = errors.New("job not found")
	// ErrInvalidStateTransition is returned when a job state change violates the state machine.
	ErrInvalidStateTransition = errors.New("invalid state transition")
)

func isValidTransition(from, to JobStatus) bool {
	validTransitions := map[JobStatus]map[JobStatus]bool{
		JobStatusPending: {
			JobStatusRunning: true,
		},
		JobStatusRunning: {
			JobStatusCompleted: true,
			JobStatusFailed:    true,
			JobStatusCancelled: true,
		},
		JobStatusFailed: {
			JobStatusPending: true,
		},
		JobStatusCompleted: {},
		JobStatusCancelled: {},
	}
	allowed, exists := validTransitions[from][to]
	return exists && allowed
}

// RestoreJobState restores job fields from persistence. Only for infrastructure use.
func (j *Job) RestoreJobState(status JobStatus, result any, resultType string, err string, startedAt, completedAt time.Time) error {
	j.mu.Lock()
	defer j.mu.Unlock()
	if !isValidTransition(j.status, status) {
		return fmt.Errorf("job %s: %w: %s -> %s", j.id, ErrInvalidStateTransition, j.status, status)
	}
	j.status = status
	j.result = result
	j.resultType = resultType
	j.err = err
	j.startedAt = startedAt
	j.completedAt = completedAt
	return nil
}

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
	if j.status == JobStatusCancelled || j.status == JobStatusPending {
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
	if j.status == JobStatusCancelled || j.status == JobStatusPending {
		return j.status == JobStatusCancelled, false
	}
	j.completedAt = time.Now()
	j.status = JobStatusFailed
	j.err = errStr
	if j.retryCount < j.maxRetries {
		j.retryCount++
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
		return fmt.Errorf("job %s cannot be cancelled (status: %s)", j.id, j.status)
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
		return fmt.Errorf("job %s is not in failed state", j.id)
	}
	j.status = JobStatusPending
	j.err = ""
	j.result = nil
	j.startedAt = time.Time{}
	j.completedAt = time.Time{}
	return nil
}

// Done returns a channel that closes when the job reaches a terminal state.
func (j *Job) Done() <-chan struct{} {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.done == nil {
		j.done = make(chan struct{})
	}
	return j.done
}

// MarkDone closes the done channel to signal job completion.
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

// ResetDone reinitializes the done channel, used when retrying a failed job.
func (j *Job) ResetDone() {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.done = make(chan struct{})
}

// Snapshot returns a copy of the job for safe concurrent reading or persistence.
func (j *Job) Snapshot() *Job {
	j.mu.Lock()
	defer j.mu.Unlock()
	done := make(chan struct{})
	if j.status == JobStatusCompleted || j.status == JobStatusFailed || j.status == JobStatusCancelled {
		close(done)
	}
	return &Job{
		id:          j.id,
		command:     j.command,
		commandType: j.commandType,
		status:      j.status,
		result:      j.result,
		resultType:  j.resultType,
		err:         j.err,
		createdAt:   j.createdAt,
		startedAt:   j.startedAt,
		completedAt: j.completedAt,
		timeout:     j.timeout,
		retryCount:  j.retryCount,
		maxRetries:  j.maxRetries,
		done:        done,
	}
}

// JobStore persists and retrieves job records.
type JobStore interface {
	Create(ctx context.Context, job *Job) error
	Get(ctx context.Context, id string) (*Job, error)
	Update(ctx context.Context, job *Job) error
	List(ctx context.Context, status JobStatus) ([]*Job, error)
	Delete(ctx context.Context, id string) error
}

// JobOption configures a Job during construction.
type JobOption func(*Job)

// WithTimeout sets the execution timeout for a job.
func WithTimeout(timeout time.Duration) JobOption {
	return func(j *Job) {
		j.timeout = timeout
	}
}

// WithMaxRetries sets the maximum number of retry attempts for a job.
func WithMaxRetries(maxRetries int) JobOption {
	return func(j *Job) {
		j.maxRetries = maxRetries
	}
}

// StoreError describes a failure in the job persistence layer.
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

// StoreErrorHandler receives notifications when job store operations fail.
type StoreErrorHandler func(ctx context.Context, storeErr *StoreError)

// JobManager manages the lifecycle of background jobs: submission, cancellation, retry, and querying.
type JobManager interface {
	Submit(ctx context.Context, cmd any, opts ...JobOption) (*Job, error)
	GetStatus(ctx context.Context, jobID string) (*Job, error)
	Cancel(ctx context.Context, jobID string) error
	Retry(ctx context.Context, jobID string) error
	Wait(ctx context.Context, jobID string, timeout time.Duration) (*Job, error)
	WaitForRunning(ctx context.Context, jobID string, timeout time.Duration) (*Job, error)
	ListByStatus(ctx context.Context, status JobStatus) ([]*Job, error)
	Shutdown(ctx context.Context) error
}

// TypeRegistry maps type names to reflect.Type for dynamic command instantiation.
type TypeRegistry struct {
	types map[string]reflect.Type
	mu    sync.RWMutex
}

// NewTypeRegistry creates an empty TypeRegistry.
func NewTypeRegistry() *TypeRegistry {
	return &TypeRegistry{types: make(map[string]reflect.Type)}
}

// Register adds a type to the registry using a sample instance.
func (r *TypeRegistry) Register(sample any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	t := reflect.TypeOf(sample)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	r.types[t.String()] = t
}

// NewInstance creates a new zero-value instance of the registered type.
func (r *TypeRegistry) NewInstance(typeName string) (any, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.types[typeName]
	if !ok {
		return nil, false
	}
	return reflect.New(t).Interface(), true
}

// TypeName returns the Go type name of v, dereferencing pointers.
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

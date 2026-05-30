# Job Field Privatization Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make all public fields on Job struct private, add getter methods, add state transition validation to RestoreJobState, and fix Snapshot to deep-copy Command.

**Architecture:** Privatize fields (ID→id, Command→command, etc.), add getter methods (ID(), Command(), etc.), validate state transitions in RestoreJobState, use JSON marshal/unmarshal for deep copy in Snapshot.

**Tech Stack:** Go, encoding/json for deep copy

---

## File Structure

**Files to modify:**
- `job/core/job.go` - Privatize fields, add getters, add RestoreJobState validation, fix Snapshot deep copy
- `job/core/job_test.go` - Update all references from direct field access to getter methods
- `job/core/jobtest/job_store_contract.go` - Update field references to getters
- `job/memory/job_manager.go` - Update all field references to getters
- `job/memory/job_store.go` - Update all field references to getters
- `job/pg/job_store.go` - Update all field references to getters

---

### Task 1: Privatize fields and add getters in job.go

**Files:**
- Modify: `job/core/job.go`

- [ ] **Step 1: Privatize fields in Job struct**

Change the Job struct fields from public to private:

```go
type Job struct {
	mu           sync.Mutex
	id           string
	command      any
	commandType  string
	status       JobStatus
	result       any
	resultType   string
	err          string
	createdAt    time.Time
	startedAt    time.Time
	completedAt  time.Time
	timeout      time.Duration
	retryCount   int
	maxRetries   int
	done         chan struct{}
}
```

- [ ] **Step 2: Update NewJob to use private fields**

```go
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
```

- [ ] **Step 3: Add getter methods for privatized fields**

Add these getters after the existing GetStatus() method:

```go
func (j *Job) ID() string            { return j.id }
func (j *Job) Command() any          { return j.command }
func (j *Job) CommandType() string   { return j.commandType }
func (j *Job) CreatedAt() time.Time  { return j.createdAt }
func (j *Job) Timeout() time.Duration { return j.timeout }
func (j *Job) RetryCount() int       { return j.retryCount }
func (j *Job) MaxRetries() int       { return j.maxRetries }
```

- [ ] **Step 4: Update WithTimeout and WithMaxRetries to use private fields**

```go
func WithTimeout(timeout time.Duration) JobOption {
	return func(j *Job) {
		j.timeout = timeout
	}
}

func WithMaxRetries(maxRetries int) JobOption {
	return func(j *Job) {
		j.maxRetries = maxRetries
	}
}
```

- [ ] **Step 5: Update TryFail to use private fields**

```go
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
```

- [ ] **Step 6: Update TryCancel to use private fields**

```go
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
```

- [ ] **Step 7: Update ResetForRetry to use private fields**

```go
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
```

- [ ] **Step 8: Add state transition validation to RestoreJobState**

Add a helper function and update RestoreJobState:

```go
var ErrInvalidStateTransition = errors.New("invalid state transition")

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
```

- [ ] **Step 9: Fix Snapshot to deep-copy Command using JSON**

Add import for `"encoding/json"` and update Snapshot:

```go
func (j *Job) Snapshot() *Job {
	j.mu.Lock()
	defer j.mu.Unlock()
	done := make(chan struct{})
	if j.status == JobStatusCompleted || j.status == JobStatusFailed || j.status == JobStatusCancelled {
		close(done)
	}
	var commandCopy any
	if j.command != nil {
		data, err := json.Marshal(j.command)
		if err == nil {
			json.Unmarshal(data, &commandCopy)
		} else {
			commandCopy = j.command
		}
	}
	return &Job{
		id:          j.id,
		command:     commandCopy,
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
```

- [ ] **Step 10: Run go build to verify compilation**

Run: `go build ./...`
Expected: No errors

---

### Task 2: Update job_test.go to use getters

**Files:**
- Modify: `job/core/job_test.go`

- [ ] **Step 1: Update TestWithTimeout to use getter**

```go
func TestWithTimeout(t *testing.T) {
	job := &Job{}
	opt := WithTimeout(5 * time.Second)
	opt(job)

	if job.Timeout() != 5*time.Second {
		t.Errorf("expected timeout 5s, got %v", job.Timeout())
	}
}
```

- [ ] **Step 2: Update TestWithMaxRetries to use getter**

```go
func TestWithMaxRetries(t *testing.T) {
	job := &Job{}
	opt := WithMaxRetries(3)
	opt(job)

	if job.MaxRetries() != 3 {
		t.Errorf("expected max retries 3, got %d", job.MaxRetries())
	}
}
```

- [ ] **Step 3: Update TestJobOptions_Combined to use getters**

```go
func TestJobOptions_Combined(t *testing.T) {
	job := &Job{}
	WithTimeout(10 * time.Second)(job)
	WithMaxRetries(5)(job)

	if job.Timeout() != 10*time.Second {
		t.Errorf("expected timeout 10s, got %v", job.Timeout())
	}
	if job.MaxRetries() != 5 {
		t.Errorf("expected max retries 5, got %d", job.MaxRetries())
	}
}
```

- [ ] **Step 4: Update TestJob_TryFail_WithRetry to use getter**

```go
func TestJob_TryFail_WithRetry(t *testing.T) {
	job := NewJob("job-1", nil, WithMaxRetries(2))
	job.MarkRunning()
	cancelled, shouldRetry := job.TryFail("fail")
	if cancelled {
		t.Error("expected not cancelled")
	}
	if !shouldRetry {
		t.Error("expected retry")
	}
	if job.RetryCount() != 1 {
		t.Errorf("expected retry count 1, got %d", job.RetryCount())
	}
}
```

- [ ] **Step 5: Update TestJob_Snapshot_IncludesCommandType to use getters**

```go
func TestJob_Snapshot_IncludesCommandType(t *testing.T) {
	job := &Job{
		command:     &testSampleCmd{Name: "test"},
		commandType: "core.testSampleCmd",
	}
	job.RestoreJobState("", &testSampleResult{File: "out.pdf"}, "core.testSampleResult", "", time.Time{}, time.Time{})
	snap := job.Snapshot()
	if snap.CommandType() != "core.testSampleCmd" {
		t.Errorf("expected CommandType preserved, got %q", snap.CommandType())
	}
	if snap.GetResultType() != "core.testSampleResult" {
		t.Errorf("expected ResultType preserved, got %q", snap.GetResultType())
	}
}
```

- [ ] **Step 6: Update TestNewJob to use getters**

```go
func TestNewJob(t *testing.T) {
	job := NewJob("j1", &testSampleCmd{Name: "test"}, WithTimeout(5*time.Second), WithMaxRetries(3))
	if job.ID() != "j1" {
		t.Errorf("expected ID 'j1', got %s", job.ID())
	}
	if job.GetStatus() != JobStatusPending {
		t.Errorf("expected pending, got %s", job.GetStatus())
	}
	if job.Timeout() != 5*time.Second {
		t.Errorf("expected timeout 5s, got %v", job.Timeout())
	}
	if job.MaxRetries() != 3 {
		t.Errorf("expected max retries 3, got %d", job.MaxRetries())
	}
	if job.CreatedAt().IsZero() {
		t.Error("expected createdAt to be set")
	}
}
```

- [ ] **Step 7: Run tests to verify**

Run: `go test ./job/core/...`
Expected: All tests pass

---

### Task 3: Update job_store_contract.go to use getters

**Files:**
- Modify: `job/core/jobtest/job_store_contract.go`

- [ ] **Step 1: Update CreateAndGet test to use getter**

```go
t.Run("CreateAndGet", func(t *testing.T) {
	store := newStore()
	job := jobcore.NewJob("contract-create-get", map[string]any{"action": "test"})
	if err := store.Create(ctx, job); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	got, err := store.Get(ctx, "contract-create-get")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got.ID() != "contract-create-get" {
		t.Errorf("expected ID 'contract-create-get', got %q", got.ID())
	}
	if got.GetStatus() != jobcore.JobStatusPending {
		t.Errorf("expected status pending, got %s", got.GetStatus())
	}
})
```

- [ ] **Step 2: Update UpdateNotFound test to use NewJob**

```go
t.Run("UpdateNotFound", func(t *testing.T) {
	store := newStore()
	job := jobcore.NewJob("contract-update-noexist", nil)
	job.RestoreJobState(jobcore.JobStatusRunning, nil, "", "", time.Time{}, time.Time{})
	if err := store.Update(ctx, job); err == nil {
		t.Fatal("expected error for updating nonexistent job")
	}
})
```

- [ ] **Step 3: Run tests to verify**

Run: `go test ./job/core/jobtest/...`
Expected: Tests pass (may need store implementations)

---

### Task 4: Update job_manager.go to use getters

**Files:**
- Modify: `job/memory/job_manager.go`

- [ ] **Step 1: Update Submit to use ID() getter**

```go
func (m *JobManager) Submit(ctx context.Context, cmd any, opts ...jobcore.JobOption) (*jobcore.Job, error) {
	jobID := uuid.New()
	job := jobcore.NewJob(hex.EncodeToString(jobID[:]), cmd, opts...)

	if err := m.store.Create(ctx, job); err != nil {
		return nil, err
	}

	m.mu.Lock()
	m.jobs[job.ID()] = job
	m.mu.Unlock()

	m.wg.Add(1)
	go m.executeJob(job, trace.GetTraceID(ctx), trace.GetSpanID(ctx))
	return job, nil
}
```

- [ ] **Step 2: Update executeJob to use getters**

```go
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
			m.handleStoreError(bgCtx, job.ID(), "update_running", err)
		}

		var execCtx context.Context
		var cancel context.CancelFunc

		if job.Timeout() > 0 {
			execCtx, cancel = context.WithTimeout(bgCtx, job.Timeout())
		} else {
			execCtx, cancel = context.WithCancel(bgCtx)
		}

		m.mu.Lock()
		m.cancelers[job.ID()] = cancel
		m.mu.Unlock()

		result, err := m.executor.Execute(execCtx, job.Command())
		cancel()

		m.mu.Lock()
		delete(m.cancelers, job.ID())
		m.mu.Unlock()

		if err != nil {
			cancelled, shouldRetry := job.TryFail(err.Error())
			if cancelled {
				job.MarkDone()
				break
			}
			if shouldRetry {
				if err := m.store.Update(bgCtx, job); err != nil {
					m.handleStoreError(bgCtx, job.ID(), "update_retry", err)
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
			m.handleStoreError(bgCtx, job.ID(), "update_final", err)
		}
		job.MarkDone()
		break
	}
}
```

- [ ] **Step 3: Update Cancel to use ID() getter**

```go
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
```

- [ ] **Step 4: Update Retry to use ID() getter**

```go
func (m *JobManager) Retry(ctx context.Context, jobID string) error {
	m.mu.Lock()
	liveJob, liveExists := m.jobs[jobID]
	m.mu.Unlock()

	if liveExists {
		select {
		case <-liveJob.Done():
		case <-ctx.Done():
			return ctx.Err()
		}

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
	m.jobs[job.ID()] = job
	m.mu.Unlock()

	m.wg.Add(1)
	go m.executeJob(job, trace.GetTraceID(ctx), trace.GetSpanID(ctx))
	return nil
}
```

- [ ] **Step 5: Update recoverJobs to use ID() getter**

```go
func (m *JobManager) recoverJobs() {
	ctx := context.Background()

	running, err := m.store.List(ctx, jobcore.JobStatusRunning)
	if err == nil {
		for _, job := range running {
		job.RestoreJobState(jobcore.JobStatusFailed, nil, "", "process restarted during execution", time.Time{}, time.Now())
			if updateErr := m.store.Update(ctx, job); updateErr != nil {
				m.handleStoreError(ctx, job.ID(), "recovery_running", updateErr)
			}
		}
	}

	pending, err := m.store.List(ctx, jobcore.JobStatusPending)
	if err == nil {
		for _, job := range pending {
			m.mu.Lock()
			m.jobs[job.ID()] = job
			m.mu.Unlock()

			m.wg.Add(1)
			go m.executeJob(job, "", "")
		}
	}
}
```

- [ ] **Step 6: Run go build to verify**

Run: `go build ./...`
Expected: No errors

---

### Task 5: Update job_store.go to use getters

**Files:**
- Modify: `job/memory/job_store.go`

- [ ] **Step 1: Update Create to use ID() getter**

```go
func (s *InMemoryJobStore) Create(ctx context.Context, job *jobcore.Job) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.jobs[job.ID()]; exists {
		return fmt.Errorf("job %s: %w", job.ID(), ddderror.ErrAlreadyExists)
	}
	s.jobs[job.ID()] = job.Snapshot()
	return nil
}
```

- [ ] **Step 2: Update Update to use ID() getter**

```go
func (s *InMemoryJobStore) Update(ctx context.Context, job *jobcore.Job) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.jobs[job.ID()]; !exists {
		return fmt.Errorf("job %s: %w", job.ID(), ddderror.ErrNotFound)
	}
	s.jobs[job.ID()] = job.Snapshot()
	return nil
}
```

- [ ] **Step 3: Run go build to verify**

Run: `go build ./...`
Expected: No errors

---

### Task 6: Update pg/job_store.go to use getters

**Files:**
- Modify: `job/pg/job_store.go`

- [ ] **Step 1: Update Create to use getters**

```go
func (s *PgJobStore) Create(ctx context.Context, job *jobcore.Job) error {
	q := corepg.GetQuerier(ctx, s.db)
	cmdData, err := json.Marshal(job.Command())
	if err != nil {
		return fmt.Errorf("marshal command: %w", err)
	}
	commandType := job.CommandType()
	if commandType == "" {
		commandType = jobcore.TypeName(job.Command())
	}
	resultData, err := corepg.JSONOrNull(job.GetResult())
	if err != nil {
		return fmt.Errorf("marshal result: %w", err)
	}
	_, err = q.ExecContext(ctx,
		`INSERT INTO ddd_jobs (id, command, command_type, status, result, result_type, error, created_at, started_at, completed_at, timeout_ns, retry_count, max_retries)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`,
		job.ID(), cmdData, commandType, string(job.GetStatus()),
		resultData, corepg.NullString(job.GetResultType()),
		corepg.NullString(job.GetError()),
		job.CreatedAt(), corepg.NullTime(job.GetStartedAt()), corepg.NullTime(job.GetCompletedAt()),
		job.Timeout().Nanoseconds(), job.RetryCount(), job.MaxRetries(),
	)
	return err
}
```

- [ ] **Step 2: Update Get to use setters for private fields**

Since fields are now private, we need to use a different approach. Create a new Job and use RestoreJobState:

```go
func (s *PgJobStore) Get(ctx context.Context, id string) (*jobcore.Job, error) {
	q := corepg.GetQuerier(ctx, s.db)
	row := q.QueryRowContext(ctx,
		`SELECT id, command, command_type, status, result, result_type, error, created_at, started_at, completed_at, timeout_ns, retry_count, max_retries
		 FROM ddd_jobs WHERE id = $1`, id,
	)
	var jobID string
	var cmdData []byte
	var resultData []byte
	var commandType string
	var status string
	var resultType sql.NullString
	var errStr sql.NullString
	var createdAt time.Time
	var startedAt, completedAt sql.NullTime
	var timeoutNs int64
	var retryCount, maxRetries int
	if err := row.Scan(&jobID, &cmdData, &commandType, &status, &resultData, &resultType, &errStr, &createdAt, &startedAt, &completedAt, &timeoutNs, &retryCount, &maxRetries); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("job %s: %w", id, ddderror.ErrNotFound)
		}
		return nil, err
	}
	timeout := time.Duration(timeoutNs)
	var startedAtVal, completedAtVal time.Time
	if startedAt.Valid {
		startedAtVal = startedAt.Time
	}
	if completedAt.Valid {
		completedAtVal = completedAt.Time
	}
	var result any
	var resultTypeStr string
	if resultType.Valid {
		resultTypeStr = resultType.String
	}
	var errStrVal string
	if errStr.Valid {
		errStrVal = errStr.String
	}
	if len(resultData) > 0 {
		resultTypeName := ""
		if resultType.Valid {
			resultTypeName = resultType.String
		}
		var err2 error
		result, err2 = s.unmarshalTyped(resultData, resultTypeName)
		if err2 != nil {
			return nil, fmt.Errorf("unmarshal result: %w", err2)
		}
	}
	var cmd any
	if len(cmdData) > 0 {
		var err error
		cmd, err = s.unmarshalTyped(cmdData, commandType)
		if err != nil {
			return nil, fmt.Errorf("unmarshal command: %w", err)
		}
	}
	job := jobcore.NewJob(jobID, cmd, jobcore.WithTimeout(timeout), jobcore.WithMaxRetries(maxRetries))
	job.RestoreJobState(jobcore.JobStatus(status), result, resultTypeStr, errStrVal, startedAtVal, completedAtVal)
	return job, nil
}
```

- [ ] **Step 3: Update Update to use getters**

```go
func (s *PgJobStore) Update(ctx context.Context, job *jobcore.Job) error {
	q := corepg.GetQuerier(ctx, s.db)
	resultType := job.GetResultType()
	if resultType == "" && job.GetResult() != nil {
		resultType = jobcore.TypeName(job.GetResult())
	}
	resultData, err := corepg.JSONOrNull(job.GetResult())
	if err != nil {
		return fmt.Errorf("marshal result: %w", err)
	}
	res, err := q.ExecContext(ctx,
		`UPDATE ddd_jobs SET status=$2, result=$3, result_type=$4, error=$5, started_at=$6, completed_at=$7, timeout_ns=$8, retry_count=$9, max_retries=$10
		 WHERE id=$1`,
		job.ID(), string(job.GetStatus()), resultData, corepg.NullString(resultType), corepg.NullString(job.GetError()),
		corepg.NullTime(job.GetStartedAt()), corepg.NullTime(job.GetCompletedAt()),
		job.Timeout().Nanoseconds(), job.RetryCount(), job.MaxRetries(),
	)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("check rows affected: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("job %s: %w", job.ID(), ddderror.ErrNotFound)
	}
	return nil
}
```

- [ ] **Step 4: Update List to use NewJob and getters**

```go
func (s *PgJobStore) List(ctx context.Context, status jobcore.JobStatus) ([]*jobcore.Job, error) {
	q := corepg.GetQuerier(ctx, s.db)
	rows, err := q.QueryContext(ctx,
		`SELECT id, command, command_type, status, result, result_type, error, created_at, started_at, completed_at, timeout_ns, retry_count, max_retries
		 FROM ddd_jobs WHERE status = $1`, string(status),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []*jobcore.Job
	for rows.Next() {
		var jobID string
		var cmdData []byte
		var resultData []byte
		var commandType string
		var statusStr string
		var resultType sql.NullString
		var errStr sql.NullString
		var createdAt time.Time
		var startedAt, completedAt sql.NullTime
		var timeoutNs int64
		var retryCount, maxRetries int
		if err := rows.Scan(&jobID, &cmdData, &commandType, &statusStr, &resultData, &resultType, &errStr, &createdAt, &startedAt, &completedAt, &timeoutNs, &retryCount, &maxRetries); err != nil {
			return nil, err
		}
		timeout := time.Duration(timeoutNs)
		var startedAtVal, completedAtVal time.Time
		if startedAt.Valid {
			startedAtVal = startedAt.Time
		}
		if completedAt.Valid {
			completedAtVal = completedAt.Time
		}
		var resultTypeStr string
		if resultType.Valid {
			resultTypeStr = resultType.String
		}
		var errStrVal string
		if errStr.Valid {
			errStrVal = errStr.String
		}
		var resultVal any
		if len(resultData) > 0 {
			resultTypeName := ""
			if resultType.Valid {
				resultTypeName = resultType.String
			}
			var err2 error
			resultVal, err2 = s.unmarshalTyped(resultData, resultTypeName)
			if err2 != nil {
				return nil, fmt.Errorf("unmarshal result for job %s: %w", jobID, err2)
			}
		}
		var cmd any
		if len(cmdData) > 0 {
			var err error
			cmd, err = s.unmarshalTyped(cmdData, commandType)
			if err != nil {
				return nil, fmt.Errorf("unmarshal command for job %s: %w", jobID, err)
			}
		}
		job := jobcore.NewJob(jobID, cmd, jobcore.WithTimeout(timeout), jobcore.WithMaxRetries(maxRetries))
		job.RestoreJobState(jobcore.JobStatus(statusStr), resultVal, resultTypeStr, errStrVal, startedAtVal, completedAtVal)
		result = append(result, job)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate jobs: %w", err)
	}
	return result, nil
}
```

- [ ] **Step 5: Run go build to verify**

Run: `go build ./...`
Expected: No errors

---

### Task 7: Run all tests and verify

**Files:**
- None (verification only)

- [ ] **Step 1: Run all job tests**

Run: `go test ./job/...`
Expected: All tests pass

- [ ] **Step 2: Run full build**

Run: `go build ./...`
Expected: No errors

---

## Self-Review Checklist

1. **Spec coverage:**
   - [x] Privatize fields: ID→id, Command→command, CommandType→commandType, CreatedAt→createdAt, Timeout→timeout, RetryCount→retryCount, MaxRetries→maxRetries
   - [x] Add getters: ID(), Command(), CommandType(), CreatedAt(), Timeout(), RetryCount(), MaxRetries()
   - [x] State transition validation in RestoreJobState
   - [x] Snapshot deep copy using JSON

2. **Placeholder scan:** No TBD, TODO, or placeholder patterns found.

3. **Type consistency:** All getter names and field names are consistent across tasks.

---

**Plan complete.** Two execution options:

1. **Subagent-Driven (recommended)** - I dispatch a fresh subagent per task, review between tasks, fast iteration

2. **Inline Execution** - Execute tasks in this session using executing-plans, batch execution with checkpoints

**Which approach?**

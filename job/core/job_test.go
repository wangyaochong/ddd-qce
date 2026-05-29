package core

import (
	"errors"
	"sync"
	"testing"
	"time"
)

func TestWithTimeout(t *testing.T) {
	job := &Job{}
	opt := WithTimeout(5 * time.Second)
	opt(job)

	if job.Timeout != 5*time.Second {
		t.Errorf("expected timeout 5s, got %v", job.Timeout)
	}
}

func TestWithMaxRetries(t *testing.T) {
	job := &Job{}
	opt := WithMaxRetries(3)
	opt(job)

	if job.MaxRetries != 3 {
		t.Errorf("expected max retries 3, got %d", job.MaxRetries)
	}
}

func TestJobOptions_Combined(t *testing.T) {
	job := &Job{}
	WithTimeout(10 * time.Second)(job)
	WithMaxRetries(5)(job)

	if job.Timeout != 10*time.Second {
		t.Errorf("expected timeout 10s, got %v", job.Timeout)
	}
	if job.MaxRetries != 5 {
		t.Errorf("expected max retries 5, got %d", job.MaxRetries)
	}
}

func TestJob_MarkRunning(t *testing.T) {
	job := NewJob("job-1", nil)
	if job.GetStatus() != JobStatusPending {
		t.Errorf("expected pending, got %s", job.GetStatus())
	}
	job.MarkRunning()
	if job.GetStatus() != JobStatusRunning {
		t.Errorf("expected running, got %s", job.GetStatus())
	}
	if job.GetStartedAt().IsZero() {
		t.Error("expected startedAt to be set")
	}
}

func TestJob_MarkRunning_Concurrent(t *testing.T) {
	job := NewJob("job-1", nil)
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			job.MarkRunning()
		}()
	}

	wg.Wait()

	if job.GetStatus() != JobStatusRunning {
		t.Errorf("expected status running, got %s", job.GetStatus())
	}
}

func TestJob_TryComplete(t *testing.T) {
	job := NewJob("job-1", nil)
	job.MarkRunning()
	if !job.TryComplete("result") {
		t.Error("expected TryComplete to succeed")
	}
	if job.GetStatus() != JobStatusCompleted {
		t.Errorf("expected completed, got %s", job.GetStatus())
	}
	if job.GetResult() != "result" {
		t.Errorf("expected result, got %v", job.GetResult())
	}
	if job.GetCompletedAt().IsZero() {
		t.Error("expected completedAt to be set")
	}
}

func TestJob_TryComplete_Cancelled(t *testing.T) {
	job := NewJob("job-1", nil)
	_ = job.TryCancel()
	if job.TryComplete("result") {
		t.Error("expected TryComplete to fail for cancelled job")
	}
	if job.GetStatus() != JobStatusCancelled {
		t.Errorf("expected cancelled, got %s", job.GetStatus())
	}
}

func TestJob_TryComplete_Pending(t *testing.T) {
	job := NewJob("job-1", nil, WithMaxRetries(1))
	job.MarkRunning()
	job.TryFail("fail")
	_ = job.ResetForRetry()
	if job.TryComplete("result") {
		t.Error("expected TryComplete to fail for pending job")
	}
	if job.GetStatus() != JobStatusPending {
		t.Errorf("expected pending, got %s", job.GetStatus())
	}
}

func TestJob_TryFail(t *testing.T) {
	job := NewJob("job-1", nil)
	job.MarkRunning()
	cancelled, shouldRetry := job.TryFail("something went wrong")
	if cancelled {
		t.Error("expected not cancelled")
	}
	if shouldRetry {
		t.Error("expected no retry (maxRetries=0)")
	}
	if job.GetStatus() != JobStatusFailed {
		t.Errorf("expected failed, got %s", job.GetStatus())
	}
	if job.GetError() != "something went wrong" {
		t.Errorf("expected error message, got %s", job.GetError())
	}
}

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
	if job.RetryCount != 1 {
		t.Errorf("expected retry count 1, got %d", job.RetryCount)
	}
}

func TestJob_TryFail_Cancelled(t *testing.T) {
	job := NewJob("job-1", nil)
	_ = job.TryCancel()
	cancelled, shouldRetry := job.TryFail("fail")
	if !cancelled {
		t.Error("expected cancelled")
	}
	if shouldRetry {
		t.Error("expected no retry for cancelled job")
	}
}

func TestJob_TryFail_Pending(t *testing.T) {
	job := NewJob("job-1", nil, WithMaxRetries(1))
	job.MarkRunning()
	job.TryFail("fail")
	_ = job.ResetForRetry()
	cancelled, shouldRetry := job.TryFail("fail again")
	if cancelled {
		t.Error("expected not cancelled for pending job")
	}
	if shouldRetry {
		t.Error("expected no retry for pending job")
	}
}

func TestJob_TryCancel(t *testing.T) {
	job := NewJob("job-1", nil)
	job.MarkRunning()
	if err := job.TryCancel(); err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if job.GetStatus() != JobStatusCancelled {
		t.Errorf("expected cancelled, got %s", job.GetStatus())
	}
	if job.GetCompletedAt().IsZero() {
		t.Error("expected completedAt to be set")
	}
}

func TestJob_TryCancel_AlreadyCompleted(t *testing.T) {
	job := NewJob("job-1", nil)
	job.MarkRunning()
	job.TryComplete("result")
	if err := job.TryCancel(); err == nil {
		t.Error("expected error when cancelling completed job")
	}
}

func TestJob_ResetForRetry(t *testing.T) {
	job := NewJob("job-1", nil, WithMaxRetries(1))
	job.MarkRunning()
	job.TryFail("fail")
	if err := job.ResetForRetry(); err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if job.GetStatus() != JobStatusPending {
		t.Errorf("expected pending, got %s", job.GetStatus())
	}
	if job.GetError() != "" {
		t.Errorf("expected empty error, got %s", job.GetError())
	}
	if job.GetResult() != nil {
		t.Errorf("expected nil result, got %v", job.GetResult())
	}
	if !job.GetStartedAt().IsZero() {
		t.Error("expected zero startedAt")
	}
	if !job.GetCompletedAt().IsZero() {
		t.Error("expected zero completedAt")
	}
}

func TestJob_ResetForRetry_NotFailed(t *testing.T) {
	job := NewJob("job-1", nil)
	if err := job.ResetForRetry(); err == nil {
		t.Error("expected error when resetting non-failed job")
	}
}

func TestJobStatus_Constants(t *testing.T) {
	statuses := []JobStatus{
		JobStatusPending,
		JobStatusRunning,
		JobStatusCompleted,
		JobStatusFailed,
		JobStatusCancelled,
	}

	expected := []string{"pending", "running", "completed", "failed", "cancelled"}

	for i, s := range statuses {
		if string(s) != expected[i] {
			t.Errorf("expected status %q, got %q", expected[i], s)
		}
	}
}

func TestTypeRegistry_RegisterAndNew(t *testing.T) {
	reg := NewTypeRegistry()
	reg.Register(&testSampleCmd{})

	inst, ok := reg.NewInstance("core.testSampleCmd")
	if !ok {
		t.Fatal("expected type to be found")
	}
	if _, ok := inst.(*testSampleCmd); !ok {
		t.Fatalf("expected *testSampleCmd, got %T", inst)
	}
}

func TestTypeRegistry_NotFound(t *testing.T) {
	reg := NewTypeRegistry()
	_, ok := reg.NewInstance("nonexistent")
	if ok {
		t.Error("expected not found")
	}
}

func TestTypeRegistry_ValueType(t *testing.T) {
	reg := NewTypeRegistry()
	reg.Register(testSampleCmd{})

	inst, ok := reg.NewInstance("core.testSampleCmd")
	if !ok {
		t.Fatal("expected type to be found")
	}
	if _, ok := inst.(*testSampleCmd); !ok {
		t.Fatalf("expected *testSampleCmd, got %T", inst)
	}
}

func TestTypeRegistry_MultipleTypes(t *testing.T) {
	reg := NewTypeRegistry()
	reg.Register(&testSampleCmd{})
	reg.Register(&testSampleResult{})

	_, ok1 := reg.NewInstance("core.testSampleCmd")
	_, ok2 := reg.NewInstance("core.testSampleResult")
	if !ok1 || !ok2 {
		t.Error("expected both types to be found")
	}
}

func TestTypeName(t *testing.T) {
	if got := TypeName(&testSampleCmd{}); got != "core.testSampleCmd" {
		t.Errorf("expected 'core.testSampleCmd', got %q", got)
	}
	if got := TypeName(testSampleCmd{}); got != "core.testSampleCmd" {
		t.Errorf("expected 'core.testSampleCmd', got %q", got)
	}
	if got := TypeName(nil); got != "" {
		t.Errorf("expected empty string for nil, got %q", got)
	}
}

func TestJob_Snapshot_IncludesCommandType(t *testing.T) {
	job := &Job{
		ID:          "j1",
		Command:     &testSampleCmd{Name: "test"},
		CommandType: "core.testSampleCmd",
	}
	job.RestoreJobState("", &testSampleResult{File: "out.pdf"}, "core.testSampleResult", "", time.Time{}, time.Time{})
	snap := job.Snapshot()
	if snap.CommandType != "core.testSampleCmd" {
		t.Errorf("expected CommandType preserved, got %q", snap.CommandType)
	}
	if snap.GetResultType() != "core.testSampleResult" {
		t.Errorf("expected ResultType preserved, got %q", snap.GetResultType())
	}
}

func TestJob_Snapshot_DoneChannelIndependence(t *testing.T) {
	job := NewJob("job-1", nil)
	snap := job.Snapshot()

	job.MarkDone()

	select {
	case <-snap.Done():
		t.Error("snapshot done channel should not be closed when original is marked done")
	default:
	}
}

func TestJob_Snapshot_DoneChannelClosedForCompletedJob(t *testing.T) {
	job := NewJob("job-1", nil)
	job.MarkRunning()
	job.TryComplete("result")

	snap := job.Snapshot()
	select {
	case <-snap.Done():
	default:
		t.Error("snapshot done channel should be closed for completed job")
	}
}

func TestNewJob(t *testing.T) {
	job := NewJob("j1", &testSampleCmd{Name: "test"}, WithTimeout(5*time.Second), WithMaxRetries(3))
	if job.ID != "j1" {
		t.Errorf("expected ID 'j1', got %s", job.ID)
	}
	if job.GetStatus() != JobStatusPending {
		t.Errorf("expected pending, got %s", job.GetStatus())
	}
	if job.Timeout != 5*time.Second {
		t.Errorf("expected timeout 5s, got %v", job.Timeout)
	}
	if job.MaxRetries != 3 {
		t.Errorf("expected max retries 3, got %d", job.MaxRetries)
	}
	if job.CreatedAt.IsZero() {
		t.Error("expected createdAt to be set")
	}
}

type testSampleCmd struct {
	Name string
}

type testSampleResult struct {
	File string
}

func TestJob_ResetDone(t *testing.T) {
	job := NewJob("job-1", nil)
	oldDone := job.Done()
	job.MarkDone()

	select {
	case <-oldDone:
	default:
		t.Fatal("expected old done channel to be closed after MarkDone")
	}

	job.ResetDone()
	newDone := job.Done()

	select {
	case <-newDone:
		t.Error("expected new done channel to be open after ResetDone")
	default:
	}

	select {
	case <-oldDone:
	default:
		t.Error("old done channel should still be closed")
	}
}

func TestJob_Done_CreatesChannel(t *testing.T) {
	job := &Job{}
	done := job.Done()
	if done == nil {
		t.Error("expected Done() to create a channel")
	}
}

func TestJob_MarkDone_AlreadyClosed(t *testing.T) {
	job := NewJob("job-1", nil)
	job.MarkDone()
	job.MarkDone()

	select {
	case <-job.Done():
	default:
		t.Error("expected done channel to be closed")
	}
}

func TestStoreError_Error(t *testing.T) {
	inner := errors.New("db connection lost")
	storeErr := &StoreError{
		JobID:     "job-1",
		Operation: "create",
		Err:       inner,
	}

	expected := "store create failed for job job-1: db connection lost"
	if storeErr.Error() != expected {
		t.Errorf("Error() = %q, want %q", storeErr.Error(), expected)
	}
}

func TestStoreError_Unwrap(t *testing.T) {
	inner := errors.New("db connection lost")
	storeErr := &StoreError{
		JobID:     "job-1",
		Operation: "update",
		Err:       inner,
	}

	if !errors.Is(storeErr, inner) {
		t.Error("errors.Is should find the underlying error")
	}
}

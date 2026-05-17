package core

import (
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

func TestJob_LockUnlock(t *testing.T) {
	job := &Job{ID: "job-1", Status: JobStatusPending}

	job.Lock()
	job.Status = JobStatusRunning
	job.Unlock()

	if job.Status != JobStatusRunning {
		t.Errorf("expected status running, got %s", job.Status)
	}
}

func TestJob_LockUnlock_Concurrent(t *testing.T) {
	job := &Job{ID: "job-1", Status: JobStatusPending}
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			job.Lock()
			job.Status = JobStatusRunning
			job.Result = n
			job.Unlock()
		}(i)
	}

	wg.Wait()

	if job.Status != JobStatusRunning {
		t.Errorf("expected status running, got %s", job.Status)
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

package memory

import (
	"context"
	"testing"
	"time"

	jobcore "github.com/ddd-qce/core/job/core"
	"github.com/ddd-qce/core/job/core/jobtest"
)

type testJobCommand struct {
	Name string
}

func TestJobStore_Create(t *testing.T) {
	store := NewJobStore()
	ctx := context.Background()

	job := jobcore.NewJob("job-1", &testJobCommand{Name: "test"})

	err := store.Create(ctx, job)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestJobStore_CreateDuplicate(t *testing.T) {
	store := NewJobStore()
	ctx := context.Background()

	job := jobcore.NewJob("job-1", &testJobCommand{Name: "test"})

	err := store.Create(ctx, job)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = store.Create(ctx, job)
	if err == nil {
		t.Fatal("expected error for duplicate job")
	}
}

func TestJobStore_Get(t *testing.T) {
	store := NewJobStore()
	ctx := context.Background()

	job := jobcore.NewJob("job-1", &testJobCommand{Name: "test"})

	err := store.Create(ctx, job)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	retrieved, err := store.Get(ctx, "job-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if retrieved.ID != "job-1" {
		t.Errorf("expected ID 'job-1', got '%s'", retrieved.ID)
	}
	if retrieved.Command.(*testJobCommand).Name != "test" {
		t.Errorf("expected command name 'test', got '%s'", retrieved.Command.(*testJobCommand).Name)
	}
}

func TestJobStore_GetNotFound(t *testing.T) {
	store := NewJobStore()
	ctx := context.Background()

	_, err := store.Get(ctx, "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent job")
	}
}

func TestJobStore_Update(t *testing.T) {
	store := NewJobStore()
	ctx := context.Background()

	job := jobcore.NewJob("job-1", &testJobCommand{Name: "test"})

	err := store.Create(ctx, job)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	job.RestoreJobState(jobcore.JobStatusRunning, nil, "", "", time.Time{}, time.Time{})

	err = store.Update(ctx, job)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	retrieved, err := store.Get(ctx, "job-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if retrieved.GetStatus() != jobcore.JobStatusRunning {
		t.Errorf("expected status 'running', got '%s'", retrieved.GetStatus())
	}
}

func TestJobStore_UpdateNotFound(t *testing.T) {
	store := NewJobStore()
	ctx := context.Background()

	job := &jobcore.Job{
		ID: "nonexistent",
	}
	job.RestoreJobState(jobcore.JobStatusRunning, nil, "", "", time.Time{}, time.Time{})

	err := store.Update(ctx, job)
	if err == nil {
		t.Fatal("expected error for updating nonexistent job")
	}
}

func TestJobStore_Delete(t *testing.T) {
	store := NewJobStore()
	ctx := context.Background()

	job := jobcore.NewJob("job-1", &testJobCommand{Name: "test"})

	err := store.Create(ctx, job)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = store.Delete(ctx, "job-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = store.Get(ctx, "job-1")
	if err == nil {
		t.Fatal("expected error after delete")
	}
}

func TestJobStore_DeleteNotFound(t *testing.T) {
	store := NewJobStore()
	ctx := context.Background()

	err := store.Delete(ctx, "nonexistent")
	if err == nil {
		t.Fatal("expected error for deleting nonexistent job")
	}
}

func TestJobStore_List(t *testing.T) {
	store := NewJobStore()
	ctx := context.Background()

	jobs := []*jobcore.Job{
		jobcore.NewJob("job-1", nil),
		jobcore.NewJob("job-2", nil),
		jobcore.NewJob("job-3", nil),
		jobcore.NewJob("job-4", nil),
	}
	jobs[1].RestoreJobState(jobcore.JobStatusRunning, nil, "", "", time.Time{}, time.Time{})
	jobs[3].RestoreJobState(jobcore.JobStatusCompleted, nil, "", "", time.Time{}, time.Time{})

	for _, job := range jobs {
		err := store.Create(ctx, job)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}

	pending, err := store.List(ctx, jobcore.JobStatusPending)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pending) != 2 {
		t.Errorf("expected 2 pending jobs, got %d", len(pending))
	}

	running, err := store.List(ctx, jobcore.JobStatusRunning)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(running) != 1 {
		t.Errorf("expected 1 running job, got %d", len(running))
	}

	completed, err := store.List(ctx, jobcore.JobStatusCompleted)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(completed) != 1 {
		t.Errorf("expected 1 completed job, got %d", len(completed))
	}

	failed, err := store.List(ctx, jobcore.JobStatusFailed)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(failed) != 0 {
		t.Errorf("expected 0 failed jobs, got %d", len(failed))
	}
}

func TestJobStore_Contract(t *testing.T) {
	jobtest.TestJobStoreContract(t, func() jobcore.JobStore { return NewJobStore() })
}

func TestJobStore_Concurrent(t *testing.T) {
	store := NewJobStore()
	ctx := context.Background()

	job := jobcore.NewJob("job-1", &testJobCommand{Name: "test"})

	err := store.Create(ctx, job)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	done := make(chan bool)
	for i := 0; i < 50; i++ {
		go func() {
			store.Get(ctx, "job-1")
			store.Update(ctx, job)
			store.List(ctx, jobcore.JobStatusPending)
			done <- true
		}()
	}

	for i := 0; i < 50; i++ {
		<-done
	}
}

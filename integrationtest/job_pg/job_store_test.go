package pg

import (
	"context"
	"fmt"
	"testing"
	"time"

	jobcore "github.com/ddd-qce/core/job/core"
	"github.com/ddd-qce/core/job/core/jobtest"
	pgjob "github.com/ddd-qce/core/job/pg"
	"github.com/ddd-qce/integrationtest/testutil"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestPgJobStore_Contract(t *testing.T) {
	db := testutil.OpenTestDB(t)
	jobtest.TestJobStoreContract(t, func() jobcore.JobStore {
		testutil.CleanDB(t, db)
		return pgjob.NewJobStore(db)
	})
}

func TestPgJobStore_CreateAndGet(t *testing.T) {
	db := testutil.OpenTestDB(t)
	store := pgjob.NewJobStore(db)
	ctx := context.Background()

	now := time.Now().Truncate(time.Microsecond)
	job := jobcore.NewJob("job-1", map[string]any{"action": "process"}, jobcore.WithMaxRetries(3))
	job.RestoreJobState(jobcore.JobStatusPending, nil, "", "", now, time.Time{})

	if err := store.Create(ctx, job); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	got, err := store.Get(ctx, "job-1")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got.ID() != "job-1" {
		t.Errorf("expected ID 'job-1', got %s", got.ID())
	}
	if got.GetStatus() != jobcore.JobStatusPending {
		t.Errorf("expected status pending, got %s", got.GetStatus())
	}
}

func TestPgJobStore_Update(t *testing.T) {
	db := testutil.OpenTestDB(t)
	store := pgjob.NewJobStore(db)
	ctx := context.Background()

	now := time.Now().Truncate(time.Microsecond)
	job := jobcore.NewJob("job-2", map[string]any{"action": "run"}, jobcore.WithMaxRetries(3), jobcore.WithTimeout(time.Minute))
	job.RestoreJobState(jobcore.JobStatusPending, nil, "", "", now, time.Time{})
	store.Create(ctx, job)

	job.RestoreJobState(jobcore.JobStatusRunning, nil, "", "", time.Now().Truncate(time.Microsecond), time.Time{})
	if err := store.Update(ctx, job); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	got, _ := store.Get(ctx, "job-2")
	if got.GetStatus() != jobcore.JobStatusRunning {
		t.Errorf("expected status running, got %s", got.GetStatus())
	}
}

func TestPgJobStore_Delete(t *testing.T) {
	db := testutil.OpenTestDB(t)
	store := pgjob.NewJobStore(db)
	ctx := context.Background()

	job := jobcore.NewJob("job-3", "delete-me", jobcore.WithTimeout(time.Minute))
	job.RestoreJobState(jobcore.JobStatusPending, nil, "", "", time.Now(), time.Time{})
	store.Create(ctx, job)

	if err := store.Delete(ctx, "job-3"); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	_, err := store.Get(ctx, "job-3")
	if err == nil {
		t.Error("expected error after delete")
	}
}

func TestPgJobStore_DeleteNotFound(t *testing.T) {
	db := testutil.OpenTestDB(t)
	store := pgjob.NewJobStore(db)
	ctx := context.Background()

	err := store.Delete(ctx, "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent job")
	}
}

func TestPgJobStore_List(t *testing.T) {
	db := testutil.OpenTestDB(t)
	store := pgjob.NewJobStore(db)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		job := jobcore.NewJob(fmt.Sprintf("job-list-%d", i), "list-test", jobcore.WithTimeout(time.Minute))
		job.RestoreJobState(jobcore.JobStatusPending, nil, "", "", time.Now(), time.Time{})
		store.Create(ctx, job)
	}

	jobs, err := store.List(ctx, jobcore.JobStatusPending)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(jobs) != 3 {
		t.Errorf("expected 3 pending jobs, got %d", len(jobs))
	}
}

func TestPgJobStore_GetNotFound(t *testing.T) {
	db := testutil.OpenTestDB(t)
	store := pgjob.NewJobStore(db)
	ctx := context.Background()

	_, err := store.Get(ctx, "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent job")
	}
}

func TestRecordJobExecution(t *testing.T) {
	db := testutil.OpenTestDB(t)
	ctx := context.Background()

	started := time.Now().Truncate(time.Microsecond)
	err := pgjob.RecordJobExecution(ctx, db, "job-exec-1", 1, "success", nil, started)
	if err != nil {
		t.Fatalf("RecordJobExecution failed: %v", err)
	}

	var status string
	db.QueryRow("SELECT status FROM ddd_job_execution_log WHERE job_id = $1", "job-exec-1").Scan(&status)
	if status != "success" {
		t.Errorf("expected status 'success', got %s", status)
	}
}

type testGenReportCmd struct {
	Duration time.Duration
}

type testGenReportResult struct {
	File string
}

func TestPgJobStore_TypedRoundTrip(t *testing.T) {
	db := testutil.OpenTestDB(t)
	reg := jobcore.NewTypeRegistry()
	reg.Register(&testGenReportCmd{})
	reg.Register(&testGenReportResult{})
	store := pgjob.NewJobStore(db, pgjob.WithTypeRegistry(reg))
	ctx := context.Background()

	cmd := &testGenReportCmd{Duration: 5 * time.Second}
	job := jobcore.NewJob("job-typed-1", cmd, jobcore.WithTimeout(time.Minute))
	job.RestoreJobState(jobcore.JobStatusPending, nil, "", "", time.Now().Truncate(time.Microsecond), time.Time{})

	if err := store.Create(ctx, job); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	got, err := store.Get(ctx, "job-typed-1")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	typedCmd, ok := got.Command().(*testGenReportCmd)
	if !ok {
		t.Fatalf("expected *testGenReportCmd, got %T", got.Command())
	}
	if typedCmd.Duration != 5*time.Second {
		t.Errorf("expected duration 5s, got %v", typedCmd.Duration)
	}
}

func TestPgJobStore_TypedResultRoundTrip(t *testing.T) {
	db := testutil.OpenTestDB(t)
	reg := jobcore.NewTypeRegistry()
	reg.Register(&testGenReportCmd{})
	reg.Register(&testGenReportResult{})
	store := pgjob.NewJobStore(db, pgjob.WithTypeRegistry(reg))
	ctx := context.Background()

	cmd := &testGenReportCmd{Duration: 3 * time.Second}
	job := jobcore.NewJob("job-typed-2", cmd, jobcore.WithTimeout(time.Minute))
	job.RestoreJobState(jobcore.JobStatusPending, nil, "", "", time.Now().Truncate(time.Microsecond), time.Time{})
	store.Create(ctx, job)

	job.RestoreJobState(jobcore.JobStatusCompleted, &testGenReportResult{File: "report.pdf"}, "", "", time.Time{}, time.Time{})
	if err := store.Update(ctx, job); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	got, err := store.Get(ctx, "job-typed-2")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	typedResult, ok := got.GetResult().(*testGenReportResult)
	if !ok {
		t.Fatalf("expected *testGenReportResult, got %T", got.GetResult())
	}
	if typedResult.File != "report.pdf" {
		t.Errorf("expected file 'report.pdf', got %s", typedResult.File)
	}
}

func TestPgJobStore_WithoutRegistry_Fallback(t *testing.T) {
	db := testutil.OpenTestDB(t)
	store := pgjob.NewJobStore(db)
	ctx := context.Background()

	cmd := &testGenReportCmd{Duration: 2 * time.Second}
	job := jobcore.NewJob("job-fallback-1", cmd, jobcore.WithTimeout(time.Minute))
	job.RestoreJobState(jobcore.JobStatusPending, nil, "", "", time.Now().Truncate(time.Microsecond), time.Time{})
	store.Create(ctx, job)

	got, err := store.Get(ctx, "job-fallback-1")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if _, ok := got.Command().(*testGenReportCmd); ok {
		t.Error("expected fallback to map[string]any without registry")
	}
	if _, ok := got.Command().(map[string]any); !ok {
		t.Errorf("expected map[string]any fallback, got %T", got.Command())
	}
}

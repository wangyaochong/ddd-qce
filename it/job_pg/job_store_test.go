package pg

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	jobcore "github.com/ddd-qce/core/job/core"
	pgjob "github.com/ddd-qce/core/job/pg"
	corepg "github.com/ddd-qce/core/pg"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func openTestDBForJob(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("TEST_PG_DSN")
	if dsn == "" {
		dsn = "host=/var/run/postgresql dbname=ddd_qce_job_test user=root password=root sslmode=disable"
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open db failed: %v", err)
	}
	if err := db.Ping(); err != nil {
		t.Fatalf("ping db failed: %v", err)
	}
	t.Cleanup(func() {
		corepg.DropAll(db)
		db.Close()
	})
	if err := corepg.Migrate(db); err != nil {
		t.Fatalf("migrate failed: %v", err)
	}
	return db
}

func TestPgJobStore_CreateAndGet(t *testing.T) {
	db := openTestDBForJob(t)
	store := pgjob.NewJobStore(db)
	ctx := context.Background()

	now := time.Now().Truncate(time.Microsecond)
	job := &jobcore.Job{
		ID:         "job-1",
		Command:    map[string]any{"action": "process"},
		Status:     jobcore.JobStatusPending,
		CreatedAt:  now,
		Timeout:    30 * time.Second,
		MaxRetries: 3,
		RetryCount: 0,
	}

	if err := store.Create(ctx, job); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	got, err := store.Get(ctx, "job-1")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got.ID != "job-1" {
		t.Errorf("expected ID 'job-1', got %s", got.ID)
	}
	if got.Status != jobcore.JobStatusPending {
		t.Errorf("expected status pending, got %s", got.Status)
	}
}

func TestPgJobStore_Update(t *testing.T) {
	db := openTestDBForJob(t)
	store := pgjob.NewJobStore(db)
	ctx := context.Background()

	job := &jobcore.Job{
		ID:         "job-2",
		Command:    map[string]any{"action": "run"},
		Status:     jobcore.JobStatusPending,
		CreatedAt:  time.Now().Truncate(time.Microsecond),
		Timeout:    time.Minute,
		MaxRetries: 3,
	}
	store.Create(ctx, job)

	job.Status = jobcore.JobStatusRunning
	job.StartedAt = time.Now().Truncate(time.Microsecond)
	if err := store.Update(ctx, job); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	got, _ := store.Get(ctx, "job-2")
	if got.Status != jobcore.JobStatusRunning {
		t.Errorf("expected status running, got %s", got.Status)
	}
}

func TestPgJobStore_Delete(t *testing.T) {
	db := openTestDBForJob(t)
	store := pgjob.NewJobStore(db)
	ctx := context.Background()

	job := &jobcore.Job{
		ID: "job-3", Command: "delete-me", Status: jobcore.JobStatusPending,
		CreatedAt: time.Now(), Timeout: time.Minute, MaxRetries: 0,
	}
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
	db := openTestDBForJob(t)
	store := pgjob.NewJobStore(db)
	ctx := context.Background()

	err := store.Delete(ctx, "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent job")
	}
}

func TestPgJobStore_List(t *testing.T) {
	db := openTestDBForJob(t)
	store := pgjob.NewJobStore(db)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		job := &jobcore.Job{
			ID: fmt.Sprintf("job-list-%d", i), Command: "list-test",
			Status: jobcore.JobStatusPending, CreatedAt: time.Now(),
			Timeout: time.Minute, MaxRetries: 0,
		}
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
	db := openTestDBForJob(t)
	store := pgjob.NewJobStore(db)
	ctx := context.Background()

	_, err := store.Get(ctx, "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent job")
	}
}

func TestRecordJobExecution(t *testing.T) {
	db := openTestDBForJob(t)
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
	db := openTestDBForJob(t)
	reg := jobcore.NewTypeRegistry()
	reg.Register(&testGenReportCmd{})
	reg.Register(&testGenReportResult{})
	store := pgjob.NewJobStore(db, pgjob.WithTypeRegistry(reg))
	ctx := context.Background()

	cmd := &testGenReportCmd{Duration: 5 * time.Second}
	job := &jobcore.Job{
		ID:        "job-typed-1",
		Command:   cmd,
		Status:    jobcore.JobStatusPending,
		CreatedAt: time.Now().Truncate(time.Microsecond),
		Timeout:   time.Minute,
	}

	if err := store.Create(ctx, job); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	got, err := store.Get(ctx, "job-typed-1")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	typedCmd, ok := got.Command.(*testGenReportCmd)
	if !ok {
		t.Fatalf("expected *testGenReportCmd, got %T", got.Command)
	}
	if typedCmd.Duration != 5*time.Second {
		t.Errorf("expected duration 5s, got %v", typedCmd.Duration)
	}
}

func TestPgJobStore_TypedResultRoundTrip(t *testing.T) {
	db := openTestDBForJob(t)
	reg := jobcore.NewTypeRegistry()
	reg.Register(&testGenReportCmd{})
	reg.Register(&testGenReportResult{})
	store := pgjob.NewJobStore(db, pgjob.WithTypeRegistry(reg))
	ctx := context.Background()

	cmd := &testGenReportCmd{Duration: 3 * time.Second}
	job := &jobcore.Job{
		ID:        "job-typed-2",
		Command:   cmd,
		Status:    jobcore.JobStatusPending,
		CreatedAt: time.Now().Truncate(time.Microsecond),
		Timeout:   time.Minute,
	}
	store.Create(ctx, job)

	job.Status = jobcore.JobStatusCompleted
	job.Result = &testGenReportResult{File: "report.pdf"}
	if err := store.Update(ctx, job); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	got, err := store.Get(ctx, "job-typed-2")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	typedResult, ok := got.Result.(*testGenReportResult)
	if !ok {
		t.Fatalf("expected *testGenReportResult, got %T", got.Result)
	}
	if typedResult.File != "report.pdf" {
		t.Errorf("expected file 'report.pdf', got %s", typedResult.File)
	}
}

func TestPgJobStore_WithoutRegistry_Fallback(t *testing.T) {
	db := openTestDBForJob(t)
	store := pgjob.NewJobStore(db)
	ctx := context.Background()

	cmd := &testGenReportCmd{Duration: 2 * time.Second}
	job := &jobcore.Job{
		ID:        "job-fallback-1",
		Command:   cmd,
		Status:    jobcore.JobStatusPending,
		CreatedAt: time.Now().Truncate(time.Microsecond),
		Timeout:   time.Minute,
	}
	store.Create(ctx, job)

	got, err := store.Get(ctx, "job-fallback-1")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if _, ok := got.Command.(*testGenReportCmd); ok {
		t.Error("expected fallback to map[string]any without registry")
	}
	if _, ok := got.Command.(map[string]any); !ok {
		t.Errorf("expected map[string]any fallback, got %T", got.Command)
	}
}

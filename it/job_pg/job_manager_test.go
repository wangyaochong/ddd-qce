package pg

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/ddd-qce/core/aspect"
	"github.com/ddd-qce/core/cqrs/cmd"
	commandmemory "github.com/ddd-qce/core/cqrs/impl/memory"
	jobcore "github.com/ddd-qce/core/job/core"
	"github.com/ddd-qce/core/job/memory"
	pgjob "github.com/ddd-qce/core/job/pg"
	"github.com/ddd-qce/it/testutil"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func openTestDBForJobManager(t *testing.T) *sql.DB {
	return testutil.OpenTestDB(t, "ddd_qce_job_mgr_test")
}

type testReportCommand struct {
	command.BaseCommand
	Value string
}

type testReportResult struct {
	Message string
}

type testReportHandler struct{}

func (h *testReportHandler) Handle(ctx context.Context, cmd *testReportCommand) (*testReportResult, error) {
	return &testReportResult{Message: "completed"}, nil
}

func TestPgJobManager_Recovery_PendingReExecuted(t *testing.T) {
	db := openTestDBForJobManager(t)
	registry := jobcore.NewTypeRegistry()
	registry.Register(&testReportCommand{})

	store := pgjob.NewJobStore(db, pgjob.WithTypeRegistry(registry))
	ctx := context.Background()

	cmd := &testReportCommand{Value: "recovery-test"}
	pendingJob := jobcore.NewJob("pg-recover-pending", cmd)
	pendingJob.SetStatus(jobcore.JobStatusPending)
	pendingJob.CommandType = jobcore.TypeName(cmd)
	if err := store.Create(ctx, pendingJob); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	chain := aspect.NewAspectChain()
	cmdBus := commandmemory.NewCommandBus(commandmemory.WithCommandBusAspectChain(chain))
	commandmemory.RegisterCommand(cmdBus, &testReportHandler{})

	manager := memory.NewJobManager(store, cmdBus, memory.WithRecovery())
	defer manager.Shutdown(context.Background())

	time.Sleep(500 * time.Millisecond)

	status, err := manager.GetStatus(ctx, "pg-recover-pending")
	if err != nil {
		t.Fatalf("GetStatus failed: %v", err)
	}
	t.Logf("Job status: %s, error: %q, result: %v", status.GetStatus(), status.GetError(), status.GetResult())
	if status.GetStatus() != jobcore.JobStatusCompleted {
		t.Errorf("expected completed after recovery, got %s, error: %q", status.GetStatus(), status.GetError())
	}
}

func TestPgJobManager_Recovery_RunningMarkedFailed(t *testing.T) {
	db := openTestDBForJobManager(t)
	registry := jobcore.NewTypeRegistry()
	registry.Register(&testReportCommand{})
	store := pgjob.NewJobStore(db, pgjob.WithTypeRegistry(registry))
	ctx := context.Background()

	cmd := &testReportCommand{Value: "recovery-test"}
	runningJob := jobcore.NewJob("pg-recover-running", cmd)
	runningJob.SetStatus(jobcore.JobStatusRunning)
	runningJob.CommandType = jobcore.TypeName(cmd)
	if err := store.Create(ctx, runningJob); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	chain := aspect.NewAspectChain()
	cmdBus := commandmemory.NewCommandBus(commandmemory.WithCommandBusAspectChain(chain))
	commandmemory.RegisterCommand(cmdBus, &testReportHandler{})

	manager := memory.NewJobManager(store, cmdBus, memory.WithRecovery())
	defer manager.Shutdown(context.Background())

	time.Sleep(100 * time.Millisecond)

	status, err := manager.GetStatus(ctx, "pg-recover-running")
	if err != nil {
		t.Fatalf("GetStatus failed: %v", err)
	}
	if status.GetStatus() != jobcore.JobStatusFailed {
		t.Errorf("expected failed after recovery, got %s", status.GetStatus())
	}
	if status.GetError() == "" {
		t.Error("expected error message for recovered running job")
	}
}

func TestPgJobManager_Recovery_FailedWithRetry(t *testing.T) {
	db := openTestDBForJobManager(t)
	registry := jobcore.NewTypeRegistry()
	registry.Register(&testReportCommand{})
	store := pgjob.NewJobStore(db, pgjob.WithTypeRegistry(registry))
	ctx := context.Background()

	cmd := &testReportCommand{Value: "retry-test"}
	failedJob := jobcore.NewJob("pg-recover-failed-retry", cmd)
	failedJob.SetStatus(jobcore.JobStatusFailed)
	failedJob.SetError("previous failure")
	failedJob.CommandType = jobcore.TypeName(cmd)
	failedJob.MaxRetries = 3
	failedJob.RetryCount = 1
	if err := store.Create(ctx, failedJob); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	chain := aspect.NewAspectChain()
	cmdBus := commandmemory.NewCommandBus(commandmemory.WithCommandBusAspectChain(chain))
	commandmemory.RegisterCommand(cmdBus, &testReportHandler{})

	manager := memory.NewJobManager(store, cmdBus, memory.WithRecovery())
	defer manager.Shutdown(context.Background())

	time.Sleep(200 * time.Millisecond)

	status, err := manager.GetStatus(ctx, "pg-recover-failed-retry")
	if err != nil {
		t.Fatalf("GetStatus failed: %v", err)
	}
	if status.GetStatus() != jobcore.JobStatusFailed {
		t.Errorf("expected failed (retry not automatic without explicit Retry call), got %s", status.GetStatus())
	}
}
package jobtest

import (
	"context"
	"testing"
	"time"

	jobcore "github.com/ddd-qce/core/job/core"
)

func TestJobStoreContract(t *testing.T, newStore func() jobcore.JobStore) {
	t.Helper()
	ctx := context.Background()

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

	t.Run("CreateDuplicate", func(t *testing.T) {
		store := newStore()
		job := jobcore.NewJob("contract-dup", map[string]any{"action": "test"})
		if err := store.Create(ctx, job); err != nil {
			t.Fatalf("first Create failed: %v", err)
		}
		if err := store.Create(ctx, job); err == nil {
			t.Fatal("expected error for duplicate job")
		}
	})

	t.Run("GetNotFound", func(t *testing.T) {
		store := newStore()
		_, err := store.Get(ctx, "contract-nonexistent")
		if err == nil {
			t.Fatal("expected error for nonexistent job")
		}
	})

	t.Run("Update", func(t *testing.T) {
		store := newStore()
		job := jobcore.NewJob("contract-update", map[string]any{"action": "test"})
		if err := store.Create(ctx, job); err != nil {
			t.Fatalf("Create failed: %v", err)
		}
		job.MarkRunning()
		if err := store.Update(ctx, job); err != nil {
			t.Fatalf("Update failed: %v", err)
		}
		got, err := store.Get(ctx, "contract-update")
		if err != nil {
			t.Fatalf("Get failed: %v", err)
		}
		if got.GetStatus() != jobcore.JobStatusRunning {
			t.Errorf("expected status running, got %s", got.GetStatus())
		}
	})

	t.Run("UpdateNotFound", func(t *testing.T) {
		store := newStore()
		job := jobcore.NewJob("contract-update-noexist", nil)
		job.RestoreJobState(jobcore.JobStatusRunning, nil, "", "", time.Time{}, time.Time{})
		if err := store.Update(ctx, job); err == nil {
			t.Fatal("expected error for updating nonexistent job")
		}
	})

	t.Run("Delete", func(t *testing.T) {
		store := newStore()
		job := jobcore.NewJob("contract-delete", map[string]any{"action": "test"})
		if err := store.Create(ctx, job); err != nil {
			t.Fatalf("Create failed: %v", err)
		}
		if err := store.Delete(ctx, "contract-delete"); err != nil {
			t.Fatalf("Delete failed: %v", err)
		}
		_, err := store.Get(ctx, "contract-delete")
		if err == nil {
			t.Fatal("expected error after delete")
		}
	})

	t.Run("DeleteNotFound", func(t *testing.T) {
		store := newStore()
		if err := store.Delete(ctx, "contract-del-noexist"); err == nil {
			t.Fatal("expected error for deleting nonexistent job")
		}
	})

	t.Run("ListByStatus", func(t *testing.T) {
		store := newStore()
		p1 := jobcore.NewJob("contract-list-p1", map[string]any{"action": "test"})
		p2 := jobcore.NewJob("contract-list-p2", map[string]any{"action": "test"})
		r1 := jobcore.NewJob("contract-list-r1", map[string]any{"action": "test"})
		r1.RestoreJobState(jobcore.JobStatusRunning, nil, "", "", time.Time{}, time.Time{})

		for _, j := range []*jobcore.Job{p1, p2, r1} {
			if err := store.Create(ctx, j); err != nil {
				t.Fatalf("Create failed: %v", err)
			}
		}

		pending, err := store.List(ctx, jobcore.JobStatusPending)
		if err != nil {
			t.Fatalf("List pending failed: %v", err)
		}
		if len(pending) != 2 {
			t.Errorf("expected 2 pending, got %d", len(pending))
		}

		running, err := store.List(ctx, jobcore.JobStatusRunning)
		if err != nil {
			t.Fatalf("List running failed: %v", err)
		}
		if len(running) != 1 {
			t.Errorf("expected 1 running, got %d", len(running))
		}

		completed, err := store.List(ctx, jobcore.JobStatusCompleted)
		if err != nil {
			t.Fatalf("List completed failed: %v", err)
		}
		if len(completed) != 0 {
			t.Errorf("expected 0 completed, got %d", len(completed))
		}
	})
}

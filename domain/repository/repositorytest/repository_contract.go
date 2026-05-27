package repositorytest

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	ddderror "github.com/ddd-qce/core/error"
	"github.com/ddd-qce/core/domain/aggregate"
	"github.com/ddd-qce/core/cqrs/event"
	"github.com/ddd-qce/core/domain/repository"
)

type TestAggregate struct {
	aggregate.AggregateRoot
	Name   string
	Amount int
}

func NewTestAggregate(id string) *TestAggregate {
	a := &TestAggregate{}
	ar, err := aggregate.NewAggregateRoot(id)
	if err != nil {
		panic(err)
	}
	a.AggregateRoot = *ar
	return a
}

func (a *TestAggregate) When(_ event.Event) error { return nil }

func (a *TestAggregate) Apply(ctx context.Context, evt event.Event) error {
	return aggregate.ApplyChange(a, ctx, evt)
}

func (a *TestAggregate) LoadFromHistory(events []event.Event) error {
	return aggregate.LoadFromHistory(a, events)
}

type testAggregateJSON struct {
	aggregate.AggregateRootJSON
	Name   string `json:"name"`
	Amount int   `json:"amount"`
}

func (a *TestAggregate) MarshalJSON() ([]byte, error) {
	return json.Marshal(testAggregateJSON{
		AggregateRootJSON: a.AggregateRoot.ToJSON(),
		Name:              a.Name,
		Amount:            a.Amount,
	})
}

func (a *TestAggregate) UnmarshalJSON(data []byte) error {
	var aux testAggregateJSON
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	a.AggregateRoot.FromJSON(aux.AggregateRootJSON)
	a.Name = aux.Name
	a.Amount = aux.Amount
	return nil
}

func TestRepositoryContract[T aggregate.AggregateRef](t *testing.T, repo repository.Repository[T], newAgg func(id string) T, setFields func(agg T)) {
	t.Helper()

	t.Run("SaveAndFindByID", func(t *testing.T) {
		ctx := context.Background()
		agg := newAgg("contract-1")
		if setFields != nil {
			setFields(agg)
		}
		if err := repo.Save(ctx, agg); err != nil {
			t.Fatalf("Save: %v", err)
		}

		found, err := repo.FindByID(ctx, "contract-1")
		if err != nil {
			t.Fatalf("FindByID: %v", err)
		}
		if found.GetAggregateRoot().ID() != "contract-1" {
			t.Errorf("GetID() = %q, want %q", found.GetAggregateRoot().ID(), "contract-1")
		}
	})

	t.Run("FindByID_NotFound", func(t *testing.T) {
		ctx := context.Background()
		_, err := repo.FindByID(ctx, "nonexistent")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !errors.Is(err, ddderror.ErrNotFound) {
			t.Errorf("error should wrap ErrNotFound, got: %v", err)
		}
	})

	t.Run("Delete", func(t *testing.T) {
		ctx := context.Background()
		agg := newAgg("contract-del")
		if err := repo.Save(ctx, agg); err != nil {
			t.Fatalf("Save: %v", err)
		}
		if err := repo.Delete(ctx, "contract-del"); err != nil {
			t.Fatalf("Delete: %v", err)
		}
		_, err := repo.FindByID(ctx, "contract-del")
		if !errors.Is(err, ddderror.ErrNotFound) {
			t.Errorf("after delete, FindByID should wrap ErrNotFound, got: %v", err)
		}
	})

	t.Run("Delete_NotFound", func(t *testing.T) {
		ctx := context.Background()
		err := repo.Delete(ctx, "nonexistent")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !errors.Is(err, ddderror.ErrNotFound) {
			t.Errorf("error should wrap ErrNotFound, got: %v", err)
		}
	})

	t.Run("OptimisticLock", func(t *testing.T) {
		ctx := context.Background()
		agg := newAgg("contract-lock")
		if err := repo.Save(ctx, agg); err != nil {
			t.Fatalf("first Save: %v", err)
		}

		loaded, err := repo.FindByID(ctx, "contract-lock")
		if err != nil {
			t.Fatalf("FindByID: %v", err)
		}

		duplicate := newAgg("contract-lock")
		err = repo.Save(ctx, duplicate)
		if err == nil {
			t.Fatal("expected optimistic lock error, got nil")
		}
		if !errors.Is(err, ddderror.ErrConcurrency) {
			t.Errorf("error should wrap ErrConcurrency, got: %v", err)
		}

		evt := event.NewBaseEvent("contract-lock", time.Now())
		if err := aggregate.ApplyChange(loaded, ctx, evt); err != nil {
			t.Fatalf("ApplyChange on loaded: %v", err)
		}
		if err := repo.Save(ctx, loaded); err != nil {
			t.Fatalf("update after failed duplicate Save: %v", err)
		}
	})

	t.Run("UpdateExistingAggregate", func(t *testing.T) {
		ctx := context.Background()
		agg := newAgg("contract-update")
		if setFields != nil {
			setFields(agg)
		}
		if err := repo.Save(ctx, agg); err != nil {
			t.Fatalf("first Save: %v", err)
		}

		loaded, err := repo.FindByID(ctx, "contract-update")
		if err != nil {
			t.Fatalf("FindByID: %v", err)
		}

		evt := event.NewBaseEvent("contract-update", time.Now())
		if err := aggregate.ApplyChange(loaded, ctx, evt); err != nil {
			t.Fatalf("ApplyChange: %v", err)
		}

		if err := repo.Save(ctx, loaded); err != nil {
			t.Fatalf("second Save: %v", err)
		}

		again, err := repo.FindByID(ctx, "contract-update")
		if err != nil {
			t.Fatalf("FindByID after update: %v", err)
		}
		if again.GetAggregateRoot().ID() != "contract-update" {
			t.Errorf("GetID() = %q, want %q", again.GetAggregateRoot().ID(), "contract-update")
		}
	})
}

func TestEventSourcingRepositoryContract[T aggregate.AggregateRef](t *testing.T, repo repository.EventSourcingRepository[T], newAgg func(id string) T, applyEvent func(agg T)) {
	t.Helper()

	t.Run("SaveAndLoad", func(t *testing.T) {
		ctx := context.Background()
		agg := newAgg("es-contract-1")
		if applyEvent != nil {
			applyEvent(agg)
		}
		if err := repo.Save(ctx, agg); err != nil {
			t.Fatalf("Save: %v", err)
		}

		loaded, err := repo.Load(ctx, "es-contract-1")
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if loaded.GetAggregateRoot().ID() != "es-contract-1" {
			t.Errorf("GetID() = %q, want %q", loaded.GetAggregateRoot().ID(), "es-contract-1")
		}
		if loaded.GetAggregateRoot().Version() != 1 {
			t.Errorf("Version() = %d, want %d", loaded.GetAggregateRoot().Version(), 1)
		}
	})

	t.Run("Load_NotFound", func(t *testing.T) {
		ctx := context.Background()
		_, err := repo.Load(ctx, "nonexistent")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !errors.Is(err, ddderror.ErrNotFound) {
			t.Errorf("error should wrap ErrNotFound, got: %v", err)
		}
	})
}

package application

import (
	"context"
	"fmt"
	"sync"

	eventmemory "github.com/ddd-qce/core/cqrs/event/memory"
	"github.com/ddd-qce/core/domain/repository"
	"github.com/ddd-qce/exampleapp/domain"
)

type OrderRepository struct {
	mu     sync.RWMutex
	orders map[string]*domain.Order
}

func NewOrderRepository() *OrderRepository {
	return &OrderRepository{orders: make(map[string]*domain.Order)}
}

func (r *OrderRepository) Save(ctx context.Context, order *domain.Order) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.orders[order.GetID()] = order
	return nil
}

func (r *OrderRepository) FindByID(ctx context.Context, id string) (*domain.Order, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	order, ok := r.orders[id]
	if !ok {
		return nil, fmt.Errorf("order %s not found", id)
	}
	return order, nil
}

func (r *OrderRepository) Delete(ctx context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.orders[id]; !ok {
		return fmt.Errorf("order %s not found", id)
	}
	delete(r.orders, id)
	return nil
}

func (r *OrderRepository) FindAll() []*domain.Order {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]*domain.Order, 0, len(r.orders))
	for _, o := range r.orders {
		result = append(result, o)
	}
	return result
}

var _ repository.Repository[*domain.Order] = (*OrderRepository)(nil)

type OrderEventSourcedRepository struct {
	eventStore *eventmemory.DomainEventStore
	orderRepo  *OrderRepository
}

func NewOrderEventSourcedRepository(
	eventStore *eventmemory.DomainEventStore,
	orderRepo *OrderRepository,
) *OrderEventSourcedRepository {
	return &OrderEventSourcedRepository{
		eventStore: eventStore,
		orderRepo:  orderRepo,
	}
}

func (r *OrderEventSourcedRepository) Save(ctx context.Context, order *domain.Order) error {
	uncommitted := order.UncommittedEvents()
	if len(uncommitted) > 0 {
		if err := r.eventStore.Append(ctx, order.GetID(), order.GetVersion()-len(uncommitted), uncommitted); err != nil {
			return err
		}
		order.MarkEventsAsCommitted()
	}
	return r.orderRepo.Save(ctx, order)
}

func (r *OrderEventSourcedRepository) Load(ctx context.Context, id string) (*domain.Order, error) {
	events, err := r.eventStore.Load(ctx, id, 0)
	if err != nil {
		return nil, err
	}
	if len(events) == 0 {
		return nil, fmt.Errorf("order %s not found in event store", id)
	}
	order := domain.NewOrderForReplay(id)
	order.LoadFromHistory(events)
	return order, nil
}

var _ repository.EventSourcingRepository[*domain.Order] = (*OrderEventSourcedRepository)(nil)

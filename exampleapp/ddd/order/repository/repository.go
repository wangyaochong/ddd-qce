package repository

import (
	"context"
	"fmt"
	"sync"

	"github.com/ddd-qce/core/cqrs/event"
	domainevent "github.com/ddd-qce/core/domain/event"
	"github.com/ddd-qce/core/domain/repository"
	orderdomain "github.com/ddd-qce/exampleapp/ddd/order/domain"
)

type OrderRepositoryAdapter interface {
	Save(ctx context.Context, order *orderdomain.Order) error
	FindByID(ctx context.Context, id string) (*orderdomain.Order, error)
	Delete(ctx context.Context, id string) error
	FindAll() []*orderdomain.Order
}

type OrderRepository struct {
	mu     sync.RWMutex
	orders map[string]*orderdomain.Order
}

func NewOrderRepository() *OrderRepository {
	return &OrderRepository{orders: make(map[string]*orderdomain.Order)}
}

func (r *OrderRepository) Save(ctx context.Context, order *orderdomain.Order) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.orders[order.ID()] = order
	return nil
}

func (r *OrderRepository) FindByID(ctx context.Context, id string) (*orderdomain.Order, error) {
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

func (r *OrderRepository) FindAll() []*orderdomain.Order {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]*orderdomain.Order, 0, len(r.orders))
	for _, o := range r.orders {
		result = append(result, o)
	}
	return result
}

var _ repository.Repository[*orderdomain.Order] = (*OrderRepository)(nil)

type OrderEventSourcedRepository struct {
	eventStore event.EventSourceStore[domainevent.Event]
	eventBus   event.EventBus
	orderRepo  OrderRepositoryAdapter
}

func NewOrderEventSourcedRepository(
	eventStore event.EventSourceStore[domainevent.Event],
	eventBus event.EventBus,
	orderRepo OrderRepositoryAdapter,
) *OrderEventSourcedRepository {
	return &OrderEventSourcedRepository{
		eventStore: eventStore,
		eventBus:   eventBus,
		orderRepo:  orderRepo,
	}
}

func (r *OrderEventSourcedRepository) Save(ctx context.Context, order *orderdomain.Order) error {
	uncommitted := order.UncommittedEvents()
	if len(uncommitted) > 0 {
		if err := r.eventStore.Append(ctx, order.ID(), order.Version()-len(uncommitted), uncommitted); err != nil {
			return err
		}
		order.MarkEventsAsCommitted()

		for _, evt := range uncommitted {
			if err := r.eventBus.Publish(ctx, evt); err != nil {
				return fmt.Errorf("publish event %T: %w", evt, err)
			}
		}
	}
	return r.orderRepo.Save(ctx, order)
}

func (r *OrderEventSourcedRepository) Load(ctx context.Context, id string) (*orderdomain.Order, error) {
	events, err := r.eventStore.Load(ctx, id, 0)
	if err != nil {
		return nil, err
	}
	if len(events) == 0 {
		return nil, fmt.Errorf("order %s not found in event store", id)
	}
	order := orderdomain.NewOrderForReplay(orderdomain.OrderID(id))
	if err := order.LoadFromHistory(events); err != nil {
		return nil, fmt.Errorf("load order %s from history: %w", id, err)
	}
	return order, nil
}

var _ repository.EventSourcingRepository[*orderdomain.Order] = (*OrderEventSourcedRepository)(nil)

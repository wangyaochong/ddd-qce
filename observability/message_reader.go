package observability

import (
	"context"
	"sync"
	"time"

	"github.com/ddd-qce/core/aspect/builtin"
)

type MessageFilter struct {
	Type        string    `json:"type,omitempty"`
	TraceID     string    `json:"traceID,omitempty"`
	AggregateID string    `json:"aggregateID,omitempty"`
	Status      string    `json:"status,omitempty"`
	Since       time.Time `json:"since,omitempty"`
	Limit       int       `json:"limit,omitempty"`
	Offset      int       `json:"offset,omitempty"`
}

type MessageStoreReader interface {
	QueryCommands(ctx context.Context, filter MessageFilter) ([]builtin.CommandEntry, error)
	QueryQueries(ctx context.Context, filter MessageFilter) ([]builtin.QueryEntry, error)
	QueryEvents(ctx context.Context, filter MessageFilter) ([]builtin.EventEntry, error)
}

type InMemoryMessageStore struct {
	inner  *builtin.InMemoryMessageStore
	mu     sync.RWMutex
	maxSize int
}

type InMemoryMessageStoreOption func(*InMemoryMessageStore)

func WithMaxSize(n int) InMemoryMessageStoreOption {
	return func(s *InMemoryMessageStore) { s.maxSize = n }
}

func NewInMemoryMessageStore(opts ...InMemoryMessageStoreOption) *InMemoryMessageStore {
	s := &InMemoryMessageStore{
		inner:   builtin.NewInMemoryMessageStore(),
		maxSize: 1000,
	}
	for _, opt := range opts {
		opt(s)
	}
	if s.maxSize != 1000 {
		s.inner = builtin.NewInMemoryMessageStore(builtin.WithInMemoryMaxSize(s.maxSize))
	}
	return s
}

func (s *InMemoryMessageStore) RecordCommand(ctx context.Context, entry *builtin.CommandEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.inner.RecordCommand(ctx, entry)
}

func (s *InMemoryMessageStore) RecordQuery(ctx context.Context, entry *builtin.QueryEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.inner.RecordQuery(ctx, entry)
}

func (s *InMemoryMessageStore) RecordEvent(ctx context.Context, entry *builtin.EventEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.inner.RecordEvent(ctx, entry)
}

func (s *InMemoryMessageStore) RecordEventHandler(_ context.Context, _ *builtin.EventHandlerEntry) error {
	return nil
}

func (s *InMemoryMessageStore) QueryCommands(_ context.Context, filter MessageFilter) ([]builtin.CommandEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []builtin.CommandEntry
	for i := len(s.inner.Commands) - 1; i >= 0; i-- {
		e := s.inner.Commands[i]
		if !matchCommandFilter(e, filter) {
			continue
		}
		result = append(result, e)
		if filter.Limit > 0 && len(result) >= filter.Limit {
			break
		}
	}
	return result, nil
}

func (s *InMemoryMessageStore) QueryQueries(_ context.Context, filter MessageFilter) ([]builtin.QueryEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []builtin.QueryEntry
	for i := len(s.inner.Queries) - 1; i >= 0; i-- {
		e := s.inner.Queries[i]
		if !matchQueryFilter(e, filter) {
			continue
		}
		result = append(result, e)
		if filter.Limit > 0 && len(result) >= filter.Limit {
			break
		}
	}
	return result, nil
}

func (s *InMemoryMessageStore) QueryEvents(_ context.Context, filter MessageFilter) ([]builtin.EventEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []builtin.EventEntry
	for i := len(s.inner.Events) - 1; i >= 0; i-- {
		e := s.inner.Events[i]
		if !matchEventFilter(e, filter) {
			continue
		}
		result = append(result, e)
		if filter.Limit > 0 && len(result) >= filter.Limit {
			break
		}
	}
	return result, nil
}

func matchStringFilter(entryType, traceID, err string, createdAt time.Time, f MessageFilter) bool {
	if f.Type != "" && entryType != f.Type {
		return false
	}
	if f.TraceID != "" && traceID != f.TraceID {
		return false
	}
	if f.Status == "error" && err == "" {
		return false
	}
	if f.Status == "success" && err != "" {
		return false
	}
	if !f.Since.IsZero() && createdAt.Before(f.Since) {
		return false
	}
	return true
}

func matchCommandFilter(e builtin.CommandEntry, f MessageFilter) bool {
	return matchStringFilter(e.CommandType, e.TraceID, e.Error, e.CreatedAt, f)
}

func matchQueryFilter(e builtin.QueryEntry, f MessageFilter) bool {
	return matchStringFilter(e.QueryType, e.TraceID, e.Error, e.CreatedAt, f)
}

func matchEventFilter(e builtin.EventEntry, f MessageFilter) bool {
	if f.AggregateID != "" && e.AggregateID != f.AggregateID {
		return false
	}
	return matchStringFilter(e.EventType, e.TraceID, e.Error, e.CreatedAt, f)
}

var _ builtin.MessageStore = (*InMemoryMessageStore)(nil)
var _ MessageStoreReader = (*InMemoryMessageStore)(nil)

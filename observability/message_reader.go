package observability

import (
	"context"
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

type ObservableMessageStore struct {
	inner   *builtin.InMemoryMessageStore
	maxSize int
}

type ObservableMessageStoreOption func(*ObservableMessageStore)

func WithMaxSize(n int) ObservableMessageStoreOption {
	return func(s *ObservableMessageStore) { s.maxSize = n }
}

func NewObservableMessageStore(opts ...ObservableMessageStoreOption) *ObservableMessageStore {
	s := &ObservableMessageStore{
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

func (s *ObservableMessageStore) RecordCommand(ctx context.Context, entry *builtin.CommandEntry) error {
	return s.inner.RecordCommand(ctx, entry)
}

func (s *ObservableMessageStore) RecordQuery(ctx context.Context, entry *builtin.QueryEntry) error {
	return s.inner.RecordQuery(ctx, entry)
}

func (s *ObservableMessageStore) RecordEvent(ctx context.Context, entry *builtin.EventEntry) error {
	return s.inner.RecordEvent(ctx, entry)
}

func (s *ObservableMessageStore) RecordEventHandler(_ context.Context, _ *builtin.EventHandlerEntry) error {
	return nil
}

func (s *ObservableMessageStore) QueryCommands(_ context.Context, filter MessageFilter) ([]builtin.CommandEntry, error) {
	commands := s.inner.GetCommands()

	var result []builtin.CommandEntry
	for i := len(commands) - 1; i >= 0; i-- {
		e := commands[i]
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

func (s *ObservableMessageStore) QueryQueries(_ context.Context, filter MessageFilter) ([]builtin.QueryEntry, error) {
	queries := s.inner.GetQueries()

	var result []builtin.QueryEntry
	for i := len(queries) - 1; i >= 0; i-- {
		e := queries[i]
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

func (s *ObservableMessageStore) QueryEvents(_ context.Context, filter MessageFilter) ([]builtin.EventEntry, error) {
	events := s.inner.GetEvents()

	var result []builtin.EventEntry
	for i := len(events) - 1; i >= 0; i-- {
		e := events[i]
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

var _ builtin.MessageStore = (*ObservableMessageStore)(nil)
var _ MessageStoreReader = (*ObservableMessageStore)(nil)

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
	mu       sync.RWMutex
	commands []builtin.CommandEntry
	queries  []builtin.QueryEntry
	events   []builtin.EventEntry
	maxSize  int
}

type InMemoryMessageStoreOption func(*InMemoryMessageStore)

func WithMaxSize(n int) InMemoryMessageStoreOption {
	return func(s *InMemoryMessageStore) { s.maxSize = n }
}

func NewInMemoryMessageStore(opts ...InMemoryMessageStoreOption) *InMemoryMessageStore {
	s := &InMemoryMessageStore{
		maxSize: 1000,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func (s *InMemoryMessageStore) RecordCommand(ctx context.Context, entry *builtin.CommandEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.commands = append(s.commands, *entry)
	if len(s.commands) > s.maxSize {
		s.commands = s.commands[len(s.commands)-s.maxSize:]
	}
	return nil
}

func (s *InMemoryMessageStore) RecordQuery(ctx context.Context, entry *builtin.QueryEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.queries = append(s.queries, *entry)
	if len(s.queries) > s.maxSize {
		s.queries = s.queries[len(s.queries)-s.maxSize:]
	}
	return nil
}

func (s *InMemoryMessageStore) RecordEvent(ctx context.Context, entry *builtin.EventEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, *entry)
	if len(s.events) > s.maxSize {
		s.events = s.events[len(s.events)-s.maxSize:]
	}
	return nil
}

func (s *InMemoryMessageStore) RecordEventHandler(ctx context.Context, entry *builtin.EventHandlerEntry) error {
	return nil
}

func (s *InMemoryMessageStore) QueryCommands(ctx context.Context, filter MessageFilter) ([]builtin.CommandEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []builtin.CommandEntry
	for i := len(s.commands) - 1; i >= 0; i-- {
		e := s.commands[i]
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

func (s *InMemoryMessageStore) QueryQueries(ctx context.Context, filter MessageFilter) ([]builtin.QueryEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []builtin.QueryEntry
	for i := len(s.queries) - 1; i >= 0; i-- {
		e := s.queries[i]
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

func (s *InMemoryMessageStore) QueryEvents(ctx context.Context, filter MessageFilter) ([]builtin.EventEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []builtin.EventEntry
	for i := len(s.events) - 1; i >= 0; i-- {
		e := s.events[i]
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

func matchCommandFilter(e builtin.CommandEntry, f MessageFilter) bool {
	if f.Type != "" && e.CommandType != f.Type {
		return false
	}
	if f.TraceID != "" && e.TraceID != f.TraceID {
		return false
	}
	if f.Status == "error" && e.Error == "" {
		return false
	}
	if f.Status == "success" && e.Error != "" {
		return false
	}
	if !f.Since.IsZero() && e.CreatedAt.Before(f.Since) {
		return false
	}
	return true
}

func matchQueryFilter(e builtin.QueryEntry, f MessageFilter) bool {
	if f.Type != "" && e.QueryType != f.Type {
		return false
	}
	if f.TraceID != "" && e.TraceID != f.TraceID {
		return false
	}
	if f.Status == "error" && e.Error == "" {
		return false
	}
	if f.Status == "success" && e.Error != "" {
		return false
	}
	if !f.Since.IsZero() && e.CreatedAt.Before(f.Since) {
		return false
	}
	return true
}

func matchEventFilter(e builtin.EventEntry, f MessageFilter) bool {
	if f.Type != "" && e.EventType != f.Type {
		return false
	}
	if f.TraceID != "" && e.TraceID != f.TraceID {
		return false
	}
	if f.AggregateID != "" && e.AggregateID != f.AggregateID {
		return false
	}
	if f.Status == "error" && e.Error == "" {
		return false
	}
	if f.Status == "success" && e.Error != "" {
		return false
	}
	if !f.Since.IsZero() && e.CreatedAt.Before(f.Since) {
		return false
	}
	return true
}

var _ builtin.MessageStore = (*InMemoryMessageStore)(nil)
var _ MessageStoreReader = (*InMemoryMessageStore)(nil)

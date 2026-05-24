package builtin

import (
	"context"
	"sync"
)

type InMemoryMessageStore struct {
	mu       sync.RWMutex
	Commands []CommandEntry
	Queries  []QueryEntry
	Events   []EventEntry
	Handlers []EventHandlerEntry
	maxSize  int
}

type InMemoryMessageStoreOption func(*InMemoryMessageStore)

func WithInMemoryMaxSize(n int) InMemoryMessageStoreOption {
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

func (s *InMemoryMessageStore) RecordCommand(_ context.Context, entry *CommandEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Commands = append(s.Commands, *entry)
	if len(s.Commands) > s.maxSize {
		s.Commands = s.Commands[len(s.Commands)-s.maxSize:]
	}
	return nil
}

func (s *InMemoryMessageStore) RecordQuery(_ context.Context, entry *QueryEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Queries = append(s.Queries, *entry)
	if len(s.Queries) > s.maxSize {
		s.Queries = s.Queries[len(s.Queries)-s.maxSize:]
	}
	return nil
}

func (s *InMemoryMessageStore) RecordEvent(_ context.Context, entry *EventEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Events = append(s.Events, *entry)
	if len(s.Events) > s.maxSize {
		s.Events = s.Events[len(s.Events)-s.maxSize:]
	}
	return nil
}

func (s *InMemoryMessageStore) RecordEventHandler(_ context.Context, entry *EventHandlerEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Handlers = append(s.Handlers, *entry)
	if len(s.Handlers) > s.maxSize {
		s.Handlers = s.Handlers[len(s.Handlers)-s.maxSize:]
	}
	return nil
}

var _ MessageStore = (*InMemoryMessageStore)(nil)

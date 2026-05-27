package builtin

import (
	"context"
	"sync"
)

type InMemoryMessageStore struct {
	mu       sync.RWMutex
	commands []CommandEntry
	queries  []QueryEntry
	events   []EventEntry
	handlers []EventHandlerEntry
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

func (s *InMemoryMessageStore) GetCommands() []CommandEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]CommandEntry, len(s.commands))
	copy(result, s.commands)
	return result
}

func (s *InMemoryMessageStore) GetQueries() []QueryEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]QueryEntry, len(s.queries))
	copy(result, s.queries)
	return result
}

func (s *InMemoryMessageStore) GetEvents() []EventEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]EventEntry, len(s.events))
	copy(result, s.events)
	return result
}

func (s *InMemoryMessageStore) GetHandlers() []EventHandlerEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]EventHandlerEntry, len(s.handlers))
	copy(result, s.handlers)
	return result
}

func (s *InMemoryMessageStore) RecordCommand(_ context.Context, entry *CommandEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.commands = append(s.commands, *entry)
	if len(s.commands) > s.maxSize {
		s.commands = s.commands[len(s.commands)-s.maxSize:]
	}
	return nil
}

func (s *InMemoryMessageStore) RecordQuery(_ context.Context, entry *QueryEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.queries = append(s.queries, *entry)
	if len(s.queries) > s.maxSize {
		s.queries = s.queries[len(s.queries)-s.maxSize:]
	}
	return nil
}

func (s *InMemoryMessageStore) RecordEvent(_ context.Context, entry *EventEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, *entry)
	if len(s.events) > s.maxSize {
		s.events = s.events[len(s.events)-s.maxSize:]
	}
	return nil
}

func (s *InMemoryMessageStore) RecordEventHandler(_ context.Context, entry *EventHandlerEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handlers = append(s.handlers, *entry)
	if len(s.handlers) > s.maxSize {
		s.handlers = s.handlers[len(s.handlers)-s.maxSize:]
	}
	return nil
}

var _ MessageStore = (*InMemoryMessageStore)(nil)

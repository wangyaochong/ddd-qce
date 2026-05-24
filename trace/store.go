package trace

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

type TraceStore interface {
	RecordSpan(ctx context.Context, span *Span) error
	GetTrace(ctx context.Context, traceID string) ([]*Span, error)
	ListTraces(ctx context.Context, filter TraceFilter) ([]string, error)
}

type InMemoryTraceStore struct {
	mu         sync.RWMutex
	spans      []*Span
	traceIndex map[string][]int
}

func NewInMemoryTraceStore() *InMemoryTraceStore {
	return &InMemoryTraceStore{
		spans:      make([]*Span, 0),
		traceIndex: make(map[string][]int),
	}
}

func (s *InMemoryTraceStore) RecordSpan(ctx context.Context, span *Span) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	idx := len(s.spans)
	s.spans = append(s.spans, span)
	s.traceIndex[span.TraceID] = append(s.traceIndex[span.TraceID], idx)
	return nil
}

func (s *InMemoryTraceStore) GetTrace(ctx context.Context, traceID string) ([]*Span, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	indexes, exists := s.traceIndex[traceID]
	if !exists {
		return nil, fmt.Errorf("trace %s not found", traceID)
	}

	result := make([]*Span, len(indexes))
	for i, idx := range indexes {
		result[i] = s.spans[idx]
	}

	return result, nil
}

func (s *InMemoryTraceStore) ListTraces(ctx context.Context, filter TraceFilter) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]string, 0, len(s.traceIndex))
	for traceID, indexes := range s.traceIndex {
		if filter.TraceID != "" && traceID != filter.TraceID {
			continue
		}

		var spans []*Span
		for _, idx := range indexes {
			spans = append(spans, s.spans[idx])
		}

		if !matchesFilter(spans, filter) {
			continue
		}
		result = append(result, traceID)
	}

	return result, nil
}

func anyMatch(spans []*Span, pred func(*Span) bool) bool {
	for _, s := range spans {
		if pred(s) {
			return true
		}
	}
	return false
}

func matchesFilter(spans []*Span, filter TraceFilter) bool {
	if filter.TraceID != "" && !anyMatch(spans, func(s *Span) bool { return s.TraceID == filter.TraceID }) {
		return false
	}
	if filter.Type != "" && !anyMatch(spans, func(s *Span) bool { return s.Type == filter.Type }) {
		return false
	}
	if filter.Status != "" && !anyMatch(spans, func(s *Span) bool { return s.Status == filter.Status }) {
		return false
	}
	if !filter.StartTime.IsZero() && !anyMatch(spans, func(s *Span) bool { return !s.StartedAt.Before(filter.StartTime) }) {
		return false
	}
	if !filter.EndTime.IsZero() && !anyMatch(spans, func(s *Span) bool { return !s.StartedAt.After(filter.EndTime) }) {
		return false
	}
	if filter.NameContains != "" && !anyMatch(spans, func(s *Span) bool { return strings.Contains(s.Name, filter.NameContains) }) {
		return false
	}
	return true
}

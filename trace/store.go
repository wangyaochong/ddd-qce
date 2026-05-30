package trace

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	ddderror "github.com/ddd-qce/core/error"
)

const (
	defaultTTL      = 24 * time.Hour
	defaultMaxSpans = 10000
)

type TraceStore interface {
	RecordSpan(ctx context.Context, span *Span) error
	GetTrace(ctx context.Context, traceID string) ([]*Span, error)
	ListTraces(ctx context.Context, filter TraceFilter) ([]string, error)
	Close() error
}

type InMemoryTraceStore struct {
	mu         sync.RWMutex
	spans      []*Span
	traceIndex map[string][]int
	ttl        time.Duration
	maxSpans   int
	stopCh     chan struct{}
}

type InMemoryTraceStoreOption func(*InMemoryTraceStore)

func WithTTL(ttl time.Duration) InMemoryTraceStoreOption {
	return func(s *InMemoryTraceStore) { s.ttl = ttl }
}

func WithMaxSpans(n int) InMemoryTraceStoreOption {
	return func(s *InMemoryTraceStore) { s.maxSpans = n }
}

func WithBackgroundCleanup(interval time.Duration) InMemoryTraceStoreOption {
	return func(s *InMemoryTraceStore) {
		go func() {
			ticker := time.NewTicker(interval)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					s.mu.Lock()
					s.evict()
					s.mu.Unlock()
				case <-s.stopCh:
					return
				}
			}
		}()
	}
}

func NewInMemoryTraceStore(opts ...InMemoryTraceStoreOption) *InMemoryTraceStore {
	s := &InMemoryTraceStore{
		spans:      make([]*Span, 0),
		traceIndex: make(map[string][]int),
		ttl:        defaultTTL,
		maxSpans:   defaultMaxSpans,
		stopCh:     make(chan struct{}),
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func (s *InMemoryTraceStore) Close() error {
	select {
	case <-s.stopCh:
	default:
		close(s.stopCh)
	}
	return nil
}

func (s *InMemoryTraceStore) RecordSpan(ctx context.Context, span *Span) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	idx := len(s.spans)
	s.spans = append(s.spans, span)
	s.traceIndex[span.TraceID] = append(s.traceIndex[span.TraceID], idx)
	s.evict()
	return nil
}

func (s *InMemoryTraceStore) evict() {
	cutoff := time.Now().Add(-s.ttl)

	firstValid := 0
	for firstValid < len(s.spans) && s.spans[firstValid].StartedAt.Before(cutoff) {
		firstValid++
	}

	if len(s.spans)-firstValid > s.maxSpans {
		firstValid = len(s.spans) - s.maxSpans
	}

	if firstValid > 0 {
		s.spans = s.spans[firstValid:]
		s.rebuildIndex()
	}
}

func (s *InMemoryTraceStore) rebuildIndex() {
	s.traceIndex = make(map[string][]int, len(s.spans))
	for i, span := range s.spans {
		s.traceIndex[span.TraceID] = append(s.traceIndex[span.TraceID], i)
	}
}

func (s *InMemoryTraceStore) GetTrace(ctx context.Context, traceID string) ([]*Span, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	indexes, exists := s.traceIndex[traceID]
	if !exists {
		return nil, fmt.Errorf("trace %s: %w", traceID, ddderror.ErrNotFound)
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

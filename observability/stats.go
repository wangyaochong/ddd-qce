package observability

import (
	"sort"
	"sync"
	"time"

	"github.com/ddd-qce/core/aspect/builtin"
)

const (
	defaultWindowSize = 3600
	opTypeCommand     = "command"
	opTypeQuery       = "query"
	opTypeEvent       = "event"
	opTypeUnknown     = "unknown"
)

type OperationStats struct {
	Name        string        `json:"name"`
	Type        string        `json:"type"`
	Count       int64         `json:"count"`
	ErrorCount  int64         `json:"errorCount"`
	AvgDuration time.Duration `json:"avgDuration"`
	MaxDuration time.Duration `json:"maxDuration"`
	MinDuration time.Duration `json:"minDuration"`
	P50Duration time.Duration `json:"p50Duration"`
	P99Duration time.Duration `json:"p99Duration"`
	LastError   string        `json:"lastError"`
	LastAt      time.Time     `json:"lastAt"`
}

type durationRecord struct {
	dur  time.Duration
	time time.Time
}

type operationRecord struct {
	name       string
	opType     string
	count      int64
	errorCount int64
	durations  []durationRecord
	lastError  string
	lastAt     time.Time
}

type StatsCollector struct {
	mu        sync.RWMutex
	records   map[string]*operationRecord
	windowSec int
}

type StatsCollectorOption func(*StatsCollector)

func WithWindowSeconds(sec int) StatsCollectorOption {
	return func(s *StatsCollector) { s.windowSec = sec }
}

func NewStatsCollector(opts ...StatsCollectorOption) *StatsCollector {
	s := &StatsCollector{
		records:   make(map[string]*operationRecord),
		windowSec: defaultWindowSize,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func (s *StatsCollector) RecordDuration(name string, d time.Duration) {
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()

	rec, ok := s.records[name]
	if !ok {
		rec = &operationRecord{
			name:   name,
			opType: classifyOpType(name),
		}
		s.records[name] = rec
	}
	rec.count++
	rec.durations = append(rec.durations, durationRecord{dur: d, time: now})
	rec.lastAt = now
	s.evict(rec, now)
}

func (s *StatsCollector) RecordError(name string, err error) {
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()

	rec, ok := s.records[name]
	if !ok {
		rec = &operationRecord{
			name:   name,
			opType: classifyOpType(name),
		}
		s.records[name] = rec
	}
	rec.errorCount++
	if err != nil {
		rec.lastError = err.Error()
	}
	rec.lastAt = now
	s.evict(rec, now)
}

func (s *StatsCollector) GetStats(name string) (OperationStats, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rec, ok := s.records[name]
	if !ok {
		return OperationStats{}, false
	}
	return s.buildStats(rec), true
}

func (s *StatsCollector) GetAllStats() []OperationStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]OperationStats, 0, len(s.records))
	for _, rec := range s.records {
		result = append(result, s.buildStats(rec))
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result
}

func (s *StatsCollector) GetStatsByType(opType string) []OperationStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]OperationStats, 0)
	for _, rec := range s.records {
		if rec.opType == opType {
			result = append(result, s.buildStats(rec))
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result
}

func (s *StatsCollector) buildStats(rec *operationRecord) OperationStats {
	stats := OperationStats{
		Name:       rec.name,
		Type:       rec.opType,
		Count:      rec.count,
		ErrorCount: rec.errorCount,
		LastError:  rec.lastError,
		LastAt:     rec.lastAt,
	}

	if len(rec.durations) == 0 {
		return stats
	}

	durations := make([]time.Duration, len(rec.durations))
	var total time.Duration
	stats.MinDuration = rec.durations[0].dur
	stats.MaxDuration = rec.durations[0].dur

	for i, d := range rec.durations {
		durations[i] = d.dur
		total += d.dur
		if d.dur < stats.MinDuration {
			stats.MinDuration = d.dur
		}
		if d.dur > stats.MaxDuration {
			stats.MaxDuration = d.dur
		}
	}

	stats.AvgDuration = total / time.Duration(len(durations))
	sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
	stats.P50Duration = durations[len(durations)*50/100]
	stats.P99Duration = durations[len(durations)*99/100]

	return stats
}

func (s *StatsCollector) evict(rec *operationRecord, now time.Time) {
	cutoff := now.Add(-time.Duration(s.windowSec) * time.Second)
	i := 0
	for i < len(rec.durations) && rec.durations[i].time.Before(cutoff) {
		i++
	}
	if i > 0 {
		rec.durations = rec.durations[i:]
	}
}

var _ builtin.MetricsRecorder = (*StatsCollector)(nil)

func classifyOpType(name string) string {
	if len(name) >= 7 && name[:7] == "Command" {
		return opTypeCommand
	}
	if len(name) >= 5 && name[:5] == "Query" {
		return opTypeQuery
	}
	if len(name) >= 5 && name[:5] == "Event" {
		return opTypeEvent
	}
	return opTypeUnknown
}

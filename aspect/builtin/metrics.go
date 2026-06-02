package builtin

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// MetricsRecorder defines the contract for recording operation durations
// and errors. Implementations may push to Prometheus, StatsD, etc.
type MetricsRecorder interface {
	RecordDuration(name string, duration time.Duration)
	RecordError(name string, err error)
}

type metricsData struct {
	mu          sync.RWMutex
	durations   map[string][]time.Duration
	errorCounts map[string]int
	totalCounts map[string]int
}

// NewInMemMetricsRecorder creates a thread-safe, in-memory metrics recorder
// suitable for testing and local development.
func NewInMemMetricsRecorder() *InMemMetricsRecorder {
	return &InMemMetricsRecorder{
		data: &metricsData{
			durations:   make(map[string][]time.Duration),
			errorCounts: make(map[string]int),
			totalCounts: make(map[string]int),
		},
	}
}

// InMemMetricsRecorder is a thread-safe, in-memory implementation of
// MetricsRecorder for testing and development use.
type InMemMetricsRecorder struct {
	data *metricsData
}

// RecordDuration appends a duration observation for the named operation.
func (m *InMemMetricsRecorder) RecordDuration(name string, duration time.Duration) {
	m.data.mu.Lock()
	defer m.data.mu.Unlock()
	m.data.durations[name] = append(m.data.durations[name], duration)
	m.data.totalCounts[name]++
}

// RecordError increments the error count for the named operation.
func (m *InMemMetricsRecorder) RecordError(name string, err error) {
	m.data.mu.Lock()
	defer m.data.mu.Unlock()
	m.data.errorCounts[name]++
	m.data.totalCounts[name]++
}

// GetDurations returns a copy of all recorded durations for the named operation.
func (m *InMemMetricsRecorder) GetDurations(name string) []time.Duration {
	m.data.mu.RLock()
	defer m.data.mu.RUnlock()
	d := make([]time.Duration, len(m.data.durations[name]))
	copy(d, m.data.durations[name])
	return d
}

// GetErrorCount returns the number of errors recorded for the named operation.
func (m *InMemMetricsRecorder) GetErrorCount(name string) int {
	m.data.mu.RLock()
	defer m.data.mu.RUnlock()
	return m.data.errorCounts[name]
}

// GetTotalCount returns the total number of observations (durations + errors)
// recorded for the named operation.
func (m *InMemMetricsRecorder) GetTotalCount(name string) int {
	m.data.mu.RLock()
	defer m.data.mu.RUnlock()
	return m.data.totalCounts[name]
}

// GetAverageDuration returns the mean duration for the named operation,
// or zero if no observations exist.
func (m *InMemMetricsRecorder) GetAverageDuration(name string) time.Duration {
	m.data.mu.RLock()
	defer m.data.mu.RUnlock()
	durations := m.data.durations[name]
	if len(durations) == 0 {
		return 0
	}
	var total time.Duration
	for _, d := range durations {
		total += d
	}
	return total / time.Duration(len(durations))
}

// Reset clears all recorded metrics data.
func (m *InMemMetricsRecorder) Reset() {
	m.data.mu.Lock()
	defer m.data.mu.Unlock()
	m.data.durations = make(map[string][]time.Duration)
	m.data.errorCounts = make(map[string]int)
	m.data.totalCounts = make(map[string]int)
}

var _ MetricsRecorder = (*InMemMetricsRecorder)(nil)

// MetricsAspect records operation durations and errors via a MetricsRecorder
// for all commands, queries, and events processed by the aspect chain.
type MetricsAspect struct {
	recorder MetricsRecorder
}

// NewMetricsAspect creates a MetricsAspect that reports to the given recorder.
func NewMetricsAspect(recorder MetricsRecorder) *MetricsAspect {
	return &MetricsAspect{recorder: recorder}
}

func (m *MetricsAspect) GetRecorder() MetricsRecorder { return m.recorder }

func (m *MetricsAspect) Name() string {
	return "metrics"
}

func (m *MetricsAspect) Order() int {
	return 100
}

func (m *MetricsAspect) BeforeQuery(ctx context.Context, query any) (context.Context, error) {
	return ctx, nil
}

func (m *MetricsAspect) AfterQuery(ctx context.Context, query any, result any, err error, duration time.Duration) error {
	name := fmt.Sprintf("Query/%T", query)
	m.recorder.RecordDuration(name, duration)
	if err != nil {
		m.recorder.RecordError(name, err)
	}
	return nil
}

func (m *MetricsAspect) BeforeCommand(ctx context.Context, cmd any) (context.Context, error) {
	return ctx, nil
}

func (m *MetricsAspect) AfterCommand(ctx context.Context, cmd any, result any, err error, duration time.Duration) error {
	name := fmt.Sprintf("Command/%T", cmd)
	m.recorder.RecordDuration(name, duration)
	if err != nil {
		m.recorder.RecordError(name, err)
	}
	return nil
}

func (m *MetricsAspect) BeforePublish(ctx context.Context, event any) (context.Context, error) {
	return ctx, nil
}

func (m *MetricsAspect) AfterPublish(ctx context.Context, event any, err error, duration time.Duration) error {
	name := fmt.Sprintf("Event/%T", event)
	m.recorder.RecordDuration(name, duration)
	if err != nil {
		m.recorder.RecordError(name, err)
	}
	return nil
}

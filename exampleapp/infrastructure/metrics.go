package infrastructure

import (
	"log"
	"sync"
	"time"
)

type MetricRecord struct {
	Name     string
	Duration time.Duration
}

type ErrorRecord struct {
	Name string
	Err  error
}

type AppMetricsRecorder struct {
	mu        sync.RWMutex
	Durations []MetricRecord
	Errors    []ErrorRecord
}

func NewAppMetricsRecorder() *AppMetricsRecorder {
	return &AppMetricsRecorder{}
}

func (r *AppMetricsRecorder) RecordDuration(name string, duration time.Duration) {
	r.mu.Lock()
	r.Durations = append(r.Durations, MetricRecord{Name: name, Duration: duration})
	r.mu.Unlock()
	log.Printf("[Metrics] %s took %v", name, duration)
}

func (r *AppMetricsRecorder) RecordError(name string, err error) {
	r.mu.Lock()
	r.Errors = append(r.Errors, ErrorRecord{Name: name, Err: err})
	r.mu.Unlock()
	log.Printf("[Metrics] %s error: %v", name, err)
}

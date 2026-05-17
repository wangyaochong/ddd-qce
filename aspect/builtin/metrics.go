package builtin

import (
	"context"
	"fmt"
	"time"
)

type MetricsRecorder interface {
	RecordDuration(name string, duration time.Duration)
	RecordError(name string, err error)
}

type MetricsAspect struct {
	Recorder MetricsRecorder
}

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
	m.Recorder.RecordDuration(name, duration)
	if err != nil {
		m.Recorder.RecordError(name, err)
	}
	return nil
}

func (m *MetricsAspect) BeforeCommand(ctx context.Context, cmd any) (context.Context, error) {
	return ctx, nil
}

func (m *MetricsAspect) AfterCommand(ctx context.Context, cmd any, result any, err error, duration time.Duration) error {
	name := fmt.Sprintf("Command/%T", cmd)
	m.Recorder.RecordDuration(name, duration)
	if err != nil {
		m.Recorder.RecordError(name, err)
	}
	return nil
}

func (m *MetricsAspect) BeforePublish(ctx context.Context, event any) (context.Context, error) {
	return ctx, nil
}

func (m *MetricsAspect) AfterPublish(ctx context.Context, event any, err error, duration time.Duration) error {
	name := fmt.Sprintf("Event/%T", event)
	m.Recorder.RecordDuration(name, duration)
	if err != nil {
		m.Recorder.RecordError(name, err)
	}
	return nil
}

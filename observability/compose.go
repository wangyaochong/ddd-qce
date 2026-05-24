package observability

import (
	"time"

	"github.com/ddd-qce/core/aspect/builtin"
)

type composedMetrics struct {
	recorders []builtin.MetricsRecorder
}

func ComposeMetrics(recorders ...builtin.MetricsRecorder) builtin.MetricsRecorder {
	return &composedMetrics{recorders: recorders}
}

func (c *composedMetrics) RecordDuration(name string, d time.Duration) {
	for _, r := range c.recorders {
		r.RecordDuration(name, d)
	}
}

func (c *composedMetrics) RecordError(name string, err error) {
	for _, r := range c.recorders {
		r.RecordError(name, err)
	}
}

var _ builtin.MetricsRecorder = (*composedMetrics)(nil)

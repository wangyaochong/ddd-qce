package observability

import (
	"time"

	"github.com/ddd-qce/core/aspect/builtin"
)

type OTelConfig struct {
	ServiceName    string
	Endpoint       string
	ExportTimeout  time.Duration
	MetricInterval time.Duration
}

type OTelMetricsExporter interface {
	ExportDuration(name string, d time.Duration)
	ExportError(name string, err error)
	Shutdown() error
}

type OTelBridge struct {
	exporter OTelMetricsExporter
	config   OTelConfig
}

func NewOTelBridge(config OTelConfig, exporter OTelMetricsExporter) *OTelBridge {
	if config.ExportTimeout == 0 {
		config.ExportTimeout = 5 * time.Second
	}
	if config.MetricInterval == 0 {
		config.MetricInterval = 15 * time.Second
	}
	return &OTelBridge{
		exporter: exporter,
		config:   config,
	}
}

func (b *OTelBridge) RecordDuration(name string, d time.Duration) {
	if b.exporter != nil {
		b.exporter.ExportDuration(name, d)
	}
}

func (b *OTelBridge) RecordError(name string, err error) {
	if b.exporter != nil {
		b.exporter.ExportError(name, err)
	}
}

func (b *OTelBridge) Shutdown() error {
	if b.exporter != nil {
		return b.exporter.Shutdown()
	}
	return nil
}

var _ builtin.MetricsRecorder = (*OTelBridge)(nil)

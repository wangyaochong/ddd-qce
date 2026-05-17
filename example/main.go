package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/ddd-qce/core/aspect"
	"github.com/ddd-qce/core/aspect/builtin"
	commandmemory "github.com/ddd-qce/core/cqrs/command/memory"
	eventmemory "github.com/ddd-qce/core/cqrs/event/memory"
	querymemory "github.com/ddd-qce/core/cqrs/query/memory"
	jobmemory "github.com/ddd-qce/core/job/memory"
	"github.com/ddd-qce/core/trace"

	"github.com/ddd-qce/example/command"
	"github.com/ddd-qce/example/event"
	"github.com/ddd-qce/example/job"
	"github.com/ddd-qce/example/query"
	"github.com/ddd-qce/example/traceexample"
)

type SimpleMetricsRecorder struct{}

func (r *SimpleMetricsRecorder) RecordDuration(name string, duration time.Duration) {
	log.Printf("[Metrics] %s took %v", name, duration)
}

func (r *SimpleMetricsRecorder) RecordError(name string, err error) {
	log.Printf("[Metrics] %s error: %v", name, err)
}

type SimpleLogger struct{}

func (l *SimpleLogger) Info(msg string, args ...interface{}) {
	log.Printf("[INFO] %s %v", msg, args)
}

func (l *SimpleLogger) Error(msg string, args ...interface{}) {
	log.Printf("[ERROR] %s %v", msg, args)
}

func (l *SimpleLogger) Debug(msg string, args ...interface{}) {
	log.Printf("[DEBUG] %s %v", msg, args)
}

func main() {
	ctx := context.Background()

	traceStore := trace.NewInMemoryTraceStore()

	chain := aspect.NewAspectChain()
	chain.RegisterCommandAspect(&builtin.TracingAspect{Store: traceStore})
	chain.RegisterCommandAspect(&builtin.MetricsAspect{Recorder: &SimpleMetricsRecorder{}})
	chain.RegisterCommandAspect(&builtin.LoggingAspect{Logger: &SimpleLogger{}})
	chain.RegisterQueryAspect(&builtin.TracingAspect{Store: traceStore})
	chain.RegisterQueryAspect(&builtin.MetricsAspect{Recorder: &SimpleMetricsRecorder{}})
	chain.RegisterQueryAspect(&builtin.LoggingAspect{Logger: &SimpleLogger{}})
	chain.RegisterEventAspect(&builtin.TracingAspect{Store: traceStore})
	chain.RegisterEventAspect(&builtin.MetricsAspect{Recorder: &SimpleMetricsRecorder{}})
	chain.RegisterEventAspect(&builtin.LoggingAspect{Logger: &SimpleLogger{}})

	qBus := querymemory.NewQueryBus(chain)
	query.RegisterHandlers(qBus)

	cBus := commandmemory.NewCommandBus(chain)
	command.RegisterHandlers(cBus)

	eventGroup := eventmemory.NewEventBusGroup(chain)
	event.RegisterHandlers(eventGroup)

	jobStore := jobmemory.NewJobStore()
	jobManager := job.NewJobManager(jobStore, chain)

	fmt.Println("========================================")
	fmt.Println("  DDD-QCE Example")
	fmt.Println("========================================")

	query.RunExample(ctx, qBus)
	fmt.Println()
	command.RunExample(ctx, cBus)
	fmt.Println()
	event.RunExample(ctx, eventGroup)
	fmt.Println()
	job.RunExample(ctx, jobManager)

	fmt.Println()
	traceexample.RunExample(ctx, cBus, eventGroup, qBus)

	fmt.Println("\n========================================")
	fmt.Println("  Traces:")
	fmt.Println("========================================")
	traceexample.PrintTraces(ctx, traceStore)

	fmt.Println("\n========================================")
	fmt.Println("  Done!")
	fmt.Println("========================================")
}

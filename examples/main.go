package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/ddd-qce/core/aspect"
	"github.com/ddd-qce/core/aspect/builtin"
	commandmemory "github.com/ddd-qce/core/cqrs/impl/memory"
	eventmemory "github.com/ddd-qce/core/cqrs/impl/memory"
	querymemory "github.com/ddd-qce/core/cqrs/impl/memory"
	jobmemory "github.com/ddd-qce/core/job/memory"
	"github.com/ddd-qce/core/trace"

	"github.com/ddd-qce/examples/command"
	"github.com/ddd-qce/examples/event"
	"github.com/ddd-qce/examples/job"
	"github.com/ddd-qce/examples/query"
	"github.com/ddd-qce/examples/traceexample"
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
	chain.RegisterAspect(builtin.NewTracingAspect(traceStore))
	chain.RegisterAspect(builtin.NewMetricsAspect(&SimpleMetricsRecorder{}))
	chain.RegisterAspect(builtin.NewLoggingAspect(&SimpleLogger{}))

	qBus := querymemory.NewQueryBus(querymemory.WithQueryBusAspectChain(chain))
	query.RegisterHandlers(qBus)

	cBus := commandmemory.NewCommandBus(commandmemory.WithCommandBusAspectChain(chain))
	command.RegisterHandlers(cBus)

	eventBus := eventmemory.NewEventBus(eventmemory.WithEventBusAspectChain(chain))
	event.RegisterHandlers(eventBus)

	jobStore := jobmemory.NewJobStore()
	jobManager := job.NewJobManager(jobStore, chain)

	fmt.Println("========================================")
	fmt.Println("  DDD-QCE Example")
	fmt.Println("========================================")

	query.RunExample(ctx, qBus)
	fmt.Println()
	command.RunExample(ctx, cBus)
	fmt.Println()
	event.RunExample(ctx, eventBus)
	fmt.Println()
	job.RunExample(ctx, jobManager)

	fmt.Println()
	traceexample.RunExample(ctx, cBus, eventBus, qBus)

	fmt.Println("\n========================================")
	fmt.Println("  Traces:")
	fmt.Println("========================================")
	traceexample.PrintTraces(ctx, traceStore)

	fmt.Println("\n========================================")
	fmt.Println("  Done!")
	fmt.Println("========================================")
}

package job

import (
	"context"
	"fmt"
	"time"

	"github.com/ddd-qce/core/aspect"
	"github.com/ddd-qce/core/cqrs/command"
	commandmemory "github.com/ddd-qce/core/cqrs/impl/memory"
	jobcore "github.com/ddd-qce/core/job/core"
	jobmemory "github.com/ddd-qce/core/job/memory"
)

type GenerateReportCommand struct {
	command.BaseCommand
	Duration time.Duration
}

type GenerateReportResult struct {
	File string
}

type GenerateReportHandler struct{}

func (h *GenerateReportHandler) Handle(ctx context.Context, cmd *GenerateReportCommand) (*GenerateReportResult, error) {
	steps := 10
	stepDuration := cmd.Duration / time.Duration(steps)

	for i := 1; i <= steps; i++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(stepDuration):
			fmt.Printf("  Progress: %d%%\n", i*10)
		}
	}

	return &GenerateReportResult{File: "report.pdf"}, nil
}

func RegisterHandlers(bus *commandmemory.CommandBus) {
	commandmemory.RegisterCommand(bus, &GenerateReportHandler{})
}

func NewJobManager(store jobcore.JobStore, chain *aspect.AspectChain) *jobmemory.JobManager {
	cmdBus := commandmemory.NewCommandBus(commandmemory.WithCommandBusAspectChain(chain))
	RegisterHandlers(cmdBus)
	return jobmemory.NewJobManager(store, cmdBus)
}

func RunExample(ctx context.Context, manager jobcore.JobManager) {
	fmt.Println("=== Long Running Job ===")
	job, err := manager.Submit(ctx, &GenerateReportCommand{Duration: 5 * time.Second},
		jobcore.WithTimeout(10*time.Second),
		jobcore.WithMaxRetries(2),
	)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Job submitted: %s\n", job.ID())

	fmt.Println("Waiting for job to complete...")
	completedJob, err := manager.Wait(ctx, job.ID(), 10*time.Second)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Job completed: Status=%s\n", completedJob.GetStatus())
	if completedJob.GetResult() != nil {
		reportResult, ok := completedJob.GetResult().(*GenerateReportResult)
		if !ok {
			fmt.Println("Result type mismatch")
			return
		}
		fmt.Printf("Report file: %s\n", reportResult.File)
	}
}

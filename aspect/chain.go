package aspect

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/ddd-qce/core/cqrs/aspect"
)

type AspectChain struct {
	mu             sync.RWMutex
	queryAspects   []aspect.QueryAspect
	commandAspects []aspect.CommandAspect
	eventAspects   []aspect.EventAspect
}

func NewAspectChain() *AspectChain {
	return &AspectChain{}
}

func (c *AspectChain) RegisterQueryAspect(a aspect.QueryAspect) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.queryAspects = append(c.queryAspects, a)
	sort.Slice(c.queryAspects, func(i, j int) bool {
		return c.queryAspects[i].Order() < c.queryAspects[j].Order()
	})
}

func (c *AspectChain) RegisterCommandAspect(a aspect.CommandAspect) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.commandAspects = append(c.commandAspects, a)
	sort.Slice(c.commandAspects, func(i, j int) bool {
		return c.commandAspects[i].Order() < c.commandAspects[j].Order()
	})
}

func (c *AspectChain) RegisterEventAspect(a aspect.EventAspect) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.eventAspects = append(c.eventAspects, a)
	sort.Slice(c.eventAspects, func(i, j int) bool {
		return c.eventAspects[i].Order() < c.eventAspects[j].Order()
	})
}

func (c *AspectChain) ExecuteWithQueryAspects(
	ctx context.Context,
	query any,
	next func(context.Context) (any, error),
) (any, error) {
	c.mu.RLock()
	aspects := make([]aspect.QueryAspect, len(c.queryAspects))
	copy(aspects, c.queryAspects)
	c.mu.RUnlock()
	return c.runQueryAspects(ctx, query, next, 0, aspects)
}

func (c *AspectChain) runQueryAspects(
	ctx context.Context,
	query any,
	next func(context.Context) (any, error),
	index int,
	aspects []aspect.QueryAspect,
) (any, error) {
	if index >= len(aspects) {
		return next(ctx)
	}

	a := aspects[index]

	newCtx, err := a.BeforeQuery(ctx, query)
	if err != nil {
		return nil, err
	}

	start := time.Now()
	result, err := c.runQueryAspects(newCtx, query, next, index+1, aspects)
	duration := time.Since(start)

	if afterErr := a.AfterQuery(newCtx, query, result, err, duration); afterErr != nil {
		if err == nil {
			err = afterErr
		}
	}

	return result, err
}

func (c *AspectChain) ExecuteWithCommandAspects(
	ctx context.Context,
	cmd any,
	next func(context.Context) (any, error),
) (any, error) {
	c.mu.RLock()
	aspects := make([]aspect.CommandAspect, len(c.commandAspects))
	copy(aspects, c.commandAspects)
	c.mu.RUnlock()
	return c.runCommandAspects(ctx, cmd, next, 0, aspects)
}

func (c *AspectChain) runCommandAspects(
	ctx context.Context,
	cmd any,
	next func(context.Context) (any, error),
	index int,
	aspects []aspect.CommandAspect,
) (any, error) {
	if index >= len(aspects) {
		return next(ctx)
	}

	a := aspects[index]

	newCtx, err := a.BeforeCommand(ctx, cmd)
	if err != nil {
		return nil, err
	}

	start := time.Now()
	result, err := c.runCommandAspects(newCtx, cmd, next, index+1, aspects)
	duration := time.Since(start)

	if afterErr := a.AfterCommand(newCtx, cmd, result, err, duration); afterErr != nil {
		if err == nil {
			err = afterErr
		}
	}

	return result, err
}

func (c *AspectChain) ExecuteWithEventAspects(
	ctx context.Context,
	event any,
	next func(context.Context) error,
) error {
	c.mu.RLock()
	aspects := make([]aspect.EventAspect, len(c.eventAspects))
	copy(aspects, c.eventAspects)
	c.mu.RUnlock()
	return c.runEventAspects(ctx, event, next, 0, aspects)
}

func (c *AspectChain) runEventAspects(
	ctx context.Context,
	event any,
	next func(context.Context) error,
	index int,
	aspects []aspect.EventAspect,
) error {
	if index >= len(aspects) {
		return next(ctx)
	}

	a := aspects[index]

	newCtx, err := a.BeforePublish(ctx, event)
	if err != nil {
		return err
	}

	start := time.Now()
	err = c.runEventAspects(newCtx, event, next, index+1, aspects)
	duration := time.Since(start)

	if afterErr := a.AfterPublish(newCtx, event, err, duration); afterErr != nil {
		if err == nil {
			err = afterErr
		}
	}

	return err
}

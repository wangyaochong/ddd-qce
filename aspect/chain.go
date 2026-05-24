package aspect

import (
	"context"
	"errors"
	"sort"
	"sync"
	"time"
)

type Aspect interface {
	Name() string
	Order() int
}

type CommandAspect interface {
	Aspect
	BeforeCommand(ctx context.Context, cmd any) (context.Context, error)
	AfterCommand(ctx context.Context, cmd any, result any, err error, duration time.Duration) error
}

type QueryAspect interface {
	Aspect
	BeforeQuery(ctx context.Context, query any) (context.Context, error)
	AfterQuery(ctx context.Context, query any, result any, err error, duration time.Duration) error
}

type EventAspect interface {
	Aspect
	BeforePublish(ctx context.Context, event any) (context.Context, error)
	AfterPublish(ctx context.Context, event any, err error, duration time.Duration) error
}

type AspectChain struct {
	mu             sync.RWMutex
	queryAspects   []QueryAspect
	commandAspects []CommandAspect
	eventAspects   []EventAspect
}

func NewAspectChain() *AspectChain {
	return &AspectChain{}
}

func (c *AspectChain) RegisterAspect(a any) {
	if ca, ok := a.(CommandAspect); ok {
		c.RegisterCommandAspect(ca)
	}
	if qa, ok := a.(QueryAspect); ok {
		c.RegisterQueryAspect(qa)
	}
	if ea, ok := a.(EventAspect); ok {
		c.RegisterEventAspect(ea)
	}
}

func (c *AspectChain) RegisterQueryAspect(a QueryAspect) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.queryAspects = append(c.queryAspects, a)
	sort.Slice(c.queryAspects, func(i, j int) bool {
		return c.queryAspects[i].Order() < c.queryAspects[j].Order()
	})
}

func (c *AspectChain) RegisterCommandAspect(a CommandAspect) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.commandAspects = append(c.commandAspects, a)
	sort.Slice(c.commandAspects, func(i, j int) bool {
		return c.commandAspects[i].Order() < c.commandAspects[j].Order()
	})
}

func (c *AspectChain) RegisterEventAspect(a EventAspect) {
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
	aspects := make([]QueryAspect, len(c.queryAspects))
	copy(aspects, c.queryAspects)
	c.mu.RUnlock()
	return c.runQueryAspects(ctx, query, next, 0, aspects)
}

func (c *AspectChain) runQueryAspects(
	ctx context.Context,
	query any,
	next func(context.Context) (any, error),
	index int,
	aspects []QueryAspect,
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
		} else {
			err = errors.Join(err, afterErr)
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
	aspects := make([]CommandAspect, len(c.commandAspects))
	copy(aspects, c.commandAspects)
	c.mu.RUnlock()
	return c.runCommandAspects(ctx, cmd, next, 0, aspects)
}

func (c *AspectChain) runCommandAspects(
	ctx context.Context,
	cmd any,
	next func(context.Context) (any, error),
	index int,
	aspects []CommandAspect,
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
		} else {
			err = errors.Join(err, afterErr)
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
	aspects := make([]EventAspect, len(c.eventAspects))
	copy(aspects, c.eventAspects)
	c.mu.RUnlock()
	return c.runEventAspects(ctx, event, next, 0, aspects)
}

func (c *AspectChain) runEventAspects(
	ctx context.Context,
	event any,
	next func(context.Context) error,
	index int,
	aspects []EventAspect,
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
		} else {
			err = errors.Join(err, afterErr)
		}
	}

	return err
}

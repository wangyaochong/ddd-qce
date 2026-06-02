package aspect

import (
	"context"
	"errors"
	"sort"
	"sync"
	"time"
)

// Aspect is the base interface that all aspects must implement.
// Name returns a unique identifier for the aspect; Order determines
// execution priority in the onion model (lower values execute first
// on the way in, last on the way out).
type Aspect interface {
	Name() string
	Order() int
}

// CommandAspect defines an aspect that intercepts command execution.
// BeforeCommand runs before the command handler and may modify the context.
// AfterCommand runs after the command handler with the result, error, and
// elapsed duration.
type CommandAspect interface {
	Aspect
	BeforeCommand(ctx context.Context, cmd any) (context.Context, error)
	AfterCommand(ctx context.Context, cmd any, result any, err error, duration time.Duration) error
}

// QueryAspect defines an aspect that intercepts query execution.
// BeforeQuery runs before the query handler and may modify the context.
// AfterQuery runs after the query handler with the result, error, and
// elapsed duration.
type QueryAspect interface {
	Aspect
	BeforeQuery(ctx context.Context, query any) (context.Context, error)
	AfterQuery(ctx context.Context, query any, result any, err error, duration time.Duration) error
}

// EventAspect defines an aspect that intercepts event publishing.
// BeforePublish runs before the event is published and may modify the context.
// AfterPublish runs after publishing with the error (if any) and elapsed duration.
type EventAspect interface {
	Aspect
	BeforePublish(ctx context.Context, event any) (context.Context, error)
	AfterPublish(ctx context.Context, event any, err error, duration time.Duration) error
}

// AspectChain executes registered aspects in onion-model order around
// command, query, and event operations. Aspects are sorted by Order
// (ascending) so that lower-order aspects wrap higher-order ones.
type AspectChain struct {
	mu             sync.RWMutex
	queryAspects   []QueryAspect
	commandAspects []CommandAspect
	eventAspects   []EventAspect
}

// NewAspectChain creates an empty AspectChain ready for aspect registration.
func NewAspectChain() *AspectChain {
	return &AspectChain{}
}

// HasAspect checks whether an aspect with the given name is registered
// across any of the command, query, or event aspect lists.
func (c *AspectChain) HasAspect(name string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	for _, a := range c.commandAspects {
		if a.Name() == name {
			return true
		}
	}
	for _, a := range c.queryAspects {
		if a.Name() == name {
			return true
		}
	}
	for _, a := range c.eventAspects {
		if a.Name() == name {
			return true
		}
	}
	return false
}

// RegisteredNames returns a deduplicated list of all registered aspect names.
func (c *AspectChain) RegisteredNames() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	seen := make(map[string]bool)
	var names []string
	for _, a := range c.commandAspects {
		if !seen[a.Name()] {
			seen[a.Name()] = true
			names = append(names, a.Name())
		}
	}
	for _, a := range c.queryAspects {
		if !seen[a.Name()] {
			seen[a.Name()] = true
			names = append(names, a.Name())
		}
	}
	for _, a := range c.eventAspects {
		if !seen[a.Name()] {
			seen[a.Name()] = true
			names = append(names, a.Name())
		}
	}
	return names
}

// RegisterAspect auto-detects which aspect interfaces the given value implements
// and registers it for each matching role (command, query, event). A single value
// can implement multiple aspect interfaces simultaneously.
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

// RegisterQueryAspect adds a QueryAspect to the chain and re-sorts by Order.
func (c *AspectChain) RegisterQueryAspect(a QueryAspect) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.queryAspects = append(c.queryAspects, a)
	sort.Slice(c.queryAspects, func(i, j int) bool {
		return c.queryAspects[i].Order() < c.queryAspects[j].Order()
	})
}

// RegisterCommandAspect adds a CommandAspect to the chain and re-sorts by Order.
func (c *AspectChain) RegisterCommandAspect(a CommandAspect) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.commandAspects = append(c.commandAspects, a)
	sort.Slice(c.commandAspects, func(i, j int) bool {
		return c.commandAspects[i].Order() < c.commandAspects[j].Order()
	})
}

// RegisterEventAspect adds an EventAspect to the chain and re-sorts by Order.
func (c *AspectChain) RegisterEventAspect(a EventAspect) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.eventAspects = append(c.eventAspects, a)
	sort.Slice(c.eventAspects, func(i, j int) bool {
		return c.eventAspects[i].Order() < c.eventAspects[j].Order()
	})
}

// ExecuteWithQueryAspects runs the given next function wrapped by all registered
// QueryAspects. Aspects are applied in onion-model order: Before/After hooks
// nest around the core operation.
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

// ExecuteWithCommandAspects runs the given next function wrapped by all registered
// CommandAspects. Aspects are applied in onion-model order: Before/After hooks
// nest around the core operation.
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

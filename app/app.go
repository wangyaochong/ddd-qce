package app

import (
	"context"
	"fmt"

	"github.com/ddd-qce/core/aspect"
	"github.com/ddd-qce/core/aspect/builtin"
	"github.com/ddd-qce/core/config"
	"github.com/ddd-qce/core/cqrs/command"
	"github.com/ddd-qce/core/cqrs/event"
	"github.com/ddd-qce/core/cqrs/query"
	"github.com/ddd-qce/core/infra"
)

// App is the top-level application container that wires buses, aspects, and infrastructure.
type App struct {
	CmdBus     command.CommandBus
	QueryBus   query.QueryBus
	EventBus   event.EventBus
	Chain      *aspect.AspectChain
	Backend    *infra.Backend
	Config     *config.Config
	lifecycles []Lifecycle
	cleanup    []func() error
}

// AppOption configures an App during construction.
type AppOption func(*App) error

// NewApp creates an App with the provided options, aborting on the first option error.
func NewApp(opts ...AppOption) (*App, error) {
	app := &App{
		Config: config.DefaultConfig(),
	}

	for _, opt := range opts {
		if err := opt(app); err != nil {
			return nil, fmt.Errorf("app option failed: %w", err)
		}
	}

	return app, nil
}

// Close shuts down all registered lifecycles and runs cleanup functions.
func (a *App) Close(ctx context.Context) error {
	var errs []error
	for _, l := range a.lifecycles {
		if err := l.Shutdown(ctx); err != nil {
			errs = append(errs, err)
		}
		if ctx.Err() != nil {
			break
		}
	}
	for _, fn := range a.cleanup {
		if err := fn(); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("shutdown errors: %v", errs)
	}
	return nil
}

// RegisterLifecycle adds a Lifecycle to be shut down when the app closes.
func (a *App) RegisterLifecycle(l Lifecycle) {
	a.lifecycles = append(a.lifecycles, l)
}

// WithAutoBackend creates a backend from DDD_STORE_TYPE env config and attaches it to the app.
func WithAutoBackend() AppOption {
	return func(a *App) error {
		storeCfg := config.ResolveStoreConfig()
		backend, err := infra.NewBackendFromConfig(storeCfg)
		if err != nil {
			return fmt.Errorf("auto backend: %w", err)
		}
		a.Backend = backend
		a.cleanup = append(a.cleanup, backend.Close)
		return nil
	}
}

// WithBackend sets the infrastructure backend for the app.
func WithBackend(backend *infra.Backend) AppOption {
	return func(a *App) error {
		a.Backend = backend
		return nil
	}
}

// WithConfigFile loads app configuration from a TOML file.
func WithConfigFile(path string) AppOption {
	return func(a *App) error {
		loader := config.NewConfigLoader()
		cfg, err := loader.Load(path)
		if err != nil {
			return fmt.Errorf("load config from %s: %w", path, err)
		}
		a.Config = cfg
		return nil
	}
}

func ensureChain(chain *aspect.AspectChain) *aspect.AspectChain {
	if chain == nil {
		return aspect.NewAspectChain()
	}
	return chain
}

// WithDefaultAspects registers logging, tracing, metrics, and transaction aspects based on config.
func WithDefaultAspects() AppOption {
	return func(a *App) error {
		a.Chain = ensureChain(a.Chain)

		logger := builtin.NewStdLogger()
		metrics := builtin.NewInMemMetricsRecorder()
		var txManager builtin.TransactionManager
		if a.Backend != nil && a.Backend.TransactionManager != nil {
			txManager = a.Backend.TransactionManager
		} else {
			txManager = builtin.NewNoOpTransactionManager()
		}

		if a.Config.Aspect.EnableLogging {
			a.Chain.RegisterAspect(builtin.NewLoggingAspect(logger))
		}
		if a.Config.Aspect.EnableTracing {
			if a.Backend != nil && a.Backend.TraceStore != nil {
				a.Chain.RegisterAspect(builtin.NewTracingAspect(a.Backend.TraceStore))
			}
		}
		if a.Config.Aspect.EnableMetrics {
			a.Chain.RegisterAspect(builtin.NewMetricsAspect(metrics))
		}
		if a.Config.Aspect.EnableTransaction {
			ta, err := builtin.NewTransactionAspect(txManager)
			if err != nil {
				return fmt.Errorf("create transaction aspect: %w", err)
			}
			a.Chain.RegisterCommandAspect(ta)
		}

		return nil
	}
}

func (a *App) busFactory() infra.BusFactory {
	if a.Backend != nil && a.Backend.BusFactory != nil {
		return a.Backend.BusFactory
	}
	return infra.NewMemoryBusFactory()
}

// WithCommandHandlers registers command handlers, creating the command bus if needed.
func WithCommandHandlers(handlers ...any) AppOption {
	return func(a *App) error {
		if a.CmdBus == nil {
			a.Chain = ensureChain(a.Chain)
			a.CmdBus = a.busFactory().CreateCommandBus(a.Chain)
		}
		for _, h := range handlers {
			if err := a.CmdBus.RegisterHandler(h); err != nil {
				return fmt.Errorf("register command handler %T: %w", h, err)
			}
		}
		return nil
	}
}

// WithQueryHandlers registers query handlers, creating the query bus if needed.
func WithQueryHandlers(handlers ...any) AppOption {
	return func(a *App) error {
		if a.QueryBus == nil {
			a.Chain = ensureChain(a.Chain)
			a.QueryBus = a.busFactory().CreateQueryBus(a.Chain)
		}
		for _, h := range handlers {
			if err := a.QueryBus.RegisterHandler(h); err != nil {
				return fmt.Errorf("register query handler %T: %w", h, err)
			}
		}
		return nil
	}
}

// WithEventSubscriptions registers event handler subscriptions, creating the event bus if needed.
func WithEventSubscriptions(subs ...any) AppOption {
	return func(a *App) error {
		if a.EventBus == nil {
			a.Chain = ensureChain(a.Chain)
			a.EventBus = a.busFactory().CreateEventBus(a.Chain)
		}
		for _, s := range subs {
			if err := a.EventBus.SubscribeHandler(s); err != nil {
				return fmt.Errorf("subscribe event handler %T: %w", s, err)
			}
		}
		return nil
	}
}

// WithBuses sets all three buses at once.
func WithBuses(cmdBus command.CommandBus, queryBus query.QueryBus, eventBus event.EventBus) AppOption {
	return func(a *App) error {
		a.CmdBus = cmdBus
		a.QueryBus = queryBus
		a.EventBus = eventBus
		return nil
	}
}

// WithCommandBus sets the command bus for the app.
func WithCommandBus(cmdBus command.CommandBus) AppOption {
	return func(a *App) error {
		a.CmdBus = cmdBus
		return nil
	}
}

// WithQueryBus sets the query bus for the app.
func WithQueryBus(queryBus query.QueryBus) AppOption {
	return func(a *App) error {
		a.QueryBus = queryBus
		return nil
	}
}

// WithEventBus sets the event bus for the app.
func WithEventBus(eventBus event.EventBus) AppOption {
	return func(a *App) error {
		a.EventBus = eventBus
		return nil
	}
}

// WithLogger registers a logging aspect using the provided logger.
func WithLogger(logger builtin.Logger) AppOption {
	return func(a *App) error {
		if a.Chain == nil {
			a.Chain = aspect.NewAspectChain()
		}
		a.Chain.RegisterAspect(builtin.NewLoggingAspect(logger))
		return nil
	}
}

// WithMetrics registers a metrics aspect using the provided recorder.
func WithMetrics(recorder builtin.MetricsRecorder) AppOption {
	return func(a *App) error {
		if a.Chain == nil {
			a.Chain = aspect.NewAspectChain()
		}
		a.Chain.RegisterAspect(builtin.NewMetricsAspect(recorder))
		return nil
	}
}

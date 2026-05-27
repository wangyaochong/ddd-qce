package app

import (
	"fmt"

	"github.com/ddd-qce/core/aspect"
	"github.com/ddd-qce/core/aspect/builtin"
	"github.com/ddd-qce/core/config"
	"github.com/ddd-qce/core/cqrs/command"
	"github.com/ddd-qce/core/cqrs/event"
	"github.com/ddd-qce/core/cqrs/impl/memory"
	"github.com/ddd-qce/core/cqrs/query"
	"github.com/ddd-qce/core/infra"
)

type App struct {
	CmdBus    command.CommandBus
	QueryBus  query.QueryBus
	EventBus  event.EventBus
	Chain     *aspect.AspectChain
	Backend   *infra.Backend
	Config    *config.Config
	cleanup   []func() error
}

type AppOption func(*App) error

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

func (a *App) Close() error {
	var errs []error
	for _, fn := range a.cleanup {
		if err := fn(); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("cleanup errors: %v", errs)
	}
	return nil
}

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

func WithBackend(backend *infra.Backend) AppOption {
	return func(a *App) error {
		a.Backend = backend
		return nil
	}
}

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

func WithDefaultAspects() AppOption {
	return func(a *App) error {
		chain := aspect.NewAspectChain()

		logger := builtin.NewStdLogger()
		metrics := builtin.NewInMemMetricsRecorder()
		var txManager builtin.TransactionManager
		if a.Backend != nil && a.Backend.TransactionManager != nil {
			txManager = a.Backend.TransactionManager
		} else {
			txManager = builtin.NewNoOpTransactionManager()
		}

		if a.Config.Aspect.EnableLogging {
			chain.RegisterAspect(builtin.NewLoggingAspect(logger))
		}
		if a.Config.Aspect.EnableTracing {
			if a.Backend != nil && a.Backend.TraceStore != nil {
				chain.RegisterAspect(builtin.NewTracingAspect(a.Backend.TraceStore))
			}
		}
		if a.Config.Aspect.EnableMetrics {
			chain.RegisterAspect(builtin.NewMetricsAspect(metrics))
		}
		if a.Config.Aspect.EnableTransaction {
			ta, err := builtin.NewTransactionAspect(txManager)
			if err != nil {
				return fmt.Errorf("create transaction aspect: %w", err)
			}
			chain.RegisterCommandAspect(ta)
		}

		a.Chain = chain
		return nil
	}
}

func WithCommandHandlers(handlers ...any) AppOption {
	return func(a *App) error {
		bus := memory.NewCommandBus(memory.WithCommandBusAspectChain(a.Chain))
		for _, h := range handlers {
			if err := bus.RegisterHandler(h); err != nil {
				return fmt.Errorf("register command handler %T: %w", h, err)
			}
		}
		a.CmdBus = bus
		return nil
	}
}

func WithQueryHandlers(handlers ...any) AppOption {
	return func(a *App) error {
		bus := memory.NewQueryBus(memory.WithQueryBusAspectChain(a.Chain))
		for _, h := range handlers {
			if err := bus.RegisterHandler(h); err != nil {
				return fmt.Errorf("register query handler %T: %w", h, err)
			}
		}
		a.QueryBus = bus
		return nil
	}
}

func WithEventSubscriptions(subs ...any) AppOption {
	return func(a *App) error {
		bus := memory.NewEventBus(memory.WithBusAspectChain(a.Chain))
		for _, s := range subs {
			if err := bus.SubscribeHandler(s); err != nil {
				return fmt.Errorf("subscribe event handler %T: %w", s, err)
			}
		}
		a.EventBus = bus
		return nil
	}
}

func WithBuses(cmdBus command.CommandBus, queryBus query.QueryBus, eventBus event.EventBus) AppOption {
	return func(a *App) error {
		a.CmdBus = cmdBus
		a.QueryBus = queryBus
		a.EventBus = eventBus
		return nil
	}
}

func WithCommandBus(cmdBus command.CommandBus) AppOption {
	return func(a *App) error {
		a.CmdBus = cmdBus
		return nil
	}
}

func WithQueryBus(queryBus query.QueryBus) AppOption {
	return func(a *App) error {
		a.QueryBus = queryBus
		return nil
	}
}

func WithEventBus(eventBus event.EventBus) AppOption {
	return func(a *App) error {
		a.EventBus = eventBus
		return nil
	}
}

func WithLogger(logger builtin.Logger) AppOption {
	return func(a *App) error {
		if a.Chain == nil {
			a.Chain = aspect.NewAspectChain()
		}
		a.Chain.RegisterAspect(builtin.NewLoggingAspect(logger))
		return nil
	}
}

func WithMetrics(recorder builtin.MetricsRecorder) AppOption {
	return func(a *App) error {
		if a.Chain == nil {
			a.Chain = aspect.NewAspectChain()
		}
		a.Chain.RegisterAspect(builtin.NewMetricsAspect(recorder))
		return nil
	}
}
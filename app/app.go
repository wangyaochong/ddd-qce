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

type App struct {
	CmdBus    command.CommandBus
	QueryBus  query.QueryBus
	EventBus  event.EventBus
	Chain     *aspect.AspectChain
	Backend   *infra.Backend
	Config    *config.Config
	lifecycles []Lifecycle
	cleanup    []func() error
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

func (a *App) RegisterLifecycle(l Lifecycle) {
	a.lifecycles = append(a.lifecycles, l)
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

func ensureChain(chain *aspect.AspectChain) *aspect.AspectChain {
	if chain == nil {
		return aspect.NewAspectChain()
	}
	return chain
}

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

func (a *App) busFactory() *infra.BusFactory {
	if a.Backend != nil && a.Backend.BusFactory != nil {
		return a.Backend.BusFactory
	}
	return infra.NewMemoryBusFactory()
}

func WithCommandHandlers(handlers ...any) AppOption {
	return func(a *App) error {
		if a.CmdBus == nil {
			a.Chain = ensureChain(a.Chain)
			a.CmdBus = a.busFactory().NewCommandBus(a.Chain)
		}
		for _, h := range handlers {
			if err := a.CmdBus.RegisterHandler(h); err != nil {
				return fmt.Errorf("register command handler %T: %w", h, err)
			}
		}
		return nil
	}
}

func WithQueryHandlers(handlers ...any) AppOption {
	return func(a *App) error {
		if a.QueryBus == nil {
			a.Chain = ensureChain(a.Chain)
			a.QueryBus = a.busFactory().NewQueryBus(a.Chain)
		}
		for _, h := range handlers {
			if err := a.QueryBus.RegisterHandler(h); err != nil {
				return fmt.Errorf("register query handler %T: %w", h, err)
			}
		}
		return nil
	}
}

func WithEventSubscriptions(subs ...any) AppOption {
	return func(a *App) error {
		if a.EventBus == nil {
			a.Chain = ensureChain(a.Chain)
			a.EventBus = a.busFactory().NewEventBus(a.Chain)
		}
		for _, s := range subs {
			if err := a.EventBus.SubscribeHandler(s); err != nil {
				return fmt.Errorf("subscribe event handler %T: %w", s, err)
			}
		}
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
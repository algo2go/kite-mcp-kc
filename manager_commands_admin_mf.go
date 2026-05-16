package kc

import (
	"context"
	"fmt"
	"log/slog"
	"reflect"

	"github.com/algo2go/kite-mcp-cqrs"
	"github.com/algo2go/kite-mcp-domain"
	"github.com/algo2go/kite-mcp-eventsourcing"
	"github.com/algo2go/kite-mcp-usecases"
)

// manager_commands_admin_mf.go holds the CommandBus-handler registration
// for mutual-fund admin commands (PlaceMFOrder / CancelMFOrder /
// PlaceMFSIP / CancelMFSIP). Split from kc/manager_commands_admin.go
// (Sprint 3 Option-a). 0 behavior change.
//
// Surface:
//   - AdminMFRegistrarDeps + registerAdminMFCommandsOnBus
//   - (m *Manager).registerMFCommands

type AdminMFRegistrarDeps struct {
	SessionSvc       *SessionService
	DispatcherGetter func() *domain.EventDispatcher
	EventStoreGetter func() *eventsourcing.EventStore
}

// registerAdminMFCommandsOnBus is the package-level pure-function
// registrar for mutual-fund admin commands.
func registerAdminMFCommandsOnBus(
	bus *cqrs.InMemoryBus,
	deps AdminMFRegistrarDeps,
	logger *slog.Logger,
) error {
	if err := bus.Register(reflect.TypeFor[cqrs.PlaceMFOrderCommand](), func(ctx context.Context, msg any) (any, error) {
		cmd, ok := msg.(cqrs.PlaceMFOrderCommand)
		if !ok {
			return nil, fmt.Errorf("cqrs: unexpected command type %T", msg)
		}
		uc := usecases.NewPlaceMFOrderUseCase(deps.SessionSvc, logger)
		if deps.EventStoreGetter != nil {
			if es := deps.EventStoreGetter(); es != nil {
				uc.SetEventStore(es)
			}
		}
		if deps.DispatcherGetter != nil {
			if d := deps.DispatcherGetter(); d != nil {
				uc.SetEventDispatcher(d)
			}
		}
		return uc.Execute(ctx, cmd)
	}); err != nil {
		return err
	}

	if err := bus.Register(reflect.TypeFor[cqrs.CancelMFOrderCommand](), func(ctx context.Context, msg any) (any, error) {
		cmd, ok := msg.(cqrs.CancelMFOrderCommand)
		if !ok {
			return nil, fmt.Errorf("cqrs: unexpected command type %T", msg)
		}
		uc := usecases.NewCancelMFOrderUseCase(deps.SessionSvc, logger)
		if deps.EventStoreGetter != nil {
			if es := deps.EventStoreGetter(); es != nil {
				uc.SetEventStore(es)
			}
		}
		if deps.DispatcherGetter != nil {
			if d := deps.DispatcherGetter(); d != nil {
				uc.SetEventDispatcher(d)
			}
		}
		return uc.Execute(ctx, cmd)
	}); err != nil {
		return err
	}

	if err := bus.Register(reflect.TypeFor[cqrs.PlaceMFSIPCommand](), func(ctx context.Context, msg any) (any, error) {
		cmd, ok := msg.(cqrs.PlaceMFSIPCommand)
		if !ok {
			return nil, fmt.Errorf("cqrs: unexpected command type %T", msg)
		}
		uc := usecases.NewPlaceMFSIPUseCase(deps.SessionSvc, logger)
		if deps.EventStoreGetter != nil {
			if es := deps.EventStoreGetter(); es != nil {
				uc.SetEventStore(es)
			}
		}
		if deps.DispatcherGetter != nil {
			if d := deps.DispatcherGetter(); d != nil {
				uc.SetEventDispatcher(d)
			}
		}
		return uc.Execute(ctx, cmd)
	}); err != nil {
		return err
	}

	if err := bus.Register(reflect.TypeFor[cqrs.CancelMFSIPCommand](), func(ctx context.Context, msg any) (any, error) {
		cmd, ok := msg.(cqrs.CancelMFSIPCommand)
		if !ok {
			return nil, fmt.Errorf("cqrs: unexpected command type %T", msg)
		}
		uc := usecases.NewCancelMFSIPUseCase(deps.SessionSvc, logger)
		if deps.EventStoreGetter != nil {
			if es := deps.EventStoreGetter(); es != nil {
				uc.SetEventStore(es)
			}
		}
		if deps.DispatcherGetter != nil {
			if d := deps.DispatcherGetter(); d != nil {
				uc.SetEventDispatcher(d)
			}
		}
		return uc.Execute(ctx, cmd)
	}); err != nil {
		return err
	}
	return nil
}

// registerMFCommands delegates to the package-level pure-function
// registrar (Tier 2.3 slice 4/6).
func (m *Manager) registerMFCommands() error {
	return registerAdminMFCommandsOnBus(m.commandBus, AdminMFRegistrarDeps{
		SessionSvc:       m.SessionSvc,
		DispatcherGetter: m.eventing.Dispatcher,
		EventStoreGetter: m.eventing.Store,
	}, m.Logger)
}

// --- Ticker: start / stop / subscribe / unsubscribe ------------------------

// AdminTickerRegistrarDeps holds the dependencies for ticker admin
// commands (Start/Stop/Subscribe/Unsubscribe; 4 commands). Single dep
// — TickerServiceGetter — because every handler in this group requires
// the ticker service and errors if nil.


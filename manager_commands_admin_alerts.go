package kc

import (
	"context"
	"fmt"
	"log/slog"
	"reflect"

	"github.com/algo2go/kite-mcp-alerts"
	"github.com/algo2go/kite-mcp-cqrs"
	"github.com/algo2go/kite-mcp-domain"
	"github.com/algo2go/kite-mcp-eventsourcing"
	"github.com/algo2go/kite-mcp-instruments"
	"github.com/algo2go/kite-mcp-usecases"
)

// manager_commands_admin_alerts.go holds the CommandBus-handler
// registration for alert-lifecycle admin commands (create / delete /
// composite / setup-telegram). Split from kc/manager_commands_admin.go
// (Sprint 3 Option-a). 0 behavior change.
//
// Surface:
//   - AdminAlertsRegistrarDeps + registerAdminAlertsCommandsOnBus
//   - (m *Manager).registerAlertCommands

type AdminAlertsRegistrarDeps struct {
	AlertStore         *alerts.Store
	InstrumentsGetter  func() *instruments.Manager      // for adminBatchInstrumentResolver
	DispatcherGetter   func() *domain.EventDispatcher
	EventStoreGetter   func() *eventsourcing.EventStore
}

// registerAdminAlertsCommandsOnBus is the package-level pure-function
// registrar for alert-lifecycle admin commands.
func registerAdminAlertsCommandsOnBus(
	bus *cqrs.InMemoryBus,
	deps AdminAlertsRegistrarDeps,
	logger *slog.Logger,
) error {
	if err := bus.Register(reflect.TypeFor[cqrs.CreateAlertCommand](), func(ctx context.Context, msg any) (any, error) {
		cmd, ok := msg.(cqrs.CreateAlertCommand)
		if !ok {
			return nil, fmt.Errorf("cqrs: unexpected command type %T", msg)
		}
		if deps.AlertStore == nil {
			return nil, fmt.Errorf("cqrs: alert store not configured")
		}
		uc := usecases.NewCreateAlertUseCase(
			deps.AlertStore,
			&adminBatchInstrumentResolver{getInstruments: deps.InstrumentsGetter},
			logger,
		)
		if deps.DispatcherGetter != nil {
			if d := deps.DispatcherGetter(); d != nil {
				uc.SetEventDispatcher(d)
			}
		}
		// Phase C ES: audit-log appender so alert.created lands in domain_events
		// without going through dispatcher→persister (prevents double-emit).
		if deps.EventStoreGetter != nil {
			if es := deps.EventStoreGetter(); es != nil {
				uc.SetEventStore(es)
			}
		}
		return uc.Execute(ctx, cmd)
	}); err != nil {
		return err
	}

	if err := bus.Register(reflect.TypeFor[cqrs.DeleteAlertCommand](), func(ctx context.Context, msg any) (any, error) {
		cmd, ok := msg.(cqrs.DeleteAlertCommand)
		if !ok {
			return nil, fmt.Errorf("cqrs: unexpected command type %T", msg)
		}
		if deps.AlertStore == nil {
			return nil, fmt.Errorf("cqrs: alert store not configured")
		}
		uc := usecases.NewDeleteAlertUseCase(deps.AlertStore, logger)
		if deps.DispatcherGetter != nil {
			if d := deps.DispatcherGetter(); d != nil {
				uc.SetEventDispatcher(d)
			}
		}
		// Phase C ES: audit-log appender owns alert.deleted persistence.
		if deps.EventStoreGetter != nil {
			if es := deps.EventStoreGetter(); es != nil {
				uc.SetEventStore(es)
			}
		}
		return nil, uc.Execute(ctx, cmd)
	}); err != nil {
		return err
	}

	// CreateCompositeAlertCommand — composite alert persistence wired per
	// the Option B design (shared alerts table with alert_type='composite').
	// Shares the same instrument resolver as single alerts.
	if err := bus.Register(reflect.TypeFor[cqrs.CreateCompositeAlertCommand](), func(ctx context.Context, msg any) (any, error) {
		cmd, ok := msg.(cqrs.CreateCompositeAlertCommand)
		if !ok {
			return nil, fmt.Errorf("cqrs: unexpected command type %T", msg)
		}
		if deps.AlertStore == nil {
			return nil, fmt.Errorf("cqrs: alert store not configured")
		}
		uc := usecases.NewCreateCompositeAlertUseCase(
			deps.AlertStore,
			&adminBatchInstrumentResolver{getInstruments: deps.InstrumentsGetter},
			logger,
		)
		// Phase C-Audit: composite alert.created event.
		if deps.EventStoreGetter != nil {
			if es := deps.EventStoreGetter(); es != nil {
				uc.SetEventStore(es)
			}
		}
		return uc.Execute(ctx, cmd)
	}); err != nil {
		return err
	}

	if err := bus.Register(reflect.TypeFor[cqrs.SetupTelegramCommand](), func(ctx context.Context, msg any) (any, error) {
		cmd, ok := msg.(cqrs.SetupTelegramCommand)
		if !ok {
			return nil, fmt.Errorf("cqrs: unexpected command type %T", msg)
		}
		if deps.AlertStore == nil {
			return nil, fmt.Errorf("cqrs: telegram (alert) store not configured")
		}
		uc := usecases.NewSetupTelegramUseCase(deps.AlertStore, logger)
		// ES: typed TelegramSubscribed/ChatBound dispatch for runtime
		// subscribers (projector etc.). Pattern mirrors watchlist
		// command-bus wiring (commit aeb3e8c).
		if deps.DispatcherGetter != nil {
			if d := deps.DispatcherGetter(); d != nil {
				uc.SetEventDispatcher(d)
			}
		}
		return nil, uc.Execute(ctx, cmd)
	}); err != nil {
		return err
	}
	return nil
}

// registerAlertCommands delegates to the package-level pure-function
// registrar (Tier 2.3 slice 3/6).
func (m *Manager) registerAlertCommands() error {
	return registerAdminAlertsCommandsOnBus(m.commandBus, AdminAlertsRegistrarDeps{
		AlertStore:        m.alertStore,
		InstrumentsGetter: func() *instruments.Manager { return m.Instruments },
		DispatcherGetter:  m.eventing.Dispatcher,
		EventStoreGetter:  m.eventing.Store,
	}, m.Logger)
}

// --- Mutual Funds: place / cancel order + SIP ------------------------------

// AdminMFRegistrarDeps holds the dependencies for mutual-fund admin
// commands (PlaceMFOrder, CancelMFOrder, PlaceMFSIP, CancelMFSIP — 4
// commands).


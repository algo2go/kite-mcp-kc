package kc

import (
	"context"
	"fmt"
	"log/slog"
	"reflect"

	"github.com/algo2go/kite-mcp-cqrs"
	"github.com/algo2go/kite-mcp-ticker"
	"github.com/algo2go/kite-mcp-usecases"
)

// manager_commands_admin_ticker.go holds the CommandBus-handler
// registration for ticker admin commands (Start / Stop / Subscribe /
// Unsubscribe). Split from kc/manager_commands_admin.go (Sprint 3
// Option-a). 0 behavior change.
//
// Surface:
//   - AdminTickerRegistrarDeps + registerAdminTickerCommandsOnBus
//   - (m *Manager).registerTickerCommands

type AdminTickerRegistrarDeps struct {
	TickerServiceGetter func() *ticker.Service // required at command-dispatch time
}

// registerAdminTickerCommandsOnBus is the package-level pure-function
// registrar for ticker admin commands.
func registerAdminTickerCommandsOnBus(
	bus *cqrs.InMemoryBus,
	deps AdminTickerRegistrarDeps,
	logger *slog.Logger,
) error {
	if err := bus.Register(reflect.TypeFor[cqrs.StartTickerCommand](), func(ctx context.Context, msg any) (any, error) {
		cmd, ok := msg.(cqrs.StartTickerCommand)
		if !ok {
			return nil, fmt.Errorf("cqrs: unexpected command type %T", msg)
		}
		if deps.TickerServiceGetter == nil {
			return nil, fmt.Errorf("cqrs: ticker service not configured")
		}
		ts := deps.TickerServiceGetter()
		if ts == nil {
			return nil, fmt.Errorf("cqrs: ticker service not configured")
		}
		uc := usecases.NewStartTickerUseCase(ts, logger)
		return nil, uc.Execute(ctx, cmd)
	}); err != nil {
		return err
	}

	if err := bus.Register(reflect.TypeFor[cqrs.StopTickerCommand](), func(ctx context.Context, msg any) (any, error) {
		cmd, ok := msg.(cqrs.StopTickerCommand)
		if !ok {
			return nil, fmt.Errorf("cqrs: unexpected command type %T", msg)
		}
		if deps.TickerServiceGetter == nil {
			return nil, fmt.Errorf("cqrs: ticker service not configured")
		}
		ts := deps.TickerServiceGetter()
		if ts == nil {
			return nil, fmt.Errorf("cqrs: ticker service not configured")
		}
		uc := usecases.NewStopTickerUseCase(ts, logger)
		return nil, uc.Execute(ctx, cmd)
	}); err != nil {
		return err
	}

	if err := bus.Register(reflect.TypeFor[cqrs.SubscribeInstrumentsCommand](), func(ctx context.Context, msg any) (any, error) {
		cmd, ok := msg.(cqrs.SubscribeInstrumentsCommand)
		if !ok {
			return nil, fmt.Errorf("cqrs: unexpected command type %T", msg)
		}
		if deps.TickerServiceGetter == nil {
			return nil, fmt.Errorf("cqrs: ticker service not configured")
		}
		ts := deps.TickerServiceGetter()
		if ts == nil {
			return nil, fmt.Errorf("cqrs: ticker service not configured")
		}
		uc := usecases.NewSubscribeInstrumentsUseCase(ts, logger)
		return nil, uc.Execute(ctx, cmd)
	}); err != nil {
		return err
	}

	if err := bus.Register(reflect.TypeFor[cqrs.UnsubscribeInstrumentsCommand](), func(ctx context.Context, msg any) (any, error) {
		cmd, ok := msg.(cqrs.UnsubscribeInstrumentsCommand)
		if !ok {
			return nil, fmt.Errorf("cqrs: unexpected command type %T", msg)
		}
		if deps.TickerServiceGetter == nil {
			return nil, fmt.Errorf("cqrs: ticker service not configured")
		}
		ts := deps.TickerServiceGetter()
		if ts == nil {
			return nil, fmt.Errorf("cqrs: ticker service not configured")
		}
		uc := usecases.NewUnsubscribeInstrumentsUseCase(ts, logger)
		return nil, uc.Execute(ctx, cmd)
	}); err != nil {
		return err
	}
	return nil
}

// registerTickerCommands delegates to the package-level pure-function
// registrar (Tier 2.3 slice 5/6).
func (m *Manager) registerTickerCommands() error {
	return registerAdminTickerCommandsOnBus(m.commandBus, AdminTickerRegistrarDeps{
		TickerServiceGetter: func() *ticker.Service { return m.tickerService },
	}, m.Logger)
}

// --- Native Alerts: place / modify / delete --------------------------------

// nativeAlertBusAdapter bridges broker.NativeAlertCapable to
// usecases.NativeAlertClient. It mirrors the mcp-layer adapter in
// mcp/native_alert_tools.go — duplicated here so the bus handler stays
// self-contained and does not depend on mcp package code.


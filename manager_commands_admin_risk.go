package kc

import (
	"context"
	"fmt"
	"log/slog"
	"reflect"

	"github.com/algo2go/kite-mcp-cqrs"
	"github.com/algo2go/kite-mcp-instruments"
	"github.com/algo2go/kite-mcp-riskguard"
	"github.com/algo2go/kite-mcp-usecases"
)

// manager_commands_admin_risk.go holds the CommandBus-handler registration
// for risk-guard admin commands (freeze / unfreeze user + global kill
// switches) plus the adminBatchInstrumentResolver helper used by both
// risk-admin and alert-admin paths to resolve instrument tokens.
// Split from kc/manager_commands_admin.go (Sprint 3 Option-a). 0 behavior
// change.
//
// Surface:
//   - AdminRiskRegistrarDeps + registerAdminRiskCommandsOnBus
//   - (m *Manager).registerAdminRiskCommands
//   - adminBatchInstrumentResolver + GetInstrumentToken

type AdminRiskRegistrarDeps struct {
	RiskGuardGetter func() *riskguard.Guard // required at command-dispatch time
}

// registerAdminRiskCommandsOnBus is the package-level pure-function
// registrar for risk-guard admin commands.
func registerAdminRiskCommandsOnBus(
	bus *cqrs.InMemoryBus,
	deps AdminRiskRegistrarDeps,
	logger *slog.Logger,
) error {
	if err := bus.Register(reflect.TypeFor[cqrs.AdminFreezeUserCommand](), func(ctx context.Context, msg any) (any, error) {
		cmd, ok := msg.(cqrs.AdminFreezeUserCommand)
		if !ok {
			return nil, fmt.Errorf("cqrs: unexpected command type %T", msg)
		}
		if deps.RiskGuardGetter == nil {
			return nil, fmt.Errorf("cqrs: risk guard not configured")
		}
		guard := deps.RiskGuardGetter()
		if guard == nil {
			return nil, fmt.Errorf("cqrs: risk guard not configured")
		}
		uc := usecases.NewAdminFreezeUserUseCase(guard, logger)
		return nil, uc.Execute(ctx, cmd)
	}); err != nil {
		return err
	}

	if err := bus.Register(reflect.TypeFor[cqrs.AdminUnfreezeUserCommand](), func(ctx context.Context, msg any) (any, error) {
		cmd, ok := msg.(cqrs.AdminUnfreezeUserCommand)
		if !ok {
			return nil, fmt.Errorf("cqrs: unexpected command type %T", msg)
		}
		if deps.RiskGuardGetter == nil {
			return nil, fmt.Errorf("cqrs: risk guard not configured")
		}
		guard := deps.RiskGuardGetter()
		if guard == nil {
			return nil, fmt.Errorf("cqrs: risk guard not configured")
		}
		uc := usecases.NewAdminUnfreezeUserUseCase(guard, logger)
		return nil, uc.Execute(ctx, cmd)
	}); err != nil {
		return err
	}

	if err := bus.Register(reflect.TypeFor[cqrs.AdminFreezeGlobalCommand](), func(ctx context.Context, msg any) (any, error) {
		cmd, ok := msg.(cqrs.AdminFreezeGlobalCommand)
		if !ok {
			return nil, fmt.Errorf("cqrs: unexpected command type %T", msg)
		}
		if deps.RiskGuardGetter == nil {
			return nil, fmt.Errorf("cqrs: risk guard not configured")
		}
		guard := deps.RiskGuardGetter()
		if guard == nil {
			return nil, fmt.Errorf("cqrs: risk guard not configured")
		}
		uc := usecases.NewAdminFreezeGlobalUseCase(guard, logger)
		return nil, uc.Execute(ctx, cmd)
	}); err != nil {
		return err
	}

	if err := bus.Register(reflect.TypeFor[cqrs.AdminUnfreezeGlobalCommand](), func(ctx context.Context, msg any) (any, error) {
		cmd, ok := msg.(cqrs.AdminUnfreezeGlobalCommand)
		if !ok {
			return nil, fmt.Errorf("cqrs: unexpected command type %T", msg)
		}
		if deps.RiskGuardGetter == nil {
			return nil, fmt.Errorf("cqrs: risk guard not configured")
		}
		guard := deps.RiskGuardGetter()
		if guard == nil {
			return nil, fmt.Errorf("cqrs: risk guard not configured")
		}
		uc := usecases.NewAdminUnfreezeGlobalUseCase(guard, logger)
		return nil, uc.Execute(ctx, cmd)
	}); err != nil {
		return err
	}
	return nil
}

// registerAdminRiskCommands delegates to the package-level pure-function
// registrar (Tier 2.3 slice 2/6).
func (m *Manager) registerAdminRiskCommands() error {
	return registerAdminRiskCommandsOnBus(m.commandBus, AdminRiskRegistrarDeps{
		RiskGuardGetter: m.RiskGuard,
	}, m.Logger)
}

// --- Alerts: create / delete / setup telegram -----------------------------

// adminBatchInstrumentResolver adapts *instruments.Manager to
// usecases.InstrumentResolver. It lives alongside the batch-C handler so
// the handler stays self-contained; the mcp layer has its own adapter of
// the same shape that is retained for mcp-internal use.
//
// Tier 2.3 slice 3/6: the adapter now reads from a closure-captured
// instruments getter rather than a *Manager back-pointer, so the
// Alerts registrar can be tested without a full Manager fixture.
type adminBatchInstrumentResolver struct {
	getInstruments func() *instruments.Manager
}

func (r *adminBatchInstrumentResolver) GetInstrumentToken(exchange, tradingsymbol string) (uint32, error) {
	if r.getInstruments == nil {
		return 0, fmt.Errorf("cqrs: instruments manager not configured")
	}
	im := r.getInstruments()
	if im == nil {
		return 0, fmt.Errorf("cqrs: instruments manager not configured")
	}
	inst, err := im.GetByTradingsymbol(exchange, tradingsymbol)
	if err != nil {
		return 0, err
	}
	return inst.InstrumentToken, nil
}

// AdminAlertsRegistrarDeps holds the dependencies for the alert-lifecycle
// admin commands (create/delete/composite/setup-telegram; 4 commands).
// All deps default to closure-getters per the Tier 2.2 lesson.


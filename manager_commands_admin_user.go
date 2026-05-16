package kc

import (
	"context"
	"fmt"
	"log/slog"
	"reflect"

	"github.com/algo2go/kite-mcp-cqrs"
	"github.com/algo2go/kite-mcp-domain"
	"github.com/algo2go/kite-mcp-riskguard"
	"github.com/algo2go/kite-mcp-usecases"
	"github.com/algo2go/kite-mcp-users"
)

// manager_commands_admin_user.go holds the CommandBus-handler registration
// for admin user-lifecycle commands (suspend / activate / change-role +
// related user-state mutations). Split from kc/manager_commands_admin.go
// (Sprint 3 Option-a) for cohesion. 0 behavior change.
//
// Surface:
//   - AdminUserRegistrarDeps (deps struct, closure-getter pattern)
//   - registerAdminUserCommandsOnBus (pure-function registrar, package-level)
//   - (m *Manager).registerAdminUserCommands (wrapper invoked by
//     registerAdminCommands in kc/manager_commands_admin.go)

// --- Admin: user lifecycle (suspend/activate/change-role) ------------------

// AdminUserRegistrarDeps holds the dependencies for the user-lifecycle
// admin command handlers (suspend/activate/change-role). All deps default
// to closure-getters per the Tier 2.2 lesson (preserve laziness semantics
// at fixture-incomplete tests; eager dereference at registration time can
// change panic-reachability profiles).
type AdminUserRegistrarDeps struct {
	UserStore         *users.Store
	RiskGuardGetter   func() *riskguard.Guard      // may return nil; handler nil-safes
	SessionManager    *SessionRegistry             // may be nil at very-minimal fixtures
	DispatcherGetter  func() *domain.EventDispatcher
}

// registerAdminUserCommandsOnBus is the package-level pure-function
// registrar for user-lifecycle admin commands. Called from
// (m *Manager) registerAdminUserCommands which constructs deps from
// Manager fields.
func registerAdminUserCommandsOnBus(
	bus *cqrs.InMemoryBus,
	deps AdminUserRegistrarDeps,
	logger *slog.Logger,
) error {
	if err := bus.Register(reflect.TypeFor[cqrs.AdminSuspendUserCommand](), func(ctx context.Context, msg any) (any, error) {
		cmd, ok := msg.(cqrs.AdminSuspendUserCommand)
		if !ok {
			return nil, fmt.Errorf("cqrs: unexpected command type %T", msg)
		}
		if deps.UserStore == nil {
			return nil, fmt.Errorf("cqrs: user store not configured")
		}
		// Avoid passing typed-nil through an interface: if RiskGuard() is
		// nil, send the use case an untyped-nil so its `!= nil` guard fires
		// correctly. Same defence the account commands use.
		var rg usecases.RiskGuardService
		if deps.RiskGuardGetter != nil {
			if guard := deps.RiskGuardGetter(); guard != nil {
				rg = guard
			}
		}
		var dispatcher *domain.EventDispatcher
		if deps.DispatcherGetter != nil {
			dispatcher = deps.DispatcherGetter()
		}
		uc := usecases.NewAdminSuspendUserUseCase(
			deps.UserStore,
			rg,
			deps.SessionManager,
			dispatcher,
			logger,
		)
		return uc.Execute(ctx, cmd)
	}); err != nil {
		return err
	}

	if err := bus.Register(reflect.TypeFor[cqrs.AdminActivateUserCommand](), func(ctx context.Context, msg any) (any, error) {
		cmd, ok := msg.(cqrs.AdminActivateUserCommand)
		if !ok {
			return nil, fmt.Errorf("cqrs: unexpected command type %T", msg)
		}
		if deps.UserStore == nil {
			return nil, fmt.Errorf("cqrs: user store not configured")
		}
		uc := usecases.NewAdminActivateUserUseCase(deps.UserStore, logger)
		return nil, uc.Execute(ctx, cmd)
	}); err != nil {
		return err
	}

	if err := bus.Register(reflect.TypeFor[cqrs.AdminChangeRoleCommand](), func(ctx context.Context, msg any) (any, error) {
		cmd, ok := msg.(cqrs.AdminChangeRoleCommand)
		if !ok {
			return nil, fmt.Errorf("cqrs: unexpected command type %T", msg)
		}
		if deps.UserStore == nil {
			return nil, fmt.Errorf("cqrs: user store not configured")
		}
		uc := usecases.NewAdminChangeRoleUseCase(deps.UserStore, logger)
		return uc.Execute(ctx, cmd)
	}); err != nil {
		return err
	}
	return nil
}

// registerAdminUserCommands delegates to the package-level pure-function
// registrar (Tier 2.3, mirrors Tier 2.2 OAuth pattern). Constructs deps
// from Manager fields with closure-getters preserving laziness.
func (m *Manager) registerAdminUserCommands() error {
	return registerAdminUserCommandsOnBus(m.commandBus, AdminUserRegistrarDeps{
		UserStore:        m.userStore,
		RiskGuardGetter:  m.RiskGuard,
		SessionManager:   m.SessionManager,
		DispatcherGetter: m.eventing.Dispatcher,
	}, m.Logger)
}

// --- Admin: risk guard (freeze/unfreeze user + global) ---------------------

// AdminRiskRegistrarDeps holds the dependencies for risk-guard admin
// command handlers (freeze/unfreeze user + global; 4 commands). Single
// dep — RiskGuardGetter — because every handler in this group requires
// the guard and errors out cleanly if it's nil.


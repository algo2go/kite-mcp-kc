package kc

import (
	"context"
	"fmt"
	"log/slog"
	"reflect"

	"github.com/algo2go/kite-mcp-broker"
	"github.com/algo2go/kite-mcp-cqrs"
	"github.com/algo2go/kite-mcp-domain"
	"github.com/algo2go/kite-mcp-eventsourcing"
	"github.com/algo2go/kite-mcp-usecases"
)

// manager_commands_admin_native_alerts.go holds the CommandBus-handler
// registration for native-alert admin commands (Create / Modify /
// Delete) plus the nativeAlertBusAdapter that bridges broker.
// NativeAlertCapable to usecases.NativeAlertClient. Split from
// kc/manager_commands_admin.go (Sprint 3 Option-a). 0 behavior change.
//
// Surface:
//   - nativeAlertBusAdapter (+ 5 adapter methods)
//   - AdminNativeAlertsRegistrarDeps + registerAdminNativeAlertsCommandsOnBus
//   - resolveNativeAlertClientForBus (package-level helper)
//   - (m *Manager).resolveNativeAlertClient
//   - (m *Manager).registerNativeAlertCommands

type nativeAlertBusAdapter struct {
	nac broker.NativeAlertCapable
}

func (a *nativeAlertBusAdapter) CreateAlert(params any) (any, error) {
	p, ok := params.(broker.NativeAlertParams)
	if !ok {
		return nil, fmt.Errorf("cqrs: native alert params must be broker.NativeAlertParams, got %T", params)
	}
	return a.nac.CreateNativeAlert(p)
}

func (a *nativeAlertBusAdapter) ModifyAlert(uuid string, params any) (any, error) {
	p, ok := params.(broker.NativeAlertParams)
	if !ok {
		return nil, fmt.Errorf("cqrs: native alert params must be broker.NativeAlertParams, got %T", params)
	}
	return a.nac.ModifyNativeAlert(uuid, p)
}

func (a *nativeAlertBusAdapter) DeleteAlerts(uuids ...string) error {
	return a.nac.DeleteNativeAlerts(uuids...)
}

func (a *nativeAlertBusAdapter) GetAlerts(filters map[string]string) (any, error) {
	return a.nac.GetNativeAlerts(filters)
}

func (a *nativeAlertBusAdapter) GetAlertHistory(uuid string) (any, error) {
	return a.nac.GetNativeAlertHistory(uuid)
}

// AdminNativeAlertsRegistrarDeps holds the dependencies for native-alert
// admin commands (Place/Modify/Delete; 3 commands).
type AdminNativeAlertsRegistrarDeps struct {
	SessionSvc       *SessionService
	DispatcherGetter func() *domain.EventDispatcher
	EventStoreGetter func() *eventsourcing.EventStore
}

// resolveNativeAlertClientForBus looks up the Kite client for the given
// email via SessionSvc and returns an adapter that satisfies
// usecases.NativeAlertClient. Package-level helper used by the
// native-alert command handlers below; callers that hit a broker without
// native alert support receive a clear error.
func resolveNativeAlertClientForBus(sessionSvc *SessionService, email string) (usecases.NativeAlertClient, error) {
	if sessionSvc == nil {
		return nil, fmt.Errorf("cqrs: session service not configured")
	}
	client, err := sessionSvc.GetBrokerForEmail(email)
	if err != nil {
		return nil, fmt.Errorf("cqrs: resolve broker for %s: %w", email, err)
	}
	nac, ok := client.(broker.NativeAlertCapable)
	if !ok {
		return nil, fmt.Errorf("cqrs: broker does not support native alerts")
	}
	return &nativeAlertBusAdapter{nac: nac}, nil
}

// resolveNativeAlertClient is the Manager-method wrapper preserved for the
// case where Manager methods (other than the registrar) need to resolve
// native-alert clients. Currently a 1-line delegator to the package-level
// helper.
func (m *Manager) resolveNativeAlertClient(email string) (usecases.NativeAlertClient, error) {
	return resolveNativeAlertClientForBus(m.SessionSvc, email)
}

// registerAdminNativeAlertsCommandsOnBus is the package-level pure-function
// registrar for native-alert admin commands.
func registerAdminNativeAlertsCommandsOnBus(
	bus *cqrs.InMemoryBus,
	deps AdminNativeAlertsRegistrarDeps,
	logger *slog.Logger,
) error {
	if err := bus.Register(reflect.TypeFor[cqrs.PlaceNativeAlertCommand](), func(ctx context.Context, msg any) (any, error) {
		cmd, ok := msg.(cqrs.PlaceNativeAlertCommand)
		if !ok {
			return nil, fmt.Errorf("cqrs: unexpected command type %T", msg)
		}
		client, err := resolveNativeAlertClientForBus(deps.SessionSvc, cmd.Email)
		if err != nil {
			return nil, err
		}
		uc := usecases.NewPlaceNativeAlertUseCase(logger)
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
		return uc.Execute(ctx, client, cmd)
	}); err != nil {
		return err
	}

	if err := bus.Register(reflect.TypeFor[cqrs.ModifyNativeAlertCommand](), func(ctx context.Context, msg any) (any, error) {
		cmd, ok := msg.(cqrs.ModifyNativeAlertCommand)
		if !ok {
			return nil, fmt.Errorf("cqrs: unexpected command type %T", msg)
		}
		client, err := resolveNativeAlertClientForBus(deps.SessionSvc, cmd.Email)
		if err != nil {
			return nil, err
		}
		uc := usecases.NewModifyNativeAlertUseCase(logger)
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
		return uc.Execute(ctx, client, cmd)
	}); err != nil {
		return err
	}

	if err := bus.Register(reflect.TypeFor[cqrs.DeleteNativeAlertCommand](), func(ctx context.Context, msg any) (any, error) {
		cmd, ok := msg.(cqrs.DeleteNativeAlertCommand)
		if !ok {
			return nil, fmt.Errorf("cqrs: unexpected command type %T", msg)
		}
		client, err := resolveNativeAlertClientForBus(deps.SessionSvc, cmd.Email)
		if err != nil {
			return nil, err
		}
		uc := usecases.NewDeleteNativeAlertUseCase(logger)
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
		return nil, uc.Execute(ctx, client, cmd)
	}); err != nil {
		return err
	}
	return nil
}

// registerNativeAlertCommands delegates to the package-level pure-function
// registrar (Tier 2.3 slice 6/6).
func (m *Manager) registerNativeAlertCommands() error {
	return registerAdminNativeAlertsCommandsOnBus(m.commandBus, AdminNativeAlertsRegistrarDeps{
		SessionSvc:       m.SessionSvc,
		DispatcherGetter: m.eventing.Dispatcher,
		EventStoreGetter: m.eventing.Store,
	}, m.Logger)
}


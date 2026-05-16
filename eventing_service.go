package kc

import (
	"github.com/algo2go/kite-mcp-domain"
	"github.com/algo2go/kite-mcp-eventsourcing"
	"github.com/algo2go/kite-mcp-alerts"
)

// EventingService groups the domain event dispatcher and append-only event
// store. Both are optional infrastructure: Manager holds the concrete values
// and this service mediates access so use-case code can depend on a narrow
// surface rather than the whole Manager.
//
// Tier 1.2 (Path A.28 follow-up to Tier 1.1's broker_services elimination):
// the back-pointer to *Manager has been replaced with closures over the
// underlying fields. Each closure dereferences the source-of-truth Manager
// field at call time (not at construction time), preserving observable
// behaviour identical to the prior `e.m.X` access. Critical for
// SetDispatcher: the propagated subscribers (projector, sessionSvc,
// trailingStopMgr, 8 Wave-D use-case fields) are nil at
// newEventingService() time and only populated later by initProjector /
// initFocusedServices / initTrailingStop / initOrderUseCases. Closures
// capture by-reference so they read the latest value when SetDispatcher
// runs.
type EventingService struct {
	// Direct dispatcher + store getters/setters.
	getDispatcher func() *domain.EventDispatcher
	setDispatcher func(*domain.EventDispatcher)

	getStore func() *eventsourcing.EventStore
	setStore func(*eventsourcing.EventStore)

	// SetDispatcher side-effect propagation closures. Each closure reads
	// the current Manager field value at call time (closures-by-reference),
	// so SetDispatcher works correctly regardless of init-phase ordering:
	// the propagated subscriber may still be nil at SetDispatcher time
	// (early init path) — getters return nil and the nil-checks skip
	// propagation.
	getProjector       func() *eventsourcing.Projector
	getSessionSvc      func() *SessionService
	getTrailingStopMgr func() *alerts.TrailingStopManager

	// Tier B Step 2 (2026-05-16): Wave D propagation collapsed from 8
	// individual UC closures into a single OrderService accessor. The 8
	// UC fields now live on OrderService; SetDispatcher calls
	// OrderSvc.PropagateDispatcher() which fans the dispatcher into each
	// UC nil-safely.
	getOrderSvc func() *OrderService
}

// newEventingService constructs EventingService with closures over the
// given Manager's fields. Call exactly once at Manager init; the closures
// permit subsequent Manager mutations (e.g., initOrderUseCases populating
// the use-case fields) to remain observable through the facade.
func newEventingService(m *Manager) *EventingService {
	return &EventingService{
		getDispatcher: func() *domain.EventDispatcher { return m.eventDispatcher },
		setDispatcher: func(d *domain.EventDispatcher) { m.eventDispatcher = d },
		getStore:      func() *eventsourcing.EventStore { return m.eventStore },
		setStore:      func(s *eventsourcing.EventStore) { m.eventStore = s },

		getProjector:       func() *eventsourcing.Projector { return m.projector },
		getSessionSvc:      func() *SessionService { return m.Identity.Session },
		getTrailingStopMgr: func() *alerts.TrailingStopManager { return m.AlertSvc.trailingStopMgr },

		// Tier B Step 2: single closure over OrderSvc replaces the 8
		// per-UC closures. OrderSvc.PropagateDispatcher() handles fan-out.
		getOrderSvc: func() *OrderService { return m.OrderSvc },
	}
}

// Dispatcher returns the domain event dispatcher, or nil if not configured.
func (e *EventingService) Dispatcher() *domain.EventDispatcher { return e.getDispatcher() }

// SetDispatcher sets the domain event dispatcher and subscribes the read-side
// projector so order/alert/position events flow into the live aggregate maps.
// Also wires the dispatcher into the session service so new MCP sessions
// emit SessionCreatedEvent, and into the trailing-stop manager so
// successful triggers emit TrailingStopTriggeredEvent.
//
// Wave D Slice D2/D3: also propagates the new dispatcher into the
// startup-once order/GTT use cases the Manager holds. Without this
// propagation, the use cases would have captured a nil dispatcher at
// initOrderUseCases time (because production wires the dispatcher AFTER
// kc.NewWithOptions returns) and silently drop OrderPlaced /
// OrderModified / OrderCancelled / GTTPlaced / GTTModified / GTTDeleted
// events — breaking the audit-log persister + read-side projector.
//
// All Set* propagation calls are nil-safe: the use case's setter accepts
// nil to disable event dispatch, and the Manager-side fields are nil
// until initOrderUseCases runs (so the early-init path before
// registerCQRSHandlers is also safe).
func (e *EventingService) SetDispatcher(d *domain.EventDispatcher) {
	e.setDispatcher(d)
	if proj := e.getProjector(); d != nil && proj != nil {
		proj.Subscribe(d)
	}
	if svc := e.getSessionSvc(); svc != nil {
		svc.SetEventDispatcher(d)
	}
	// Trailing-stop trigger events flow through the same dispatcher so a
	// forensic walk of the SL OrderID sees trailing modifications inline
	// with place/modify/cancel transitions. Nil-safe: trailingStopMgr may
	// be unset in DEV_MODE / no-SQLite configurations.
	if tsm := e.getTrailingStopMgr(); tsm != nil {
		tsm.SetEventDispatcher(d)
	}
	// Tier B Step 2 (2026-05-16): Wave D propagation into the 8 dispatch-
	// aware use cases now flows through OrderService.PropagateDispatcher.
	// The OrderService holds the UC fields and performs the same 8 nil-
	// safe SetEventDispatcher calls internally. OrderSvc itself is nil-
	// tolerant (early-init path before initFocusedServices builds it).
	if os := e.getOrderSvc(); os != nil {
		os.PropagateDispatcher(d)
	}
}

// Store returns the domain audit log (append-only event store), or nil.
func (e *EventingService) Store() *eventsourcing.EventStore { return e.getStore() }

// SetStore sets the domain audit log.
func (e *EventingService) SetStore(s *eventsourcing.EventStore) { e.setStore(s) }

// ---------------------------------------------------------------------------
// Manager-level delegators (moved from manager.go).
// ---------------------------------------------------------------------------

// Eventing returns the eventing service.
func (m *Manager) Eventing() *EventingService { return m.eventing }

// EventDispatcher returns the domain event dispatcher, or nil if not configured.
func (m *Manager) EventDispatcher() *domain.EventDispatcher { return m.eventing.Dispatcher() }

// SetEventDispatcher sets the domain event dispatcher.
func (m *Manager) SetEventDispatcher(d *domain.EventDispatcher) { m.eventing.SetDispatcher(d) }

// EventStoreConcrete returns the domain audit log, or nil if not configured.
func (m *Manager) EventStoreConcrete() *eventsourcing.EventStore { return m.eventing.Store() }

// SetEventStore sets the domain audit log.
func (m *Manager) SetEventStore(s *eventsourcing.EventStore) { m.eventing.SetStore(s) }

// Tier B Step 5a (2026-05-16): Manager.Projector() removed — zero external
// callers across kite-mcp-kc, kite-mcp-bootstrap, kite-mcp-usecases, and
// kite-mcp-server. The projector field remains on Manager; internal users
// read m.projector directly (manager_cqrs_register.go, manager_orders_
// fallback.go). Re-introduce the accessor only when a need arises.

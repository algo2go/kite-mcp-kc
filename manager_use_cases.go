package kc

import (
	"github.com/algo2go/kite-mcp-usecases"
)

// manager_use_cases.go holds the startup-once construction of use case
// instances that the CommandBus / QueryBus handlers dispatch into.
//
// Tier B Step 2 (2026-05-16): the 13 Wave D Phase 1 use cases were
// absorbed into OrderService (see kc/order_service.go). This file now
// holds the thin Manager-side init shim that gathers the dependencies
// available at this phase of NewWithOptions and forwards them to
// OrderService.InitUseCases. The construction sequence inside
// OrderService mirrors the prior monolithic init body verbatim — no
// behavior change.
//
// Preconditions (load-bearing, unchanged):
//   - sessionSvc must be non-nil → satisfied after initFocusedServices
//   - OrderSvc must be non-nil → wired in initFocusedServices
//   - riskGuard MAY be nil → use cases are nil-safe per their docs
//   - eventing must be non-nil (the facade itself; its Dispatcher() is
//     also nil-safe because the use case wraps the dispatch path)
//   - Logger must be non-nil (validated up-front in NewWithOptions)
//   - Instruments MAY be nil at this point if the test fixture left it
//     out; the LotSizeLookup wiring uses InstrumentsManagerConcrete()
//     which is nil-safe at the call site.
//
// EVENT-DISPATCHER NOTE (load-bearing, unchanged):
//
// Production wiring (app/wire.go) calls
// kcManager.SetEventDispatcher(eventDispatcher) AFTER kc.NewWithOptions
// returns. At init-time below, m.eventing.Dispatcher() returns nil. We
// pass nil here; EventingService.SetDispatcher then propagates the real
// dispatcher into each use case via OrderService.PropagateDispatcher
// when app/wire.go finishes wiring. Without this two-phase dispatcher
// wiring, OrderPlaced / GTTPlaced / etc. events would silently drop
// because the use case captured a nil pointer at construction.

// initOrderUseCases gathers the dependencies available at this phase
// and forwards them to OrderService.InitUseCases. Called from
// NewWithOptions after initFocusedServices (which constructs OrderSvc
// + SessionSvc) and BEFORE registerCQRSHandlers (which wires the
// CommandBus handlers that read OrderSvc.* fields).
func (m *Manager) initOrderUseCases() {
	var lotLookup usecases.LotSizeLookup
	if im := m.InstrumentsManagerConcrete(); im != nil {
		lotLookup = &lotSizeLookupAdapter{mgr: im}
	}

	// alertStore is nil-tolerant: OrderService.InitUseCases skips
	// GetAlertsForWidgetUC when the store is nil, matching pre-Step-2
	// behaviour. *alerts.Store implements usecases.WidgetAlertStore
	// structurally (it has List(email) []*alerts.Alert).
	var widgetAlertStore usecases.WidgetAlertStore
	if m.alertStore != nil {
		widgetAlertStore = m.alertStore
	}

	m.OrderSvc.InitUseCases(
		m.riskGuard,
		m.eventing.Dispatcher(), // nil at init-time; PropagateDispatcher fills later
		m.eventStore,
		lotLookup,
		widgetAlertStore,
	)
}

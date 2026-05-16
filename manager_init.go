package kc

import (
	"fmt"

	"github.com/algo2go/kite-mcp-cqrs"
	"github.com/algo2go/kite-mcp-instruments"
)

// manager_init.go holds the package-level helpers that compose Manager.New:
//
//   - initInstrumentsManager — create or accept pre-built instruments mgr
//   - newEmptyManager        — allocate struct + facades + bus instances
//
// The 14 per-phase init METHODS on *Manager that previously lived here have
// been split into per-concern files (manager_init_alerts.go,
// manager_init_persistence.go, manager_init_stores.go, manager_init_services.go)
// to improve cohesion. Phase ordering, mutation targets, and error semantics
// are preserved verbatim — the orchestrator in kc/manager.go (NewWithOptions)
// continues to call them in the load-bearing order documented there.
//
// Every helper takes the Config by value so callers pass the same struct
// they received at the top of New(); no helper introduces a new error
// mode that wasn't already produced at the matching inline line.

// initInstrumentsManager returns the instruments.Manager to assign to
// Manager.Instruments. If the caller already provided one via
// Config.InstrumentsManager we pass it through unchanged; otherwise we
// build one honoring the InstrumentsSkipFetch test-isolation seam.
func initInstrumentsManager(cfg Config) (*instruments.Manager, error) {
	if cfg.InstrumentsManager != nil {
		return cfg.InstrumentsManager, nil
	}
	instrumentsCfg := instruments.Config{
		UpdateConfig: cfg.InstrumentsConfig,
		Logger:       cfg.Logger,
	}
	// Test-isolation seam: when InstrumentsSkipFetch is true, pass an
	// empty TestData map so instruments.New skips the HTTP fetch. This
	// keeps the full Manager wiring exercised (registries, services,
	// event dispatcher) while eliminating the external dependency that
	// causes flaky CI under api.kite.trade rate limits.
	if cfg.InstrumentsSkipFetch {
		instrumentsCfg.TestData = map[uint32]*instruments.Instrument{}
	}
	mgr, err := instruments.New(instrumentsCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create instruments manager: %w", err)
	}
	return mgr, nil
}

// newEmptyManager allocates the Manager struct, fills the fields that
// come straight from Config, and wires the decomposed facades. After
// this helper returns, every subsequent init* method can rely on
// m.Logger, m.tokenStore, m.credentialStore, and the five facade
// services being non-nil.
func newEmptyManager(cfg Config) *Manager {
	m := &Manager{
		apiKey:            cfg.APIKey,
		apiSecret:         cfg.APISecret,
		accessToken:       cfg.AccessToken,
		Logger:            cfg.Logger,
		metrics:           cfg.Metrics,
		appMode:           cfg.AppMode,
		externalURL:       cfg.ExternalURL,
		adminSecretPath:   cfg.AdminSecretPath,
		devMode:           cfg.DevMode,
		kiteClientFactory: &defaultKiteClientFactory{},
		tokenStore:        NewKiteTokenStore(),
		credentialStore:   NewKiteCredentialStore(),
		commandBus:        cqrs.NewInMemoryBus(cqrs.LoggingMiddleware(cfg.Logger)),
		queryBus:          cqrs.NewInMemoryBus(cqrs.LoggingMiddleware(cfg.Logger)),
	}
	// Initialize the decomposed facades. The stores + sessionLifecycle
	// facades still hold a back-pointer to Manager, so each accessor reads
	// the current field value (no stale snapshot). The brokers + eventing
	// + scheduling facades are back-pointer-free as of Tier 1.1 / Tier 1.2 /
	// Tier 1.3 (Path A.28 follow-ups, the "facade-without-back-pointer"
	// closure-DI track): they capture closures over the same Manager fields,
	// preserving the same "read current value" semantics without the
	// *Manager reference. scheduling additionally uses a closure-with-
	// write-back for sessionManager (initialize() constructs the registry
	// and hands it back via setSessionManager).
	m.stores = newStoreRegistry(m)
	m.eventing = newEventingService(m)
	m.brokers = newBrokerServices(m)
	m.scheduling = newSchedulingService(m)
	m.sessionLifecycle = newSessionLifecycleService(m)
	return m
}


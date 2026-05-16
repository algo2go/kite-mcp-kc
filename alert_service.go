package kc

import (
	"github.com/algo2go/kite-mcp-alerts"
)

// AlertService owns alert lifecycle: CRUD, evaluation, trailing stops,
// Telegram notifications, P&L snapshots, AND alert-DB lifecycle.
// Extracted from Manager as part of Clean Architecture / SOLID refactoring
// (Tier B Step 3 — alert subsystem bundle).
//
// Tier B Step 3 absorbed the 6 raw alert fields that previously lived on
// Manager directly: alertStore, alertEvaluator, trailingStopMgr,
// telegramNotifier, alertDB, ownsAlertDB. AlertService is now the single
// source of truth for these; Manager keeps only a *AlertService pointer
// plus thin delegator methods on the Manager surface for backward
// compatibility with the ~60 call sites that read these fields.
//
// Field-write protocol during Manager.NewWithOptions:
//   newEmptyManager constructs an empty AlertService up front. The init*
//   phases (initAlertSystem, initPersistence, initTelegramNotifier,
//   initAlertEvaluator, initTrailingStop) write into m.AlertSvc.<field>
//   directly using same-package field access. After initFocusedServices
//   the service is fully populated; consumer code reads via the accessor
//   methods on either *AlertService or *Manager.
//
// Lifecycle invariant preserved verbatim: Manager.Shutdown closes the
// alert DB iff ownsAlertDB == true (manager opened it via OpenDB rather
// than receiving it from cfg.AlertDB).
type AlertService struct {
	alertStore       *alerts.Store
	alertEvaluator   *alerts.Evaluator
	trailingStopMgr  *alerts.TrailingStopManager
	telegramNotifier *alerts.TelegramNotifier
	pnlService       *alerts.PnLSnapshotService
	alertDB          *alerts.DB
	ownsAlertDB      bool
}

// AlertServiceConfig holds dependencies for creating an AlertService via
// the legacy NewAlertService path. Retained for the two tests in
// service_test.go that construct an AlertService directly; production
// wiring uses NewEmptyAlertService + same-package field writes during
// the manager init phases.
type AlertServiceConfig struct {
	AlertStore       *alerts.Store
	AlertEvaluator   *alerts.Evaluator
	TrailingStopMgr  *alerts.TrailingStopManager
	TelegramNotifier *alerts.TelegramNotifier
}

// NewAlertService creates an AlertService from a pre-built dependency
// bundle. Retained for backward compatibility with the test sites in
// service_test.go.
func NewAlertService(cfg AlertServiceConfig) *AlertService {
	return &AlertService{
		alertStore:       cfg.AlertStore,
		alertEvaluator:   cfg.AlertEvaluator,
		trailingStopMgr:  cfg.TrailingStopMgr,
		telegramNotifier: cfg.TelegramNotifier,
	}
}

// NewEmptyAlertService allocates an AlertService with all fields zero.
// Used by newEmptyManager so the init* phases can populate fields
// in-place via same-package field access. Mirror of Step 2's
// OrderService pattern where the service is constructed before
// dependencies are wired.
func NewEmptyAlertService() *AlertService {
	return &AlertService{}
}

// AlertStore returns the per-user alert store (alert CRUD).
func (as *AlertService) AlertStore() *alerts.Store {
	return as.alertStore
}

// AlertEvaluator returns the tick-to-alert matcher.
func (as *AlertService) AlertEvaluator() *alerts.Evaluator {
	return as.alertEvaluator
}

// TrailingStopManager returns the trailing stop-loss manager.
func (as *AlertService) TrailingStopManager() *alerts.TrailingStopManager {
	return as.trailingStopMgr
}

// TelegramNotifier returns the Telegram alert sender (nil if not configured).
func (as *AlertService) TelegramNotifier() *alerts.TelegramNotifier {
	return as.telegramNotifier
}

// PnLService returns the P&L snapshot service (nil if not initialized).
func (as *AlertService) PnLService() *alerts.PnLSnapshotService {
	return as.pnlService
}

// AlertDB returns the optional SQLite database used for persistence
// (nil when persistence is disabled or no DB path was configured).
func (as *AlertService) AlertDB() *alerts.DB {
	return as.alertDB
}

// OwnsAlertDB reports whether this manager opened the alert DB itself
// (and therefore must Close it on shutdown). Returns false when the DB
// was supplied externally via Config.AlertDB.
func (as *AlertService) OwnsAlertDB() bool {
	return as.ownsAlertDB
}

// SetPnLService sets the P&L snapshot service (called from app layer after initialization).
func (as *AlertService) SetPnLService(svc *alerts.PnLSnapshotService) {
	as.pnlService = svc
}

// ---------------------------------------------------------------------------
// Manager-level delegators (thin pass-throughs to m.AlertSvc)
// ---------------------------------------------------------------------------

// TelegramNotifier returns the Telegram alert sender (nil if not configured).
func (m *Manager) TelegramNotifier() *alerts.TelegramNotifier {
	return m.AlertSvc.TelegramNotifier()
}

// TrailingStopManager returns the trailing stop-loss manager.
func (m *Manager) TrailingStopManager() *alerts.TrailingStopManager {
	return m.AlertSvc.TrailingStopManager()
}

// PnLService returns the P&L snapshot service (nil if not initialized).
func (m *Manager) PnLService() *alerts.PnLSnapshotService {
	return m.AlertSvc.PnLService()
}

// SetPnLService sets the P&L snapshot service.
func (m *Manager) SetPnLService(svc *alerts.PnLSnapshotService) {
	m.AlertSvc.SetPnLService(svc)
}

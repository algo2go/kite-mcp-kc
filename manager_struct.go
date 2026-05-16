package kc

import (
	"errors"
	"html/template"
	"log/slog"

	"github.com/algo2go/kite-mcp-metrics"
	"github.com/algo2go/kite-mcp-audit"
	"github.com/algo2go/kite-mcp-billing"
	"github.com/algo2go/kite-mcp-cqrs"
	"github.com/algo2go/kite-mcp-domain"
	"github.com/algo2go/kite-mcp-eventsourcing"
	"github.com/algo2go/kite-mcp-instruments"
	"github.com/algo2go/kite-mcp-papertrading"
	"github.com/algo2go/kite-mcp-registry"
	"github.com/algo2go/kite-mcp-riskguard"
	"github.com/algo2go/kite-mcp-ticker"
	"github.com/algo2go/kite-mcp-users"
	"github.com/algo2go/kite-mcp-watchlist"
)

// manager_struct.go holds the Manager state declaration plus the small set
// of package-level constants and sentinel errors that surround it.
//
// Anchor 6 PR 6.15 relocated this from kc/manager.go so manager.go can
// stay focused on the constructor surface (New / NewWithOptions /
// NewManager). Pure file move — no behaviour change.

const (
	// Template names
	indexTemplate = "login_success.html"

	// HTTP error messages
	missingParamsMessage  = "missing MCP session_id or Kite request_token"
	sessionErrorMessage   = "error completing Kite session"
	templateNotFoundError = "template not found"
)

var (
	ErrSessionNotFound  = errors.New("MCP session not found or Kite session not associated, try to login again")
	ErrInvalidSessionID = errors.New("invalid MCP session ID, please try logging in again")
)

// KiteSessionData is the transient runtime state of an authenticated MCP
// session. Anchor 5 PR 5.6 relocated the canonical declaration to
// kc/domain/session.go (its proper bounded-context home — see
// anchor-5-prs-design.md). This alias preserves the legacy
// kc.KiteSessionData reference path so the 56-file reverse-dep set
// continues to compile via existing imports.
//
// The PR also collapsed the kc.KiteConnect wrapper indirection: the
// Kite field is now zerodha.KiteSDK (interface) directly, not
// *KiteConnect (one-field struct holding the same SDK). All callsites
// migrated from `.Kite.Client.METHOD(...)` to `.Kite.METHOD(...)`.
// The kc.KiteConnect type + NewKiteConnect helper remain available
// (see kc/kite_connect.go) for the few callers that still need a
// *KiteConnect pointer; the field type change is a deliberate decoupling
// so kc/domain can host KiteSessionData without depending on the kc
// parent package.
type KiteSessionData = domain.KiteSessionData

type Manager struct {
	apiKey      string
	apiSecret   string
	accessToken string
	Logger      *slog.Logger
	metrics     *metrics.Manager

	templates map[string]*template.Template

	// Focused service objects (Clean Architecture)
	Identity      *IdentityService // identity bundle: Credential / Session / ManagedSession / Signer / Lifecycle (Tier B Step 4)
	PortfolioSvc  *PortfolioService // portfolio queries (holdings, positions, margins, profile)
	OrderSvc      *OrderService     // order placement, modification, cancellation
	AlertSvc      *AlertService     // alert lifecycle (CRUD, evaluation, trailing stops, Telegram, P&L)
	FamilyService *FamilyService    // family billing (invite, remove, list, tier resolution)

	// Decomposed facades over the raw fields below (Task 7 — Manager decomposition)
	stores     *StoreRegistry     // all persistence stores
	eventing   *EventingService   // domain event dispatcher + append-only store
	brokers    *BrokerServices    // kite factory, instruments, ticker, paper, riskguard
	scheduling *SchedulingService // cleanup routines, session cleanup hooks, metrics recording

	Instruments       *instruments.Manager
	SessionManager    *SessionRegistry
	tokenStore        *KiteTokenStore             // per-email Kite token cache
	credentialStore   *KiteCredentialStore        // per-email Kite developer app credentials
	tickerService     *ticker.Service             // per-user WebSocket ticker connections
	watchlistStore    *watchlist.Store            // per-user watchlists
	userStore         *users.Store                // registered users (RBAC, lifecycle)
	registryStore     *registry.Store             // pre-registered Kite app credentials (key registry)
	encryptionKey     []byte                      // AES-256 key derived via HKDF from cfg.EncryptionSecret; mirrors alertDB.encryptionKey for stores that encrypt outside the alerts.DB layer (e.g. users.Store TOTP secrets)
	auditStore        *audit.Store                // optional: audit trail for synthetic events
	riskGuard         *riskguard.Guard            // optional: financial safety controls
	paperEngine       *papertrading.PaperEngine   // optional: virtual trading engine
	billingStore      *billing.Store              // optional: billing tier enforcement
	invitationStore   *users.InvitationStore      // optional: family invitation management
	eventDispatcher   *domain.EventDispatcher     // optional: domain event pub/sub
	eventStore        *eventsourcing.EventStore   // optional: domain audit log (append-only, not used for state reconstitution)
	projector         *eventsourcing.Projector    // read-side projection of order/alert/position aggregates; subscribes to eventDispatcher
	mcpServer         any                         // *server.MCPServer — stored as any to avoid circular import
	kiteClientFactory KiteClientFactory           // creates zerodha.KiteSDK instances; mockable in tests
	commandBus        *cqrs.InMemoryBus           // CQRS command bus (nil until wired by app/wire.go)
	queryBus          *cqrs.InMemoryBus           // CQRS query bus (nil until wired by app/wire.go)
	appMode           string
	externalURL       string
	adminSecretPath   string
	devMode           bool

	// Tier B Step 2 (2026-05-16): the 13 Wave D Phase 1 use-case fields
	// that previously lived here have been absorbed into OrderService.
	// They are now reachable as m.OrderSvc.{PlaceOrderUC, ModifyOrderUC,
	// CancelOrderUC, PlaceGTTUC, ModifyGTTUC, DeleteGTTUC, ClosePositionUC,
	// CloseAllPositionsUC, GetOrderMarginsUC, GetBasketMarginsUC,
	// GetOrderChargesUC, GetPortfolioForWidgetUC, GetAlertsForWidgetUC}.
	//
	// EventDispatcher propagation (production wire.go calls
	// SetEventDispatcher AFTER kc.NewWithOptions returns) now flows
	// through OrderService.PropagateDispatcher — see EventingService.
	//
	// Tier B Step 3 (2026-05-16): the 6 raw alert subsystem fields that
	// previously lived here (alertStore, alertEvaluator, trailingStopMgr,
	// telegramNotifier, alertDB, ownsAlertDB) have been absorbed into
	// AlertService. They are now reachable as m.AlertSvc.<Accessor>() or
	// via the existing Manager-level delegators (m.AlertStore(),
	// m.AlertDB(), m.TelegramNotifier(), m.TrailingStopManager(), etc.).
	// encryptionKey stays on Manager because it crosses cluster boundaries
	// — also used for TOTP MFA secrets via users.Store (see kc/users/mfa.go).
	//
	// Tier B Step 4 (2026-05-16): the 5 identity/session fields that
	// previously lived here (CredentialSvc, SessionSvc, ManagedSessionSvc,
	// SessionSigner, sessionLifecycle) have been absorbed into IdentityService.
	// They are now reachable as m.Identity.{Credential, Session,
	// ManagedSession, Signer, Lifecycle}. Manager field count reduced
	// 43 → 38 by this move. SessionManager stays on Manager (per preflight
	// §2.4) because it has 4 external direct-readers in
	// kite-mcp-bootstrap / kite-mcp-usecases.
}

package kc

import (
	"github.com/zerodha/gokiteconnect/v4/models"
	"github.com/algo2go/kite-mcp-alerts"
	"github.com/algo2go/kite-mcp-eventsourcing"
	"github.com/algo2go/kite-mcp-instruments"
	"github.com/algo2go/kite-mcp-ticker"
)

// manager_init_services.go holds the late-stage service-wiring phase methods:
// initTelegramNotifier (Telegram bot, optional), initTickerService
// (per-user WebSocket ticker), initFocusedServices (Clean-Architecture sub-
// services: session/portfolio/order/alert), initSessionPersistence (session
// registry DB adapter), initTokenRotation (token->ticker update observer),
// and initProjector (read-side projector for event sourcing).
//
// Phase ordering is load-bearing — see kc/manager.go NewWithOptions for the
// canonical 16-phase sequence. Split from kc/manager_init.go for cohesion;
// 0 behavior change.

// initTelegramNotifier wires the Telegram bot when a token is provided.
// Failure is non-fatal; the server runs without Telegram notifications.
//
// When cfg.BotFactory is non-nil (test injection), the per-Manager factory
// is used directly — bypassing the kc/alerts package-level newBotFunc global.
// Production wiring leaves BotFactory nil and the package-default tgbotapi
// factory is consulted.
func (m *Manager) initTelegramNotifier(cfg Config) {
	if cfg.TelegramBotToken == "" {
		return
	}
	var notifier *alerts.TelegramNotifier
	var tgErr error
	if cfg.BotFactory != nil {
		notifier, tgErr = alerts.NewTelegramNotifierWithFactory(cfg.TelegramBotToken, m.AlertSvc.alertStore, cfg.Logger, cfg.BotFactory)
	} else {
		notifier, tgErr = alerts.NewTelegramNotifier(cfg.TelegramBotToken, m.AlertSvc.alertStore, cfg.Logger)
	}
	if tgErr != nil {
		cfg.Logger.Warn("Telegram notifier failed to initialize", "error", tgErr)
		return
	}
	m.AlertSvc.telegramNotifier = notifier
}

// initTickerService constructs the per-user WebSocket ticker with the
// alert-evaluator + trailing-stop-manager as OnTick callbacks.
func (m *Manager) initTickerService(cfg Config) {
	m.tickerService = ticker.New(ticker.Config{
		Logger: cfg.Logger,
		OnTick: func(email string, tick models.Tick) {
			m.AlertSvc.alertEvaluator.Evaluate(email, tick)
			m.AlertSvc.trailingStopMgr.Evaluate(email, tick)
		},
	})
}

// initFocusedServices builds the Clean-Architecture sub-services on
// top of the raw stores/clients wired by the earlier phases. Order
// matters within this method: sessionSvc depends on sessionManager
// (built in newEmptyManager via newSessionLifecycleService); portfolio
// and order services depend on sessionSvc.
func (m *Manager) initFocusedServices(cfg Config, instrumentsManager *instruments.Manager) {
	m.Instruments = instrumentsManager
	m.scheduling.initialize()

	// Initialize session service (uses credential service + session manager)
	var metricsImpl metricsTracker
	if cfg.Metrics != nil {
		metricsImpl = cfg.Metrics
	}
	m.Identity.Session = NewSessionService(SessionServiceConfig{
		CredentialSvc: m.Identity.Credential,
		TokenStore:    m.tokenStore,
		SessionSigner: m.Identity.Signer,
		Logger:        cfg.Logger,
		Metrics:       metricsImpl,
		DevMode:       cfg.DevMode,
	})
	m.Identity.Session.SetSessionManager(m.SessionManager)
	m.Identity.ManagedSession = NewManagedSessionService(m.SessionManager)

	// Initialize portfolio and order services
	m.PortfolioSvc = NewPortfolioService(m.Identity.Session, cfg.Logger)
	m.OrderSvc = NewOrderService(m.Identity.Session, cfg.Logger)

	// AlertSvc was constructed empty in newEmptyManager and populated
	// in-place by the earlier alert/persistence/telegram init phases
	// (Tier B Step 3). Nothing to do here for the alert subsystem.
}

// initSessionPersistence threads the shared alert DB into the session
// registry so MCP sessions survive restart. No-op when persistence is
// disabled.
func (m *Manager) initSessionPersistence(cfg Config) {
	if m.AlertSvc.alertDB == nil {
		return
	}
	m.SessionManager.SetDB(&sessionDBAdapter{db: m.AlertSvc.alertDB})
	if err := m.SessionManager.LoadFromDB(); err != nil {
		cfg.Logger.Error("Failed to load sessions from DB", "error", err)
	} else {
		cfg.Logger.Info("Sessions loaded from database")
	}
}

// initTokenRotation registers the token→ticker update observer so a
// refreshed Kite token seamlessly propagates to any live ticker.
func (m *Manager) initTokenRotation() {
	m.tokenStore.OnChange(func(email string, entry *KiteTokenEntry) {
		if m.tickerService.IsRunning(email) {
			apiKey := m.Identity.Credential.GetAPIKeyForEmail(email)
			if err := m.tickerService.UpdateToken(email, apiKey, entry.AccessToken); err != nil {
				m.Logger.Error("Failed to update ticker token", "email", email, "error", err)
			} else {
				m.Logger.Info("Ticker token rotated automatically", "email", email)
			}
		}
	})
}

// initProjector allocates the read-side projector. Kept in a named
// helper so the parent New() stays purely a composition sequence.
func (m *Manager) initProjector() {
	// The projector is empty until SetEventDispatcher wires it to a
	// real dispatcher in app/wire.go; tests that skip dispatcher setup
	// still get a usable empty projector.
	m.projector = eventsourcing.NewProjector()
}


package kc

import (
	"github.com/algo2go/kite-mcp-alerts"
)

// manager_init_persistence.go holds the persistence-related phase methods:
// initPersistence (alert DB sourcing + token/credential persistence +
// credential encryption via HKDF), initCredentialWiring (credential->token
// invalidation hook), and initInjectedStores (audit/riskguard/billing/
// invitation store injection seam for app/wire.go).
//
// Phase ordering is load-bearing — see kc/manager.go NewWithOptions for the
// canonical 16-phase sequence. Split from kc/manager_init.go for cohesion;
// 0 behavior change.

// initPersistence wires an alert DB into the alert / token / credential
// stores. Credential encryption is enabled when Config.EncryptionSecret
// is supplied. Errors at each step are logged and tolerated — the
// server falls back to in-memory storage rather than failing startup
// (matches the prior inline behaviour).
//
// DB sourcing precedence:
//  1. cfg.AlertDB (externally-opened) — used as-is, manager does NOT
//     own its lifecycle. This is the inversion seam for app/wire.go,
//     which opens the DB once and constructs DB-backed stores
//     (audit/riskguard/billing/invitation) before kc.NewWithOptions.
//  2. cfg.AlertDBPath (legacy) — manager opens via alerts.OpenDB and
//     owns the lifecycle (closes on Manager.Shutdown).
//  3. Both empty — in-memory mode, no persistence.
func (m *Manager) initPersistence(cfg Config) {
	var alertDB *alerts.DB
	if cfg.AlertDB != nil {
		alertDB = cfg.AlertDB
		m.ownsAlertDB = false
	} else {
		if cfg.AlertDBPath == "" {
			return
		}
		opened, dbErr := alerts.OpenDB(cfg.AlertDBPath)
		if dbErr != nil {
			cfg.Logger.Error("Failed to open alert DB, using in-memory only", "error", dbErr)
			return
		}
		alertDB = opened
		m.ownsAlertDB = true
	}
	m.alertDB = alertDB
	// Set up credential encryption if a secret is provided
	if cfg.EncryptionSecret != "" {
		encKey, encErr := alerts.EnsureEncryptionSalt(alertDB, cfg.EncryptionSecret)
		if encErr != nil {
			cfg.Logger.Error("Failed to derive encryption key with salt", "error", encErr)
		} else {
			alertDB.SetEncryptionKey(encKey)
			// Cache the key on Manager so initStores can wire it into
			// userStore (TOTP MFA secrets are AES-256-GCM-encrypted with
			// the same key — see kc/users/mfa.go).
			m.encryptionKey = encKey
			cfg.Logger.Info("Credential encryption enabled (with HKDF salt)")
		}
	}
	m.alertStore.SetDB(alertDB)
	if err := m.alertStore.LoadFromDB(); err != nil {
		cfg.Logger.Error("Failed to load alerts from DB", "error", err)
	} else {
		cfg.Logger.Info("Alerts loaded from database", "path", cfg.AlertDBPath)
	}
	// Token persistence: share the same DB
	m.tokenStore.SetDB(alertDB)
	m.tokenStore.SetLogger(cfg.Logger)
	if err := m.tokenStore.LoadFromDB(); err != nil {
		cfg.Logger.Error("Failed to load tokens from DB", "error", err)
	} else {
		cfg.Logger.Info("Tokens loaded from database", "count", m.tokenStore.Count())
	}
	// Credential persistence: share the same DB
	m.credentialStore.SetDB(alertDB)
	m.credentialStore.SetLogger(cfg.Logger)
	if err := m.credentialStore.LoadFromDB(); err != nil {
		cfg.Logger.Error("Failed to load credentials from DB", "error", err)
	} else {
		cfg.Logger.Info("Credentials loaded from database", "count", m.credentialStore.Count())
	}
}

// initCredentialWiring installs the cross-store hook that clears a
// user's cached Kite token when their API key changes. Tiny helper
// but kept separate so the tight dependency — credentialStore hook
// reads m.tokenStore — is visible on one line.
func (m *Manager) initCredentialWiring() {
	// Wire credential → token invalidation: when a user's API key changes,
	// delete the cached Kite token (it was issued for the old app).
	m.credentialStore.OnTokenInvalidate(func(email string) {
		m.tokenStore.Delete(email)
	})
}

// initInjectedStores populates the four DB-backed store fields
// (auditStore, riskGuard, billingStore, invitationStore) from the
// matching Config fields when supplied via With* options. This is the
// constructor-injection seam that replaces the post-init SetX setters
// for production wiring (app/wire.go); the SetX setters remain as
// deprecated shims for the ~70+ test sites that mutate the manager
// after construction.
//
// nil-tolerant: any field left nil on Config is a no-op here, matching
// the legacy "store wired later via SetX or never wired at all" path.
func (m *Manager) initInjectedStores(cfg Config) {
	if cfg.AuditStore != nil {
		m.auditStore = cfg.AuditStore
	}
	if cfg.RiskGuard != nil {
		m.riskGuard = cfg.RiskGuard
	}
	if cfg.BillingStore != nil {
		m.billingStore = cfg.BillingStore
	}
	if cfg.InvitationStore != nil {
		m.invitationStore = cfg.InvitationStore
	}
}


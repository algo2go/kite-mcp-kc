package kc

import (
	"fmt"

	"github.com/algo2go/kite-mcp-alerts"
	"github.com/algo2go/kite-mcp-registry"
	"github.com/algo2go/kite-mcp-users"
	"github.com/algo2go/kite-mcp-watchlist"
)

// manager_init_stores.go holds the store-construction phase methods:
// initSideStores (watchlist + user + key-registry) and
// initCredentialService (CredentialService + registry backfill + trailing-
// stop modifier wiring).
//
// Phase ordering is load-bearing — see kc/manager.go NewWithOptions for the
// canonical 16-phase sequence. Split from kc/manager_init.go for cohesion;
// 0 behavior change.

// initSideStores brings up the watchlist, user, and key-registry stores.
// All three share the same SQLite DB (m.alertDB) when persistence is
// enabled and fall back to in-memory when it isn't.
func (m *Manager) initSideStores(cfg Config) {
	// Initialize watchlist store
	m.watchlistStore = watchlist.NewStore()
	m.watchlistStore.SetLogger(cfg.Logger)
	if m.alertDB != nil {
		if err := watchlist.InitTables(m.alertDB); err != nil {
			cfg.Logger.Error("Failed to create watchlist tables", "error", err)
		} else {
			m.watchlistStore.SetDB(m.alertDB)
			if err := m.watchlistStore.LoadFromDB(); err != nil {
				cfg.Logger.Error("Failed to load watchlists from DB", "error", err)
			} else {
				cfg.Logger.Info("Watchlists loaded from database")
			}
		}
	}

	// Initialize user store (RBAC, lifecycle)
	m.userStore = users.NewStore()
	m.userStore.SetLogger(cfg.Logger)
	if m.alertDB != nil {
		m.userStore.SetDB(m.alertDB)
		if err := m.userStore.InitTable(); err != nil {
			cfg.Logger.Error("Failed to create users table", "error", err)
		} else if err := m.userStore.LoadFromDB(); err != nil {
			cfg.Logger.Error("Failed to load users from DB", "error", err)
		} else {
			cfg.Logger.Info("Users loaded from database", "count", m.userStore.Count())
		}
	}
	// Wire the encryption key for TOTP MFA secrets. Same HKDF-derived key
	// the rest of T1 storage uses — rotation via cmd/rotate-key already
	// handles the round-trip migration. SetEncryptionKey is a no-op when
	// the key is empty (DEV_MODE without OAUTH_JWT_SECRET).
	if len(m.encryptionKey) > 0 {
		m.userStore.SetEncryptionKey(m.encryptionKey)
	}

	// Initialize key registry store (zero-config onboarding)
	m.registryStore = registry.New()
	m.registryStore.SetLogger(cfg.Logger)
	if m.alertDB != nil {
		m.registryStore.SetDB(m.alertDB)
		if err := m.registryStore.LoadFromDB(); err != nil {
			cfg.Logger.Error("Failed to load registry from DB", "error", err)
		} else {
			cfg.Logger.Info("App registry loaded from database", "count", m.registryStore.Count())
		}
	}
}

// initCredentialService builds the focused CredentialService on top of
// the three stores (credentialStore, tokenStore, registryStore) and
// wires it into the trailing-stop order-modifier hook. The backfill
// pass brings pre-registry self-provisioned keys into the new registry
// store so later lookups are uniform.
func (m *Manager) initCredentialService(cfg Config) {
	m.CredentialSvc = NewCredentialService(CredentialServiceConfig{
		APIKey:          cfg.APIKey,
		APISecret:       cfg.APISecret,
		AccessToken:     cfg.AccessToken,
		CredentialStore: m.credentialStore,
		TokenStore:      m.tokenStore,
		RegistryStore:   m.registryStore,
		Logger:          cfg.Logger,
	})

	// Backfill registry from existing credentials (handles pre-registry self-provisioned keys)
	m.CredentialSvc.BackfillRegistryFromCredentials()

	// Wire the order modifier: creates a Kite client from cached tokens.
	// This depends on CredentialSvc existing — that's why it lives here
	// rather than in initTrailingStop above.
	m.trailingStopMgr.SetModifier(func(email string) (alerts.KiteOrderModifier, error) {
		apiKey := m.CredentialSvc.GetAPIKeyForEmail(email)
		accessToken := m.CredentialSvc.GetAccessTokenForEmail(email)
		if accessToken == "" {
			return nil, fmt.Errorf("no Kite access token for %s", email)
		}
		client := m.kiteClientFactory.NewClientWithToken(apiKey, accessToken)
		return client, nil
	})
}


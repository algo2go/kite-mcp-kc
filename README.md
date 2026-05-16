# kite-mcp-kc

[![Go Reference](https://pkg.go.dev/badge/github.com/algo2go/kite-mcp-kc.svg)](https://pkg.go.dev/github.com/algo2go/kite-mcp-kc)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

Core MCP orchestration for the `algo2go/kite-mcp-*` module family. Hosts the
`Manager` composition root that owns per-user Kite sessions, the 5 Tier-1
facade services (stores, eventing, brokers, scheduling, session lifecycle),
the 7 hexagonal-architecture port interfaces in `kc/ports`, the operations
runbook handlers in `kc/ops`, and the CQRS command/query bus registrar.
Largest module in the family by LOC (~18K production, ~37K test).

## Status

**v0.1.3 — production.** Active development; functional-option API stable.
Consumed via GOPROXY by `kite-mcp-bootstrap` (composition root) and
`kite-mcp-tools-common` (typed Provider ports in `ToolHandlerDeps`).

## Install

```bash
go get github.com/algo2go/kite-mcp-kc@v0.1.3
```

## Quick start

```go
import (
    "context"

    "github.com/algo2go/kite-mcp-kc"
    "github.com/algo2go/kite-mcp-metrics"
)

// Functional-options constructor (preferred).
mgr, err := kc.NewWithOptions(context.Background(),
    kc.WithKiteCredentials(apiKey, apiSecret),
    kc.WithLogger(slogLogger),
    kc.WithExternalURL("https://example.fly.dev"),
    kc.WithAlertDBPath("/data/alerts.db"),
    kc.WithEncryptionSecret(jwtSecret),
    kc.WithMetrics(metrics.New(metrics.Config{Logger: slogLogger})),
    kc.WithAdminSecretPath("/admin/ops"),
)
if err != nil {
    log.Fatal(err)
}
defer mgr.Shutdown(context.Background())

// Per-user session lifecycle
sessionID := mgr.GenerateSession()
loginURL  := mgr.LoginURL(sessionID, "user@example.com")
// ... user completes Kite OAuth → /callback hits mgr.CompleteSession(...)

// Read-side access (used by every MCP tool body)
brokerClient, ok := mgr.GetBrokerForEmail(email)

// Wire the per-domain register methods into the MCP server registrations
// (account / admin / alerts / exit / MF / native-alerts / OAuth-bridge /
//  orders / setup / ticker). See bootstrap for full wiring.
```

## Public API

### Root package `kc`

- **`kc.Manager`** — composition root. ~91 exported methods spanning store
  accessors (`AlertStore`, `AuditStore`, `BillingStore`, `CredentialStore`,
  `EventStoreConcrete`, …), broker registry (`Brokers`,
  `GetBrokerForEmail`), session lifecycle (`GenerateSession`,
  `CompleteSession`, `CompleteSessionAndRotate`, `ClearSession`,
  `CleanupExpiredSessions`, `GetActiveSessionCount`), credential
  surface (`APIKey`, `GetAPIKeyForEmail`, `GetAPISecretForEmail`,
  `GetAccessTokenForEmail`), config (`AdminSecretPath`, `DevMode`,
  `ExternalURL`), CQRS (`CommandBus`), eventing (`Eventing`,
  `EventDispatcher`), and instruments (`ForceInstrumentsUpdate`).
- **Constructors** — three patterns:
  - **`NewWithOptions(ctx, opts...)`** — preferred; functional options.
  - **`New(cfg Config)`** — config-struct constructor.
  - **`NewManager(apiKey, apiSecret, logger)`** — minimal-credentials
    helper for tests and tiny embedders.
- **`kc.Option`** — 19 functional-option builders: `WithConfig`,
  `WithContext`, `WithLogger`, `WithKiteCredentials`, `WithAccessToken`,
  `WithMetrics`, `WithTelegramBotToken`, `WithAlertDBPath`, `WithAppMode`,
  `WithExternalURL`, `WithAdminSecretPath`, `WithEncryptionSecret`,
  `WithDevMode`, `WithInstrumentsManager`, `WithInstrumentsConfig`,
  `WithInstrumentsSkipFetch`, `WithSessionSigner`, `WithBotFactory`.
- **`kc.Config`** — flat config struct (used by `New` + composed by
  `WithConfig`).
- Per-domain registration methods on `*Manager` invoked by the
  bootstrap's tool-registry init sequence: `RegisterAccount`,
  `RegisterAdmin`, `RegisterAlerts`, `RegisterExit`, `RegisterMF`,
  `RegisterNativeAlerts`, `RegisterOAuthBridge`, `RegisterOrders`,
  `RegisterSetup`, `RegisterTicker`.

### Sub-package `kc/ports`

Hexagonal-architecture port interfaces. Every Provider here narrows the
god-object `*kc.Manager` to the minimum surface a tool handler actually
needs — the Sprint 5 typed-deps architectural payoff. Seven interfaces:

- **`ports.SessionPort`** — session lifecycle (create, lookup, complete,
  rotate, clear, cleanup, active-count).
- **`ports.CredentialPort`** — per-user Kite-credential surface
  (`GetAPIKey`, `GetAPISecret`, `GetAccessToken`, encrypted store CRUD).
- **`ports.AlertPort`** — alert-store read/write.
- **`ports.OrderPort`** — broker-order read/write narrowed to the
  fields a tool handler needs (no full `gokiteconnect.Client`).
- **`ports.InstrumentPort`** — instrument-master lookups + freeze-qty.
- **`ports.AuditStoreConcreteProvider`** — concrete audit-store reach
  for the Phase 3 ops sub-git.
- **`ports.SessionRegistryProvider`** — concrete session-registry reach
  for the same sub-git.

**Leaf-stability invariant**: `kc/ports` must NOT import `kc` parent.
Enforced by `kc/ports/leaf_stability_test.go` (compile-time test).
Inverse direction (`kc/ports/assertions.go` declaring
`var _ SessionPort = (*kc.Manager)(nil)`) is intentionally allowed; the
one-way graph is what makes the port abstraction work.

### Sub-package `kc/ops`

Operations runbook handlers — admin endpoints + user-account
self-service flows. Sub-packages:

- **`ops/admin`** — admin-role-gated endpoints (user listing, session
  introspection, kill switches, metrics).
- **`ops/shared`** — handler primitives reused by both admin + user.
- **`ops/user`** — per-user self-service (credential rotation, alert
  CRUD, paper-portfolio reset).

### Sub-package `kc/internal`

Implementation details NOT part of the public contract. Pinned reserve
for utility code that should never escape the module boundary
(see `kc/internal/util/`).

## Used by

- **`algo2go/kite-mcp-bootstrap`** — composition root; constructs the
  `*kc.Manager` via `NewWithOptions` and wires it through the per-domain
  Register methods into the HTTP mux + MCP tool registry.
- **`algo2go/kite-mcp-tools-common`** — the typed Provider ports in
  `common/handler_deps.go` reference `kc/ports.*` interfaces by name;
  the legacy `Tool.Handler(*kc.Manager)` signature also keeps a direct
  `*kc.Manager` reference during the Sprint 5 migration window.

## Design

Origin: extracted 2026-05-16 from
`github.com/algo2go/kite-mcp-bootstrap/kc` as Phase 1 of the
bootstrap-decomposition arc (Shape A\* in the strategy doc at
`kite-mcp-server/.research/bootstrap-decomp-strategy.md`, commit
`280ae67`). Zero behavior change at extraction.

**Why kc is the composition root**: every other algo2go/kite-mcp-\*
module is either a leaf (clockport, metrics, isttz, sectors, money,
i18n, legaldocs, logger, templates, aop, domain) or a bounded-context
service (alerts, audit, billing, broker, riskguard, oauth,
papertrading, usecases, scheduler, telegram, users, watchlist,
instruments, ticker, registry, cqrs, eventsourcing). kc binds them
into one orchestrated unit via `Manager` — the only place that
references most of the others simultaneously. This is the "monolith"
that the decomposition arc is actively chipping down: every kc method
that can be pushed into a per-domain service object reduces kc's
god-object weight.

**Tier-1 facade services** owned by `Manager`:
1. **Stores facade** — alert / audit / billing / credential / event /
   instrument / watchlist / user / session-registry stores.
2. **Eventing facade** — `EventDispatcher` + `Eventing` accessor for
   domain-event publication.
3. **Brokers facade** — per-user broker-client registry
   (gokiteconnect-backed; one client cached per authenticated user).
4. **Scheduling facade** — periodic-task scheduler (token-rotation,
   alert-evaluation, instrument-update polls).
5. **Session lifecycle facade** — `GenerateSession`, `CompleteSession`,
   `CompleteSessionAndRotate`, `ClearSession`, `CleanupExpiredSessions`.

**Seven focused service objects** (Sprint 5 cohesion target):
`CredentialService`, `SessionService`, `ManagedSessionService`,
`PortfolioService`, `OrderService`, `AlertService`, `FamilyService`.
Each is a narrow surface on top of the stores + brokers facades; the
Sprint 5 / Tier B Step 2 refactor (commit `0534edd`) folded 13 Wave D
use-case fields into `OrderService` as the most recent cohesion pass.

Related modules:
- `algo2go/kite-mcp-clockport` — `Clock` port for deterministic
  scheduler tests.
- `algo2go/kite-mcp-metrics` — telemetry; mounted via `WithMetrics`.
- `algo2go/kite-mcp-broker` — broker port interface; gokiteconnect
  adapter wraps it.
- `algo2go/kite-mcp-tools-common` — narrows kc's surface to the typed
  Provider ports the tool handlers actually need.

## Contributing

Issues + PRs at [github.com/algo2go/kite-mcp-kc](https://github.com/algo2go/kite-mcp-kc).
MIT licensed.

## License

MIT — see [LICENSE](LICENSE).

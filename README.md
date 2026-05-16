# kite-mcp-kc

Core MCP orchestration for the `algo2go/*` module family. Hosts the `Manager`
composition root that owns per-user Kite sessions, the 5 Tier-1 facade
services (stores, eventing, brokers, scheduling, session lifecycle), the
27-port `ToolHandler` dep container's session+credential surface, and the
CQRS command/query bus registrar.

## Packages

- **`kc`** (root) — `Manager` struct + constructor + 16-phase init sequence
  + per-domain register methods (account / admin / alerts / exit / MF /
  native-alerts / OAuth-bridge / orders / setup / ticker). Plus the 5
  Tier-1 facade types and the 7 focused service objects (Credential,
  Session, ManagedSession, Portfolio, Order, Alert, Family).
- **`kc/ports`** — Hexagonal-architecture port interfaces. Leaf-stability
  invariant: this package must NOT import `kc` parent. Verified via a
  build-time test in `kc/ports/leaf_stability_test.go`.
- **`kc/ops`** — Operations runbook handlers (admin endpoints + user
  account self-service flows). Sub-packages `ops/admin`, `ops/shared`,
  `ops/user` hold per-role implementations.

## Origin

Extracted 2026-05-16 from `github.com/algo2go/kite-mcp-bootstrap/kc` as
Phase 1 of the bootstrap-decomposition arc (Shape A* in the strategy doc
at `kite-mcp-server/.research/bootstrap-decomp-strategy.md`, commit
`280ae67`). Zero behavior change at extraction; the canonical consumer
(`kite-mcp-bootstrap`) imports this module via `replace` during the
Phase A canary window, then via GOPROXY once Phase B canary deletion
lands.

## License

MIT — see [LICENSE](LICENSE).

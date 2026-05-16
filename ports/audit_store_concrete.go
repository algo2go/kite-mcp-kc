package ports

import (
	"github.com/algo2go/kite-mcp-audit"
)

// AuditStoreConcreteProvider exposes the concrete *audit.Store for the
// "forensics" methods that are NOT part of the canonical AuditStoreInterface
// contract — UserOrderStats() and StatsCacheHitRate() in particular.
//
// Distinct from AlertPort.AlertStore() etc., which return interface-typed
// surfaces. This port is the explicit, documented escape hatch for read-only
// operational inspection. mcp/admin tools (admin_baseline_tool.go,
// admin_cache_info_tool.go) reach for these forensics methods to surface
// per-user order statistics and cache hit-rate metrics in the admin
// dashboard — operations that are deliberately scoped out of the
// AuditStoreInterface to keep the consumer surface narrow.
//
// Phase 3 sub-git brief 3 (kite-mcp-tools-ops): the two residual
// `manager.AuditStoreConcrete()` call sites currently reach through
// *kc.Manager. After this port lands, ToolHandlerDeps in
// kite-mcp-tools-common can expose AuditStoreConcreteProvider() and the
// admin tools migrate to handler.AuditStoreConcreteProvider().AuditStoreConcrete()
// — keeping the concrete type behind a Provider port rather than a raw
// *kc.Manager dependency.
//
// Leaf-stability: this file imports the external kite-mcp-audit module
// (NOT the kc parent), preserving the 4-of-N ports-with-zero-kc-imports
// invariant. *kc.Manager satisfies this port via the AuditStoreConcrete()
// method defined in kc/store_registry.go:200.
type AuditStoreConcreteProvider interface {
	AuditStoreConcrete() *audit.Store
}

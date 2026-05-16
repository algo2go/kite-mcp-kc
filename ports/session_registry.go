package ports

import "github.com/algo2go/kite-mcp-kc"

// SessionRegistryProvider exposes the concrete *kc.SessionRegistry for
// admin-side session operations that need the full registry surface —
// ListActiveSessions() and TerminateByEmail() in particular. Distinct
// from SessionPort, which is the narrow per-tool user-facing contract.
//
// Phase 3 sub-git brief 3 (kite-mcp-tools-ops): the two residual
// `manager.SessionManager` field-access call sites at
// mcp/misc/session_admin_tools.go:93,211 currently reach the registry
// directly. After this port lands, ToolHandlerDeps in
// kite-mcp-tools-common can expose SessionRegistryProvider() and the
// admin tools migrate to handler.SessionRegistryProvider().SessionRegistry()
// — keeping the concrete *kc.SessionRegistry behind a Provider port
// rather than a raw *kc.Manager dependency.
//
// Leaf-stability deviation (documented exception, parallel to assertions.go):
// This file imports the kc parent because *kc.SessionRegistry, *kc.MCPSession,
// and the surrounding registry types are defined in the kc package and
// cannot be referenced without importing kc. The alternative — relocating
// SessionRegistry + MCPSession to a leaf module — is out of scope for
// Phase 3 readiness. This is the second BY-DESIGN kc-import in ports/
// (alongside assertions.go's compile-time satisfaction check). The
// invariant downgrades from "4 of 5 ports have ZERO kc-parent imports"
// to "4 of N ports have ZERO kc-parent imports" — order.go was the prior
// exception when it referenced *kc.OrderService (since cleaned via
// Anchor 6 PR 6.8). A future cleanup can promote SessionRegistry to a
// kite-mcp-session module and sever this import, mirroring the
// AlertStoreInterface relocation pattern (PR 5.2-5.3).
//
// *kc.Manager satisfies this port via the SessionManager exposed field
// (manager_struct.go:91, post-B4). A thin SessionRegistry() method-form
// accessor is added to satisfy the interface — see manager_accessors.go.
type SessionRegistryProvider interface {
	// SessionRegistry returns the concrete session registry for admin operations
	// (ListActiveSessions, TerminateByEmail).
	SessionRegistry() *kc.SessionRegistry
}

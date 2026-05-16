package kc

// IdentityService bundles the identity / authentication / session-lifecycle
// services that previously lived as 5 separate fields directly on Manager.
//
// Tier B Step 4 (2026-05-16) — identity bundle. Manager 43 → 38 fields.
//
// Membership decision (per .research/tier-b-steps-4-5-preflight-2026-05-16.md
// §2.2):
//
//   - Credential     — per-user OAuth credential resolution
//   - Session        — MCP session lifecycle (login → token → session)
//   - ManagedSession — thin facade over SessionManager (active count, terminate)
//   - Signer         — HMAC session signing (crypto bundle, cohesive w/ identity)
//   - Lifecycle      — thin Manager-shaped facade exposing get/create/clear/
//                      complete via delegation to Session
//
// NOT in scope (stay outside IdentityService):
//   - SessionManager (*SessionRegistry) — has 4 external direct-readers
//     (kite-mcp-bootstrap / kite-mcp-usecases); kept as a Manager field so
//     those call sites are not forced to migrate yet (Option A per preflight
//     §2.4). Identity.Session and Identity.ManagedSession both hold a
//     pointer to the same registry, set during initFocusedServices.
//   - tokenStore / credentialStore / registryStore / userStore — owned by
//     StoreRegistry per Phase 3a design. Identity reaches them via interface
//     ports threaded through Session/Credential configs.
//   - encryptionKey — cross-cutting concern (also used by users.Store TOTP);
//     stays on Manager per Step 3 footnote.
//
// Pattern: matches the Step 2 OrderService + Step 3 AlertService precedent —
// an empty service is allocated up front in newEmptyManager, then the init*
// phases populate the fields in-place via same-package field access. After
// initFocusedServices the bundle is fully populated; consumers reach the
// individual services via m.Identity.<Field> (e.g. m.Identity.Session).
type IdentityService struct {
	Credential     *CredentialService
	Session        *SessionService
	ManagedSession *ManagedSessionService
	Signer         *SessionSigner
	Lifecycle      *SessionLifecycleService
}

// newEmptyIdentityService allocates an IdentityService with all fields zero.
// Used by newEmptyManager so the init* phases (initSideStores,
// initializeSessionSigner, initFocusedServices) can populate fields in-place
// via same-package field access. Mirror of Step 2/3 (Order/AlertService).
func newEmptyIdentityService() *IdentityService {
	return &IdentityService{}
}

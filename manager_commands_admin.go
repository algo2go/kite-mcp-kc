package kc

import (
)

// manager_commands_admin.go now holds only the top-level orchestrator
// (registerAdminCommands) that calls each per-domain sub-register. The
// 6 domain implementations were extracted to per-concern files
// (Sprint 3 Option-a):
//
//   - manager_commands_admin_user.go          (suspend/activate/change-role)
//   - manager_commands_admin_risk.go          (freeze/unfreeze + kill switches)
//   - manager_commands_admin_alerts.go        (create/delete/composite alerts)
//   - manager_commands_admin_mf.go            (MF order/SIP commands)
//   - manager_commands_admin_ticker.go        (ticker Start/Stop/Sub/Unsub)
//   - manager_commands_admin_native_alerts.go (native alert lifecycle)
//
// Each sub-file is self-contained (its own Deps struct, on-bus registrar,
// and (m *Manager) wrapper method). The wrapper-methods continue to live
// on *Manager so this orchestrator just calls them in the original order.
//
// 0 behavior change. 0 method-count change.

// registerAdminCommands wires CommandBus handlers for the Admin (user +
// risk), Alerts, Mutual Funds, Ticker, and Native Alerts domains
// (CommandBus batch C — STEP 10). Each handler constructs its use case
// lazily from the Manager's concrete stores/services, mirroring the Family
// and Account patterns. Use cases are not deleted — handlers call them,
// keeping the single source of business logic.
func (m *Manager) registerAdminCommands() error {
	if err := m.registerAdminUserCommands(); err != nil {
		return err
	}
	if err := m.registerAdminRiskCommands(); err != nil {
		return err
	}
	if err := m.registerAlertCommands(); err != nil {
		return err
	}
	if err := m.registerMFCommands(); err != nil {
		return err
	}
	if err := m.registerTickerCommands(); err != nil {
		return err
	}
	if err := m.registerNativeAlertCommands(); err != nil {
		return err
	}
	return nil
}


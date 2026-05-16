package kc

import (
	"context"
	"fmt"
	"time"

	"github.com/algo2go/kite-mcp-alerts"
	"github.com/algo2go/kite-mcp-audit"
	"github.com/algo2go/kite-mcp-domain"
)

// manager_init_alerts.go holds the alert-related phase methods that compose
// Manager.New: initAlertSystem (store + trigger callbacks for Telegram +
// audit + domain event), initAlertEvaluator (tick->alert matcher), and
// initTrailingStop (trailing-stop manager + modification callbacks).
//
// Phase ordering is load-bearing — see kc/manager.go NewWithOptions for the
// canonical 16-phase sequence. Split from kc/manager_init.go for cohesion;
// 0 behavior change.

// initAlertSystem wires the alert store and its trigger callback
// (Telegram + audit enqueue + domain-event dispatch). Must run before
// initAlertEvaluator — the evaluator takes alertStore as a dependency.
func (m *Manager) initAlertSystem(cfg Config) {
	// Initialize alert system: store → notifier → evaluator → ticker
	m.alertStore = alerts.NewStore(func(alert *alerts.Alert, currentPrice float64) {
		if m.telegramNotifier != nil {
			m.telegramNotifier.Notify(alert, currentPrice)
		}
		// Log alert trigger to audit trail for SSE browser notifications.
		// Alert-trigger callback runs from the alerts evaluator goroutine
		// with no request ctx in scope; service-ctx fallback is correct.
		if m.auditStore != nil {
			now := time.Now()
			m.auditStore.EnqueueCtx(context.Background(), &audit.ToolCall{
				CallID:        fmt.Sprintf("alert-%s-%d", alert.ID, now.UnixNano()),
				Email:         alert.Email,
				ToolName:      "alert_triggered",
				ToolCategory:  "notification",
				InputSummary:  fmt.Sprintf("%s:%s %s %.2f", alert.Exchange, alert.Tradingsymbol, alert.Direction, alert.TargetPrice),
				OutputSummary: fmt.Sprintf("Triggered at %.2f, notified via Telegram", currentPrice),
				StartedAt:     now,
				CompletedAt:   now,
			})
		}
		// Dispatch domain event for alert trigger.
		if m.eventDispatcher != nil {
			m.eventDispatcher.Dispatch(domain.AlertTriggeredEvent{
				Email:        alert.Email,
				AlertID:      alert.ID,
				Instrument:   domain.NewInstrumentKey(alert.Exchange, alert.Tradingsymbol),
				TargetPrice:  domain.NewINR(alert.TargetPrice),
				CurrentPrice: domain.NewINR(currentPrice),
				Direction:    string(alert.Direction),
				Timestamp:    time.Now().UTC(),
			})
		}
	})
	m.alertStore.SetLogger(cfg.Logger)
}

// initAlertEvaluator builds the tick→alert matcher. Depends on
// alertStore existing (initAlertSystem runs earlier).
func (m *Manager) initAlertEvaluator(cfg Config) {
	m.alertEvaluator = alerts.NewEvaluator(m.alertStore, cfg.Logger)
}

// initTrailingStop creates the trailing stop manager and wires the
// "modification" audit + Telegram notification callback. The Kite
// client modifier hook is wired separately from initCredentialService
// because it needs CredentialSvc, which isn't constructed yet at this
// point in the phase order.
func (m *Manager) initTrailingStop(cfg Config) {
	m.trailingStopMgr = alerts.NewTrailingStopManager(cfg.Logger)
	if m.alertDB != nil {
		m.trailingStopMgr.SetDB(m.alertDB)
		if err := m.trailingStopMgr.LoadFromDB(); err != nil {
			cfg.Logger.Error("Failed to load trailing stops from DB", "error", err)
		}
	}

	// Wire trailing stop modification notification to Telegram + audit.
	m.trailingStopMgr.SetOnModify(func(ts *alerts.TrailingStop, oldStop, newStop float64) {
		// Log trailing stop modification to audit trail for SSE browser notifications.
		// Trailing-stop callback runs from the trailing-stop manager
		// goroutine with no request ctx in scope; service-ctx fallback.
		if m.auditStore != nil {
			now := time.Now()
			trailDesc := fmt.Sprintf("%.2f", ts.TrailAmount)
			if ts.TrailPct > 0 {
				trailDesc = fmt.Sprintf("%.1f%%", ts.TrailPct)
			}
			m.auditStore.EnqueueCtx(context.Background(), &audit.ToolCall{
				CallID:        fmt.Sprintf("trail-%s-%d", ts.ID, now.UnixNano()),
				Email:         ts.Email,
				ToolName:      "trailing_stop_modified",
				ToolCategory:  "notification",
				InputSummary:  fmt.Sprintf("%s:%s SL moved %.2f -> %.2f", ts.Exchange, ts.Tradingsymbol, oldStop, newStop),
				OutputSummary: fmt.Sprintf("High: %.2f, Trail: %s", ts.HighWaterMark, trailDesc),
				StartedAt:     now,
				CompletedAt:   now,
			})
		}

		if m.telegramNotifier == nil {
			return
		}
		chatID, ok := m.alertStore.GetTelegramChatID(ts.Email)
		if !ok {
			return
		}
		arrow := "\u2B06\uFE0F" // up arrow
		if newStop < oldStop {
			arrow = "\u2B07\uFE0F" // down arrow
		}
		msg := fmt.Sprintf(
			"%s <b>Trailing Stop Modified</b>\n\n"+
				"%s:%s (%s)\n"+
				"SL: \u20B9%.2f \u2192 \u20B9%.2f\n"+
				"High water mark: \u20B9%.2f\n"+
				"Modifications: %d",
			arrow,
			ts.Exchange, ts.Tradingsymbol, ts.Direction,
			oldStop, newStop,
			ts.HighWaterMark,
			ts.ModifyCount,
		)
		if err := m.telegramNotifier.SendHTMLMessage(chatID, msg); err != nil {
			m.Logger.Warn("Failed to send trailing stop Telegram notification",
				"email", ts.Email, "error", err)
		}
	})
}


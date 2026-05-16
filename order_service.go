package kc

import (
	"fmt"
	"log/slog"

	"github.com/algo2go/kite-mcp-broker"
	"github.com/algo2go/kite-mcp-domain"
	"github.com/algo2go/kite-mcp-riskguard"
	"github.com/algo2go/kite-mcp-usecases"
)

// OrderService owns order placement, modification, and cancellation.
// It resolves a broker.Client per user and delegates to the broker interface.
// Extracted from Manager as part of Clean Architecture / SOLID refactoring.
//
// Tier B Step 2 (2026-05-16): absorbed the 13 Wave D Phase 1 use-case
// fields that previously lived on Manager (placeOrderUC, modifyOrderUC,
// cancelOrderUC, placeGTTUC, modifyGTTUC, deleteGTTUC, closePositionUC,
// closeAllPositionsUC, getOrderMarginsUC, getBasketMarginsUC,
// getOrderChargesUC, getPortfolioForWidgetUC, getAlertsForWidgetUC).
// Manager goes 63 → 50 fields by this move. The constructor signature
// stays narrow (sessionSvc + logger); the additional 13 UCs are wired
// in a separate InitUseCases() call from Manager.initOrderUseCases
// after riskGuard / eventDispatcher / eventStore / instruments are
// available. This two-phase init matches the existing Wave D pattern.
type OrderService struct {
	sessionSvc *SessionService
	logger     *slog.Logger

	// Wave D Phase 1 Slice D2: order-write use cases.
	PlaceOrderUC  *usecases.PlaceOrderUseCase
	ModifyOrderUC *usecases.ModifyOrderUseCase
	CancelOrderUC *usecases.CancelOrderUseCase

	// Slice D3: GTT (Good Till Triggered) write use cases.
	PlaceGTTUC  *usecases.PlaceGTTUseCase
	ModifyGTTUC *usecases.ModifyGTTUseCase
	DeleteGTTUC *usecases.DeleteGTTUseCase

	// Slice D4: position-exit write use cases.
	ClosePositionUC     *usecases.ClosePositionUseCase
	CloseAllPositionsUC *usecases.CloseAllPositionsUseCase

	// Slice D5: margin-query read use cases.
	GetOrderMarginsUC  *usecases.GetOrderMarginsUseCase
	GetBasketMarginsUC *usecases.GetBasketMarginsUseCase
	GetOrderChargesUC  *usecases.GetOrderChargesUseCase

	// Slice D6: widget-query read use cases (the 2 hoistable ones).
	// GetOrdersForWidgetUC + GetActivityForWidgetUC stay per-dispatch
	// per their ctx-bound audit-store override contract.
	GetPortfolioForWidgetUC *usecases.GetPortfolioForWidgetUseCase
	GetAlertsForWidgetUC    *usecases.GetAlertsForWidgetUseCase
}

// NewOrderService creates a new OrderService.
func NewOrderService(sessionSvc *SessionService, logger *slog.Logger) *OrderService {
	return &OrderService{
		sessionSvc: sessionSvc,
		logger:     logger,
	}
}

// InitUseCases wires the 13 absorbed Wave D use cases into OrderService.
// Called from Manager.initOrderUseCases after riskGuard / eventDispatcher /
// eventStore / instruments are available (post initFocusedServices and
// pre registerCQRSHandlers). All parameters are nil-tolerant per the
// pre-existing Wave D contract — riskGuard may be nil (DEV_MODE or no
// SQLite), dispatcher is nil at this point (production wires after
// kc.NewWithOptions; tests usually don't wire one), eventStore may be
// nil, instruments may be nil in test fixtures.
//
// The construction sequence mirrors the prior manager_use_cases.go
// initOrderUseCases body verbatim. No behavior change.
func (os *OrderService) InitUseCases(
	riskGuard *riskguard.Guard,
	dispatcher *domain.EventDispatcher,
	eventStore usecases.EventAppender,
	lotSizeLookup usecases.LotSizeLookup,
	alertStore usecases.WidgetAlertStore,
) {
	// PlaceOrder — full pipeline.
	placeUC := usecases.NewPlaceOrderUseCase(os.sessionSvc, riskGuard, dispatcher, os.logger)
	if eventStore != nil {
		placeUC.SetEventStore(eventStore)
	}
	if lotSizeLookup != nil {
		placeUC.SetLotSizeLookup(lotSizeLookup)
	}
	os.PlaceOrderUC = placeUC

	// ModifyOrder — same pipeline minus the instruments lookup.
	modifyUC := usecases.NewModifyOrderUseCase(os.sessionSvc, riskGuard, dispatcher, os.logger)
	if eventStore != nil {
		modifyUC.SetEventStore(eventStore)
	}
	os.ModifyOrderUC = modifyUC

	// CancelOrder — no riskguard (cancel always allowed).
	cancelUC := usecases.NewCancelOrderUseCase(os.sessionSvc, dispatcher, os.logger)
	if eventStore != nil {
		cancelUC.SetEventStore(eventStore)
	}
	os.CancelOrderUC = cancelUC

	// PlaceGTT / ModifyGTT / DeleteGTT — Slice D3.
	placeGTT := usecases.NewPlaceGTTUseCase(os.sessionSvc, os.logger)
	if eventStore != nil {
		placeGTT.SetEventStore(eventStore)
	}
	if dispatcher != nil {
		placeGTT.SetEventDispatcher(dispatcher)
	}
	os.PlaceGTTUC = placeGTT

	modifyGTT := usecases.NewModifyGTTUseCase(os.sessionSvc, os.logger)
	if eventStore != nil {
		modifyGTT.SetEventStore(eventStore)
	}
	if dispatcher != nil {
		modifyGTT.SetEventDispatcher(dispatcher)
	}
	os.ModifyGTTUC = modifyGTT

	deleteGTT := usecases.NewDeleteGTTUseCase(os.sessionSvc, os.logger)
	if eventStore != nil {
		deleteGTT.SetEventStore(eventStore)
	}
	if dispatcher != nil {
		deleteGTT.SetEventDispatcher(dispatcher)
	}
	os.DeleteGTTUC = deleteGTT

	// ClosePosition / CloseAllPositions — Slice D4.
	os.ClosePositionUC = usecases.NewClosePositionUseCase(os.sessionSvc, riskGuard, dispatcher, os.logger)
	os.CloseAllPositionsUC = usecases.NewCloseAllPositionsUseCase(os.sessionSvc, riskGuard, dispatcher, os.logger)

	// Margin queries — Slice D5 (read-only; no event dispatch).
	os.GetOrderMarginsUC = usecases.NewGetOrderMarginsUseCase(os.sessionSvc, os.logger)
	os.GetBasketMarginsUC = usecases.NewGetBasketMarginsUseCase(os.sessionSvc, os.logger)
	os.GetOrderChargesUC = usecases.NewGetOrderChargesUseCase(os.sessionSvc, os.logger)

	// Widget queries — Slice D6 (the 2 hoistable).
	os.GetPortfolioForWidgetUC = usecases.NewGetPortfolioForWidgetUseCase(os.sessionSvc, os.logger)
	if alertStore != nil {
		os.GetAlertsForWidgetUC = usecases.NewGetAlertsForWidgetUseCase(os.sessionSvc, alertStore, os.logger)
	}
}

// PropagateDispatcher fans the dispatcher into the 8 dispatch-aware
// use cases. Called by EventingService.SetDispatcher when production
// (app/wire.go) wires the real dispatcher AFTER kc.NewWithOptions has
// returned. Each call is nil-safe per the use case's SetEventDispatcher
// contract; the inner UC field-nil check guards against the pre-
// InitUseCases path.
func (os *OrderService) PropagateDispatcher(d *domain.EventDispatcher) {
	if uc := os.PlaceOrderUC; uc != nil {
		uc.SetEventDispatcher(d)
	}
	if uc := os.ModifyOrderUC; uc != nil {
		uc.SetEventDispatcher(d)
	}
	if uc := os.CancelOrderUC; uc != nil {
		uc.SetEventDispatcher(d)
	}
	if uc := os.PlaceGTTUC; uc != nil {
		uc.SetEventDispatcher(d)
	}
	if uc := os.ModifyGTTUC; uc != nil {
		uc.SetEventDispatcher(d)
	}
	if uc := os.DeleteGTTUC; uc != nil {
		uc.SetEventDispatcher(d)
	}
	if uc := os.ClosePositionUC; uc != nil {
		uc.SetEventDispatcher(d)
	}
	if uc := os.CloseAllPositionsUC; uc != nil {
		uc.SetEventDispatcher(d)
	}
}

// getBroker resolves a broker.Client for the given email.
func (os *OrderService) getBroker(email string) (broker.Client, error) {
	b, err := os.sessionSvc.GetBrokerForEmail(email)
	if err != nil {
		return nil, fmt.Errorf("order: %w", err)
	}
	return b, nil
}

// PlaceOrder places a new order for the user.
func (os *OrderService) PlaceOrder(email string, params broker.OrderParams) (broker.OrderResponse, error) {
	b, err := os.getBroker(email)
	if err != nil {
		return broker.OrderResponse{}, err
	}
	resp, err := b.PlaceOrder(params)
	if err != nil {
		os.logger.Error("Failed to place order", "email", email, "error", err)
		return broker.OrderResponse{}, fmt.Errorf("failed to place order: %w", err)
	}
	return resp, nil
}

// ModifyOrder modifies an existing pending order.
func (os *OrderService) ModifyOrder(email, orderID string, params broker.OrderParams) (broker.OrderResponse, error) {
	b, err := os.getBroker(email)
	if err != nil {
		return broker.OrderResponse{}, err
	}
	resp, err := b.ModifyOrder(orderID, params)
	if err != nil {
		os.logger.Error("Failed to modify order", "email", email, "order_id", orderID, "error", err)
		return broker.OrderResponse{}, fmt.Errorf("failed to modify order: %w", err)
	}
	return resp, nil
}

// CancelOrder cancels an existing pending order.
func (os *OrderService) CancelOrder(email, orderID, variety string) (broker.OrderResponse, error) {
	b, err := os.getBroker(email)
	if err != nil {
		return broker.OrderResponse{}, err
	}
	resp, err := b.CancelOrder(orderID, variety)
	if err != nil {
		os.logger.Error("Failed to cancel order", "email", email, "order_id", orderID, "error", err)
		return broker.OrderResponse{}, fmt.Errorf("failed to cancel order: %w", err)
	}
	return resp, nil
}

// GetOrders returns all orders for the user's current trading day.
func (os *OrderService) GetOrders(email string) ([]broker.Order, error) {
	b, err := os.getBroker(email)
	if err != nil {
		return nil, err
	}
	orders, err := b.GetOrders()
	if err != nil {
		os.logger.Error("Failed to get orders", "email", email, "error", err)
		return nil, fmt.Errorf("failed to get orders: %w", err)
	}
	return orders, nil
}

// GetTrades returns all executed trades for the user's current trading day.
func (os *OrderService) GetTrades(email string) ([]broker.Trade, error) {
	b, err := os.getBroker(email)
	if err != nil {
		return nil, err
	}
	trades, err := b.GetTrades()
	if err != nil {
		os.logger.Error("Failed to get trades", "email", email, "error", err)
		return nil, fmt.Errorf("failed to get trades: %w", err)
	}
	return trades, nil
}

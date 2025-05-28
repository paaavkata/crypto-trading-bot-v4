package trader

import (
	"context"
	"fmt"
	"time"

	"github.com/paaavkata/crypto-trading-bot-v4/trading-engine/internal/database"
	"github.com/paaavkata/crypto-trading-bot-v4/trading-engine/internal/exchange"
	"github.com/paaavkata/crypto-trading-bot-v4/trading-engine/internal/signals"
	"github.com/paaavkata/crypto-trading-bot-v4/trading-engine/pkg/models"
	"github.com/sirupsen/logrus"
)

type Engine struct {
	repo            *database.Repository
	exchange        *exchange.KuCoinExchange
	signalGenerator *signals.Generator
	gridStrategy    *GridStrategy
	riskManager     *RiskManager
	logger          *logrus.Logger
	config          EngineConfig
}

type EngineConfig struct {
	MaxPositionsPerPair int
	DefaultPositionSize float64
	StopLossPercent     float64
	TakeProfitPercent   float64
}

func NewEngine(repo *database.Repository, exchange *exchange.KuCoinExchange,
	config EngineConfig, logger *logrus.Logger) *Engine { // Removed signalGen from parameters

	// Initialize the new signal generator, passing the repository
	signalGenerator := signals.NewGenerator(logger, repo)

	return &Engine{
		repo:            repo,
		exchange:        exchange,
		signalGenerator: signalGenerator, // Use the newly initialized generator
		gridStrategy:    NewGridStrategy(logger),
		riskManager:     NewRiskManager(config, logger),
		logger:          logger,
		config:          config,
	}
}

func (e *Engine) Run(ctx context.Context) error {
	e.logger.Info("Starting trading engine")

	ticker := time.NewTicker(30 * time.Second) // Run every 30 seconds
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			e.logger.Info("Trading engine stopped")
			return nil
		case <-ticker.C:
			if err := e.processTradingCycle(ctx); err != nil {
				e.logger.WithError(err).Error("Error in trading cycle")
			}
		}
	}
}

func (e *Engine) processTradingCycle(ctx context.Context) error {
	// Get active selected pairs
	pairs, err := e.repo.GetActiveSelectedPairs(ctx)
	if err != nil {
		return fmt.Errorf("failed to get active pairs: %w", err)
	}

	e.logger.WithField("active_pairs", len(pairs)).Debug("Processing trading cycle")

	for _, pair := range pairs {
		if err := e.processPair(ctx, pair); err != nil {
			e.logger.WithError(err).WithField("symbol", pair.Symbol).Error("Failed to process pair")
			continue
		}
	}

	return nil
}

func (e *Engine) synchronizeOrderStatuses(ctx context.Context) error {
	e.logger.Debug("Starting order status synchronization cycle")
	// Fetch orders that are not in a final state (e.g., pending, open, partially_filled)
	// Assuming Repository has a method like GetNonFinalOrders
	// For now, let's assume it fetches orders with status "pending" or "open" or "partially_filled"
	// This list of statuses should be comprehensive.
	nonFinalOrders, err := e.repo.GetOrdersByStatuses(ctx, []string{"pending", "open", "partially_filled"})
	if err != nil {
		e.logger.WithError(err).Error("Failed to fetch non-final orders for status sync")
		return fmt.Errorf("failed to fetch non-final orders: %w", err)
	}

	if len(nonFinalOrders) == 0 {
		e.logger.Debug("No non-final orders to synchronize")
		return nil
	}

	e.logger.WithField("orders_to_sync", len(nonFinalOrders)).Info("Synchronizing order statuses")

	for _, localOrder := range nonFinalOrders {
		if localOrder.KuCoinOrderID == "" {
			e.logger.WithField("local_order_id", localOrder.ID).Warn("Local order has no KuCoinOrderID, skipping sync")
			continue
		}

		exchangeOrderDetail, err := e.exchange.GetOrderStatus(ctx, localOrder.KuCoinOrderID)
		if err != nil {
			e.logger.WithError(err).WithFields(logrus.Fields{
				"local_order_id":  localOrder.ID,
				"kucoin_order_id": localOrder.KuCoinOrderID,
			}).Error("Failed to get order status from exchange")
			// Handle specific errors, e.g., order not found after a certain period might mean it's permanently failed or canceled by exchange
			// For now, we just log and continue to the next order.
			continue
		}

		// Begin transaction for updating order and potentially position
		tx, err := e.repo.BeginTx(ctx)
		if err != nil {
			e.logger.WithError(err).Error("Failed to begin database transaction for order sync")
			continue // Skip this order, try next cycle
		}
		// Defer rollback, commit on success
		defer func() {
			if r := recover(); r != nil {
				e.logger.WithField("panic", r).Error("Panic recovered during order sync transaction, rolling back")
				tx.Rollback() // Rollback on panic
				// Re-panic or handle as appropriate
			} else if err != nil {
				e.logger.WithError(err).Error("Error in order sync transaction, rolling back")
				tx.Rollback() // Rollback on error
			} else {
				e.logger.Debug("Committing transaction for order sync")
				if commitErr := tx.Commit(); commitErr != nil {
					e.logger.WithError(commitErr).Error("Failed to commit transaction for order sync")
					// Data might be inconsistent if commit fails
				}
			}
		}()


		// Compare and update
		needsUpdate := false
		if localOrder.Status != exchangeOrderDetail.Status {
			e.logger.WithFields(logrus.Fields{
				"order_id":        localOrder.KuCoinOrderID,
				"local_status":    localOrder.Status,
				"exchange_status": exchangeOrderDetail.Status,
			}).Info("Order status mismatch, updating local order")
			localOrder.Status = exchangeOrderDetail.Status
			needsUpdate = true
		}

		if localOrder.FilledQuantity != exchangeOrderDetail.DealSize {
			localOrder.FilledQuantity = exchangeOrderDetail.DealSize
			needsUpdate = true
		}
		
		// Update fee if available and different (assuming Fee is total fee for the order)
		if exchangeOrderDetail.Fee > 0 && localOrder.Fee != exchangeOrderDetail.Fee {
			localOrder.Fee = exchangeOrderDetail.Fee
			// Note: FeeCurrency handling might be needed if it can change or is per fill.
			needsUpdate = true
		}


		if exchangeOrderDetail.Status == "filled" || exchangeOrderDetail.Status == "canceled" || exchangeOrderDetail.Status == "rejected" {
			now := time.Now()
			localOrder.UpdatedAt = now
			if localOrder.FilledAt == nil && (exchangeOrderDetail.Status == "filled" || (exchangeOrderDetail.Status == "canceled" && localOrder.FilledQuantity > 0)) {
				localOrder.FilledAt = &now // Or use a timestamp from exchangeOrderDetail if available and more accurate
			}
		}


		if needsUpdate {
			err = e.repo.UpdateOrderInTx(ctx, tx, localOrder)
			if err != nil {
				e.logger.WithError(err).WithField("order_id", localOrder.KuCoinOrderID).Error("Failed to update local order record in transaction")
				// Error will trigger rollback via defer
				continue
			}
		}

		// Update related position if the order is filled or partially filled
		if (exchangeOrderDetail.Status == "filled" || exchangeOrderDetail.Status == "partially_filled" || 
		   (exchangeOrderDetail.Status == "canceled" && localOrder.FilledQuantity > 0) ) && localOrder.PositionID != nil {
			
			position, err := e.repo.GetPositionByIDInTx(ctx, tx, *localOrder.PositionID)
			if err != nil {
				e.logger.WithError(err).WithField("position_id", *localOrder.PositionID).Error("Failed to get position for filled/partially_filled order in transaction")
				// Error will trigger rollback
				continue
			}
			if position == nil {
				e.logger.WithField("position_id", *localOrder.PositionID).Error("Position not found for filled/partially_filled order")
				// Error will trigger rollback
				err = fmt.Errorf("position not found: %s", (*localOrder.PositionID).String())
				continue
			}

			positionNeedsUpdate := false
			// Assuming DealSize is the total filled quantity for the order from the exchange
			// This logic might need adjustment if DealSize represents individual fills
			// For simplicity, we assume DealSize is the total filled quantity for this order.
			
			// If the order is fully filled:
			if exchangeOrderDetail.Status == "filled" {
				if position.Status == "open_pending" { // Custom status to indicate position waiting for initial fill
					position.Status = "open"
					position.EntryPrice = exchangeOrderDetail.Price // Or average fill price if available from exchangeOrderDetail
					position.Quantity = exchangeOrderDetail.DealSize // Ensure position quantity matches filled size
					positionNeedsUpdate = true
				} else if position.Status == "open" && localOrder.Side != position.Side { // Closing part of a position or full close
					// This is a closing order.
					// PnL calculation for closing orders is typically handled when the closing order is *initiated* (e.g., by SL/TP or strategy signal).
					// Here, we confirm it's filled.
					// If the filled quantity matches position quantity, close the position.
					if exchangeOrderDetail.DealSize >= position.Quantity { // Using >= for safety with float precision
						position.Status = "closed"
						now := time.Now()
						position.ClosedAt = &now
						// Realized PnL should have been calculated at the point of initiating the close (e.g. SL/TP logic) using the expected fill price.
						// If not, it needs to be calculated here using fill price from exchangeOrderDetail.
						// For now, we assume PnL was set when the closing decision was made.
						// position.RealizedPnL = (exchangeOrderDetail.Price - position.EntryPrice) * position.Quantity // if buy side
					} else {
						// Partial close of position. Reduce quantity. PnL for this part should be realized.
						// This part is complex: requires tracking average entry price, realizing PnL for the closed portion.
						e.logger.WithFields(logrus.Fields{
							"order_id": localOrder.KuCoinOrderID, 
							"position_id": position.ID,
							"message": "Partial close of position detected - complex PnL logic required here.",
						}).Warn("Partial position close handling not fully implemented for PnL")
						position.Quantity -= exchangeOrderDetail.DealSize // Reduce position size
					}
					positionNeedsUpdate = true
				}
			} else if exchangeOrderDetail.Status == "partially_filled" {
				if position.Status == "open_pending" {
					position.Status = "open" // Or a specific "open_partially_filled"
					// Average entry price calculation needed if multiple partial fills
					position.EntryPrice = exchangeOrderDetail.Price // First partial fill sets entry price
					position.Quantity = exchangeOrderDetail.DealSize
					positionNeedsUpdate = true
				} else if position.Status == "open" {
					// Position already open, this is an additional partial fill (e.g. for a limit order that's part of a grid)
					// Or, a closing order is partially filled.
					// This logic needs to be very robust depending on strategy.
					// For now, just log. More detailed handling of average price / quantity updates needed.
					e.logger.WithFields(logrus.Fields{
						"order_id": localOrder.KuCoinOrderID, 
						"position_id": position.ID,
						"message": "Position already open, additional partial fill detected.",
					}).Info("Handling additional partial fill.")
					// Example: If it's an order adding to the position:
					// newTotalQuantity := position.Quantity + (exchangeOrderDetail.DealSize - localOrder.FilledQuantity) // assuming localOrder.FilledQuantity was its previous state
					// newAvgPrice := ((position.EntryPrice * position.Quantity) + (exchangeOrderDetail.Price * (exchangeOrderDetail.DealSize - localOrder.FilledQuantity))) / newTotalQuantity
					// position.EntryPrice = newAvgPrice
					// position.Quantity = newTotalQuantity
					// positionNeedsUpdate = true
				}
			} else if exchangeOrderDetail.Status == "canceled" && localOrder.FilledQuantity > 0 { // Canceled but partially filled
				if position.Status == "open_pending" && localOrder.FilledQuantity > 0 {
					position.Status = "open" // It did partially fill, so position is open
					position.EntryPrice = exchangeOrderDetail.Price // Based on what filled
					position.Quantity = localOrder.FilledQuantity // The amount that actually filled
					positionNeedsUpdate = true
					e.logger.WithField("order_id", localOrder.KuCoinOrderID).Info("Order canceled but was partially filled, position opened with filled amount.")
				} else if position.Status == "open" && localOrder.Side != position.Side { // A closing order was partially filled then canceled
					e.logger.WithFields(logrus.Fields{
						"order_id": localOrder.KuCoinOrderID, 
						"position_id": position.ID,
						"message": "Closing order partially filled then canceled. Position quantity may need adjustment.",
					}).Warn("Partial close then cancel handling not fully implemented for PnL")
                    // position.Quantity -= localOrder.FilledQuantity // Reduce by what was filled before cancel
					positionNeedsUpdate = true
				}
			}


			if positionNeedsUpdate {
				position.UpdatedAt = time.Now()
				err = e.repo.UpdatePositionInTx(ctx, tx, *position)
				if err != nil {
					e.logger.WithError(err).WithField("position_id", position.ID).Error("Failed to update position in transaction")
					// Error will trigger rollback
					continue
				}
			}
		}
		// If loop finishes without error, commit happens in defer
	}
	e.logger.Debug("Finished order status synchronization cycle")
	return nil
}


func (e *Engine) processPair(ctx context.Context, pair models.SelectedPair) error {
	// Get or create trading config
	config, err := e.repo.GetTradingConfig(ctx, pair.ID)
	if err != nil {
		return fmt.Errorf("failed to get trading config: %w", err)
	}

	if config == nil {
		// Create default config
		config = e.createDefaultConfig(pair)
		if err := e.repo.CreateTradingConfig(ctx, *config); err != nil {
			e.logger.WithError(err).WithField("symbol", pair.Symbol).Error("Failed to create trading config")
			return err
		}
	}

	// Get current price
	currentPrice, err := e.repo.GetLatestPrice(ctx, pair.Symbol)
	if err != nil {
		return fmt.Errorf("failed to get current price: %w", err)
	}

	// Generate trading signal
	signal := e.signalGenerator.GenerateSignal(ctx, pair.Symbol, currentPrice)

	// Get open positions
	positions, err := e.repo.GetOpenPositions(ctx, pair.ID)
	if err != nil {
		return fmt.Errorf("failed to get open positions: %w", err)
	}

	// Update position PnL and check for SL/TP
	var activePositions []models.Position // Positions not closed by SL/TP
	for _, p := range positions {
		position := p // Create a new variable to avoid issues with pointer in loop
		if err := e.updatePositionPnL(ctx, &position, currentPrice); err != nil {
			e.logger.WithError(err).WithField("position_id", position.ID).Error("Failed to update position PnL")
			// Decide if we should skip SL/TP check for this position or continue
			// For now, we'll continue and try to check SL/TP
		}

		// Check and execute Stop-Loss/Take-Profit
		closedBySLTP, err := e.checkAndExecuteSLTP(ctx, &position, *config, currentPrice, pair.Symbol)
		if err != nil {
			e.logger.WithError(err).WithField("position_id", position.ID).Error("Failed to check/execute SL/TP")
			// Even if SL/TP check fails, the position might still be open
		}

		if !closedBySLTP {
			// If not closed by SL/TP, add to active positions for strategy execution
			activePositions = append(activePositions, position)
		}
	}
	// Update positions list to reflect any closures by SL/TP for subsequent logic
	// positions = activePositions // This line is now part of the main SL/TP loop logic if positions are re-fetched or filtered after SL/TP

	// Risk management checks after SL/TP and before new signal processing
	// Re-fetch positions as SL/TP might have closed some, or use the filtered activePositions
	currentOpenPositions, err := e.repo.GetOpenPositions(ctx, pair.ID)
	if err != nil {
		return fmt.Errorf("failed to re-fetch open positions after SL/TP: %w", err)
	}


	if !e.riskManager.CanTrade(pair, currentOpenPositions, currentPrice) {
		e.logger.WithField("symbol", pair.Symbol).Debug("Risk management blocked trading after SL/TP check")
		return nil
	}

	// Execute trading strategy using the latest state of open positions
	switch config.StrategyType {
	case "grid":
		return e.gridStrategy.Execute(ctx, pair, *config, signal, currentOpenPositions, currentPrice)
	default:
		return e.executeBasicStrategy(ctx, pair, *config, signal, currentOpenPositions, currentPrice)
	}
}

// processTradingCycle is modified to include synchronizeOrderStatuses
func (e *Engine) processTradingCycle(ctx context.Context) error {
	// Synchronize order statuses first
	if err := e.synchronizeOrderStatuses(ctx); err != nil {
		// Log error but continue the cycle if possible, as pair processing might still be viable
		e.logger.WithError(err).Error("Error during order status synchronization")
	}

	// Get active selected pairs
	pairs, err := e.repo.GetActiveSelectedPairs(ctx)
	if err != nil {
		return fmt.Errorf("failed to get active pairs: %w", err)
	}

	e.logger.WithField("active_pairs", len(pairs)).Debug("Processing trading cycle for pairs")

	for _, pair := range pairs {
		if err := e.processPair(ctx, pair); err != nil {
			e.logger.WithError(err).WithField("symbol", pair.Symbol).Error("Failed to process pair")
			continue
		}
	}
	return nil
}


func (e *Engine) checkAndExecuteSLTP(ctx context.Context, position *models.Position, config models.TradingConfig, currentPrice float64, symbol string) (bool, error) {
	if position.Status != "open" {
		return false, nil // Not an open position
	}

	var slPrice, tpPrice float64
	var closePosition bool = false
	var reason string

	if config.StopLossPercent <= 0 || config.TakeProfitPercent <= 0 {
		// SL/TP not configured for this position
		return false, nil
	}

	if position.Side == "buy" {
		slPrice = position.EntryPrice * (1 - config.StopLossPercent) // config.StopLossPercent should be like 0.05 for 5%
		tpPrice = position.EntryPrice * (1 + config.TakeProfitPercent)

		if currentPrice <= slPrice {
			closePosition = true
			reason = "stop-loss"
		} else if currentPrice >= tpPrice {
			closePosition = true
			reason = "take-profit"
		}
	} else if position.Side == "sell" { // Assuming short selling capability, though basic strategy doesn't open shorts
		slPrice = position.EntryPrice * (1 + config.StopLossPercent)
		tpPrice = position.EntryPrice * (1 - config.TakeProfitPercent)

		if currentPrice >= slPrice {
			closePosition = true
			reason = "stop-loss"
		} else if currentPrice <= tpPrice {
			closePosition = true
			reason = "take-profit"
		}
	}

	if closePosition {
		e.logger.WithFields(logrus.Fields{
			"symbol":      symbol,
			"position_id": position.ID,
			"entry_price": position.EntryPrice,
			"current_price": currentPrice,
			"sl_price":    slPrice,
			"tp_price":    tpPrice,
			"reason":      reason,
		}).Info("Stop-Loss/Take-Profit condition met")

		err := e.executeMarketCloseOrder(ctx, symbol, position, currentPrice, reason)
		if err != nil {
			return false, fmt.Errorf("failed to execute market close order for %s: %w", reason, err)
		}
		return true, nil // Position closed
	}

	return false, nil // No SL/TP triggered
}

func (e *Engine) createDefaultConfig(pair models.SelectedPair) *models.TradingConfig {
	// Calculate price range based on volatility
	priceRangePercent := pair.Volatility24h * 2 // 2x volatility for grid range
	if priceRangePercent < 0.05 {
		priceRangePercent = 0.05 // Minimum 5%
	}
	if priceRangePercent > 0.15 {
		priceRangePercent = 0.15 // Maximum 15%
	}

	return &models.TradingConfig{
		PairID:            pair.ID,
		StrategyType:      "grid",
		GridLevels:        10,
		PriceRangeMin:     0, // Will be set dynamically
		PriceRangeMax:     0, // Will be set dynamically
		PositionSizeUSDT:  e.config.DefaultPositionSize,
		StopLossPercent:   e.config.StopLossPercent,
		TakeProfitPercent: e.config.TakeProfitPercent,
		MaxPositions:      e.config.MaxPositionsPerPair,
		IsActive:          true,
	}
}

func (e *Engine) updatePositionPnL(ctx context.Context, position *models.Position, currentPrice float64) error {
	position.CurrentPrice = currentPrice

	// Calculate unrealized PnL
	if position.Side == "buy" {
		position.UnrealizedPnL = (currentPrice - position.EntryPrice) * position.Quantity
	} else {
		position.UnrealizedPnL = (position.EntryPrice - currentPrice) * position.Quantity
	}

	return e.repo.UpdatePosition(ctx, *position)
}

func (e *Engine) executeBasicStrategy(ctx context.Context, pair models.SelectedPair, config models.TradingConfig,
	signal models.Signal, positions []models.Position, currentPrice float64) error {

	e.logger.WithFields(logrus.Fields{
		"symbol": pair.Symbol,
		"signal": signal.Action,
		"price":  currentPrice,
	}).Debug("Executing basic strategy")

	switch signal.Action {
	case "BUY":
		if len(positions) < config.MaxPositions {
			return e.executeBuyOrder(ctx, pair, config, currentPrice)
		}
	case "SELL":
		// Close profitable positions
		for _, position := range positions {
			if position.Side == "buy" && position.UnrealizedPnL > 0 {
				return e.executeSellOrder(ctx, pair, position, currentPrice)
			}
		}
	}

	return nil
}

func (e *Engine) executeBuyOrder(ctx context.Context, pair models.SelectedPair, config models.TradingConfig, price float64) error {
	quantity := config.PositionSizeUSDT / price

	orderResp, err := e.exchange.PlaceBuyOrder(pair.Symbol, quantity, price)
	if err != nil {
		return fmt.Errorf("failed to place buy order: %w", err)
	}

	// Create position record
	position := models.Position{
		PairID:       pair.ID,
		ConfigID:     config.ID,
		Side:         "buy",
		Quantity:     quantity,
		EntryPrice:   price,
		CurrentPrice: price,
		Status:       "open",
		OrderID:      orderResp.OrderId,
	}

	if err := e.repo.CreatePosition(ctx, position); err != nil {
		return fmt.Errorf("failed to create position record: %w", err)
	}

	// Create order record
	order := models.Order{
		PairID:        pair.ID,
		KuCoinOrderID: orderResp.OrderId,
		Side:          "buy",
		Type:          "limit",
		Quantity:      quantity,
		Price:         price,
		Status:        "pending",
	}

	return e.repo.CreateOrder(ctx, order)
}

func (e *Engine) executeSellOrder(ctx context.Context, pair models.SelectedPair, position models.Position, price float64) error {
	orderResp, err := e.exchange.PlaceSellOrder(pair.Symbol, position.Quantity, price)
	if err != nil {
		return fmt.Errorf("failed to place sell order: %w", err)
	}

	// Update position status
	now := time.Now()
	position.Status = "closed"
	position.ClosedAt = &now
	position.RealizedPnL = position.UnrealizedPnL

	if err := e.repo.UpdatePosition(ctx, position); err != nil {
		return fmt.Errorf("failed to update position: %w", err)
	}

	// Create order record
	order := models.Order{
		PositionID:    &position.ID,
		PairID:        pair.ID, // pair.ID corresponds to selected_pairs.id which is used as positions.pair_id
		KuCoinOrderID: orderResp.OrderId,
		Side:          "sell",
		Type:          "limit",
		Quantity:      position.Quantity,
		Price:         price,
		Status:        "pending",
	}

	return e.repo.CreateOrder(ctx, order)
}

func (e *Engine) executeMarketCloseOrder(ctx context.Context, symbol string, position *models.Position, exitPrice float64, reason string) error {
	closeSide := "sell"
	if position.Side == "sell" { // If it's a short position
		closeSide = "buy"
	}

	orderResp, err := e.exchange.PlaceMarketOrder(symbol, closeSide, position.Quantity)
	if err != nil {
		e.logger.WithError(err).WithFields(logrus.Fields{
			"symbol":      symbol,
			"position_id": position.ID,
			"side":        closeSide,
			"quantity":    position.Quantity,
		}).Error("Failed to place market close order for SL/TP")
		return fmt.Errorf("failed to place market %s order for %s: %w", closeSide, reason, err)
	}

	e.logger.WithFields(logrus.Fields{
		"symbol":        symbol,
		"position_id":   position.ID,
		"kucoin_order_id": orderResp.OrderId,
		"reason":        reason,
		"exit_price":    exitPrice, // Approximated with currentPrice at trigger
	}).Info("Market close order placed for SL/TP")

	// Update position status
	now := time.Now()
	position.Status = "closed"
	position.ClosedAt = &now
	position.CurrentPrice = exitPrice // Update current price to exit price for PnL calculation

	// Recalculate PnL with exit price
	if position.Side == "buy" {
		position.RealizedPnL = (exitPrice - position.EntryPrice) * position.Quantity
	} else { // sell position
		position.RealizedPnL = (position.EntryPrice - exitPrice) * position.Quantity
	}
	position.UnrealizedPnL = 0 // No longer unrealized

	if err := e.repo.UpdatePosition(ctx, *position); err != nil {
		// Log critical error, as position is closed on exchange but not in DB
		e.logger.WithError(err).WithField("position_id", position.ID).Error("CRITICAL: Failed to update position status after market close for SL/TP")
		return fmt.Errorf("failed to update position after %s: %w", reason, err)
	}

	// Create order record for the closing trade
	order := models.Order{
		PositionID:    &position.ID,
		PairID:        position.PairID, // Use PairID from the position struct
		KuCoinOrderID: orderResp.OrderId,
		Side:          closeSide,
		Type:          "market", // SL/TP orders are market orders
		Quantity:      position.Quantity,
		Price:         exitPrice, // Record the approximate exit price
		FilledQuantity: position.Quantity, // Assume market order fills completely for now
		Status:        "filled",          // Assume market order fills immediately for now
		Fee:           0,                 // Fee data not available yet
		CreatedAt:     now,
		UpdatedAt:     now,
		FilledAt:      &now,
	}

	if err := e.repo.CreateOrder(ctx, order); err != nil {
		e.logger.WithError(err).WithField("position_id", position.ID).Error("Failed to create closing order record for SL/TP")
		// Non-critical for position state, but order history is incomplete
		return fmt.Errorf("failed to create order record for %s: %w", reason, err)
	}
	return nil
}

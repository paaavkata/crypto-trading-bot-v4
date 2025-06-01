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

	// TotalAccountBalance is used for portfolio drawdown calculations.
	// Env: TOTAL_ACCOUNT_BALANCE, Default: 10000.0 (example, should be set realistically)
	TotalAccountBalance float64

	// CBPortfolioDrawdownPercent: Max portfolio drawdown percentage to trigger trading halt.
	// e.g., 10.0 for 10%. If negative unrealized PnL exceeds this, halt.
	// Env: CB_PORTFOLIO_DRAWDOWN_PERCENT, Default: 10.0
	CBPortfolioDrawdownPercent float64

	// CBPortfolioDrawdownCheckIntervalHours: How far back to look for realized PnL for drawdown.
	// This is NOT directly used in the simplified PnL calculation chosen (totalUnrealizedPnL / totalAccountBalance),
	// but kept for potential future enhancement. For now, the drawdown is point-in-time based on unrealized PnL.
	// Env: CB_PORTFOLIO_DRAWDOWN_INTERVAL_HOURS, Default: 24
	CBPortfolioDrawdownCheckIntervalHours int // In hours

	// CBFlashCrashDropPercent: Price drop percentage to trigger flash crash halt for a pair.
	// e.g., 15.0 for 15% drop.
	// Env: CB_FLASH_CRASH_DROP_PERCENT, Default: 15.0
	CBFlashCrashDropPercent float64

	// CBFlashCrashWindowMinutes: Time window (in minutes) for flash crash detection.
	// e.g., 5 for last 5 minutes.
	// Env: CB_FLASH_CRASH_WINDOW_MINUTES, Default: 5
	CBFlashCrashWindowMinutes int // In minutes

	// CBTradingHaltDurationMinutes: How long to halt trading (globally or for a pair) after a CB triggers.
	// e.g., 60 for 60 minutes.
	// Env: CB_TRADING_HALT_DURATION_MINUTES, Default: 60
	CBTradingHaltDurationMinutes int // In minutes

	// --- Default Strategy Parameters ---
	// DefaultStrategyGridLevels: Number of grid levels for the default grid strategy.
	// Env: DEFAULT_STRATEGY_GRID_LEVELS, Default: 10
	DefaultStrategyGridLevels int

	// DefaultStrategyVolatilityMultiplier: Multiplier for pair.Volatility24h to determine price range.
	// Env: DEFAULT_STRATEGY_VOLATILITY_MULTIPLIER, Default: 2.0
	DefaultStrategyVolatilityMultiplier float64

	// DefaultStrategyMinPriceRangePercent: Minimum price range percentage for the grid.
	// Env: DEFAULT_STRATEGY_MIN_PRICE_RANGE_PERCENT, Default: 0.05 (5%)
	DefaultStrategyMinPriceRangePercent float64

	// DefaultStrategyMaxPriceRangePercent: Maximum price range percentage for the grid.
	// Env: DEFAULT_STRATEGY_MAX_PRICE_RANGE_PERCENT, Default: 0.15 (15%)
	DefaultStrategyMaxPriceRangePercent float64
}

func NewEngine(repo *database.Repository, exchange *exchange.KuCoinExchange,
	signalGen *signals.Generator, config EngineConfig, logger *logrus.Logger) *Engine {

	return &Engine{
		repo:            repo,
		exchange:        exchange,
		signalGenerator: signalGen,
		gridStrategy:    NewGridStrategy(logger),
		riskManager:     NewRiskManager(repo, config, logger), // Pass repo to NewRiskManager
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
	// --- Portfolio Drawdown Circuit Breaker Check ---
	// 1. Fetch all open positions globally
	allOpenPositions, err := e.repo.GetAllOpenPositions(ctx)
	if err != nil {
		e.logger.WithError(err).Error("Failed to get all open positions for portfolio drawdown check")
		return fmt.Errorf("failed to get all open positions: %w", err)
	}

	// 2. Update PnL for these positions (needed for accurate unrealized PnL)
	// This requires knowing the symbol for each position to fetch its current price.
	// We can create a map of PairID to Symbol from the selected pairs list first.
	// Or, more simply, RiskManager's CheckPortfolioDrawdown will use position.UnrealizedPnL,
	// which should be updated within processPair before strategies are run.
	// For a global check, we need to ensure PnL is fresh.
	// Let's assume updatePositionPnL has been called recently enough, or call it here.
	// The current design updates PnL per-pair in processPair.
	// For a global CB, it's better to update all PnLs first.
	tempSymbolPriceMap := make(map[string]float64) // Store latest prices per symbol

	// Create a map of PairID to Symbol for efficient lookup
	pairIDToSymbol := make(map[int64]string)
	activePairs, err := e.repo.GetActiveSelectedPairs(ctx) // Fetch active pairs for symbols
	if err != nil {
		e.logger.WithError(err).Error("Failed to get active selected pairs for PnL update")
		return fmt.Errorf("failed to get active selected pairs for PnL update: %w", err)
	}
	for _, p := range activePairs {
		pairIDToSymbol[p.ID] = p.Symbol
	}

	for i := range allOpenPositions {
		pos := &allOpenPositions[i]
		symbol, ok := pairIDToSymbol[pos.PairID]
		if !ok {
			e.logger.WithField("pair_id", pos.PairID).Warn("Could not find symbol for pair ID to update PnL for drawdown check")
			continue
		}

		currentPrice, found := tempSymbolPriceMap[symbol]
		if !found {
			price, err := e.repo.GetLatestPrice(ctx, symbol)
			if err != nil {
				e.logger.WithError(err).WithFields(logrus.Fields{"symbol": symbol}).Warn("Failed to get latest price for PnL update in drawdown check")
				continue
			}
			currentPrice = price
			tempSymbolPriceMap[symbol] = currentPrice
		}
		// Update PnL for this position
		if err := e.updatePositionPnL(ctx, pos, currentPrice); err != nil {
			e.logger.WithError(err).WithField("position_id", pos.ID).Error("Failed to update PnL for position in drawdown check")
			// Continue, try to check with potentially stale PnL for this one position
		}
	}

	portfolioHalted, err := e.riskManager.CheckPortfolioDrawdown(ctx, allOpenPositions)
	if err != nil {
		e.logger.WithError(err).Error("Error checking portfolio drawdown circuit breaker")
		// Decide if to proceed or not. For safety, maybe return err.
		return fmt.Errorf("error in portfolio drawdown check: %w", err)
	}
	if portfolioHalted {
		e.logger.Warn("Portfolio drawdown circuit breaker is active. Halting trading cycle.")
		// Optionally, could also trigger closing all positions here if that's desired.
		// For now, just halts new trading activities for this cycle.
		return nil // Skip the rest of the trading cycle
	}
	// --- End Portfolio Drawdown Circuit Breaker Check ---

	// Get active selected pairs (already fetched for pairIDToSymbol map, reuse `activePairs`)
	// pairs, err := e.repo.GetActiveSelectedPairs(ctx)
	// if err != nil {
	// 	return fmt.Errorf("failed to get active pairs: %w", err)
	// }

	e.logger.WithField("active_pairs", len(activePairs)).Debug("Processing trading cycle")

	for _, pair := range activePairs {
		if err := e.processPair(ctx, pair); err != nil { // pair here is models.SelectedPair
			e.logger.WithError(err).WithField("symbol", pair.Symbol).Error("Failed to process pair")
			continue
		}
	}

	return nil
}

func (e *Engine) processPair(ctx context.Context, pair models.SelectedPair) error { // pair is models.SelectedPair
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
	currentPriceFloat, err := e.repo.GetLatestPrice(ctx, pair.Symbol) // Renamed to avoid conflict
	if err != nil {
		e.logger.WithError(err).WithField("symbol", pair.Symbol).Error("Failed to get latest price for pair")
		return fmt.Errorf("failed to get latest price for pair %s: %w", pair.Symbol, err)
	}

	// --- Flash Crash Circuit Breaker Check ---
	flashCrashHalted, err := e.riskManager.CheckFlashCrash(ctx, pair.Symbol, currentPriceFloat)
	if err != nil {
		e.logger.WithError(err).WithFields(logrus.Fields{
			"symbol": pair.Symbol,
		}).Error("Error checking flash crash circuit breaker for pair")
		// If check fails, perhaps safer to skip this pair for the cycle.
		return fmt.Errorf("error in flash crash check for %s: %w", pair.Symbol, err)
	}
	if flashCrashHalted {
		e.logger.Warnf("Flash crash circuit breaker is active for pair %s. Skipping processing for this pair.", pair.Symbol)
		return nil // Skip this pair for the cycle
	}
	// --- End Flash Crash Circuit Breaker Check ---

	// Generate trading signal
	signal := e.signalGenerator.GenerateSignal(ctx, pair.Symbol, currentPriceFloat)

	// Get open positions for this specific pair
	positions, err := e.repo.GetOpenPositions(ctx, pair.ID) // pair.ID is int64
	if err != nil {
		return fmt.Errorf("failed to get open positions: %w", err)
	}

	// Update position PnL (for positions of this specific pair)
	for i := range positions {
		pos := &positions[i]
		if err := e.updatePositionPnL(ctx, pos, currentPriceFloat); err != nil { // Pass pointer
			e.logger.WithError(err).WithField("position_id", pos.ID).Error("Failed to update position PnL")
		}
	}

	// --- New Stop-Loss and Take-Profit Logic ---
	survivingPositions := make([]models.Position, 0, len(positions))
	for _, pos := range positions { // pos is a copy
		// PnL for pos is based on currentPriceFloat and was updated just above.

		stoppedOut := e.riskManager.shouldStopLoss(pos, currentPriceFloat) // pos is a copy, its PnL is current

		if stoppedOut {
			e.logger.WithFields(logrus.Fields{
				"position_id": pos.ID,
				"symbol":      pair.Symbol,
				"price":       currentPriceFloat,
			}).Info("Stop-loss triggered for position")
			if errClose := e.closePosition(ctx, pos, pair.Symbol, currentPriceFloat, "closed_stoploss"); errClose != nil {
				e.logger.WithError(err).WithFields(logrus.Fields{
					"position_id": pos.ID,
					"symbol":      pair.Symbol,
				}).Error("Failed to close position on stop-loss")
			}
			continue
		}

		tookProfit := e.riskManager.shouldTakeProfit(pos, currentPriceFloat)

		if tookProfit {
			e.logger.WithFields(logrus.Fields{
				"position_id": pos.ID,
				"symbol":      pair.Symbol,
				"price":       currentPriceFloat,
			}).Info("Take-profit triggered for position")
			if errClose := e.closePosition(ctx, pos, pair.Symbol, currentPriceFloat, "closed_takeprofit"); errClose != nil {
				e.logger.WithError(err).WithFields(logrus.Fields{
					"position_id": pos.ID,
					"symbol":      pair.Symbol,
				}).Error("Failed to close position on take-profit")
			}
			continue
		}

		survivingPositions = append(survivingPositions, pos)
	}
	// --- End of New Stop-Loss and Take-Profit Logic ---

	// Risk management checks using only surviving positions for new trades
	// Note: currentPriceFloat is the current market price for pair.Symbol
	if !e.riskManager.CanTrade(pair, survivingPositions, currentPriceFloat) {
		e.logger.WithField("symbol", pair.Symbol).Debug("Risk management blocked trading after SL/TP checks")
		return nil
	}

	// Execute trading strategy with surviving positions
	switch config.StrategyType {
	case "grid":
		return e.gridStrategy.Execute(ctx, pair, *config, signal, survivingPositions, currentPriceFloat)
	default:
		return e.executeBasicStrategy(ctx, pair, *config, signal, survivingPositions, currentPriceFloat)
	}
}

func (e *Engine) createDefaultConfig(pair models.SelectedPair) *models.TradingConfig {
	// Calculate price range based on volatility using configurable parameters
	priceRangePercent := pair.Volatility24h * e.config.DefaultStrategyVolatilityMultiplier
	if priceRangePercent < e.config.DefaultStrategyMinPriceRangePercent {
		priceRangePercent = e.config.DefaultStrategyMinPriceRangePercent
	}
	if priceRangePercent > e.config.DefaultStrategyMaxPriceRangePercent {
		priceRangePercent = e.config.DefaultStrategyMaxPriceRangePercent
	}

	return &models.TradingConfig{
		PairID:            pair.ID,
		StrategyType:      "grid", // Default strategy type remains hardcoded for now
		GridLevels:        e.config.DefaultStrategyGridLevels,
		PriceRangeMin:     0, // Will be set dynamically by the grid strategy based on current price and range
		PriceRangeMax:     0, // Will be set dynamically by the grid strategy based on current price and range
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

func (e *Engine) closePosition(ctx context.Context, position models.Position, symbol string, closePrice float64, reason string) error {
	e.logger.WithFields(logrus.Fields{
		"position_id": position.ID,
		"symbol":      symbol,
		"close_price": closePrice,
		"reason":      reason,
	}).Info("Attempting to close position")

	var orderSide string
	if position.Side == "buy" {
		orderSide = "sell"
	} else if position.Side == "sell" {
		orderSide = "buy" // For short positions, if ever supported
	} else {
		return fmt.Errorf("unknown position side: %s for position ID %s", position.Side, position.ID)
	}

	// Ensure e.exchange has PlaceMarketOrder or adapt if only limit orders are available
	// For now, assuming PlaceMarketOrder exists as per task description
	// orderResp, err := e.exchange.PlaceMarketOrder(position.Symbol, orderSide, position.Quantity)
	// TEMP: Using PlaceSellOrder for now, assuming it can act as market if price is not set or is current market.
	// This part needs to align with actual exchange capabilities.
	// If PlaceMarketOrder is the correct method on the exchange interface, this should be used.
	// The existing PlaceSellOrder / PlaceBuyOrder take a price argument, implying limit orders.
	// For a true market order, the price param might be ignored or need to be set to 0 or current market.
	// Let's simulate a market sell by using PlaceSellOrder with the currentPrice.

	var kucoinOrderID string // To store the actual exchange order ID

	if orderSide == "sell" {
		// This is effectively a market sell if the exchange treats a limit sell at current price as immediate.
		orderResp, err := e.exchange.PlaceSellOrder(symbol, position.Quantity, closePrice)
		if err != nil {
			e.logger.WithError(err).WithField("position_id", position.ID).Error("Failed to place market sell order to close position")
			// Optionally, update position status to "pending_close_failed" or similar
			return fmt.Errorf("failed to place market sell order for position %s: %w", position.ID, err)
		}
		kucoinOrderID = orderResp.OrderId
	} else {
		// Similar logic for closing a short position with a market buy
		orderResp, err := e.exchange.PlaceBuyOrder(symbol, position.Quantity, closePrice)
		if err != nil {
			e.logger.WithError(err).WithField("position_id", position.ID).Error("Failed to place market buy order to close position")
			return fmt.Errorf("failed to place market buy order for position %s: %w", position.ID, err)
		}
		kucoinOrderID = orderResp.OrderId
	}

	e.logger.WithFields(logrus.Fields{
		"position_id":       position.ID,
		"exchange_order_id": kucoinOrderID,
	}).Info("Position close order placed successfully")

	// Update position
	now := time.Now().UTC()
	position.Status = reason // e.g., "closed_stoploss", "closed_takeprofit"
	position.ClosedAt = &now
	// position.ClosePrice = &closePrice // Add ClosePrice to Position model if it's not there

	if position.Side == "buy" {
		position.RealizedPnL = (closePrice - position.EntryPrice) * position.Quantity
	} else if position.Side == "sell" { // For short positions
		position.RealizedPnL = (position.EntryPrice - closePrice) * position.Quantity
	}
	// Assuming UnrealizedPnL is cleared or becomes RealizedPnL.
	// The current updatePositionPnL calculates Unrealized. For a closed position, Unrealized should be 0.
	position.UnrealizedPnL = 0

	if err := e.repo.UpdatePosition(ctx, position); err != nil {
		e.logger.WithError(err).WithField("position_id", position.ID).Error("Failed to update position after closing")
		return fmt.Errorf("failed to update position %s after closing: %w", position.ID, err)
	}

	// Create corresponding order record
	order := models.Order{
		// ID: will be generated by DB
		PositionID:     &position.ID,
		PairID:         position.PairID,
		KuCoinOrderID:  kucoinOrderID, // From exchange response
		Side:           orderSide,
		Type:           "market", // Explicitly market
		Quantity:       position.Quantity,
		Price:          closePrice,        // Record the price at which it was closed
		FilledQuantity: position.Quantity, // Assuming market order fills completely and immediately
		Status:         "filled",          // Assuming market order fills immediately
		Fee:            0,                 // TODO: Need to get fee from exchange response if available
		CreatedAt:      now,
		UpdatedAt:      now,
		FilledAt:       &now,
	}

	if err := e.repo.CreateOrder(ctx, order); err != nil {
		e.logger.WithError(err).WithField("position_id", position.ID).Error("Failed to create order record for closed position")
		// This is not fatal for the closure itself, but an inconsistency.
		// Depending on requirements, this could be handled more robustly.
	}

	e.logger.WithFields(logrus.Fields{
		"position_id": position.ID,
		"reason":      reason,
		"pnl":         position.RealizedPnL,
	}).Info("Position closed and records updated")

	return nil
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
		PairID:        pair.ID,
		KuCoinOrderID: orderResp.OrderId,
		Side:          "sell",
		Type:          "limit",
		Quantity:      position.Quantity,
		Price:         price,
		Status:        "pending",
	}

	return e.repo.CreateOrder(ctx, order)
}

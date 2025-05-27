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
	signalGen *signals.Generator, config EngineConfig, logger *logrus.Logger) *Engine {

	return &Engine{
		repo:            repo,
		exchange:        exchange,
		signalGenerator: signalGen,
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

	// Update position PnL
	for _, position := range positions {
		if err := e.updatePositionPnL(ctx, &position, currentPrice); err != nil {
			e.logger.WithError(err).WithField("position_id", position.ID).Error("Failed to update position PnL")
		}
	}

	// Risk management checks
	if !e.riskManager.CanTrade(pair, positions, currentPrice) {
		e.logger.WithField("symbol", pair.Symbol).Debug("Risk management blocked trading")
		return nil
	}

	// Execute trading strategy
	switch config.StrategyType {
	case "grid":
		return e.gridStrategy.Execute(ctx, pair, *config, signal, positions, currentPrice)
	default:
		return e.executeBasicStrategy(ctx, pair, *config, signal, positions, currentPrice)
	}
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

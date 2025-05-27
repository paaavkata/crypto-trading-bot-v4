package trader

import (
	"github.com/paaavkata/crypto-trading-bot-v4/trading-engine/pkg/models"
	"github.com/sirupsen/logrus"
)

type RiskManager struct {
	config EngineConfig
	logger *logrus.Logger
}

func NewRiskManager(config EngineConfig, logger *logrus.Logger) *RiskManager {
	return &RiskManager{
		config: config,
		logger: logger,
	}
}

func (r *RiskManager) CanTrade(pair models.SelectedPair, positions []models.Position, currentPrice float64) bool {
	// Check maximum positions per pair
	if len(positions) >= r.config.MaxPositionsPerPair {
		r.logger.WithField("symbol", pair.Symbol).Debug("Maximum positions reached")
		return false
	}

	// Check total exposure
	totalExposure := r.calculateTotalExposure(positions, currentPrice)
	maxExposure := float64(r.config.MaxPositionsPerPair) * r.config.DefaultPositionSize

	if totalExposure > maxExposure {
		r.logger.WithFields(logrus.Fields{
			"symbol":         pair.Symbol,
			"total_exposure": totalExposure,
			"max_exposure":   maxExposure,
		}).Debug("Maximum exposure reached")
		return false
	}

	// Check for stop loss conditions
	for _, position := range positions {
		if r.shouldStopLoss(position, currentPrice) {
			r.logger.WithFields(logrus.Fields{
				"symbol":         pair.Symbol,
				"position_id":    position.ID,
				"entry_price":    position.EntryPrice,
				"current_price":  currentPrice,
				"unrealized_pnl": position.UnrealizedPnL,
			}).Warn("Stop loss condition detected")
			return false
		}
	}

	return true
}

func (r *RiskManager) calculateTotalExposure(positions []models.Position, currentPrice float64) float64 {
	totalExposure := 0.0

	for _, position := range positions {
		if position.Status == "open" {
			exposure := position.Quantity * currentPrice
			totalExposure += exposure
		}
	}

	return totalExposure
}

func (r *RiskManager) shouldStopLoss(position models.Position, currentPrice float64) bool {
	if position.Status != "open" {
		return false
	}

	var lossPercent float64
	if position.Side == "buy" {
		lossPercent = (position.EntryPrice - currentPrice) / position.EntryPrice
	} else {
		lossPercent = (currentPrice - position.EntryPrice) / position.EntryPrice
	}

	return lossPercent > r.config.StopLossPercent
}

func (r *RiskManager) shouldTakeProfit(position models.Position, currentPrice float64) bool {
	if position.Status != "open" {
		return false
	}

	var profitPercent float64
	if position.Side == "buy" {
		profitPercent = (currentPrice - position.EntryPrice) / position.EntryPrice
	} else {
		profitPercent = (position.EntryPrice - currentPrice) / position.EntryPrice
	}

	return profitPercent > r.config.TakeProfitPercent
}

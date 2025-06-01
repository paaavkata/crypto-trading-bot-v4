package trader

import (
	"context"
	"fmt"
	"time"

	"github.com/paaavkata/crypto-trading-bot-v4/trading-engine/internal/database"
	"github.com/paaavkata/crypto-trading-bot-v4/trading-engine/pkg/models"
	"github.com/sirupsen/logrus"
)

type RiskManager struct {
	repo                          *database.Repository
	config                        EngineConfig
	logger                        *logrus.Logger
	portfolioTradingHaltedUntil   time.Time
	pairFlashCrashHaltedUntil     map[string]time.Time
	portfolioDrawdownInitialValue float64 // For more complex drawdown logic, not used in simplified version
	isPortfolioInitialValueSet    bool    // For more complex drawdown logic
}

func NewRiskManager(repo *database.Repository, config EngineConfig, logger *logrus.Logger) *RiskManager {
	return &RiskManager{
		repo:                      repo,
		config:                    config,
		logger:                    logger,
		pairFlashCrashHaltedUntil: make(map[string]time.Time),
	}
}

// CheckPortfolioDrawdown checks if the portfolio has experienced a drawdown exceeding configured limits.
// The current simplified implementation considers drawdown as (TotalUnrealizedPnL / TotalAccountBalance).
// It halts trading globally if the drawdown is too severe.
func (r *RiskManager) CheckPortfolioDrawdown(ctx context.Context, currentOpenPositions []models.Position) (bool, error) {
	if !r.portfolioTradingHaltedUntil.IsZero() && time.Now().Before(r.portfolioTradingHaltedUntil) {
		r.logger.Warnf("Portfolio trading is currently halted until %s", r.portfolioTradingHaltedUntil.Format(time.RFC3339))
		return true, nil // Trading is halted
	}
	// Reset halt if current time is past the halt time.
	if !r.portfolioTradingHaltedUntil.IsZero() && time.Now().After(r.portfolioTradingHaltedUntil) {
		r.logger.Info("Portfolio trading halt duration expired. Resuming trading.")
		r.portfolioTradingHaltedUntil = time.Time{} // Reset
	}

	if r.config.TotalAccountBalance <= 0 {
		r.logger.Warn("TotalAccountBalance is not configured or is zero, cannot calculate portfolio drawdown.")
		return false, nil // Cannot assess, so don't halt
	}
	if r.config.CBPortfolioDrawdownPercent <= 0 {
		r.logger.Debug("Portfolio drawdown circuit breaker is not enabled (CBPortfolioDrawdownPercent <= 0).")
		return false, nil // Breaker not enabled
	}

	totalUnrealizedPnL := 0.0
	for _, pos := range currentOpenPositions {
		// Assuming PnL is already updated for currentPrice before this check
		totalUnrealizedPnL += pos.UnrealizedPnL
	}

	// Simplified drawdown calculation: (TotalUnrealizedPnL / TotalAccountBalance)
	// Note: This doesn't use recentRealizedPnL as per the simplified plan.
	// A more comprehensive drawdown would be: (currentValue - initialValueOverPeriod) / initialValueOverPeriod
	// where currentValue = TotalAccountBalance + totalUnrealizedPnL + recentRealizedPnL (since interval)
	// and initialValueOverPeriod = TotalAccountBalance + realizedPnLAtStartOfInterval.
	// For now, using the simpler (totalUnrealizedPnL / TotalAccountBalance) * 100% < -CBPortfolioDrawdownPercent

	drawdownPercent := 0.0
	if r.config.TotalAccountBalance > 0 { // Avoid division by zero
		drawdownPercent = (totalUnrealizedPnL / r.config.TotalAccountBalance) * 100.0
	}

	r.logger.WithFields(logrus.Fields{
		"total_unrealized_pnl":              totalUnrealizedPnL,
		"total_account_balance":             r.config.TotalAccountBalance,
		"drawdown_percent":                  fmt.Sprintf("%.2f%%", drawdownPercent),
		"configured_drawdown_limit_percent": -r.config.CBPortfolioDrawdownPercent,
	}).Debug("Portfolio drawdown check")

	if totalUnrealizedPnL < 0 && drawdownPercent < -r.config.CBPortfolioDrawdownPercent {
		haltDuration := time.Duration(r.config.CBTradingHaltDurationMinutes) * time.Minute
		r.portfolioTradingHaltedUntil = time.Now().Add(haltDuration)
		r.logger.WithFields(logrus.Fields{
			"calculated_drawdown_percent": drawdownPercent,
			"limit_percent":               -r.config.CBPortfolioDrawdownPercent,
			"halt_duration_minutes":       r.config.CBTradingHaltDurationMinutes,
			"halted_until":                r.portfolioTradingHaltedUntil.Format(time.RFC3339),
		}).Error("Portfolio drawdown circuit breaker triggered! Halting all trading.")
		// TODO: Consider if it should close all open positions as well. Current plan is just to halt new trades.
		return true, nil // Trading halted
	}

	return false, nil // Trading not halted
}

// CheckFlashCrash checks for a significant price drop for a specific pair within a configured time window.
// It halts trading for that specific pair if a flash crash is detected.
func (r *RiskManager) CheckFlashCrash(ctx context.Context, pairSymbol string, currentPrice float64) (bool, error) {
	if haltTime, ok := r.pairFlashCrashHaltedUntil[pairSymbol]; ok && time.Now().Before(haltTime) {
		r.logger.WithFields(logrus.Fields{
			"symbol":       pairSymbol,
			"halted_until": haltTime.Format(time.RFC3339),
		}).Warnf("Trading for pair %s is currently halted due to flash crash detection.", pairSymbol)
		return true, nil // Trading for this pair is halted
	}
	// Reset halt if current time is past the halt time for the pair.
	if haltTime, ok := r.pairFlashCrashHaltedUntil[pairSymbol]; ok && time.Now().After(haltTime) {
		r.logger.Infof("Flash crash trading halt for pair %s expired. Resuming trading for this pair.", pairSymbol)
		delete(r.pairFlashCrashHaltedUntil, pairSymbol) // Remove from map
	}

	if r.config.CBFlashCrashDropPercent <= 0 || r.config.CBFlashCrashWindowMinutes <= 0 {
		r.logger.WithField("symbol", pairSymbol).Debug("Flash crash circuit breaker not enabled for this pair.")
		return false, nil // Breaker not enabled
	}

	// Fetch recent price data. This requires a repository method.
	// The PricePoint model needs to be consistent with what GetPriceHistory returns.
	// Assuming models.PricePoint { Timestamp, High, Low, Close }
	priceHistory, err := r.repo.GetPriceHistory(ctx, pairSymbol, r.config.CBFlashCrashWindowMinutes)
	if err != nil {
		r.logger.WithError(err).WithField("symbol", pairSymbol).Error("Failed to get price history for flash crash check")
		return false, fmt.Errorf("failed to get price history for %s: %w", pairSymbol, err)
	}

	if len(priceHistory) == 0 {
		r.logger.WithField("symbol", pairSymbol).Debug("No price history found for flash crash check.")
		return false, nil // Not enough data
	}

	// Find the high price in the window
	maxPriceInWindow := 0.0
	for _, p := range priceHistory {
		// Assuming PricePoint has High field, or use Close if not.
		// Let's assume we are checking drop from highest point in the window.
		// If PricePoint model is just {Timestamp, Close}, then adapt.
		// For now, let's work with Close prices to find the peak.
		if p.Close > maxPriceInWindow { // Or p.High if available and preferred
			maxPriceInWindow = p.Close
		}
	}
	// Also consider the current price as part of the window's end state if not in history
	// However, typical use is history up to T-1, and currentPrice is T.
	// If currentPrice is significantly lower than maxPriceInWindow from recent history:

	if maxPriceInWindow == 0 { // Should not happen if priceHistory is not empty and prices are positive
		return false, nil
	}

	dropPercent := ((maxPriceInWindow - currentPrice) / maxPriceInWindow) * 100.0

	r.logger.WithFields(logrus.Fields{
		"symbol":                        pairSymbol,
		"current_price":                 currentPrice,
		"max_price_in_window":           maxPriceInWindow,
		"drop_percent":                  fmt.Sprintf("%.2f%%", dropPercent),
		"configured_drop_limit_percent": r.config.CBFlashCrashDropPercent,
		"window_minutes":                r.config.CBFlashCrashWindowMinutes,
	}).Debug("Flash crash check")

	if dropPercent >= r.config.CBFlashCrashDropPercent {
		haltDuration := time.Duration(r.config.CBTradingHaltDurationMinutes) * time.Minute
		r.pairFlashCrashHaltedUntil[pairSymbol] = time.Now().Add(haltDuration)
		r.logger.WithFields(logrus.Fields{
			"symbol":                  pairSymbol,
			"current_price":           currentPrice,
			"max_price_in_window":     maxPriceInWindow,
			"calculated_drop_percent": dropPercent,
			"limit_percent":           r.config.CBFlashCrashDropPercent,
			"halt_duration_minutes":   r.config.CBTradingHaltDurationMinutes,
			"halted_until":            r.pairFlashCrashHaltedUntil[pairSymbol].Format(time.RFC3339),
		}).Errorf("Flash crash circuit breaker triggered for pair %s! Halting trading for this pair.", pairSymbol)
		return true, nil // Trading for this pair halted
	}

	return false, nil // Trading for this pair not halted
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
	// This check is now primarily handled in engine.processPair before CanTrade is called.
	// Keeping it here could be a defense-in-depth measure, but might also lead to
	// redundant logging or immediate re-flagging if a position is in the process of being closed.
	// For now, commenting out as per the plan, assuming processPair handles closure.
	/*
		for _, position := range positions {
			if r.shouldStopLoss(position, currentPrice) {
				r.logger.WithFields(logrus.Fields{
					"symbol":         pair.Symbol,
					"position_id":    position.ID,
					"entry_price":    position.EntryPrice,
					"current_price":  currentPrice,
					"unrealized_pnl": position.UnrealizedPnL,
				}).Warn("Stop loss condition detected in CanTrade (should have been handled by processPair)")
				return false
			}
		}
	*/

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

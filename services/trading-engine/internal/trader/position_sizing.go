package trader

import (
	"math"

	"github.com/paaavkata/crypto-trading-bot-v4/trading-engine/pkg/models"
	"github.com/sirupsen/logrus"
)

type PositionSizer struct {
	logger                *logrus.Logger
	maxRiskPerTrade       float64
	maxPortfolioRisk      float64
	basePositionSize      float64
	volatilityAdjustment  bool
	kellyCriterionEnabled bool
}

type PositionSizingConfig struct {
	MaxRiskPerTrade       float64 // Maximum risk per single trade (e.g., 0.02 = 2%)
	MaxPortfolioRisk      float64 // Maximum total portfolio risk (e.g., 0.10 = 10%)
	BasePositionSize      float64 // Base position size in USDT
	VolatilityAdjustment  bool    // Adjust position size based on volatility
	KellyCriterionEnabled bool    // Use Kelly Criterion for position sizing
}

type RiskMetrics struct {
	WinRate          float64
	AverageWin       float64
	AverageLoss      float64
	SharpeRatio      float64
	MaxDrawdown      float64
	VaR              float64 // Value at Risk
	PortfolioBalance float64
}

func NewPositionSizer(config PositionSizingConfig, logger *logrus.Logger) *PositionSizer {
	return &PositionSizer{
		logger:                logger,
		maxRiskPerTrade:       config.MaxRiskPerTrade,
		maxPortfolioRisk:      config.MaxPortfolioRisk,
		basePositionSize:      config.BasePositionSize,
		volatilityAdjustment:  config.VolatilityAdjustment,
		kellyCriterionEnabled: config.KellyCriterionEnabled,
	}
}

func (ps *PositionSizer) CalculatePositionSize(
	pair models.SelectedPair,
	signal models.Signal,
	currentPrice float64,
	riskMetrics RiskMetrics,
	openPositions []models.Position,
) float64 {

	// Base position size
	positionSize := ps.basePositionSize

	// Risk-based adjustment
	if ps.maxRiskPerTrade > 0 {
		stopLossPercent := 0.05 // Default 5% stop loss
		if pair.StopLossPercent > 0 {
			stopLossPercent = pair.StopLossPercent / 100.0
		}

		// Risk amount = Portfolio Balance * Max Risk Per Trade
		riskAmount := riskMetrics.PortfolioBalance * ps.maxRiskPerTrade

		// Position size = Risk Amount / (Entry Price * Stop Loss %)
		riskBasedSize := riskAmount / (currentPrice * stopLossPercent)

		// Use smaller of base size and risk-based size
		if riskBasedSize < positionSize {
			positionSize = riskBasedSize
		}
	}

	// Kelly Criterion adjustment
	if ps.kellyCriterionEnabled && riskMetrics.WinRate > 0 {
		kellyFraction := ps.calculateKellyFraction(riskMetrics)

		// Apply Kelly fraction with cap at 25% of portfolio
		kellySize := riskMetrics.PortfolioBalance * math.Min(kellyFraction, 0.25) / currentPrice

		if kellySize > 0 && kellySize < positionSize {
			positionSize = kellySize
		}

		ps.logger.WithFields(logrus.Fields{
			"symbol":         pair.Symbol,
			"kelly_fraction": kellyFraction,
			"kelly_size":     kellySize,
		}).Debug("Applied Kelly Criterion")
	}

	// Volatility adjustment
	if ps.volatilityAdjustment {
		volatilityMultiplier := ps.calculateVolatilityMultiplier(pair.Volatility24h)
		positionSize *= volatilityMultiplier

		ps.logger.WithFields(logrus.Fields{
			"symbol":                pair.Symbol,
			"volatility":            pair.Volatility24h,
			"volatility_multiplier": volatilityMultiplier,
		}).Debug("Applied volatility adjustment")
	}

	// Signal strength adjustment
	signalMultiplier := ps.calculateSignalMultiplier(signal.Strength)
	positionSize *= signalMultiplier

	// Portfolio concentration limits
	currentExposure := ps.calculateCurrentExposure(openPositions, currentPrice)
	maxAllowedExposure := riskMetrics.PortfolioBalance * ps.maxPortfolioRisk

	if currentExposure+(positionSize*currentPrice) > maxAllowedExposure {
		remainingCapacity := maxAllowedExposure - currentExposure
		if remainingCapacity > 0 {
			positionSize = remainingCapacity / currentPrice
		} else {
			positionSize = 0
		}
	}

	// Minimum position size check
	minPositionUSDT := 10.0 // Minimum $10 position
	if positionSize*currentPrice < minPositionUSDT {
		positionSize = 0
	}

	ps.logger.WithFields(logrus.Fields{
		"symbol":           pair.Symbol,
		"base_size":        ps.basePositionSize,
		"final_size":       positionSize,
		"signal_strength":  signal.Strength,
		"current_exposure": currentExposure,
		"max_exposure":     maxAllowedExposure,
	}).Debug("Calculated position size")

	return positionSize
}

func (ps *PositionSizer) calculateKellyFraction(metrics RiskMetrics) float64 {
	if metrics.AverageLoss <= 0 || metrics.WinRate <= 0 || metrics.WinRate >= 1 {
		return 0
	}

	// Kelly Criterion: f = (bp - q) / b
	// where:
	// f = fraction of capital to wager
	// b = odds received on the wager (average win / average loss)
	// p = probability of winning (win rate)
	// q = probability of losing (1 - win rate)

	b := metrics.AverageWin / metrics.AverageLoss
	p := metrics.WinRate
	q := 1 - p

	kellyFraction := (b*p - q) / b

	// Cap Kelly fraction to prevent over-leverage
	return math.Max(0, math.Min(kellyFraction, 0.25))
}

func (ps *PositionSizer) calculateVolatilityMultiplier(volatility float64) float64 {
	// Inverse relationship: higher volatility = smaller position
	// Target volatility: 5%
	targetVolatility := 0.05

	if volatility <= 0 {
		return 1.0
	}

	multiplier := targetVolatility / volatility

	// Cap between 0.5 and 2.0
	return math.Max(0.5, math.Min(multiplier, 2.0))
}

func (ps *PositionSizer) calculateSignalMultiplier(strength float64) float64 {
	// Signal strength between 0 and 1
	// Multiplier between 0.5 and 1.5
	return 0.5 + strength
}

func (ps *PositionSizer) calculateCurrentExposure(positions []models.Position, currentPrice float64) float64 {
	totalExposure := 0.0

	for _, position := range positions {
		if position.Status == "open" {
			// Calculate current value of position
			positionValue := position.Quantity * currentPrice
			totalExposure += positionValue
		}
	}

	return totalExposure
}

// CalculateStopLoss determines optimal stop loss based on volatility and risk tolerance
func (ps *PositionSizer) CalculateStopLoss(
	entryPrice float64,
	volatility float64,
	side string,
) float64 {

	// Base stop loss: 2x ATR or 5%, whichever is smaller
	baseStopPercent := math.Min(volatility*2, 0.05)

	// Minimum stop loss: 2%
	stopPercent := math.Max(baseStopPercent, 0.02)

	if side == "buy" {
		return entryPrice * (1 - stopPercent)
	} else {
		return entryPrice * (1 + stopPercent)
	}
}

// CalculateTakeProfit determines optimal take profit based on risk-reward ratio
func (ps *PositionSizer) CalculateTakeProfit(
	entryPrice float64,
	stopLossPrice float64,
	side string,
	riskRewardRatio float64,
) float64 {

	if riskRewardRatio <= 0 {
		riskRewardRatio = 2.0 // Default 1:2 risk-reward
	}

	stopLossDistance := math.Abs(entryPrice - stopLossPrice)
	takeProfitDistance := stopLossDistance * riskRewardRatio

	if side == "buy" {
		return entryPrice + takeProfitDistance
	} else {
		return entryPrice - takeProfitDistance
	}
}

// UpdateRiskMetrics calculates current risk metrics from trading history
func (ps *PositionSizer) UpdateRiskMetrics(positions []models.Position) RiskMetrics {
	closedPositions := []models.Position{}

	for _, pos := range positions {
		if pos.Status == "closed" && pos.RealizedPnL != 0 {
			closedPositions = append(closedPositions, pos)
		}
	}

	if len(closedPositions) == 0 {
		return RiskMetrics{
			WinRate:     0.5, // Default assumption
			AverageWin:  100,
			AverageLoss: 50,
		}
	}

	wins := 0
	totalWin := 0.0
	totalLoss := 0.0
	totalPnL := 0.0

	for _, pos := range closedPositions {
		totalPnL += pos.RealizedPnL

		if pos.RealizedPnL > 0 {
			wins++
			totalWin += pos.RealizedPnL
		} else {
			totalLoss += math.Abs(pos.RealizedPnL)
		}
	}

	winRate := float64(wins) / float64(len(closedPositions))
	avgWin := 0.0
	avgLoss := 0.0

	if wins > 0 {
		avgWin = totalWin / float64(wins)
	}

	if len(closedPositions)-wins > 0 {
		avgLoss = totalLoss / float64(len(closedPositions)-wins)
	}

	return RiskMetrics{
		WinRate:     winRate,
		AverageWin:  avgWin,
		AverageLoss: avgLoss,
		// Other metrics would be calculated here in a real implementation
	}
}

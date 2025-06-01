package selector

import (
	"math"

	"github.com/paaavkata/crypto-trading-bot-v4/pair-selector/pkg/models"
	"github.com/paaavkata/crypto-trading-bot-v4/shared/pkg/utils"
	"github.com/sirupsen/logrus"
)

type VolatilityAnalyzer struct {
	logger *logrus.Logger
}

type VolatilityMetrics struct {
	Volatility24h float64
	ATR14         float64
	StdDev        float64
}

func NewVolatilityAnalyzer(logger *logrus.Logger) *VolatilityAnalyzer {
	return &VolatilityAnalyzer{logger: logger}
}

// VolatilityMetrics holds the results of volatility analysis.
// Note: ATR14 now uses a configurable period via SelectionCriteria.ATRPeriod.
type VolatilityMetrics struct {
	Volatility24h float64
	ATR14         float64 // Represents ATR calculated with criteria.ATRPeriod
	StdDev        float64
}

func NewVolatilityAnalyzer(logger *logrus.Logger) *VolatilityAnalyzer {
	return &VolatilityAnalyzer{logger: logger}
}

func (v *VolatilityAnalyzer) AnalyzeVolatility(priceData []models.PricePoint, criteria models.SelectionCriteria) VolatilityMetrics {
	if len(priceData) < 2 {
		return VolatilityMetrics{}
	}

	// Extract price arrays for calculations
	closes := make([]float64, len(priceData))
	highs := make([]float64, len(priceData))
	lows := make([]float64, len(priceData))

	for i, point := range priceData {
		closes[i] = point.Close
		highs[i] = point.High
		lows[i] = point.Low
	}

	// Calculate 24h volatility (standard deviation of returns)
	volatility := utils.CalculateVolatility(closes)

	// Calculate ATR (Average True Range) using the configured period
	atrPeriod := criteria.ATRPeriod
	if atrPeriod <= 0 { // Default to 14 if not specified or invalid
		v.logger.Warnf("Invalid ATRPeriod configured: %d. Defaulting to 14.", criteria.ATRPeriod)
		atrPeriod = 14
	}
	if len(priceData) < atrPeriod {
		atrPeriod = len(priceData)
	}

	atr := utils.CalculateATR(highs, lows, closes, atrPeriod)

	// Calculate standard deviation for additional context
	stdDev := v.calculateStandardDeviation(closes)

	return VolatilityMetrics{
		Volatility24h: volatility,
		ATR14:         atr,
		StdDev:        stdDev,
	}
}

func (v *VolatilityAnalyzer) calculateStandardDeviation(prices []float64) float64 {
	if len(prices) < 2 {
		return 0
	}

	// Calculate mean
	sum := 0.0
	for _, price := range prices {
		sum += price
	}
	mean := sum / float64(len(prices))

	// Calculate variance
	variance := 0.0
	for _, price := range prices {
		variance += math.Pow(price-mean, 2)
	}
	variance /= float64(len(prices))

	return math.Sqrt(variance)
}

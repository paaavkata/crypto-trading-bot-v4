package selector

import (
	"math"

	"github.com/paaavkata/crypto-trading-bot-v4/pair-selector/pkg/models"
	"github.com/sirupsen/logrus"
)

type Scorer struct {
	logger *logrus.Logger
}

func NewScorer(logger *logrus.Logger) *Scorer {
	return &Scorer{logger: logger}
}

func (s *Scorer) CalculateVolumeScore(volumeUSDT, minVolumeUSDT float64) float64 {
	if volumeUSDT <= minVolumeUSDT {
		return 0.0
	}

	// Logarithmic scoring for volume (diminishing returns)
	ratio := volumeUSDT / minVolumeUSDT
	score := math.Log10(ratio) / math.Log10(10) // Normalize to 0-1 range for 10x volume

	if score > 1.0 {
		score = 1.0
	}

	return score
}

func (s *Scorer) CalculateVolatilityScore(volatility, minVol, maxVol float64) float64 {
	if volatility < minVol || volatility > maxVol {
		return 0.0
	}

	// Optimal volatility is in the middle of the range
	optimalVol := (minVol + maxVol) / 2

	// Calculate distance from optimal
	distance := math.Abs(volatility - optimalVol)
	maxDistance := (maxVol - minVol) / 2

	// Score decreases as distance from optimal increases
	score := 1.0 - (distance / maxDistance)

	if score < 0 {
		score = 0
	}

	return score
}

func (s *Scorer) CalculateATRScore(atr float64) float64 {
	if atr <= 0 {
		return 0.0
	}

	// ATR score based on reasonable ranges
	// Higher ATR indicates more trading opportunities but also higher risk
	if atr > 0.1 { // Very high ATR
		return 0.6
	} else if atr > 0.05 { // High ATR - good for trading
		return 1.0
	} else if atr > 0.02 { // Medium ATR
		return 0.8
	} else if atr > 0.01 { // Low ATR
		return 0.4
	}

	return 0.2 // Very low ATR
}

func (s *Scorer) CalculateCorrelationScore(correlation float64) float64 {
	absCorr := math.Abs(correlation)

	// Moderate correlation is preferred for diversification
	// Very high correlation means pairs move together (less diversification)
	// Very low correlation might indicate unstable or manipulated pairs

	if absCorr >= 0.3 && absCorr <= 0.7 {
		return 1.0 // Optimal range
	} else if absCorr >= 0.2 && absCorr < 0.3 {
		return 0.7 // Acceptable lower bound
	} else if absCorr > 0.7 && absCorr <= 0.85 {
		return 0.8 // Still good but less diversification
	} else if absCorr > 0.85 {
		return 0.4 // Too highly correlated
	}

	return 0.2 // Very low correlation - potentially risky
}

func (s *Scorer) CalculateFinalScore(analysis models.PairAnalysis, criteria models.SelectionCriteria) float64 {
	// Weighted sum of all scores
	finalScore := (analysis.VolumeScore * criteria.VolumeWeight) +
		(analysis.VolatilityScore * criteria.VolatilityWeight) +
		(analysis.ATRScore * criteria.ATRWeight) +
		(analysis.CorrelationScore * criteria.CorrelationWeight)

	// Ensure score is between 0 and 1
	if finalScore > 1.0 {
		finalScore = 1.0
	} else if finalScore < 0.0 {
		finalScore = 0.0
	}

	return finalScore
}

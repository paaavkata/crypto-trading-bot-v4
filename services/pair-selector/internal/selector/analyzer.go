package selector

import (
	"context"
	"fmt"
	"math"
	"sort"

	"github.com/paaavkata/crypto-trading-bot-v4/pair-selector/internal/database"
	"github.com/paaavkata/crypto-trading-bot-v4/pair-selector/pkg/models"
	"github.com/sirupsen/logrus"
)

type Analyzer struct {
	repo                *database.Repository
	volatilityAnalyzer  *VolatilityAnalyzer
	volumeAnalyzer      *VolumeAnalyzer
	correlationAnalyzer *CorrelationAnalyzer
	scorer              *Scorer
	logger              *logrus.Logger
}

func NewAnalyzer(repo *database.Repository, logger *logrus.Logger) *Analyzer {
	return &Analyzer{
		repo:                repo,
		volatilityAnalyzer:  NewVolatilityAnalyzer(logger),
		volumeAnalyzer:      NewVolumeAnalyzer(logger),
		correlationAnalyzer: NewCorrelationAnalyzer(repo, logger),
		scorer:              NewScorer(logger),
		logger:              logger,
	}
}

func (a *Analyzer) AnalyzePairs(ctx context.Context, criteria models.SelectionCriteria) ([]models.PairAnalysis, error) {
	a.logger.Info("Starting comprehensive pair analysis")

	// Get all trading pairs
	pairs, err := a.repo.GetTradingPairs(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get trading pairs: %w", err)
	}

	a.logger.WithField("total_pairs", len(pairs)).Info("Fetched trading pairs")

	var analyses []models.PairAnalysis

	for _, pair := range pairs {
		analysis, err := a.analyzeSinglePair(ctx, pair, criteria)
		if err != nil {
			a.logger.WithError(err).WithField("symbol", pair.Symbol).Warn("Failed to analyze pair")
			continue
		}

		if analysis != nil {
			analyses = append(analyses, *analysis)
		}
	}

	// Sort by final score
	sort.Slice(analyses, func(i, j int) bool {
		return analyses[i].FinalScore > analyses[j].FinalScore
	})

	// Limit to watchlist size
	if len(analyses) > criteria.WatchlistSize {
		analyses = analyses[:criteria.WatchlistSize]
	}

	a.logger.WithField("analyzed_pairs", len(analyses)).Info("Completed pair analysis")
	return analyses, nil
}

func (a *Analyzer) analyzeSinglePair(ctx context.Context, pair models.TradingPair, criteria models.SelectionCriteria) (*models.PairAnalysis, error) {
	// Get price history for the last 24 hours for volatility analysis
	priceHistory, err := a.repo.GetPriceHistory(ctx, pair.Symbol, 24)
	if err != nil {
		return nil, fmt.Errorf("failed to get price history: %w", err)
	}

	// Skip pairs with insufficient price data
	if len(priceHistory) < 20 { // Need at least 20 data points
		return nil, nil
	}

	analysis := models.PairAnalysis{
		Symbol:    pair.Symbol,
		PriceData: priceHistory,
	}

	// Volume Analysis
	volumeMetrics := a.volumeAnalyzer.AnalyzeVolume(priceHistory)
	analysis.Volume24hUSDT = volumeMetrics.Volume24hUSDT

	// Skip pairs below minimum volume threshold
	if analysis.Volume24hUSDT < criteria.MinVolumeUSDT {
		return nil, nil
	}

	// Volatility Analysis
	volatilityMetrics := a.volatilityAnalyzer.AnalyzeVolatility(priceHistory, criteria)
	analysis.Volatility = volatilityMetrics.Volatility24h
	analysis.ATR14 = volatilityMetrics.ATR14

	// Skip pairs outside volatility range
	if analysis.Volatility < criteria.MinVolatility || analysis.Volatility > criteria.MaxVolatility {
		return nil, nil
	}

	// Correlation Analysis (with BTC)
	correlationMetrics, err := a.correlationAnalyzer.AnalyzeCorrelation(ctx, pair.Symbol, "BTC-USDT", 24)
	if err != nil {
		a.logger.WithError(err).WithField("symbol", pair.Symbol).Warn("Failed to analyze correlation")
		analysis.CorrelationBTC = 0
	} else {
		analysis.CorrelationBTC = correlationMetrics.Correlation
	}

	// Calculate individual scores
	analysis.VolumeScore = a.scorer.CalculateVolumeScore(analysis.Volume24hUSDT, criteria.MinVolumeUSDT)
	analysis.VolatilityScore = a.scorer.CalculateVolatilityScore(analysis.Volatility, criteria.MinVolatility, criteria.MaxVolatility)
	analysis.ATRScore = a.scorer.CalculateATRScore(analysis.ATR14)
	analysis.CorrelationScore = a.scorer.CalculateCorrelationScore(analysis.CorrelationBTC)

	// Calculate final weighted score
	analysis.FinalScore = a.scorer.CalculateFinalScore(analysis, criteria)

	// Determine risk level
	analysis.RiskLevel = a.determineRiskLevel(analysis, criteria.RiskThresholds)

	// Update trading pair metrics in database
	metrics := map[string]float64{
		"volume_usdt":     analysis.Volume24hUSDT,
		"volatility":      analysis.Volatility,
		"atr_14":          analysis.ATR14,
		"correlation_btc": analysis.CorrelationBTC,
	}

	if err := a.repo.UpdateTradingPairMetrics(ctx, pair.Symbol, metrics); err != nil {
		a.logger.WithError(err).WithField("symbol", pair.Symbol).Warn("Failed to update pair metrics")
	}

	return &analysis, nil
}

func (a *Analyzer) determineRiskLevel(analysis models.PairAnalysis, thresholds models.RiskThresholdsConfig) string {
	// Enhanced risk assessment with market regime detection
	riskScore := 0.0
	volCfg := thresholds.VolatilityRisk
	corrCfg := thresholds.CorrelationRisk
	voluCfg := thresholds.VolumeRisk
	atrCfg := thresholds.ATRRisk
	momCfg := thresholds.MomentumRisk
	overallCfg := thresholds.OverallRisk

	// Volatility component
	volatilityRisk := 0.0
	if analysis.Volatility > volCfg.Band1 {
		volatilityRisk = volCfg.Score1
	} else if analysis.Volatility > volCfg.Band2 {
		volatilityRisk = volCfg.Score2
	} else if analysis.Volatility > volCfg.Band3 {
		volatilityRisk = volCfg.Score3
	} else if analysis.Volatility > volCfg.Band4 {
		volatilityRisk = volCfg.Score4
	} else {
		volatilityRisk = volCfg.Score5
	}
	riskScore += volatilityRisk * volCfg.Weight

	// Correlation component - Low correlation increases risk
	correlationRisk := 0.0
	absCorrelation := math.Abs(analysis.CorrelationBTC)
	if absCorrelation < corrCfg.Band1 {
		correlationRisk = corrCfg.Score1 // Very uncorrelated = higher risk
	} else if absCorrelation < corrCfg.Band2 {
		correlationRisk = corrCfg.Score2
	} else if absCorrelation < corrCfg.Band3 {
		correlationRisk = corrCfg.Score3
	} else {
		correlationRisk = corrCfg.Score4 // High correlation = lower risk
	}
	riskScore += correlationRisk * corrCfg.Weight

	// Volume stability component
	volumeRisk := 0.0
	if analysis.Volume24hUSDT < voluCfg.Band1 {
		volumeRisk = voluCfg.Score1
	} else if analysis.Volume24hUSDT < voluCfg.Band2 {
		volumeRisk = voluCfg.Score2
	} else if analysis.Volume24hUSDT < voluCfg.Band3 {
		volumeRisk = voluCfg.Score3
	} else {
		volumeRisk = voluCfg.Score4
	}
	riskScore += volumeRisk * voluCfg.Weight

	// ATR/Volatility ratio component - High ATR relative to volatility indicates instability
	atrRiskValue := 0.0
	if analysis.Volatility > 0 {
		atrRatio := analysis.ATR14 / analysis.Volatility
		if atrRatio > atrCfg.RatioBand1 {
			atrRiskValue = atrCfg.Score1
		} else if atrRatio > atrCfg.RatioBand2 {
			atrRiskValue = atrCfg.Score2
		} else {
			atrRiskValue = atrCfg.Score3
		}
	} else {
		atrRiskValue = atrCfg.DefaultScoreNoVol
	}
	riskScore += atrRiskValue * atrCfg.Weight

	// Price momentum component
	momentumRisk := a.calculateMomentumRisk(analysis.PriceData, momCfg)
	riskScore += momentumRisk * momCfg.Weight

	// Normalize risk score (assuming max raw score is dynamic or weights sum to 1 and max individual score is capped e.g. at 4)
	// The original code divided by 4.0. This implies the max possible weighted sum should be around 4.0.
	// For now, we keep the division by 4.0. If weights or scores change, this might need adjustment or become configurable.
	normalizedRisk := riskScore / 4.0 // TODO: Consider making normalization factor configurable or dynamically calculated.

	a.logger.WithFields(logrus.Fields{
		"symbol":           analysis.Symbol,
		"volatility_risk":  volatilityRisk,
		"correlation_risk": correlationRisk,
		"volume_risk":      volumeRisk,
		"atr_risk":         atrRiskValue,
		"momentum_risk":    momentumRisk,
		"final_risk_score": normalizedRisk,
	}).Debug("Risk assessment completed")

	if normalizedRisk >= overallCfg.HighThreshold {
		return "high"
	} else if normalizedRisk >= overallCfg.MediumThreshold {
		return "medium"
	}
	return "low"
}

func (a *Analyzer) calculateMomentumRisk(priceData []models.PricePoint, momentumCfg models.MomentumRiskConfig) float64 {
	// Simple momentum risk calculation based on price data
	if len(priceData) < momentumCfg.MinDataPoints {
		return momentumCfg.DefaultScoreForSafe // Default risk for insufficient data
	}

	// Calculate short-term vs long-term price change
	// Ensure indices are within bounds
	recentEnd := momentumCfg.RecentPeriods
	if recentEnd > len(priceData) {
		recentEnd = len(priceData)
	}
	recent := priceData[:recentEnd]

	olderStart := momentumCfg.OlderPeriodsStart
	olderEnd := momentumCfg.OlderPeriodsEnd

	if olderStart >= len(priceData) || olderStart >= olderEnd || olderEnd > len(priceData) {
		// Not enough data for older period as configured, return default safe score
		return momentumCfg.DefaultScoreForSafe
	}
	older := priceData[olderStart:olderEnd]

	if len(recent) == 0 || len(older) == 0 { // Should not happen if MinDataPoints is reasonable
		return momentumCfg.DefaultScoreForSafe
	}

	recentAvg := 0.0
	for _, price := range recent {
		recentAvg += price.Close
	}
	recentAvg /= float64(len(recent))

	olderAvg := 0.0
	for _, price := range older {
		olderAvg += price.Close
	}
	olderAvg /= float64(len(older))

	if olderAvg > 0 {
		momentumChange := (recentAvg - olderAvg) / olderAvg
		absMomentum := math.Abs(momentumChange)

		if absMomentum > momentumCfg.ChangeBand1 {
			return momentumCfg.Score1 // High momentum = high risk
		} else if absMomentum > momentumCfg.ChangeBand2 {
			return momentumCfg.Score2 // Medium momentum = medium risk
		}
	}

	return momentumCfg.Score3 // Low momentum = low risk
}

func (a *Analyzer) SelectTopPairs(analyses []models.PairAnalysis, maxPairs int) []models.PairAnalysis {
	if len(analyses) <= maxPairs {
		return analyses
	}

	// Ensure diversity in risk levels
	lowRisk := []models.PairAnalysis{}
	mediumRisk := []models.PairAnalysis{}
	highRisk := []models.PairAnalysis{}

	for _, analysis := range analyses {
		switch analysis.RiskLevel {
		case "low":
			lowRisk = append(lowRisk, analysis)
		case "medium":
			mediumRisk = append(mediumRisk, analysis)
		case "high":
			highRisk = append(highRisk, analysis)
		}
	}

	selected := []models.PairAnalysis{}

	// Select pairs with balanced risk distribution
	lowCount := maxPairs / 3
	mediumCount := maxPairs / 3
	highCount := maxPairs - lowCount - mediumCount

	// Add low risk pairs
	for i := 0; i < lowCount && i < len(lowRisk); i++ {
		selected = append(selected, lowRisk[i])
	}

	// Add medium risk pairs
	for i := 0; i < mediumCount && i < len(mediumRisk); i++ {
		selected = append(selected, mediumRisk[i])
	}

	// Add high risk pairs
	for i := 0; i < highCount && i < len(highRisk); i++ {
		selected = append(selected, highRisk[i])
	}

	// Fill remaining slots with best scoring pairs
	remaining := maxPairs - len(selected)
	if remaining > 0 {
		for _, analysis := range analyses {
			if len(selected) >= maxPairs {
				break
			}

			// Check if already selected
			found := false
			for _, sel := range selected {
				if sel.Symbol == analysis.Symbol {
					found = true
					break
				}
			}

			if !found {
				selected = append(selected, analysis)
			}
		}
	}

	a.logger.WithFields(logrus.Fields{
		"total_analyzed": len(analyses),
		"selected_pairs": len(selected),
		"low_risk":       lowCount,
		"medium_risk":    mediumCount,
		"high_risk":      highCount,
	}).Info("Completed pair selection with risk distribution")

	return selected
}

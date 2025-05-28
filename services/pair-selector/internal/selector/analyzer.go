package selector

import (
	"context"
	"fmt"
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
	volatilityMetrics := a.volatilityAnalyzer.AnalyzeVolatility(priceHistory)
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
	analysis.RiskLevel = a.determineRiskLevel(analysis)

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

func (a *Analyzer) determineRiskLevel(analysis models.PairAnalysis) string {
	// Enhanced risk assessment with market regime detection
	riskScore := 0.0
	
	// Volatility component (35% weight)
	volatilityRisk := 0.0
	if analysis.Volatility > 0.12 {
		volatilityRisk = 4.0
	} else if analysis.Volatility > 0.08 {
		volatilityRisk = 3.0
	} else if analysis.Volatility > 0.05 {
		volatilityRisk = 2.0
	} else if analysis.Volatility > 0.03 {
		volatilityRisk = 1.5
	} else {
		volatilityRisk = 1.0
	}
	riskScore += volatilityRisk * 0.35
	
	// Correlation component (25% weight) - Low correlation increases risk
	correlationRisk := 0.0
	absCorrelation := math.Abs(analysis.CorrelationBTC)
	if absCorrelation < 0.2 {
		correlationRisk = 4.0 // Very uncorrelated = higher risk
	} else if absCorrelation < 0.4 {
		correlationRisk = 3.0
	} else if absCorrelation < 0.6 {
		correlationRisk = 2.0
	} else {
		correlationRisk = 1.0 // High correlation = lower risk
	}
	riskScore += correlationRisk * 0.25
	
	// Volume stability component (20% weight)
	volumeRisk := 0.0
	if analysis.Volume24hUSDT < 1000000 {
		volumeRisk = 4.0
	} else if analysis.Volume24hUSDT < 3000000 {
		volumeRisk = 3.0
	} else if analysis.Volume24hUSDT < 10000000 {
		volumeRisk = 2.0
	} else {
		volumeRisk = 1.0
	}
	riskScore += volumeRisk * 0.20
	
	// ATR/Volatility ratio component (10% weight) - High ATR relative to volatility indicates instability
	atrRisk := 0.0
	if analysis.Volatility > 0 {
		atrRatio := analysis.ATR14 / analysis.Volatility
		if atrRatio > 2.0 {
			atrRisk = 3.0
		} else if atrRatio > 1.5 {
			atrRisk = 2.0
		} else {
			atrRisk = 1.0
		}
	} else {
		atrRisk = 2.0
	}
	riskScore += atrRisk * 0.10
	
	// Price momentum component (10% weight) - Add momentum analysis
	momentumRisk := a.calculateMomentumRisk(analysis)
	riskScore += momentumRisk * 0.10
	
	// Normalize risk score (0-4 scale)
	normalizedRisk := riskScore / 4.0
	
	a.logger.WithFields(logrus.Fields{
		"symbol":          analysis.Symbol,
		"volatility_risk": volatilityRisk,
		"correlation_risk": correlationRisk,
		"volume_risk":     volumeRisk,
		"atr_risk":        atrRisk,
		"momentum_risk":   momentumRisk,
		"final_risk_score": normalizedRisk,
	}).Debug("Risk assessment completed")
	
	if normalizedRisk >= 0.75 {
		return "high"
	} else if normalizedRisk >= 0.5 {
		return "medium"
	}
	return "low"
}

func (a *Analyzer) calculateMomentumRisk(analysis models.PairAnalysis) float64 {
	// Simple momentum risk calculation based on price data
	if len(analysis.PriceData) < 10 {
		return 2.0 // Default medium risk for insufficient data
	}
	
	// Calculate short-term vs long-term price change
	recent := analysis.PriceData[:5]   // Last 5 periods
	older := analysis.PriceData[5:10]  // Previous 5 periods
	
	recentAvg := 0.0
	olderAvg := 0.0
	
	for _, price := range recent {
		recentAvg += price.Close
	}
	recentAvg /= float64(len(recent))
	
	for _, price := range older {
		olderAvg += price.Close
	}
	olderAvg /= float64(len(older))
	
	if olderAvg > 0 {
		momentumChange := (recentAvg - olderAvg) / olderAvg
		
		// High absolute momentum indicates higher risk
		absMomentum := math.Abs(momentumChange)
		if absMomentum > 0.1 {
			return 3.0 // High momentum = high risk
		} else if absMomentum > 0.05 {
			return 2.0 // Medium momentum = medium risk
		}
	}
	
	return 1.0 // Low momentum = low risk
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

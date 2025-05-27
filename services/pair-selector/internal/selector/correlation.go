package selector

import (
	"context"
	"fmt"

	"github.com/paaavkata/crypto-trading-bot-v4/pair-selector/internal/database"
	"github.com/paaavkata/crypto-trading-bot-v4/pair-selector/pkg/models"
	"github.com/paaavkata/crypto-trading-bot-v4/shared/pkg/utils"
	"github.com/sirupsen/logrus"
)

type CorrelationAnalyzer struct {
	repo   *database.Repository
	logger *logrus.Logger
}

type CorrelationMetrics struct {
	Correlation float64
	Strength    string
}

func NewCorrelationAnalyzer(repo *database.Repository, logger *logrus.Logger) *CorrelationAnalyzer {
	return &CorrelationAnalyzer{
		repo:   repo,
		logger: logger,
	}
}

func (c *CorrelationAnalyzer) AnalyzeCorrelation(ctx context.Context, symbol1, symbol2 string, hours int) (CorrelationMetrics, error) {
	// Get price history for both symbols
	prices1, err := c.repo.GetPriceHistory(ctx, symbol1, hours)
	if err != nil {
		return CorrelationMetrics{}, fmt.Errorf("failed to get price history for %s: %w", symbol1, err)
	}

	prices2, err := c.repo.GetPriceHistory(ctx, symbol2, hours)
	if err != nil {
		return CorrelationMetrics{}, fmt.Errorf("failed to get price history for %s: %w", symbol2, err)
	}

	// Align price data by timestamps
	aligned1, aligned2 := c.alignPriceData(prices1, prices2)

	if len(aligned1) < 10 || len(aligned2) < 10 {
		c.logger.WithFields(logrus.Fields{
			"symbol1": symbol1,
			"symbol2": symbol2,
			"points1": len(aligned1),
			"points2": len(aligned2),
		}).Warn("Insufficient data points for correlation analysis")
		return CorrelationMetrics{}, fmt.Errorf("insufficient data points")
	}

	// Calculate correlation coefficient
	correlation := utils.CalculateCorrelation(aligned1, aligned2)

	// Determine correlation strength
	strength := c.determineCorrelationStrength(correlation)

	return CorrelationMetrics{
		Correlation: correlation,
		Strength:    strength,
	}, nil
}

func (c *CorrelationAnalyzer) alignPriceData(prices1, prices2 []models.PricePoint) ([]float64, []float64) {
	// Create maps for quick lookup
	priceMap1 := make(map[int64]float64)
	priceMap2 := make(map[int64]float64)

	for _, price := range prices1 {
		timestamp := price.Timestamp.Unix()
		priceMap1[timestamp] = price.Close
	}

	for _, price := range prices2 {
		timestamp := price.Timestamp.Unix()
		priceMap2[timestamp] = price.Close
	}

	// Find common timestamps
	var aligned1, aligned2 []float64

	for timestamp, price1 := range priceMap1 {
		if price2, exists := priceMap2[timestamp]; exists {
			aligned1 = append(aligned1, price1)
			aligned2 = append(aligned2, price2)
		}
	}

	return aligned1, aligned2
}

func (c *CorrelationAnalyzer) determineCorrelationStrength(correlation float64) string {
	absCorr := correlation
	if correlation < 0 {
		absCorr = -correlation
	}

	if absCorr >= 0.8 {
		return "very_strong"
	} else if absCorr >= 0.6 {
		return "strong"
	} else if absCorr >= 0.4 {
		return "moderate"
	} else if absCorr >= 0.2 {
		return "weak"
	}
	return "very_weak"
}

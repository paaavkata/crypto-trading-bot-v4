package collector

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/paaavkata/crypto-trading-bot-v4/price-collector/internal/database"
	"github.com/paaavkata/crypto-trading-bot-v4/price-collector/pkg/models"
	"github.com/sirupsen/logrus"
)

type Processor struct {
	repo              *database.Repository
	logger            *logrus.Logger
	dataRetentionDays int
}

func NewProcessor(repo *database.Repository, logger *logrus.Logger, dataRetentionDays int) *Processor {
	return &Processor{
		repo:              repo,
		logger:            logger,
		dataRetentionDays: dataRetentionDays,
	}
}

func (p *Processor) ProcessTickers(ctx context.Context, tickers []models.TickerData) error {
	if len(tickers) == 0 {
		p.logger.Warn("No tickers to process")
		return nil
	}

	start := time.Now()

	// Convert ticker data to price data with normalization
	priceData := make([]models.PriceData, 0, len(tickers))
	symbols := make([]string, 0, len(tickers))
	normalizedCount := 0

	for _, ticker := range tickers {
		// Normalize data to fit database precision limits
		normalizedTicker := p.normalizePriceData(ticker)

		// Basic validation (just check for completely invalid data)
		if !p.isBasicDataValid(normalizedTicker) {
			p.logger.WithFields(logrus.Fields{
				"symbol": ticker.Symbol,
				"reason": "invalid after normalization",
			}).Debug("Skipping completely invalid data")
			continue
		}

		// Track if normalization occurred
		if p.wasNormalized(ticker, normalizedTicker) {
			normalizedCount++
			p.logger.WithFields(logrus.Fields{
				"symbol":     ticker.Symbol,
				"original":   p.formatOriginalData(ticker),
				"normalized": p.formatNormalizedData(normalizedTicker),
			}).Debug("Data normalized for database storage")
		}

		price := models.PriceData{
			Symbol:      normalizedTicker.Symbol,
			Timestamp:   normalizedTicker.Timestamp,
			Open:        normalizedTicker.Open,
			High:        normalizedTicker.High,
			Low:         normalizedTicker.Low,
			Close:       normalizedTicker.Close,
			Volume:      normalizedTicker.Volume,
			QuoteVolume: normalizedTicker.QuoteVolume,
			ChangeRate:  normalizedTicker.ChangeRate,
			ChangePrice: normalizedTicker.ChangePrice,
		}

		priceData = append(priceData, price)
		symbols = append(symbols, normalizedTicker.Symbol)
	}

	if normalizedCount > 0 {
		p.logger.WithField("normalized_count", normalizedCount).Info("Normalized price data for database storage")
	}

	// Bulk insert price data
	if len(priceData) > 0 {
		if err := p.repo.BulkInsertPriceData(ctx, priceData); err != nil {
			p.logger.WithError(err).Error("Failed to insert price data")
			return err
		}
	}

	duration := time.Since(start)
	p.logger.WithFields(logrus.Fields{
		"processed_count":  len(priceData),
		"normalized_count": normalizedCount,
		"duration_ms":      duration.Milliseconds(),
	}).Info("Successfully processed tickers")

	return nil
}
func (p *Processor) normalizePriceData(ticker models.TickerData) models.TickerData {
	return models.TickerData{
		Symbol:      ticker.Symbol,
		Timestamp:   ticker.Timestamp,
		Open:        p.normalizePriceField(ticker.Open),
		High:        p.normalizePriceField(ticker.High),
		Low:         p.normalizePriceField(ticker.Low),
		Close:       p.normalizePriceField(ticker.Close),
		Volume:      p.normalizeVolumeField(ticker.Volume),
		QuoteVolume: p.normalizeVolumeField(ticker.QuoteVolume),
		ChangeRate:  p.normalizeChangeRateField(ticker.ChangeRate),
		ChangePrice: p.normalizePriceField(ticker.ChangePrice),
	}
}

// Normalize price fields to fit DECIMAL(20,8) - 20 total digits, 8 after decimal
func (p *Processor) normalizePriceField(value float64) float64 {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0.0
	}

	// Handle negative values
	sign := 1.0
	if value < 0 {
		sign = -1.0
		value = -value
	}

	// DECIMAL(20,8) means max 12 digits before decimal, 8 after
	// Max value: 999999999999.99999999
	const maxValue = 999999999999.0
	const precision = 8

	if value > maxValue {
		p.logger.WithFields(logrus.Fields{
			"original_value": value * sign,
			"capped_value":   maxValue * sign,
		}).Debug("Price value capped to maximum")
		return maxValue * sign
	}

	// Round to 8 decimal places
	multiplier := math.Pow(10, precision)
	return math.Round(value*multiplier) / multiplier * sign
}

// Normalize volume fields to fit DECIMAL(20,8) - 20 total digits, 8 after decimal
func (p *Processor) normalizeVolumeField(value float64) float64 {
	if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 {
		return 0.0
	}

	// DECIMAL(20,8) means max 12 digits before decimal, 8 after
	// Max value: 999999999999.99999999
	const maxValue = 999999999999.0
	const precision = 8

	if value > maxValue {
		p.logger.WithFields(logrus.Fields{
			"original_value": value,
			"capped_value":   maxValue,
		}).Debug("Volume value capped to maximum")
		return maxValue
	}

	// Round to 8 decimal places
	multiplier := math.Pow(10, precision)
	return math.Round(value*multiplier) / multiplier
}

// Normalize change rate to fit DECIMAL(10,6) - 10 total digits, 6 after decimal
func (p *Processor) normalizeChangeRateField(value float64) float64 {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0.0
	}

	// Handle negative values
	sign := 1.0
	if value < 0 {
		sign = -1.0
		value = -value
	}

	// DECIMAL(10,6) means max 4 digits before decimal, 6 after
	// Max value: 9999.999999
	const maxValue = 9999.0
	const precision = 6

	if value > maxValue {
		p.logger.WithFields(logrus.Fields{
			"original_value": value * sign,
			"capped_value":   maxValue * sign,
		}).Debug("Change rate capped to maximum")
		return maxValue * sign
	}

	// Round to 6 decimal places
	multiplier := math.Pow(10, precision)
	return math.Round(value*multiplier) / multiplier * sign
}

// Basic validation after normalization - only reject completely invalid data
func (p *Processor) isBasicDataValid(ticker models.TickerData) bool {
	// Only reject data that's completely unusable
	if ticker.Open <= 0 || ticker.High <= 0 || ticker.Low <= 0 || ticker.Close <= 0 {
		return false
	}

	// Basic logic checks
	if ticker.High < ticker.Low {
		return false
	}

	if ticker.Close > ticker.High || ticker.Close < ticker.Low {
		return false
	}

	if ticker.Open > ticker.High || ticker.Open < ticker.Low {
		return false
	}

	return true
}

// Check if normalization occurred
func (p *Processor) wasNormalized(original, normalized models.TickerData) bool {
	tolerance := 1e-12 // Very small tolerance for floating point comparison

	return math.Abs(original.Open-normalized.Open) > tolerance ||
		math.Abs(original.High-normalized.High) > tolerance ||
		math.Abs(original.Low-normalized.Low) > tolerance ||
		math.Abs(original.Close-normalized.Close) > tolerance ||
		math.Abs(original.Volume-normalized.Volume) > tolerance ||
		math.Abs(original.QuoteVolume-normalized.QuoteVolume) > tolerance ||
		math.Abs(original.ChangeRate-normalized.ChangeRate) > tolerance ||
		math.Abs(original.ChangePrice-normalized.ChangePrice) > tolerance
}

// Helper functions for logging
func (p *Processor) formatOriginalData(ticker models.TickerData) string {
	return fmt.Sprintf("O:%.8f H:%.8f L:%.8f C:%.8f V:%.8f QV:%.8f CR:%.6f CP:%.8f",
		ticker.Open, ticker.High, ticker.Low, ticker.Close,
		ticker.Volume, ticker.QuoteVolume, ticker.ChangeRate, ticker.ChangePrice)
}

func (p *Processor) formatNormalizedData(ticker models.TickerData) string {
	return fmt.Sprintf("O:%.8f H:%.8f L:%.8f C:%.8f V:%.8f QV:%.8f CR:%.6f CP:%.8f",
		ticker.Open, ticker.High, ticker.Low, ticker.Close,
		ticker.Volume, ticker.QuoteVolume, ticker.ChangeRate, ticker.ChangePrice)
}

func (p *Processor) CleanupOldData(ctx context.Context) error {
	p.logger.WithField("retention_days", p.dataRetentionDays).Info("Starting cleanup of old price data")

	if err := p.repo.CleanupOldData(ctx, p.dataRetentionDays); err != nil {
		p.logger.WithError(err).Error("Failed to cleanup old data")
		return err
	}

	p.logger.Info("Successfully cleaned up old price data")
	return nil
}

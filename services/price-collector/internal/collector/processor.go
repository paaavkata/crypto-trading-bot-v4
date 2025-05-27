package collector

import (
	"context"
	"time"

	"github.com/paaavkata/crypto-trading-bot-v4/price-collector/internal/database"
	"github.com/paaavkata/crypto-trading-bot-v4/price-collector/pkg/models"
	"github.com/sirupsen/logrus"
)

type Processor struct {
	repo   *database.Repository
	logger *logrus.Logger
}

func NewProcessor(repo *database.Repository, logger *logrus.Logger) *Processor {
	return &Processor{
		repo:   repo,
		logger: logger,
	}
}

func (p *Processor) ProcessTickers(ctx context.Context, tickers []models.TickerData) error {
	if len(tickers) == 0 {
		p.logger.Warn("No tickers to process")
		return nil
	}

	start := time.Now()

	// Convert ticker data to price data
	priceData := make([]models.PriceData, 0, len(tickers))
	symbols := make([]string, 0, len(tickers))

	for _, ticker := range tickers {
		price := models.PriceData{
			Symbol:      ticker.Symbol,
			Timestamp:   ticker.Timestamp,
			Open:        ticker.Open,
			High:        ticker.High,
			Low:         ticker.Low,
			Close:       ticker.Close,
			Volume:      ticker.Volume,
			QuoteVolume: ticker.QuoteVolume,
			ChangeRate:  ticker.ChangeRate,
			ChangePrice: ticker.ChangePrice,
		}

		priceData = append(priceData, price)
		symbols = append(symbols, ticker.Symbol)
	}

	// Bulk insert price data
	if err := p.repo.BulkInsertPriceData(ctx, priceData); err != nil {
		p.logger.WithError(err).Error("Failed to insert price data")
		return err
	}

	// Update trading pairs
	if err := p.repo.UpdateTradingPairs(ctx, symbols); err != nil {
		p.logger.WithError(err).Error("Failed to update trading pairs")
		return err
	}

	duration := time.Since(start)
	p.logger.WithFields(logrus.Fields{
		"processed_count": len(priceData),
		"duration_ms":     duration.Milliseconds(),
	}).Info("Successfully processed tickers")

	return nil
}

func (p *Processor) CleanupOldData(ctx context.Context) error {
	// Keep data for 30 days
	const retentionDays = 30

	p.logger.Info("Starting cleanup of old price data")

	if err := p.repo.CleanupOldData(ctx, retentionDays); err != nil {
		p.logger.WithError(err).Error("Failed to cleanup old data")
		return err
	}

	p.logger.Info("Successfully cleaned up old price data")
	return nil
}

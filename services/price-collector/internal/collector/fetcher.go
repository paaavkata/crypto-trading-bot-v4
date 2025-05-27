package collector

import (
	"context"
	"fmt"
	"time"

	"github.com/paaavkata/crypto-trading-bot-v4/price-collector/pkg/models"
	"github.com/paaavkata/crypto-trading-bot-v4/shared/pkg/kucoin"
	"github.com/paaavkata/crypto-trading-bot-v4/shared/pkg/utils"
	"github.com/sirupsen/logrus"
)

type Fetcher struct {
	client      *kucoin.Client
	rateLimiter *kucoin.RateLimiter
	logger      *logrus.Logger
}

func NewFetcher(client *kucoin.Client, logger *logrus.Logger) *Fetcher {
	// KuCoin allows 1800 requests per minute for public endpoints (30 per second)
	rateLimiter := kucoin.NewRateLimiter(25) // Conservative rate limiting

	return &Fetcher{
		client:      client,
		rateLimiter: rateLimiter,
		logger:      logger,
	}
}

func (f *Fetcher) FetchAllTickers(ctx context.Context) ([]models.TickerData, error) {
	f.rateLimiter.Wait()

	start := time.Now()
	tickersResp, err := f.client.GetAllTickers()
	if err != nil {
		f.logger.WithError(err).Error("Failed to fetch tickers from KuCoin")
		return nil, fmt.Errorf("failed to fetch tickers: %w", err)
	}

	timestamp := time.Now().Truncate(time.Minute) // Round to nearest minute
	tickers := make([]models.TickerData, 0, len(tickersResp.Ticker))

	for _, ticker := range tickersResp.Ticker {
		// Parse numeric values
		open, err := utils.ParseFloat(ticker.Last) // Use last price as open for minute data
		if err != nil {
			f.logger.WithField("symbol", ticker.Symbol).WithError(err).Warn("Failed to parse open price")
			continue
		}

		high, err := utils.ParseFloat(ticker.High)
		if err != nil {
			f.logger.WithField("symbol", ticker.Symbol).WithError(err).Warn("Failed to parse high price")
			continue
		}

		low, err := utils.ParseFloat(ticker.Low)
		if err != nil {
			f.logger.WithField("symbol", ticker.Symbol).WithError(err).Warn("Failed to parse low price")
			continue
		}

		close, err := utils.ParseFloat(ticker.Last)
		if err != nil {
			f.logger.WithField("symbol", ticker.Symbol).WithError(err).Warn("Failed to parse close price")
			continue
		}

		volume, err := utils.ParseFloat(ticker.Vol)
		if err != nil {
			f.logger.WithField("symbol", ticker.Symbol).WithError(err).Warn("Failed to parse volume")
			continue
		}

		quoteVolume, err := utils.ParseFloat(ticker.VolValue)
		if err != nil {
			f.logger.WithField("symbol", ticker.Symbol).WithError(err).Warn("Failed to parse quote volume")
			continue
		}

		changeRate, err := utils.ParseFloat(ticker.ChangeRate)
		if err != nil {
			f.logger.WithField("symbol", ticker.Symbol).WithError(err).Warn("Failed to parse change rate")
			continue
		}

		changePrice, err := utils.ParseFloat(ticker.ChangePrice)
		if err != nil {
			f.logger.WithField("symbol", ticker.Symbol).WithError(err).Warn("Failed to parse change price")
			continue
		}

		// Skip pairs with zero volume or invalid prices
		if volume <= 0 || quoteVolume <= 0 || close <= 0 {
			continue
		}

		tickerData := models.TickerData{
			Symbol:      ticker.Symbol,
			Open:        open,
			High:        high,
			Low:         low,
			Close:       close,
			Volume:      volume,
			QuoteVolume: quoteVolume,
			ChangeRate:  changeRate,
			ChangePrice: changePrice,
			Timestamp:   timestamp,
		}

		tickers = append(tickers, tickerData)
	}

	duration := time.Since(start)
	f.logger.WithFields(logrus.Fields{
		"total_tickers": len(tickersResp.Ticker),
		"valid_tickers": len(tickers),
		"duration_ms":   duration.Milliseconds(),
		"timestamp":     timestamp,
	}).Info("Successfully fetched and processed tickers")

	return tickers, nil
}

func (f *Fetcher) FetchSymbols(ctx context.Context) ([]string, error) {
	f.rateLimiter.Wait()

	symbols, err := f.client.GetSymbols()
	if err != nil {
		f.logger.WithError(err).Error("Failed to fetch symbols from KuCoin")
		return nil, fmt.Errorf("failed to fetch symbols: %w", err)
	}

	symbolList := make([]string, 0, len(symbols))
	for _, symbol := range symbols {
		if symbol.EnableTrading {
			symbolList = append(symbolList, symbol.Symbol)
		}
	}

	f.logger.WithField("symbols_count", len(symbolList)).Info("Successfully fetched trading symbols")
	return symbolList, nil
}

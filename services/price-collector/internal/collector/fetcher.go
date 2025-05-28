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
	parseErrors := 0

	for _, ticker := range tickersResp.Ticker {
		tickerData, err := f.parseTickerData(ticker, timestamp)
		if err != nil {
			f.logger.WithFields(logrus.Fields{
				"symbol": ticker.Symbol,
				"error":  err.Error(),
			}).Debug("Failed to parse ticker data")
			parseErrors++
			continue
		}

		tickers = append(tickers, *tickerData)
	}

	duration := time.Since(start)
	f.logger.WithFields(logrus.Fields{
		"total_tickers": len(tickersResp.Ticker),
		"valid_tickers": len(tickers),
		"parse_errors":  parseErrors,
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

func (f *Fetcher) parseTickerData(ticker kucoin.Ticker, timestamp time.Time) (*models.TickerData, error) {
	// Parse values - allow more flexibility, normalization will handle precision
	open, err := f.parseFloatSafe(ticker.Last, "open")
	if err != nil {
		return nil, err
	}

	high, err := f.parseFloatSafe(ticker.High, "high")
	if err != nil {
		return nil, err
	}

	low, err := f.parseFloatSafe(ticker.Low, "low")
	if err != nil {
		return nil, err
	}

	close, err := f.parseFloatSafe(ticker.Last, "close")
	if err != nil {
		return nil, err
	}

	volume, err := f.parseFloatSafe(ticker.Vol, "volume")
	if err != nil {
		return nil, err
	}

	quoteVolume, err := f.parseFloatSafe(ticker.VolValue, "quote_volume")
	if err != nil {
		return nil, err
	}

	changeRate, err := f.parseFloatSafe(ticker.ChangeRate, "change_rate")
	if err != nil {
		return nil, err
	}

	changePrice, err := f.parseFloatSafe(ticker.ChangePrice, "change_price")
	if err != nil {
		return nil, err
	}

	// Only reject completely invalid data - let normalization handle the rest
	if close <= 0 || high <= 0 || low <= 0 || open <= 0 {
		return nil, fmt.Errorf("invalid prices: open=%.12f, high=%.12f, low=%.12f, close=%.12f",
			open, high, low, close)
	}

	return &models.TickerData{
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
	}, nil
}

func (f *Fetcher) parseFloatSafe(value, fieldName string) (float64, error) {
	if value == "" {
		return 0, nil
	}

	parsed, err := utils.ParseFloat(value)
	if err != nil {
		return 0, fmt.Errorf("failed to parse %s '%s': %w", fieldName, value, err)
	}

	// Allow NaN and Inf here - normalization will handle them
	return parsed, nil
}

package signals

import (
	"context"
	"fmt"
	"time"

	"github.com/paaavkata/crypto-trading-bot-v4/trading-engine/internal/database"
	"github.com/paaavkata/crypto-trading-bot-v4/trading-engine/pkg/models"
	"github.com/sirupsen/logrus"
)

const (
	shortMAPeriod = 10
	longMAPeriod  = 20 // ensure this is less than dataFetchLimit - 1 for crossover check
	// Fetch enough data for the longest MA period plus one previous point for crossover comparison,
	// and a few extra for buffer/stability if data is sparse.
	dataFetchLimit = longMAPeriod + 5
)

type Generator struct {
	logger *logrus.Logger
	repo   *database.Repository
}

func NewGenerator(logger *logrus.Logger, repo *database.Repository) *Generator {
	return &Generator{logger: logger, repo: repo}
}

// calculateSMA calculates the Simple Moving Average for a series of prices.
func (g *Generator) calculateSMA(prices []float64, period int) []float64 {
	if len(prices) < period {
		return nil // Not enough data
	}

	smaValues := make([]float64, len(prices)-period+1)
	for i := 0; i <= len(prices)-period; i++ {
		sum := 0.0
		for j := i; j < i+period; j++ {
			sum += prices[j]
		}
		smaValues[i] = sum / float64(period)
	}
	return smaValues
}

func (g *Generator) GenerateSignal(ctx context.Context, symbol string, currentPrice float64) models.Signal {
	action := "HOLD"
	strength := 1.0 // Default strength
	reason := "No clear signal"

	// Fetch historical price data
	// Price data is returned oldest first by the repository modification
	priceData, err := g.repo.GetPriceDataForSymbol(ctx, symbol, dataFetchLimit)
	if err != nil {
		g.logger.WithError(err).WithField("symbol", symbol).Error("Failed to fetch price data for MA signal")
		return models.Signal{
			Symbol:    symbol,
			Action:    "HOLD",
			Price:     currentPrice,
			Strength:  0.0, // Indicate error or inability to generate signal
			Timestamp: time.Now(),
			Reason:    fmt.Sprintf("Error fetching price data: %v", err),
		}
	}

	if len(priceData) < longMAPeriod {
		g.logger.WithFields(logrus.Fields{
			"symbol":        symbol,
			"data_points":   len(priceData),
			"required_long": longMAPeriod,
		}).Warn("Insufficient data points for long MA calculation")
		return models.Signal{
			Symbol:    symbol,
			Action:    "HOLD",
			Price:     currentPrice,
			Strength:  0.0,
			Timestamp: time.Now(),
			Reason:    fmt.Sprintf("Insufficient data for MA(%d): got %d points", longMAPeriod, len(priceData)),
		}
	}

	closePrices := make([]float64, len(priceData))
	for i, pd := range priceData {
		closePrices[i] = pd.Close
	}

	// Calculate SMAs
	// calculateSMA returns values aligned with the end of the period,
	// so smaValues[0] is the SMA for prices[0]...prices[period-1]
	// The last value in smaValues will be the most recent SMA.
	shortSMAValues := g.calculateSMA(closePrices, shortMAPeriod)
	longSMAValues := g.calculateSMA(closePrices, longMAPeriod)

	if shortSMAValues == nil || longSMAValues == nil {
		g.logger.WithField("symbol", symbol).Warn("SMA calculation resulted in nil, likely due to insufficient filtered data")
		return models.Signal{Symbol: symbol, Action: "HOLD", Price: currentPrice, Strength: 0.0, Timestamp: time.Now(), Reason: "SMA calculation error"}
	}
	
	// We need at least two points from the shorter SMA series to detect a crossover.
	// The longSMAValues will be shorter than shortSMAValues if shortMAPeriod < longMAPeriod.
	// We need to align them from the end of the series.
	lenShortSMA := len(shortSMAValues)
	lenLongSMA := len(longSMAValues)

	if lenShortSMA < 2 || lenLongSMA < 2 {
		g.logger.WithFields(logrus.Fields{
			"symbol":         symbol,
			"len_short_sma":  lenShortSMA,
			"len_long_sma":   lenLongSMA,
		}).Warn("Insufficient SMA values for crossover detection")
        return models.Signal{Symbol: symbol, Action: "HOLD", Price: currentPrice, Strength: 0.0, Timestamp: time.Now(), Reason: "Insufficient SMA values for crossover"}
	}

	// Get the most recent two SMA values for crossover detection
	// The last element is current, second to last is previous.
	currentShortSMA := shortSMAValues[lenShortSMA-1]
	previousShortSMA := shortSMAValues[lenShortSMA-2]

	// Align long SMA values with short SMA values from the end
	currentLongSMA := longSMAValues[lenLongSMA-1]
	previousLongSMA := longSMAValues[lenLongSMA-2]


	g.logger.WithFields(logrus.Fields{
		"symbol":             symbol,
		"current_short_sma":  currentShortSMA,
		"previous_short_sma": previousShortSMA,
		"current_long_sma":   currentLongSMA,
		"previous_long_sma":  previousLongSMA,
		"current_price":      currentPrice,
	}).Debug("SMA values for crossover check")


	// Check for BUY signal (short SMA crosses above long SMA)
	if currentShortSMA > currentLongSMA && previousShortSMA <= previousLongSMA {
		action = "BUY"
		reason = fmt.Sprintf("SMA(%d) crossed above SMA(%d)", shortMAPeriod, longMAPeriod)
		g.logger.WithFields(logrus.Fields{"symbol": symbol, "reason": reason, "price": currentPrice}).Info("MA Crossover BUY Signal")
	}

	// Check for SELL signal (short SMA crosses below long SMA)
	if currentShortSMA < currentLongSMA && previousShortSMA >= previousLongSMA {
		action = "SELL"
		reason = fmt.Sprintf("SMA(%d) crossed below SMA(%d)", shortMAPeriod, longMAPeriod)
		g.logger.WithFields(logrus.Fields{"symbol": symbol, "reason": reason, "price": currentPrice}).Info("MA Crossover SELL Signal")
	}
	
	if action == "HOLD" {
		reason = fmt.Sprintf("No SMA(%d)/SMA(%d) crossover", shortMAPeriod, longMAPeriod)
	}


	return models.Signal{
		Symbol:    symbol,
		Action:    action,
		Price:     currentPrice, // current market price, not used for signal generation directly but for order
		Strength:  strength,
		Timestamp: time.Now(),
		Reason:    reason,
	}
}

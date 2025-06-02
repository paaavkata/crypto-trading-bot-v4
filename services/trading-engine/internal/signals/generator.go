// Package signals provides functionalities for generating trading signals based on technical analysis
// and determining overall market conditions.
package signals

/*
Backtesting Considerations:
A comprehensive backtesting framework for this signal generator would involve:
1. Data Source: Utilizing the same historical price data (e.g., from the 'price_data' table via
   the database.Repository or a dedicated data access layer for backtesting) that the live
   engine uses. This ensures consistency.
2. Time Iteration: Simulating the progression of time by feeding historical data candles
   (Open, High, Low, Close, Volume, Timestamp) one by one or in batches to the
   GenerateSignal method for each trading pair.
3. Parameter Optimization: The configurable parameters within the Generator (indicator periods,
   thresholds, weights) are designed to be tuned. Backtesting is essential for finding optimal
   parameter sets for different assets or market conditions, potentially using techniques like
   grid search or evolutionary algorithms.
4. Performance Metrics: Calculating standard trading performance metrics such as Total Profit/Loss,
   Sharpe Ratio, Sortino Ratio, Win/Loss Ratio, Average Win/Loss, Max Drawdown, and
   trade frequency.
5. Trade Execution Simulation: For more realistic results, the backtester should simulate
   trade execution, including order types, potential slippage (difference between expected
   and actual fill price), and trading fees.
6. Decoupled Engine: This Generator can be used as a core component within a separate
   backtesting engine/package. The backtesting engine would manage the data feed,
   simulation loop, parameter iteration, trade simulation, and metric calculation,
   while relying on this Generator for producing trading signals.
*/

import (
	"context"
	"fmt"
	// "io" // Removed unused import
	"math"
	"time" // Added import for time package

	"github.com/markcheno/go-talib" // Added go-talib import
	// "github.com/paaavkata/crypto-trading-bot-v4/trading-engine/internal/database" // Will be replaced by interface
	"github.com/paaavkata/crypto-trading-bot-v4/trading-engine/pkg/models" // Corrected import path
	"github.com/sirupsen/logrus"
)

// PriceHistoryProvider defines the interface for fetching price history.
// This allows for mocking in tests and decouples Generator from a concrete database.Repository or other data sources.
type PriceHistoryProvider interface {
	GetPriceHistory(ctx context.Context, symbol string, windowMinutes int) ([]models.PricePoint, error)
	// Add other methods from a data repository that Generator might need directly in the future.
}

// Generator is responsible for generating trading signals and analyzing market conditions
// based on technical indicators and configurable parameters.
type Generator struct {
	logger *logrus.Logger       // logger for logging messages.
	repo   PriceHistoryProvider // repo provides access to price history data.

	// Indicator Periods
	rsiPeriod        int // rsiPeriod is the lookback period for RSI calculation (e.g., 14).
	macdFastPeriod   int // macdFastPeriod is the fast period for MACD calculation (e.g., 12).
	macdSlowPeriod   int // macdSlowPeriod is the slow period for MACD calculation (e.g., 26).
	macdSignalPeriod int // macdSignalPeriod is the signal period for MACD calculation (e.g., 9).
	emaShortPeriod   int // emaShortPeriod is the lookback period for the shorter EMA (e.g., 20).
	emaLongPeriod    int // emaLongPeriod is the lookback period for the longer EMA (e.g., 50).
	volumeAvgPeriod  int // volumeAvgPeriod is the lookback period for calculating average volume (e.g., 20).

	// Data Fetching Parameters
	priceHistoryCandles     int // priceHistoryCandles specifies the number of historical candles to fetch for indicator calculation.
	priceDataIntervalMinutes int // priceDataIntervalMinutes is the interval of the price data in minutes (e.g., 60 for 1-hour candles).

	// Thresholds for Signal Logic
	rsiOversold             float64 // rsiOversold is the RSI level below which an asset is considered oversold.
	rsiOverbought           float64 // rsiOverbought is the RSI level above which an asset is considered overbought.
	volumeConfirmationRatio float64 // volumeConfirmationRatio is the ratio of current volume to average volume to consider it a strong confirmation.
	buyThreshold            float64 // buyThreshold is the minimum signal score required to generate a BUY signal.
	sellThreshold           float64 // sellThreshold is the maximum signal score (typically negative) to generate a SELL signal.

	// Weights for Signal Scoring
	rsiWeight           float64 // rsiWeight determines the contribution of the RSI indicator to the overall signal score.
	macdWeight          float64 // macdWeight determines the contribution of the MACD crossover to the signal score.
	emaTrendWeight      float64 // emaTrendWeight determines the contribution of the EMA trend to the signal score.
	volumeConfirmWeight float64 // volumeConfirmWeight determines the contribution of volume confirmation to the signal score.

	// Market Analysis Parameters
	marketDominanceFactor float64 // marketDominanceFactor is used in AnalyzeMarketConditions to determine if bullish or bearish sentiment is dominant.
}

// TechnicalIndicators holds the calculated values of various technical indicators for a specific asset.
type TechnicalIndicators struct {
	RSI          float64 // Relative Strength Index value.
	MACD         float64 // MACD line value.
	MACDSignal   float64 // MACD signal line value.
	BollingerUp  float64 // Upper Bollinger Band value (currently not calculated).
	BollingerLow float64 // Lower Bollinger Band value (currently not calculated).
	EMA20        float64 // 20-period Exponential Moving Average value (short-term).
	EMA50        float64 // 50-period Exponential Moving Average value (long-term).
	Volume       float64 // Latest trading volume.
	AvgVolume    float64 // Average trading volume over a defined period.
}

// NewGenerator creates and configures a new Generator instance.
// It initializes the Generator with the provided logger, data repository, and various
// trading parameters. If any of the numeric parameters (periods, thresholds, weights, etc.)
// are passed as their zero-value (0 for int, 0.0 for float64), they will be set to
// sensible default values within the constructor.
func NewGenerator(
	logger *logrus.Logger,
	repo PriceHistoryProvider,
	rsiPeriod int, macdFastPeriod int, macdSlowPeriod int, macdSignalPeriod int,
	emaShortPeriod int, emaLongPeriod int, volumeAvgPeriod int,
	priceHistoryCandles int, priceDataIntervalMinutes int,
	rsiOversold float64, rsiOverbought float64,
	volumeConfirmationRatio float64,
	buyThreshold float64, sellThreshold float64,
	rsiWeight float64, macdWeight float64, emaTrendWeight float64, volumeConfirmWeight float64,
	marketDominanceFactor float64,
) *Generator {
	gen := &Generator{
		logger:                  logger,
		repo:                    repo,
		rsiPeriod:               14,
		macdFastPeriod:          12,
		macdSlowPeriod:          26,
		macdSignalPeriod:        9,
		emaShortPeriod:          20,
		emaLongPeriod:           50,
		volumeAvgPeriod:         20,
		priceHistoryCandles:     100,
		priceDataIntervalMinutes: 60,
		rsiOversold:             30.0,
		rsiOverbought:           70.0,
		volumeConfirmationRatio: 1.5,
		buyThreshold:            0.3,
		sellThreshold:           -0.3,
		rsiWeight:               0.3,
		macdWeight:              0.25,
		emaTrendWeight:          0.25,
		volumeConfirmWeight:     0.20,
		marketDominanceFactor:   1.5,
	}

	if rsiPeriod != 0 {
		gen.rsiPeriod = rsiPeriod
	}
	if macdFastPeriod != 0 {
		gen.macdFastPeriod = macdFastPeriod
	}
	if macdSlowPeriod != 0 {
		gen.macdSlowPeriod = macdSlowPeriod
	}
	if macdSignalPeriod != 0 {
		gen.macdSignalPeriod = macdSignalPeriod
	}
	if emaShortPeriod != 0 {
		gen.emaShortPeriod = emaShortPeriod
	}
	if emaLongPeriod != 0 {
		gen.emaLongPeriod = emaLongPeriod
	}
	if volumeAvgPeriod != 0 {
		gen.volumeAvgPeriod = volumeAvgPeriod
	}
	if priceHistoryCandles != 0 {
		gen.priceHistoryCandles = priceHistoryCandles
	}
	if priceDataIntervalMinutes != 0 {
		gen.priceDataIntervalMinutes = priceDataIntervalMinutes
	}
	if rsiOversold != 0.0 {
		gen.rsiOversold = rsiOversold
	}
	if rsiOverbought != 0.0 {
		gen.rsiOverbought = rsiOverbought
	}
	if volumeConfirmationRatio != 0.0 {
		gen.volumeConfirmationRatio = volumeConfirmationRatio
	}
	if buyThreshold != 0.0 {
		gen.buyThreshold = buyThreshold
	}
	if sellThreshold != 0.0 { // Check for non-zero, as it's typically negative
		gen.sellThreshold = sellThreshold
	}
	if rsiWeight != 0.0 {
		gen.rsiWeight = rsiWeight
	}
	if macdWeight != 0.0 {
		gen.macdWeight = macdWeight
	}
	if emaTrendWeight != 0.0 {
		gen.emaTrendWeight = emaTrendWeight
	}
	if volumeConfirmWeight != 0.0 {
		gen.volumeConfirmWeight = volumeConfirmWeight
	}
	if marketDominanceFactor != 0.0 {
		gen.marketDominanceFactor = marketDominanceFactor
	}

	return gen
}

func (g *Generator) GenerateSignal(ctx context.Context, symbol string, currentPrice float64) models.Signal {
	// ************************************************************************************
	// ** WARNING: THIS SIGNAL GENERATOR IS A PLACEHOLDER AND USES SIMULATED DATA!       **
	// ** IT IS FOR DEMONSTRATION PURPOSES ONLY AND MUST NOT BE USED FOR LIVE TRADING.   **
	// ** Real historical data and proper technical indicator libraries are required     **
	// ** for any production use.                                                        **
	// ************************************************************************************

	// In a real implementation, you would fetch historical price data here
	// For now, we'll use simplified logic with some basic technical analysis concepts

	action := "HOLD"
	strength := 0.5
	reason := "neutral market conditions"

	// Attempt to calculate technical indicators for the given symbol.
	indicators, err := g.CalculateTechnicalIndicators(ctx, symbol)
	if err != nil {
		// Log the error and return a default HOLD signal if indicators cannot be calculated.
		g.logger.WithError(err).Errorf("Error calculating indicators for %s, returning HOLD signal", symbol)
		return models.Signal{
			Symbol:    symbol,
			Action:    "HOLD",
			Price:     currentPrice,
			Strength:  0.1,
			Timestamp: g.getCurrentTime(),
			Reason:    "Error calculating indicators",
		}
	}

	// Multi-factor signal generation
	signalScore := 0.0
	factors := []string{}

	// --- RSI Analysis ---
	// Check if RSI indicates oversold or overbought conditions.
	// Non-zero RSI is checked to avoid acting on potentially uninitialized default values.
	if indicators.RSI < g.rsiOversold && indicators.RSI != 0 {
		signalScore += g.rsiWeight
		factors = append(factors, fmt.Sprintf("oversold RSI (<%.1f)", g.rsiOversold))
	} else if indicators.RSI > g.rsiOverbought { // No need to check RSI != 0 here as overbought is usually a positive value.
		signalScore -= g.rsiWeight
		factors = append(factors, fmt.Sprintf("overbought RSI (>%.1f)", g.rsiOverbought))
	}

	// --- MACD Analysis ---
	// Check for bullish (MACD line crosses above signal line) or bearish (MACD line crosses below signal line) crossovers.
	// Non-zero values are checked to avoid issues with uninitialized data.
	if indicators.MACD > indicators.MACDSignal && indicators.MACD != 0 && indicators.MACDSignal != 0 {
		signalScore += g.macdWeight
		factors = append(factors, "bullish MACD crossover")
	} else if indicators.MACD < indicators.MACDSignal && indicators.MACD != 0 && indicators.MACDSignal != 0 {
		signalScore -= g.macdWeight
		factors = append(factors, "bearish MACD crossover")
	}

	// --- EMA Trend Analysis ---
	// Determine trend direction based on the relationship between short-term and long-term EMAs.
	// Non-zero values ensure EMAs were actually calculated.
	if indicators.EMA20 > indicators.EMA50 && indicators.EMA20 != 0 && indicators.EMA50 != 0 {
		signalScore += g.emaTrendWeight
		factors = append(factors, "bullish EMA trend (EMA20 > EMA50)")
	} else if indicators.EMA20 < indicators.EMA50 && indicators.EMA20 != 0 && indicators.EMA50 != 0 {
		signalScore -= g.emaTrendWeight
		factors = append(factors, "bearish EMA trend (EMA20 < EMA50)")
	}

	// --- Volume Confirmation ---
	// Check if current volume is significantly higher than average volume, potentially confirming the signal's strength.
	// Requires AvgVolume to be positive to avoid division by zero.
	if indicators.AvgVolume > 0 {
		volumeRatio := indicators.Volume / indicators.AvgVolume
		if volumeRatio > g.volumeConfirmationRatio {
			// Only apply volume confirmation if there's already a directional bias.
			if signalScore > 0.05 { 
				signalScore += g.volumeConfirmWeight
				factors = append(factors, "high volume confirmation")
			} else if signalScore < -0.05 {
				signalScore -= g.volumeConfirmWeight
				factors = append(factors, "high volume confirmation")
			}
		}
	}

	// --- Determine Action and Strength based on Signal Score ---
	action = "HOLD" // Default action
	strength = 0.5  // Default strength for HOLD signals or when score is near zero.

	if signalScore > g.buyThreshold {
		action = "BUY"
		// Strength for BUY signals, scales from 0.5 up to 1.0.
		// The scaling factor (0.5 here) determines how quickly strength increases with score.
		strength = math.Min(0.5+(signalScore-g.buyThreshold)*0.5, 1.0)
	} else if signalScore < g.sellThreshold {
		action = "SELL"
		// Strength for SELL signals, similar scaling.
		strength = math.Min(0.5+(math.Abs(signalScore)-math.Abs(g.sellThreshold))*0.5, 1.0)
	}

	// Build a descriptive reason string for the signal.
	if len(factors) > 0 {
		reason = fmt.Sprintf("Technical analysis: %v. Score: %.2f", factors, signalScore)
	} else if action == "HOLD" {
		reason = fmt.Sprintf("Neutral market conditions or insufficient data. Score: %.2f", signalScore)
	}


	signal := models.Signal{
		Symbol:    symbol,
		Action:    action,
		Price:     currentPrice,
		Strength:  strength,
		Timestamp: g.getCurrentTime(),
		Reason:    reason,
		Metadata: map[string]interface{}{
			"rsi":              indicators.RSI,
			"macd":             indicators.MACD,
			"macd_signal":      indicators.MACDSignal,
			"ema_short":        indicators.EMA20,
			"ema_long":         indicators.EMA50,
			"current_volume":   indicators.Volume,
			"average_volume":   indicators.AvgVolume,
			"signal_score":     signalScore,
			// "volume_ratio": is implicitly part of volume confirmation factor
		},
	}

	g.logger.WithFields(logrus.Fields{
		"symbol":       symbol,
		"action":       action,
		"strength":     strength,
		"price":        currentPrice,
		"signal_score": signalScore,
		"factors":      factors,
	}).Debug("Generated trading signal")

	return signal
}

// CalculateTechnicalIndicators calculates the technical indicators for a given trading pair.
// CalculateTechnicalIndicators calculates various technical indicators for a given trading symbol
// using historical price data.
// Parameters:
//   ctx: Context for managing cancellation and deadlines.
//   symbol: The trading symbol (e.g., "BTC-USD") for which to calculate indicators.
// Returns:
//   A TechnicalIndicators struct populated with calculated values.
//   An error if fetching price history fails, data is insufficient, or any indicator calculation fails.
// It is exported for potential use outside the standard signal generation flow (e.g., direct inspection or testing).
func (g *Generator) CalculateTechnicalIndicators(ctx context.Context, symbol string) (TechnicalIndicators, error) {
	// Calculate the total duration for which price history is needed.
	// This depends on the number of candles and the interval per candle.
	// A warning is logged if priceDataIntervalMinutes is not positive, as it's crucial for this calculation.
	// However, the primary validation for such parameters should be in NewGenerator or config loading.
	if g.priceDataIntervalMinutes <= 0 {
		// This warning helps identify configuration issues if they bypass initial checks.
		g.logger.Warnf("priceDataIntervalMinutes is %d, which is invalid; this might lead to incorrect window calculation for %s.", g.priceDataIntervalMinutes, symbol)
		// Defaulting here is a local safeguard; ideally, config ensures valid values.
		// For this calculation, if it's invalid, GetPriceHistory might behave unexpectedly or error out.
		// We proceed with the configured value, highlighting the importance of valid config.
	}
	windowMinutes := g.priceHistoryCandles * g.priceDataIntervalMinutes

	// Fetch price history from the repository.
	priceHistory, err := g.repo.GetPriceHistory(ctx, symbol, windowMinutes)
	if err != nil {
		g.logger.WithError(err).Errorf("Failed to get price history for %s", symbol)
		return TechnicalIndicators{}, err // Propagate the error.
	}

	// Determine the minimum number of data points required by TA-Lib for the configured periods.
	// This is typically the longest period among EMAs and MACD, plus a buffer for TA-Lib's initialization (NaNs).
	requiredPoints := g.emaLongPeriod
	if g.macdSlowPeriod > requiredPoints {
		requiredPoints = g.macdSlowPeriod
	}
	requiredPoints += 5 // Buffer for TA-Lib's initial NaN values.

	if len(priceHistory) < requiredPoints {
		warnMsg := fmt.Sprintf("Not enough historical data for %s: got %d points, need at least %d for calculations", symbol, len(priceHistory), requiredPoints)
		g.logger.Warn(warnMsg)
		return TechnicalIndicators{}, fmt.Errorf(warnMsg)
	}

	// Prepare slices for close prices and volumes, required by TA-Lib functions.
	closePrices := make([]float64, len(priceHistory))
	volumes := make([]float64, len(priceHistory))
	for i, p := range priceHistory {
		closePrices[i] = p.Close
		volumes[i] = p.Volume
	}

	// Calculate indicators using TA-Lib.
	rsi := talib.Rsi(closePrices, g.rsiPeriod)
	macd, macdSignal, _ := talib.Macd(closePrices, g.macdFastPeriod, g.macdSlowPeriod, g.macdSignalPeriod) // MACD histogram is ignored for now.
	ema20 := talib.Ema(closePrices, g.emaShortPeriod)
	ema50 := talib.Ema(closePrices, g.emaLongPeriod)
	avgVolume := talib.Sma(volumes, g.volumeAvgPeriod) // Simple Moving Average for average volume.

	indicators := TechnicalIndicators{}

	// Helper function to extract the last non-NaN value from a TA-Lib result slice.
	// TA-Lib functions often return slices with leading NaN values if data is insufficient for the full period.
	lastNonNaN := func(data []float64) (float64, error) {
		for i := len(data) - 1; i >= 0; i-- {
			if !math.IsNaN(data[i]) {
				return data[i], nil
			}
		}
		return 0, fmt.Errorf("all values are NaN, not enough data for TA-Lib calculation")
	}

	// Populate the TechnicalIndicators struct, handling potential errors from lastNonNaN.
	indicators.RSI, err = lastNonNaN(rsi)
	if err != nil {
		return TechnicalIndicators{}, fmt.Errorf("RSI calculation failed for %s: %w", symbol, err)
	}
	indicators.MACD, err = lastNonNaN(macd)
	if err != nil {
		return TechnicalIndicators{}, fmt.Errorf("MACD calculation failed for %s: %w", symbol, err)
	}
	indicators.MACDSignal, err = lastNonNaN(macdSignal)
	if err != nil {
		return TechnicalIndicators{}, fmt.Errorf("MACD Signal calculation failed for %s: %w", symbol, err)
	}
	indicators.EMA20, err = lastNonNaN(ema20)
	if err != nil {
		return TechnicalIndicators{}, fmt.Errorf("EMA20 calculation failed for %s: %w", symbol, err)
	}
	indicators.EMA50, err = lastNonNaN(ema50)
	if err != nil {
		return TechnicalIndicators{}, fmt.Errorf("EMA50 calculation failed for %s: %w", symbol, err)
	}
	indicators.AvgVolume, err = lastNonNaN(avgVolume)
	if err != nil {
		return TechnicalIndicators{}, fmt.Errorf("AvgVolume calculation failed for %s: %w", symbol, err)
	}

	// Set the latest volume.
	if len(volumes) > 0 {
		indicators.Volume = volumes[len(volumes)-1]
	} else {
		// This case should ideally be caught by the len(priceHistory) check,
		// but as a safeguard:
		return TechnicalIndicators{}, fmt.Errorf("no volume data available for %s", symbol)
	}
	
	// Bollinger Bands are not calculated in this version. Set to 0.
	indicators.BollingerUp = 0
	indicators.BollingerLow = 0

	return indicators, nil
}

// getCurrentTime returns the current time. Used for timestamping signals.
// This could be abstracted further if specific time handling (e.g., for backtesting) is needed.
func (g *Generator) getCurrentTime() time.Time {
	return time.Now()
}

// AnalyzeMarketConditions analyzes a list of trading pairs to determine the overall market sentiment.
// It calculates technical indicators for each pair and aggregates their individual sentiments
// (bullish, bearish, or neutral) to output a general market condition.
// Parameters:
//   ctx: Context for managing cancellation and deadlines.
//   pairs: A slice of trading pair symbols (e.g., ["BTC-USD", "ETH-USD"]) to analyze.
// Returns:
//   A string representing the determined market condition: "bullish", "bearish", or "neutral".
func (g *Generator) AnalyzeMarketConditions(ctx context.Context, pairs []string) string {
	// Counters for bullish, bearish, and neutral sentiment across pairs.
	bullishCount := 0
	bearishCount := 0
	neutralCount := 0
	determinedCondition := "neutral" // Default condition

	for _, pair := range pairs {
		indicators, err := g.CalculateTechnicalIndicators(ctx, pair)
		if err != nil {
			g.logger.WithError(err).Warnf("Failed to calculate indicators for pair %s in AnalyzeMarketConditions, counting as neutral", pair)
			neutralCount++
			continue
		}

		// Determine sentiment for the current pair based on EMA and MACD alignment.
		// Both short-term EMA > long-term EMA AND MACD line > MACD signal line suggest bullishness.
		// Opposite conditions suggest bearishness.
		// Non-zero checks are included as a safeguard against uninitialized indicator values.
		isBullish := indicators.EMA20 > indicators.EMA50 && indicators.EMA20 != 0 && indicators.EMA50 != 0 &&
			indicators.MACD > indicators.MACDSignal && indicators.MACD != 0 && indicators.MACDSignal != 0
		isBearish := indicators.EMA20 < indicators.EMA50 && indicators.EMA20 != 0 && indicators.EMA50 != 0 &&
			indicators.MACD < indicators.MACDSignal && indicators.MACD != 0 && indicators.MACDSignal != 0

		if isBullish {
			bullishCount++
		} else if isBearish {
			bearishCount++
		} else {
			neutralCount++ // If neither clearly bullish nor bearish, count as neutral.
		}
	}

	// Aggregate individual pair sentiments to determine overall market condition.
	// The market is considered "bullish" if the count of bullish pairs significantly outweighs bearish ones,
	// and "bearish" if the opposite is true, based on the marketDominanceFactor.
	if bullishCount > 0 && float64(bullishCount) > float64(bearishCount)*g.marketDominanceFactor {
		determinedCondition = "bullish"
	} else if bearishCount > 0 && float64(bearishCount) > float64(bullishCount)*g.marketDominanceFactor {
		determinedCondition = "bearish"
	}
	// Otherwise, the market condition remains "neutral".

	g.logger.WithFields(logrus.Fields{
		"bullish_pairs":    bullishCount,
		"bearish_pairs":    bearishCount,
		"neutral_pairs":    neutralCount,
		"market_condition": determinedCondition,
	}).Debug("Market condition analysis complete")

	return determinedCondition
}

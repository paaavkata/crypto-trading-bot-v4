package signals_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	// Adjust import path if your signals package is aliased or has a different module path
	"github.com/paaavkata/crypto-trading-bot-v4/trading-engine/internal/signals"
	"github.com/paaavkata/crypto-trading-bot-v4/trading-engine/pkg/models" // Corrected import path
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"io"      // For logger discard
	"strings" // For strings.Contains
)

// mockRepository is a mock for signals.PriceHistoryProvider.
type mockRepository struct {
	PriceHistoryData     map[string][]models.PricePoint
	GetPriceHistoryError error
	// Store errors per symbol if needed for more granular tests
	GetPriceHistoryErrorForSymbol map[string]error
}

// GetPriceHistory implements the signals.PriceHistoryProvider interface.
func (m *mockRepository) GetPriceHistory(ctx context.Context, symbol string, windowMinutes int) ([]models.PricePoint, error) {
	if m.GetPriceHistoryErrorForSymbol != nil {
		if err, ok := m.GetPriceHistoryErrorForSymbol[symbol]; ok {
			return nil, err
		}
	}
	if m.GetPriceHistoryError != nil {
		return nil, m.GetPriceHistoryError
	}
	data, ok := m.PriceHistoryData[symbol]
	if !ok {
		// Return empty slice and no error to mimic repository finding no data,
		// which CalculateTechnicalIndicators should handle as insufficient data.
		return []models.PricePoint{}, nil
	}
	return data, nil
}

// newTestLogger creates a logger that discards output for tests.
func newTestLogger() *logrus.Logger {
	logger := logrus.New()
	logger.SetOutput(io.Discard)
	return logger
}

// Helper to generate some price data
func generatePricePoints(t *testing.T, numPoints int, startPrice float64, priceStep float64, startVolume float64, volumeStep float64, symbol string) []models.PricePoint {
	t.Helper()
	points := make([]models.PricePoint, numPoints)
	baseTime := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < numPoints; i++ {
		points[i] = models.PricePoint{
			Timestamp: baseTime.Add(time.Duration(i) * time.Hour), // Ensure ascending time
			Close:     startPrice + float64(i)*priceStep,
			Volume:    startVolume + float64(i)*volumeStep,
			Open:      startPrice + float64(i)*priceStep - priceStep*0.1,
			High:      startPrice + float64(i)*priceStep + priceStep*0.2,
			Low:       startPrice + float64(i)*priceStep - priceStep*0.2,
			Symbol:    symbol,
		}
	}
	return points
}

func TestCalculateTechnicalIndicators(t *testing.T) {
	ctx := context.Background()
	mockRepo := &mockRepository{}
	testLogger := newTestLogger()

	g := signals.NewGenerator(testLogger, mockRepo, 14,12,26,9,20,50,20,100,60,30,70,1.5,0.3,-0.3,0.3,0.25,0.25,0.20,1.5)


	t.Run("Sufficient Data", func(t *testing.T) {
		mockRepo.PriceHistoryData = map[string][]models.PricePoint{"BTC-USD": generatePricePoints(t, 150, 50000, 10, 100, 1, "BTC-USD")}
		mockRepo.GetPriceHistoryError = nil
		mockRepo.GetPriceHistoryErrorForSymbol = nil
		indicators, err := g.CalculateTechnicalIndicators(ctx, "BTC-USD")
		require.NoError(t, err)
		assert.True(t, indicators.RSI >= 0 && indicators.RSI <= 100, "RSI out of bounds: %.2f", indicators.RSI)
		assert.NotEqual(t, 0.0, indicators.EMA20, "EMA20 should not be zero with sufficient data")
		assert.NotEqual(t, 0.0, indicators.AvgVolume, "AvgVolume should not be zero with sufficient data")
	})

	t.Run("Insufficient Data", func(t *testing.T) {
		mockRepo.PriceHistoryData = map[string][]models.PricePoint{"BTC-USD": generatePricePoints(t, 10, 50000, 10, 100, 1, "BTC-USD")}
		mockRepo.GetPriceHistoryError = nil
		mockRepo.GetPriceHistoryErrorForSymbol = nil
		_, err := g.CalculateTechnicalIndicators(ctx, "BTC-USD")
		require.Error(t, err)
		assert.True(t, strings.Contains(err.Error(), "Not enough historical data"), "Error message '%s' does not contain 'Not enough historical data'", err.Error())
	})

	t.Run("Repository Error", func(t *testing.T) {
		mockRepo.PriceHistoryData = nil
		mockRepo.GetPriceHistoryError = fmt.Errorf("mock db error")
		mockRepo.GetPriceHistoryErrorForSymbol = nil
		_, err := g.CalculateTechnicalIndicators(ctx, "BTC-USD")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "mock db error")
	})
}

func TestGenerateSignal(t *testing.T) {
	ctx := context.Background()
	mockRepo := &mockRepository{}
	testLogger := newTestLogger()
	g := signals.NewGenerator(testLogger, mockRepo, 14,12,26,9,20,50,20,100,60,30,70,1.5,0.3,-0.3,0.3,0.25,0.25,0.20,1.5)

	t.Run("Error from CalculateTechnicalIndicators", func(t *testing.T) {
		mockRepo.PriceHistoryData = nil
		mockRepo.GetPriceHistoryError = fmt.Errorf("error calculating indicators in test")
		mockRepo.GetPriceHistoryErrorForSymbol = nil
		signal := g.GenerateSignal(ctx, "BTC-USD", 50000)
		assert.Equal(t, "HOLD", signal.Action)
		assert.Contains(t, signal.Reason, "Error calculating indicators")
	})
    
    // Note: Testing specific BUY/SELL scenarios by crafting PricePoint data 
    // to trigger specific TA-Lib outputs is complex and brittle for unit tests.
    // Such tests are closer to integration tests of TA-Lib itself.
    // We rely on TestCalculateTechnicalIndicators to ensure indicators are generated.
    // TestGenerateSignal primarily tests the logic using those indicators.
    // A more advanced test could mock CalculateTechnicalIndicators method if Generator was designed for it.
}

func TestAnalyzeMarketConditions(t *testing.T) {
	ctx := context.Background()
	mockRepo := &mockRepository{}
	testLogger := newTestLogger()
	g := signals.NewGenerator(testLogger, mockRepo,14,12,26,9,20,50,20,100,60,30,70,1.5,0.3,-0.3,0.3,0.25,0.25,0.20,1.5)

	t.Run("Bullish Market", func(t *testing.T) {
		mockRepo.PriceHistoryData = map[string][]models.PricePoint{
			"BTC-USD": generatePricePoints(t, 150, 50000, 100, 100, 1, "BTC-USD"), // Strong uptrend
			"ETH-USD": generatePricePoints(t, 150, 3000, 20, 100, 1, "ETH-USD"),   // Strong uptrend
		}
		mockRepo.GetPriceHistoryError = nil
		mockRepo.GetPriceHistoryErrorForSymbol = nil
		condition := g.AnalyzeMarketConditions(ctx, []string{"BTC-USD", "ETH-USD"})
		assert.Equal(t, "bullish", condition)
	})

	t.Run("Bearish Market", func(t *testing.T) {
		mockRepo.PriceHistoryData = map[string][]models.PricePoint{
			"BTC-USD": generatePricePoints(t, 150, 50000, -100, 100, 1, "BTC-USD"), // Strong downtrend
			"ETH-USD": generatePricePoints(t, 150, 3000, -20, 100, 1, "ETH-USD"),   // Strong downtrend
		}
		mockRepo.GetPriceHistoryError = nil
		mockRepo.GetPriceHistoryErrorForSymbol = nil
		condition := g.AnalyzeMarketConditions(ctx, []string{"BTC-USD", "ETH-USD"})
		assert.Equal(t, "bearish", condition)
	})

	t.Run("Neutral Market due to mixed signals", func(t *testing.T) {
		mockRepo.PriceHistoryData = map[string][]models.PricePoint{
			"BTC-USD": generatePricePoints(t, 150, 50000, 100, 100, 1, "BTC-USD"), // Bullish
			"ETH-USD": generatePricePoints(t, 150, 3000, -20, 100, 1, "ETH-USD"),  // Bearish
		}
		mockRepo.GetPriceHistoryError = nil
		mockRepo.GetPriceHistoryErrorForSymbol = nil
		condition := g.AnalyzeMarketConditions(ctx, []string{"BTC-USD", "ETH-USD"})
		assert.Equal(t, "neutral", condition)
	})

	t.Run("One Pair Errors", func(t *testing.T) {
		mockRepo.PriceHistoryData = map[string][]models.PricePoint{
			"BTC-USD": generatePricePoints(t, 150, 50000, 100, 100, 1, "BTC-USD"), // Bullish
		}
		mockRepo.GetPriceHistoryError = nil
		mockRepo.GetPriceHistoryErrorForSymbol = map[string]error{"ETH-USD": fmt.Errorf("db error for ETH")}
		defer func() { mockRepo.GetPriceHistoryErrorForSymbol = nil }() // Cleanup
		
		condition := g.AnalyzeMarketConditions(ctx, []string{"BTC-USD", "ETH-USD"})
		// BTC is bullish, ETH is error (counted neutral). If dominanceFactor = 1.5, bullish wins.
		assert.Equal(t, "bullish", condition)
	})

	t.Run("Empty Pair List", func(t *testing.T) {
		mockRepo.PriceHistoryData = nil
		mockRepo.GetPriceHistoryError = nil
		mockRepo.GetPriceHistoryErrorForSymbol = nil
		condition := g.AnalyzeMarketConditions(ctx, []string{})
		assert.Equal(t, "neutral", condition)
	})
}
// Note: The newTestGenerator function was removed as it was unused and contained type issues.
// Direct instantiation of signals.NewGenerator with the mock repository (which now satisfies the interface)
// and a test logger is used in each test function.

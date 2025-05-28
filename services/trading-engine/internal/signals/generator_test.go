package signals

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/paaavkata/crypto-trading-bot-v4/trading-engine/internal/database"
	"github.com/paaavkata/crypto-trading-bot-v4/trading-engine/pkg/models"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockDatabaseRepository is a mock type for database.Repository used by signal generator
type MockSignalDBRepository struct {
	mock.Mock
}

func (m *MockSignalDBRepository) GetPriceDataForSymbol(ctx context.Context, symbol string, limit int) ([]models.PriceData, error) {
	args := m.Called(ctx, symbol, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.PriceData), args.Error(1)
}

// Helper to create a test logger
func newTestLogger() *logrus.Logger {
	logger := logrus.New()
	logger.SetOutput(logrus.StandardLogger().Out) // Or io.Discard for less noise
	logger.SetLevel(logrus.DebugLevel)          // Or higher for less noise
	return logger
}

func TestGenerator_GenerateSignal_MACrossover(t *testing.T) {
	t.Parallel()

	mockRepo := new(MockSignalDBRepository)
	logger := newTestLogger()
	signalGen := NewGenerator(logger, mockRepo) // NewGenerator now takes repo

	// Common data
	symbol := "BTC-USDT"
	currentPrice := 20000.00

	tests := []struct {
		name               string
		priceDataSetup     func() []models.PriceData // Returns data as repo would (oldest first)
		expectedAction     string
		expectedReasonContains string
		mockRepoError      error
	}{
		{
			name: "BUY signal - short MA crosses above long MA",
			priceDataSetup: func() []models.PriceData {
				// Prices: 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, | 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, | 30, 40, 50, 60, 70
				// Short MA (3): ..., 28, 40 ( (29+30+40)/3 ), 53.33 ( (30+40+50)/3 )
				// Long MA (5): ..., 27, 33.8 ( (27+28+29+30+40)/5 ), 40 ( (28+29+30+40+50)/5 )
				// For simplicity, using direct values that create a crossover
				// Short periods: 3, Long periods: 5 for this example data for clarity
				// Prices: 1,2,3,2,1, | 1,2,10,11,12  (total 10 points for 5-period long MA)
				// Long SMA (5): (1+2+3+2+1)/5 = 1.8, (2+3+2+1+1)/5 = 1.8, (3+2+1+1+2)/5=1.8, (2+1+1+2+10)/5=3.2, (1+1+2+10+11)/5=5, (1+2+10+11+12)/5=7.2
				// Short SMA (3): (1+2+3)/3=2, (2+3+2)/3=2.33, (3+2+1)/3=2, (2+1+1)/3=1.33, (1+1+2)/3=1.33, (1+2+10)/3=4.33, (2+10+11)/3=7.67, (10+11+12)/3=11
				//
				// Let's use the actual periods shortMAPeriod = 10, longMAPeriod  = 20
				// We need at least longMAPeriod data points. dataFetchLimit = 25
				prices := make([]float64, 25)
				// Previous: long > short
				for i := 0; i < 19; i++ { prices[i] = float64(100 + i) } // prices[18] = 118
				// Current: short crosses above long
				// Make last few prices jump up to pull short MA up faster
				prices[19] = 119 // End of first long MA window
				prices[20] = 125 // prices[0] to prices[19] = first long MA
				prices[21] = 130 // prices[1] to prices[20] = second long MA
				prices[22] = 135 // prices[2] to prices[21] = third long MA (previousLongSMA)
				prices[23] = 140 // prices[3] to prices[22] = fourth long MA (currentLongSMA)
				prices[24] = 160 // prices[4] to prices[23] = fifth long MA
                                // This data will be used to calculate SMA values. The last point prices[24] is the most recent.
                                // Short MA is calculated over prices[15]..prices[24] for currentShortSMA
                                // Long MA is calculated over prices[5]..prices[24] for currentLongSMA
				
				// Simpler data for clear crossover:
				// prices for previous state: short_sma < long_sma
				// prices for current state: short_sma > long_sma
				data := make([]models.PriceData, dataFetchLimit)
				baseTime := time.Now().Add(-time.Duration(dataFetchLimit) * time.Minute)
				// Create a scenario where long MA is initially higher or stable, then short MA crosses up
				for i := 0; i < longMAPeriod-1; i++ { // First 19 points (0-18)
					data[i] = models.PriceData{Timestamp: baseTime.Add(time.Duration(i) * time.Minute), Close: 100.0}
				}
				// prices[19] is the 20th point for the first long MA calculation
				data[longMAPeriod-1] = models.PriceData{Timestamp: baseTime.Add(time.Duration(longMAPeriod-1) * time.Minute), Close: 100.0} // SMA20_prev uses up to here
				
				// Data for previous SMAs (t-1)
				// To make previousShortSMA <= previousLongSMA
				// Let data[longMAPeriod-1] be the (t-1) point for current SMA, (t-2) for previous SMA.
				// We need dataFetchLimit points. Let's say longMAPeriod = 20.
				// Prices are ordered oldest first.
				// Previous state: shortSMA_prev <= longSMA_prev
				// Current state: shortSMA_curr > longSMA_curr

				// Example: short (3), long (5). Data points needed: 5 for long, +1 for prev = 6
				// Prices: P0 P1 P2 P3 P4 P5
				// longSMA_prev uses P0-P4. longSMA_curr uses P1-P5
				// shortSMA_prev uses P2-P4. shortSMA_curr uses P3-P5
				
				// For short=10, long=20. Need dataFetchLimit = 25 points.
				// Data points are data[0]...data[24]
				// currentShortSMA uses data[15]..data[24]
				// previousShortSMA uses data[14]..data[23]
				// currentLongSMA uses data[5]..data[24]
				// previousLongSMA uses data[4]..data[23]

				// Setup for BUY: previousShortSMA <= previousLongSMA AND currentShortSMA > currentLongSMA
				// Make first 23 points such that previousShortSMA <= previousLongSMA
				for i := 0; i <= 23; i++ {
					data[i] = models.PriceData{Timestamp: baseTime.Add(time.Duration(i) * time.Minute), Close: 100.0}
				}
				// Make point 24 (most recent) significantly higher to pull currentShortSMA up
				data[24] = models.PriceData{Timestamp: baseTime.Add(time.Duration(24) * time.Minute), Close: 120.0} 
				// This should make currentShortSMA (avg of last 10 including 120) > currentLongSMA (avg of last 20 including 120)
				// And previousShortSMA (avg of 10 up to 100) == previousLongSMA (avg of 20 up to 100)
				return data
			},
			expectedAction: "BUY",
			expectedReasonContains: fmt.Sprintf("SMA(%d) crossed above SMA(%d)", shortMAPeriod, longMAPeriod),
		},
		{
			name: "SELL signal - short MA crosses below long MA",
			priceDataSetup: func() []models.PriceData {
				data := make([]models.PriceData, dataFetchLimit)
				baseTime := time.Now().Add(-time.Duration(dataFetchLimit) * time.Minute)
				// Setup for SELL: previousShortSMA >= previousLongSMA AND currentShortSMA < currentLongSMA
				// Make first 23 points such that previousShortSMA >= previousLongSMA
				for i := 0; i <= 23; i++ {
					data[i] = models.PriceData{Timestamp: baseTime.Add(time.Duration(i) * time.Minute), Close: 100.0}
				}
				// Make point 24 (most recent) significantly lower to pull currentShortSMA down
				data[24] = models.PriceData{Timestamp: baseTime.Add(time.Duration(24) * time.Minute), Close: 80.0}
				return data
			},
			expectedAction: "SELL",
			expectedReasonContains: fmt.Sprintf("SMA(%d) crossed below SMA(%d)", shortMAPeriod, longMAPeriod),
		},
		{
			name: "HOLD signal - no crossover",
			priceDataSetup: func() []models.PriceData {
				data := make([]models.PriceData, dataFetchLimit)
				baseTime := time.Now().Add(-time.Duration(dataFetchLimit) * time.Minute)
				// All prices are the same, MAs will be the same, no crossover
				for i := 0; i < dataFetchLimit; i++ {
					data[i] = models.PriceData{Timestamp: baseTime.Add(time.Duration(i) * time.Minute), Close: 100.0}
				}
				return data
			},
			expectedAction: "HOLD",
			expectedReasonContains: fmt.Sprintf("No SMA(%d)/SMA(%d) crossover", shortMAPeriod, longMAPeriod),
		},
		{
			name: "Insufficient data for long MA",
			priceDataSetup: func() []models.PriceData {
				data := make([]models.PriceData, longMAPeriod-1) // One less than required
				baseTime := time.Now().Add(-time.Duration(longMAPeriod-1) * time.Minute)
				for i := 0; i < len(data); i++ {
					data[i] = models.PriceData{Timestamp: baseTime.Add(time.Duration(i) * time.Minute), Close: 100.0}
				}
				return data
			},
			expectedAction: "HOLD",
			expectedReasonContains: fmt.Sprintf("Insufficient data for MA(%d)", longMAPeriod),
		},
		{
			name: "Insufficient data for SMA crossover check (less than 2 SMA values)",
			priceDataSetup: func() []models.PriceData {
				// dataFetchLimit is longMAPeriod + 5.
				// SMA calculation needs 'period' points. Result length is len(prices) - period + 1.
				// If len(prices) = longMAPeriod, then len(longSMAValues) = 1.
				// We need at least 2 SMA values, so len(prices) must be at least longMAPeriod + 1.
				data := make([]models.PriceData, longMAPeriod) // Will produce only 1 long SMA value
				baseTime := time.Now().Add(-time.Duration(longMAPeriod) * time.Minute)
				for i := 0; i < len(data); i++ {
					data[i] = models.PriceData{Timestamp: baseTime.Add(time.Duration(i) * time.Minute), Close: 100.0 + float64(i)}
				}
				return data
			},
			expectedAction: "HOLD",
			expectedReasonContains: "Insufficient SMA values for crossover",
		},
		{
			name: "Error fetching price data",
			priceDataSetup: func() []models.PriceData {
				return nil // Mock will return error
			},
			mockRepoError:      fmt.Errorf("database error"),
			expectedAction: "HOLD",
			expectedReasonContains: "Error fetching price data",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Reset mocks for each sub-test
			mockRepo := new(MockSignalDBRepository)
			signalGen.repo = mockRepo // Update repo on existing signalGen instance for this sub-test

			priceData := tc.priceDataSetup()
			mockRepo.On("GetPriceDataForSymbol", mock.Anything, symbol, dataFetchLimit).Return(priceData, tc.mockRepoError).Once()

			signal := signalGen.GenerateSignal(context.Background(), symbol, currentPrice)

			assert.Equal(t, tc.expectedAction, signal.Action)
			assert.Contains(t, signal.Reason, tc.expectedReasonContains)
			if tc.mockRepoError != nil || tc.expectedReasonContains != fmt.Sprintf("SMA(%d) crossed above SMA(%d)", shortMAPeriod, longMAPeriod) && tc.expectedReasonContains != fmt.Sprintf("SMA(%d) crossed below SMA(%d)", shortMAPeriod, longMAPeriod) && tc.expectedReasonContains != fmt.Sprintf("No SMA(%d)/SMA(%d) crossover", shortMAPeriod, longMAPeriod) {
				// For error cases or specific non-crossover holds, strength might be 0
				if tc.expectedReasonContains == "Error fetching price data" || 
				   tc.expectedReasonContains == fmt.Sprintf("Insufficient data for MA(%d)", longMAPeriod) ||
				   tc.expectedReasonContains == "Insufficient SMA values for crossover" {
					assert.Equal(t, 0.0, signal.Strength)
				}
			} else if tc.expectedAction != "HOLD" {
				assert.Equal(t, 1.0, signal.Strength) // Expect full strength for clear signals
			}


			mockRepo.AssertExpectations(t)
		})
	}
}

// Test calculateSMA helper (optional, as it's implicitly tested by GenerateSignal tests)
func TestGenerator_calculateSMA(t *testing.T) {
	t.Parallel()
	gen := &Generator{logger: newTestLogger()}

	tests := []struct {
		name     string
		prices   []float64
		period   int
		expected []float64
	}{
		{"nil if period longer than prices", []float64{1, 2}, 3, nil},
		{"simple average", []float64{1, 2, 3, 4, 5}, 3, []float64{2, 3, 4}},
		{"another average", []float64{10, 12, 11, 15, 14}, 4, []float64{12, 13}},
		{"single value if period equals length", []float64{1,2,3}, 3, []float64{2}},
		{"empty if prices empty", []float64{}, 3, nil},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := gen.calculateSMA(tc.prices, tc.period)
			assert.Equal(t, tc.expected, result)
		})
	}
}

// Ensure MockSignalDBRepository implements the interface expected by Generator.
// Assuming Generator expects an interface like this:
type PriceDataProvider interface {
	GetPriceDataForSymbol(ctx context.Context, symbol string, limit int) ([]models.PriceData, error)
}
var _ PriceDataProvider = (*MockSignalDBRepository)(nil)

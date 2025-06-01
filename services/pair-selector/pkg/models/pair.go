package models

import (
	"time"
)

type TradingPair struct {
	ID              int64     `db:"id"`
	Symbol          string    `db:"symbol"`
	BaseAsset       string    `db:"base_asset"`
	QuoteAsset      string    `db:"quote_asset"`
	Status          string    `db:"status"`
	DailyVolume     float64   `db:"daily_volume"`
	DailyVolumeUSDT float64   `db:"daily_volume_usdt"`
	VolatilityScore float64   `db:"volatility_score"`
	ATR14           float64   `db:"atr_14"`
	CorrelationBTC  float64   `db:"correlation_btc"`
	PriceChange24h  float64   `db:"price_change_24h"`
	LastPrice       float64   `db:"last_price"`
	LastUpdated     time.Time `db:"last_updated"`
	CreatedAt       time.Time `db:"created_at"`
}

type SelectedPair struct {
	ID               int64     `db:"id"`
	Symbol           string    `db:"symbol"`
	SelectionScore   float64   `db:"selection_score"`
	Volatility24h    float64   `db:"volatility_24h"`
	Volume24hUSDT    float64   `db:"volume_24h_usdt"`
	ATRScore         float64   `db:"atr_score"`
	VolumeScore      float64   `db:"volume_score"`
	CorrelationScore float64   `db:"correlation_score"`
	RiskLevel        string    `db:"risk_level"`
	Status           string    `db:"status"`
	SelectedAt       time.Time `db:"selected_at"`
	LastEvaluated    time.Time `db:"last_evaluated"`
}

type PairAnalysis struct {
	Symbol           string
	Volume24hUSDT    float64
	Volatility       float64
	ATR14            float64 // Note: ATR14 is calculated using the ATRPeriod from SelectionCriteria
	CorrelationBTC   float64
	VolumeScore      float64
	VolatilityScore  float64
	ATRScore         float64
	CorrelationScore float64
	FinalScore       float64
	RiskLevel        string
	PriceData        []PricePoint
}

type PricePoint struct {
	Timestamp time.Time
	Close     float64
	Volume    float64
	High      float64
	Low       float64
}

type SelectionCriteria struct {
	MinVolumeUSDT     float64 // $1M minimum
	MaxVolatility     float64 // 8% maximum
	MinVolatility     float64 // 3% minimum
	MaxActivesPairs   int     // 8 maximum active pairs
	WatchlistSize     int     // 20 pairs in watchlist
	VolumeWeight      float64 // Weight for volume score
	VolatilityWeight  float64 // Weight for volatility score
	ATRWeight         float64 // Weight for ATR score
	CorrelationWeight float64 // Weight for correlation score
	ATRPeriod         int     // ATR period in minutes
	RiskThresholds    RiskThresholdsConfig
}

// RiskThresholdsConfig holds all configurable thresholds for risk assessment.
type RiskThresholdsConfig struct {
	VolatilityRisk  VolatilityRiskConfig
	CorrelationRisk CorrelationRiskConfig
	VolumeRisk      VolumeRiskConfig
	ATRRisk         ATRRiskConfig
	MomentumRisk    MomentumRiskConfig
	OverallRisk     OverallRiskConfig
}

type VolatilityRiskConfig struct {
	Weight float64
	// Thresholds for volatility values (e.g., 0.12, 0.08, ...)
	Band1 float64 // Example: 0.12 (Very High)
	Band2 float64 // Example: 0.08 (High)
	Band3 float64 // Example: 0.05 (Medium)
	Band4 float64 // Example: 0.03 (Low)
	// Corresponding risk scores for each band
	Score1 float64 // Example: 4.0
	Score2 float64 // Example: 3.0
	Score3 float64 // Example: 2.0
	Score4 float64 // Example: 1.5
	Score5 float64 // Example: 1.0 (Below Band4)
}

type CorrelationRiskConfig struct {
	Weight float64
	// Thresholds for absolute correlation values (e.g., 0.2, 0.4, ...)
	Band1 float64 // Example: 0.2 (Very Low Correlation)
	Band2 float64 // Example: 0.4 (Low Correlation)
	Band3 float64 // Example: 0.6 (Medium Correlation)
	// Corresponding risk scores for each band
	Score1 float64 // Example: 4.0
	Score2 float64 // Example: 3.0
	Score3 float64 // Example: 2.0
	Score4 float64 // Example: 1.0 (Above Band3)
}

type VolumeRiskConfig struct {
	Weight float64
	// Thresholds for Volume in USDT (e.g., 1M, 3M, 10M)
	Band1 float64 // Example: 1,000,000 (Low Volume)
	Band2 float64 // Example: 3,000,000 (Medium-Low Volume)
	Band3 float64 // Example: 10,000,000 (Medium Volume)
	// Corresponding risk scores
	Score1 float64 // Example: 4.0
	Score2 float64 // Example: 3.0
	Score3 float64 // Example: 2.0
	Score4 float64 // Example: 1.0 (Above Band3)
}

type ATRRiskConfig struct {
	Weight                float64
	DefaultScoreNoVol     float64 // Default ATR risk score if volatility is zero
	// Thresholds for ATR/Volatility ratio
	RatioBand1 float64 // Example: 2.0
	RatioBand2 float64 // Example: 1.5
	// Corresponding risk scores
	Score1 float64 // Example: 3.0
	Score2 float64 // Example: 2.0
	Score3 float64 // Example: 1.0 (Below RatioBand2)
}

type MomentumRiskConfig struct {
	Weight              float64
	MinDataPoints       int     // Min price data points required
	DefaultScoreForSafe float64 // Default score when data is insufficient (considered safer)
	RecentPeriods       int     // Number of recent periods for avg calc
	OlderPeriodsStart   int     // Start index for older periods (relative to recent)
	OlderPeriodsEnd     int     // End index for older periods (relative to recent)
	// Thresholds for absolute momentum change (e.g., 0.1, 0.05)
	ChangeBand1 float64 // Example: 0.1 (High Momentum Change)
	ChangeBand2 float64 // Example: 0.05 (Medium Momentum Change)
	// Corresponding risk scores
	Score1 float64 // Example: 3.0
	Score2 float64 // Example: 2.0
	Score3 float64 // Example: 1.0 (Below ChangeBand2)
}

type OverallRiskConfig struct {
	// Thresholds for final normalized risk score to categorize as "high", "medium", "low"
	HighThreshold   float64 // Example: 0.75
	MediumThreshold float64 // Example: 0.5
	// NormalizationFactor is not explicitly in the code but implied by division by 4.0.
	// We can make this configurable if needed, or assume it's tied to max possible raw score.
}

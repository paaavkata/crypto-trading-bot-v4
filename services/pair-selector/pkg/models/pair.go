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
	ATR14            float64
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
}

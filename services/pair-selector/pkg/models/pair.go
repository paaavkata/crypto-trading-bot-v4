package models

import (
	"time"

	"github.com/paaavkata/crypto-trading-bot-v4/shared/pkg/database"
)

// TradingPair represents a pair with its tracked statistics.
type TradingPair struct {
	ID              int64            `db:"id"`
	Symbol          string           `db:"symbol"`
	BaseAsset       string           `db:"base_asset"`
	QuoteAsset      string           `db:"quote_asset"`
	Status          string           `db:"status"`
	DailyVolume     database.Decimal `db:"daily_volume"`
	DailyVolumeUSDT database.Decimal `db:"daily_volume_usdt"`
	VolatilityScore database.Decimal `db:"volatility_score"`
	ATR14           database.Decimal `db:"atr_14"`
	CorrelationBTC  database.Decimal `db:"correlation_btc"`
	PriceChange24h  database.Decimal `db:"price_change_24h"`
	LastPrice       database.Decimal `db:"last_price"`
	LastUpdated     time.Time        `db:"last_updated"`
	CreatedAt       time.Time        `db:"created_at"`
}

// SelectedPair for active trading.
type SelectedPair struct {
	ID               int64            `db:"id"`
	Symbol           string           `db:"symbol"`
	SelectionScore   database.Decimal `db:"selection_score"`
	Volatility24h    database.Decimal `db:"volatility_24h"`
	Volume24hUSDT    database.Decimal `db:"volume_24h_usdt"`
	ATRScore         database.Decimal `db:"atr_score"`
	VolumeScore      database.Decimal `db:"volume_score"`
	CorrelationScore database.Decimal `db:"correlation_score"`
	RiskLevel        string           `db:"risk_level"`
	Status           string           `db:"status"`
	SelectedAt       time.Time        `db:"selected_at"`
	LastEvaluated    time.Time        `db:"last_evaluated"`
}

// PairAnalysis and PricePoint: update any monetary fields to database.Decimal.
type PricePoint struct {
	Timestamp time.Time        `json:"timestamp"`
	Close     database.Decimal `json:"close"`
	Volume    database.Decimal `json:"volume"`
	High      database.Decimal `json:"high"`
	Low       database.Decimal `json:"low"`
}

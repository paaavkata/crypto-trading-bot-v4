package models

import (
	"time"
)

type Position struct {
	ID            string     `db:"id"`
	PairID        int64      `db:"pair_id"`
	ConfigID      string     `db:"config_id"`
	Side          string     `db:"side"` // 'buy' or 'sell'
	Quantity      float64    `db:"quantity"`
	EntryPrice    float64    `db:"entry_price"`
	CurrentPrice  float64    `db:"current_price"`
	UnrealizedPnL float64    `db:"unrealized_pnl"`
	RealizedPnL   float64    `db:"realized_pnl"`
	Status        string     `db:"status"` // 'open', 'closed', 'partial'
	OrderID       string     `db:"order_id"`
	CreatedAt     time.Time  `db:"created_at"`
	UpdatedAt     time.Time  `db:"updated_at"`
	ClosedAt      *time.Time `db:"closed_at"`
}

type Order struct {
	ID             string     `db:"id"`
	PositionID     *string    `db:"position_id"`
	PairID         int64      `db:"pair_id"`
	KuCoinOrderID  string     `db:"kucoin_order_id"`
	Side           string     `db:"side"`
	Type           string     `db:"type"`
	Quantity       float64    `db:"quantity"`
	Price          float64    `db:"price"`
	FilledQuantity float64    `db:"filled_quantity"`
	Status         string     `db:"status"`
	Fee            float64    `db:"fee"`
	CreatedAt      time.Time  `db:"created_at"`
	UpdatedAt      time.Time  `db:"updated_at"`
	FilledAt       *time.Time `db:"filled_at"`
}

type TradingConfig struct {
	ID                string    `db:"id"`
	PairID            int64     `db:"pair_id"`
	StrategyType      string    `db:"strategy_type"`
	GridLevels        int       `db:"grid_levels"`
	PriceRangeMin     float64   `db:"price_range_min"`
	PriceRangeMax     float64   `db:"price_range_max"`
	PositionSizeUSDT  float64   `db:"position_size_usdt"`
	StopLossPercent   float64   `db:"stop_loss_percent"`
	TakeProfitPercent float64   `db:"take_profit_percent"`
	MaxPositions      int       `db:"max_positions"`
	IsActive          bool      `db:"is_active"`
	CreatedAt         time.Time `db:"created_at"`
	UpdatedAt         time.Time `db:"updated_at"`
}

type Signal struct {
	Symbol    string
	Action    string // 'BUY', 'SELL', 'HOLD'
	Price     float64
	Strength  float64 // 0.0 to 1.0
	Timestamp time.Time
	Reason    string
	Metadata  map[string]interface{} // Additional data for the signal
}

type GridLevel struct {
	Price    float64
	Quantity float64
	Type     string // 'buy' or 'sell'
	IsActive bool
	OrderID  string
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
	StopLossPercent  float64   `db:"stop_loss_percent"`
	RiskLevel        string    `db:"risk_level"`
	Status           string    `db:"status"`
	SelectedAt       time.Time `db:"selected_at"`
	LastEvaluated    time.Time `db:"last_evaluated"`
}

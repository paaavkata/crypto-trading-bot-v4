package models

import (
	"time"

	"github.com/paaavkata/crypto-trading-bot-v4/shared/pkg/database"
)

// Position represents an open or closed trading position.
type Position struct {
	ID            string           `db:"id"`
	PairID        int64            `db:"pair_id"`
	ConfigID      string           `db:"config_id"`
	Side          string           `db:"side"` // 'buy' or 'sell'
	Quantity      database.Decimal `db:"quantity"`
	EntryPrice    database.Decimal `db:"entry_price"`
	CurrentPrice  database.Decimal `db:"current_price"`
	UnrealizedPnL database.Decimal `db:"unrealized_pnl"`
	RealizedPnL   database.Decimal `db:"realized_pnl"`
	Status        string           `db:"status"` // 'open', 'closed', 'partial'
	OrderID       string           `db:"order_id"`
	CreatedAt     time.Time        `db:"created_at"`
	UpdatedAt     time.Time        `db:"updated_at"`
	ClosedAt      *time.Time       `db:"closed_at"`
}

// Order represents a trade order.
type Order struct {
	ID             string           `db:"id"`
	PositionID     *string          `db:"position_id"`
	PairID         int64            `db:"pair_id"`
	KuCoinOrderID  string           `db:"kucoin_order_id"`
	Side           string           `db:"side"`
	Type           string           `db:"type"`
	Quantity       database.Decimal `db:"quantity"`
	Price          database.Decimal `db:"price"`
	FilledQuantity database.Decimal `db:"filled_quantity"`
	Status         string           `db:"status"`
	Fee            database.Decimal `db:"fee"`
	CreatedAt      time.Time        `db:"created_at"`
	UpdatedAt      time.Time        `db:"updated_at"`
	FilledAt       *time.Time       `db:"filled_at"`
}

// TradingConfig holds strategy configuration per trading pair.
type TradingConfig struct {
	ID                string           `db:"id"`
	PairID            int64            `db:"pair_id"`
	StrategyType      string           `db:"strategy_type"`
	GridLevels        int              `db:"grid_levels"`
	PriceRangeMin     database.Decimal `db:"price_range_min"`
	PriceRangeMax     database.Decimal `db:"price_range_max"`
	PositionSizeUSDT  database.Decimal `db:"position_size_usdt"`
	StopLossPercent   database.Decimal `db:"stop_loss_percent"`
	TakeProfitPercent database.Decimal `db:"take_profit_percent"`
	MaxPositions      int              `db:"max_positions"`
	IsActive          bool             `db:"is_active"`
	CreatedAt         time.Time        `db:"created_at"`
	UpdatedAt         time.Time        `db:"updated_at"`
}

// Signal, GridLevel, and SelectedPair struct definitions remain unchanged for non-monetary fields.
// If any monetary fields are present, update to use database.Decimal or decimal.Decimal accordingly.

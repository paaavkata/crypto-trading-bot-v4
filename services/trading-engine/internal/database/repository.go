package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/paaavkata/crypto-trading-bot-v4/shared/pkg/database"
	"github.com/paaavkata/crypto-trading-bot-v4/trading-engine/pkg/models"
	"github.com/sirupsen/logrus"
)

type Repository struct {
	db     *database.DB
	logger *logrus.Logger
}

func NewRepository(db *database.DB, logger *logrus.Logger) *Repository {
	return &Repository{
		db:     db,
		logger: logger,
	}
}

func (r *Repository) GetActiveSelectedPairs(ctx context.Context) ([]models.SelectedPair, error) {
	query := `
        SELECT id, symbol, selection_score, volatility_24h, volume_24h_usdt,
               atr_score, volume_score, correlation_score, risk_level,
               status, selected_at, last_evaluated
        FROM selected_pairs
        WHERE status = 'active'
        ORDER BY selection_score DESC
    `

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query active selected pairs: %w", err)
	}
	defer rows.Close()

	var pairs []models.SelectedPair
	for rows.Next() {
		var pair models.SelectedPair
		err := rows.Scan(
			&pair.ID, &pair.Symbol, &pair.SelectionScore, &pair.Volatility24h,
			&pair.Volume24hUSDT, &pair.ATRScore, &pair.VolumeScore,
			&pair.CorrelationScore, &pair.RiskLevel, &pair.Status,
			&pair.SelectedAt, &pair.LastEvaluated,
		)
		if err != nil {
			r.logger.WithError(err).Error("Failed to scan selected pair")
			continue
		}
		pairs = append(pairs, pair)
	}

	return pairs, nil
}

func (r *Repository) GetTradingConfig(ctx context.Context, pairID int64) (*models.TradingConfig, error) {
	query := `
        SELECT id, pair_id, strategy_type, grid_levels, price_range_min, price_range_max,
               position_size_usdt, stop_loss_percent, take_profit_percent, max_positions,
               is_active, created_at, updated_at
        FROM trading_configs
        WHERE pair_id = $1 AND is_active = true
        LIMIT 1
    `

	var config models.TradingConfig
	err := r.db.QueryRowContext(ctx, query, pairID).Scan(
		&config.ID, &config.PairID, &config.StrategyType, &config.GridLevels,
		&config.PriceRangeMin, &config.PriceRangeMax, &config.PositionSizeUSDT,
		&config.StopLossPercent, &config.TakeProfitPercent, &config.MaxPositions,
		&config.IsActive, &config.CreatedAt, &config.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // No config found
		}
		return nil, fmt.Errorf("failed to get trading config: %w", err)
	}

	return &config, nil
}

func (r *Repository) CreateTradingConfig(ctx context.Context, config models.TradingConfig) error {
	config.ID = uuid.New().String()
	config.CreatedAt = time.Now()
	config.UpdatedAt = time.Now()

	query := `
        INSERT INTO trading_configs 
        (id, pair_id, strategy_type, grid_levels, price_range_min, price_range_max,
         position_size_usdt, stop_loss_percent, take_profit_percent, max_positions,
         is_active, created_at, updated_at)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
    `

	_, err := r.db.ExecContext(ctx, query,
		config.ID, config.PairID, config.StrategyType, config.GridLevels,
		config.PriceRangeMin, config.PriceRangeMax, config.PositionSizeUSDT,
		config.StopLossPercent, config.TakeProfitPercent, config.MaxPositions,
		config.IsActive, config.CreatedAt, config.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to create trading config: %w", err)
	}

	r.logger.WithFields(logrus.Fields{
		"config_id": config.ID,
		"pair_id":   config.PairID,
		"strategy":  config.StrategyType,
	}).Info("Created new trading config")

	return nil
}

func (r *Repository) GetOpenPositions(ctx context.Context, pairID int64) ([]models.Position, error) {
	query := `
        SELECT id, pair_id, config_id, side, quantity, entry_price, current_price,
               unrealized_pnl, realized_pnl, status, order_id, created_at, updated_at, closed_at
        FROM positions
        WHERE pair_id = $1 AND status IN ('open', 'partial')
        ORDER BY created_at DESC
    `

	rows, err := r.db.QueryContext(ctx, query, pairID)
	if err != nil {
		return nil, fmt.Errorf("failed to query open positions: %w", err)
	}
	defer rows.Close()

	var positions []models.Position
	for rows.Next() {
		var pos models.Position
		err := rows.Scan(
			&pos.ID, &pos.PairID, &pos.ConfigID, &pos.Side, &pos.Quantity,
			&pos.EntryPrice, &pos.CurrentPrice, &pos.UnrealizedPnL, &pos.RealizedPnL,
			&pos.Status, &pos.OrderID, &pos.CreatedAt, &pos.UpdatedAt, &pos.ClosedAt,
		)
		if err != nil {
			r.logger.WithError(err).Error("Failed to scan position")
			continue
		}
		positions = append(positions, pos)
	}

	return positions, nil
}

func (r *Repository) CreatePosition(ctx context.Context, position models.Position) error {
	position.ID = uuid.New().String()
	position.CreatedAt = time.Now()
	position.UpdatedAt = time.Now()

	query := `
        INSERT INTO positions
        (id, pair_id, config_id, side, quantity, entry_price, current_price,
         unrealized_pnl, realized_pnl, status, order_id, created_at, updated_at)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
    `

	_, err := r.db.ExecContext(ctx, query,
		position.ID, position.PairID, position.ConfigID, position.Side,
		position.Quantity, position.EntryPrice, position.CurrentPrice,
		position.UnrealizedPnL, position.RealizedPnL, position.Status,
		position.OrderID, position.CreatedAt, position.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to create position: %w", err)
	}

	r.logger.WithFields(logrus.Fields{
		"position_id": position.ID,
		"pair_id":     position.PairID,
		"side":        position.Side,
		"quantity":    position.Quantity,
		"entry_price": position.EntryPrice,
	}).Info("Created new position")

	return nil
}

func (r *Repository) UpdatePosition(ctx context.Context, position models.Position) error {
	position.UpdatedAt = time.Now()

	query := `
        UPDATE positions
        SET current_price = $2, unrealized_pnl = $3, realized_pnl = $4,
            status = $5, updated_at = $6, closed_at = $7
        WHERE id = $1
    `

	_, err := r.db.ExecContext(ctx, query,
		position.ID, position.CurrentPrice, position.UnrealizedPnL,
		position.RealizedPnL, position.Status, position.UpdatedAt, position.ClosedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to update position: %w", err)
	}

	return nil
}

func (r *Repository) CreateOrder(ctx context.Context, order models.Order) error {
	order.ID = uuid.New().String()
	order.CreatedAt = time.Now()
	order.UpdatedAt = time.Now()

	query := `
        INSERT INTO orders
        (id, position_id, pair_id, kucoin_order_id, side, type, quantity, price,
         filled_quantity, status, fee, created_at, updated_at)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
    `

	_, err := r.db.ExecContext(ctx, query,
		order.ID, order.PositionID, order.PairID, order.KuCoinOrderID,
		order.Side, order.Type, order.Quantity, order.Price,
		order.FilledQuantity, order.Status, order.Fee,
		order.CreatedAt, order.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to create order: %w", err)
	}

	r.logger.WithFields(logrus.Fields{
		"order_id":        order.ID,
		"kucoin_order_id": order.KuCoinOrderID,
		"pair_id":         order.PairID,
		"side":            order.Side,
		"quantity":        order.Quantity,
		"price":           order.Price,
	}).Info("Created new order")

	return nil
}

func (r *Repository) GetLatestPrice(ctx context.Context, symbol string) (float64, error) {
	query := `
        SELECT close
        FROM price_data
        WHERE symbol = $1
        ORDER BY timestamp DESC
        LIMIT 1
    `

	var price float64
	err := r.db.QueryRowContext(ctx, query, symbol).Scan(&price)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, fmt.Errorf("no price data found for symbol %s", symbol)
		}
		return 0, fmt.Errorf("failed to get latest price: %w", err)
	}

	return price, nil
}

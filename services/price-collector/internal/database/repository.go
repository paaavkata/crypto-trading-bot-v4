package database

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/paaavkata/crypto-trading-bot-v4/price-collector/pkg/models"
	"github.com/paaavkata/crypto-trading-bot-v4/shared/pkg/database"
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

func (r *Repository) BulkInsertPriceData(ctx context.Context, data []models.PriceData) error {
	if len(data) == 0 {
		return nil
	}

	start := time.Now()

	// Build bulk insert query
	query := `
        INSERT INTO price_data (symbol, timestamp, open, high, low, close, volume, quote_volume, change_rate, change_price)
        VALUES `

	values := make([]string, 0, len(data))
	args := make([]interface{}, 0, len(data)*10)

	for i, price := range data {
		values = append(values, fmt.Sprintf("($%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d)",
			i*10+1, i*10+2, i*10+3, i*10+4, i*10+5, i*10+6, i*10+7, i*10+8, i*10+9, i*10+10))

		args = append(args, price.Symbol, price.Timestamp, price.Open, price.High,
			price.Low, price.Close, price.Volume, price.QuoteVolume, price.ChangeRate, price.ChangePrice)
	}

	query += strings.Join(values, ", ")
	query += " ON CONFLICT (symbol, timestamp) DO UPDATE SET " +
		"open = EXCLUDED.open, high = EXCLUDED.high, low = EXCLUDED.low, " +
		"close = EXCLUDED.close, volume = EXCLUDED.volume, quote_volume = EXCLUDED.quote_volume, " +
		"change_rate = EXCLUDED.change_rate, change_price = EXCLUDED.change_price"

	_, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		r.logger.WithError(err).Error("Failed to bulk insert price data")
		return fmt.Errorf("failed to bulk insert price data: %w", err)
	}

	duration := time.Since(start)
	r.logger.WithFields(logrus.Fields{
		"records_count": len(data),
		"duration_ms":   duration.Milliseconds(),
	}).Info("Successfully bulk inserted price data")

	return nil
}

func (r *Repository) GetLatestPriceData(ctx context.Context, symbol string) (*models.PriceData, error) {
	query := `
        SELECT id, symbol, timestamp, open, high, low, close, volume, quote_volume, change_rate, change_price, created_at
        FROM price_data
        WHERE symbol = $1
        ORDER BY timestamp DESC
        LIMIT 1
    `

	var price models.PriceData
	err := r.db.QueryRowContext(ctx, query, symbol).Scan(
		&price.ID, &price.Symbol, &price.Timestamp, &price.Open, &price.High,
		&price.Low, &price.Close, &price.Volume, &price.QuoteVolume,
		&price.ChangeRate, &price.ChangePrice, &price.CreatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get latest price data: %w", err)
	}

	return &price, nil
}

func (r *Repository) CleanupOldData(ctx context.Context, retentionDays int) error {
	query := `DELETE FROM price_data WHERE created_at < $1`
	cutoffTime := time.Now().AddDate(0, 0, -retentionDays)

	result, err := r.db.ExecContext(ctx, query, cutoffTime)
	if err != nil {
		return fmt.Errorf("failed to cleanup old data: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	r.logger.WithFields(logrus.Fields{
		"rows_deleted":   rowsAffected,
		"cutoff_time":    cutoffTime,
		"retention_days": retentionDays,
	}).Info("Cleaned up old price data")

	return nil
}

func (r *Repository) UpdateTradingPairs(ctx context.Context, symbols []string) error {
	if len(symbols) == 0 {
		return nil
	}

	// Build bulk upsert for trading pairs
	query := `
        INSERT INTO trading_pairs (symbol, base_asset, quote_asset, status, last_updated)
        VALUES `

	values := make([]string, 0, len(symbols))
	args := make([]interface{}, 0, len(symbols)*5)

	for i, symbol := range symbols {
		// Parse symbol to get base and quote assets (e.g., BTC-USDT -> BTC, USDT)
		parts := strings.Split(symbol, "-")
		if len(parts) != 2 {
			continue
		}

		baseAsset := parts[0]
		quoteAsset := parts[1]

		values = append(values, fmt.Sprintf("($%d, $%d, $%d, $%d, $%d)",
			i*5+1, i*5+2, i*5+3, i*5+4, i*5+5))

		args = append(args, symbol, baseAsset, quoteAsset, "active", time.Now())
	}

	if len(values) == 0 {
		return nil
	}

	query += strings.Join(values, ", ")
	query += ` ON CONFLICT (symbol) DO UPDATE SET 
        last_updated = EXCLUDED.last_updated,
        status = CASE WHEN trading_pairs.status = 'inactive' THEN 'active' ELSE trading_pairs.status END`

	_, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		r.logger.WithError(err).Error("Failed to update trading pairs")
		return fmt.Errorf("failed to update trading pairs: %w", err)
	}

	r.logger.WithField("pairs_count", len(values)).Info("Successfully updated trading pairs")
	return nil
}

package database

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/crypto-trading-bot-v4/pair-selector/pkg/models"
	"github.com/crypto-trading-bot-v4/shared/pkg/database"
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

func (r *Repository) GetTradingPairs(ctx context.Context) ([]models.TradingPair, error) {
	query := `
        SELECT id, symbol, base_asset, quote_asset, status, 
               COALESCE(daily_volume, 0), COALESCE(daily_volume_usdt, 0),
               COALESCE(volatility_score, 0), COALESCE(atr_14, 0),
               COALESCE(correlation_btc, 0), COALESCE(price_change_24h, 0),
               COALESCE(last_price, 0), last_updated, created_at
        FROM trading_pairs 
        WHERE status = 'active'
        ORDER BY daily_volume_usdt DESC NULLS LAST
    `

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query trading pairs: %w", err)
	}
	defer rows.Close()

	var pairs []models.TradingPair
	for rows.Next() {
		var pair models.TradingPair
		err := rows.Scan(
			&pair.ID, &pair.Symbol, &pair.BaseAsset, &pair.QuoteAsset, &pair.Status,
			&pair.DailyVolume, &pair.DailyVolumeUSDT, &pair.VolatilityScore,
			&pair.ATR14, &pair.CorrelationBTC, &pair.PriceChange24h,
			&pair.LastPrice, &pair.LastUpdated, &pair.CreatedAt,
		)
		if err != nil {
			r.logger.WithError(err).Error("Failed to scan trading pair")
			continue
		}
		pairs = append(pairs, pair)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating trading pairs: %w", err)
	}

	return pairs, nil
}

func (r *Repository) GetPriceHistory(ctx context.Context, symbol string, hours int) ([]models.PricePoint, error) {
	query := `
        SELECT timestamp, close, volume, high, low
        FROM price_data 
        WHERE symbol = $1 
          AND timestamp >= NOW() - INTERVAL '%d hours'
        ORDER BY timestamp ASC
    `

	rows, err := r.db.QueryContext(ctx, fmt.Sprintf(query, hours), symbol)
	if err != nil {
		return nil, fmt.Errorf("failed to query price history for %s: %w", symbol, err)
	}
	defer rows.Close()

	var prices []models.PricePoint
	for rows.Next() {
		var price models.PricePoint
		err := rows.Scan(&price.Timestamp, &price.Close, &price.Volume, &price.High, &price.Low)
		if err != nil {
			r.logger.WithError(err).WithField("symbol", symbol).Error("Failed to scan price point")
			continue
		}
		prices = append(prices, price)
	}

	return prices, nil
}

func (r *Repository) UpdateTradingPairMetrics(ctx context.Context, symbol string, metrics map[string]float64) error {
	query := `
        UPDATE trading_pairs 
        SET daily_volume_usdt = $2,
            volatility_score = $3,
            atr_14 = $4,
            correlation_btc = $5,
            last_updated = NOW()
        WHERE symbol = $1
    `

	_, err := r.db.ExecContext(ctx, query, symbol,
		metrics["volume_usdt"], metrics["volatility"],
		metrics["atr_14"], metrics["correlation_btc"])

	if err != nil {
		return fmt.Errorf("failed to update metrics for %s: %w", symbol, err)
	}

	return nil
}

func (r *Repository) GetCurrentSelectedPairs(ctx context.Context) ([]models.SelectedPair, error) {
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
		return nil, fmt.Errorf("failed to query selected pairs: %w", err)
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

func (r *Repository) UpdateSelectedPairs(ctx context.Context, analyses []models.PairAnalysis, criteria models.SelectionCriteria) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Deactivate all current selections
	_, err = tx.ExecContext(ctx, "UPDATE selected_pairs SET status = 'inactive', last_evaluated = NOW()")
	if err != nil {
		return fmt.Errorf("failed to deactivate current selections: %w", err)
	}

	// Insert new selections
	if len(analyses) > 0 {
		query := `
            INSERT INTO selected_pairs 
            (symbol, selection_score, volatility_24h, volume_24h_usdt, atr_score, 
             volume_score, correlation_score, risk_level, status, selected_at, last_evaluated)
            VALUES `

		values := make([]string, 0, len(analyses))
		args := make([]interface{}, 0, len(analyses)*11)

		for i, analysis := range analyses {
			values = append(values, fmt.Sprintf("($%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d)",
				i*11+1, i*11+2, i*11+3, i*11+4, i*11+5, i*11+6, i*11+7, i*11+8, i*11+9, i*11+10, i*11+11))

			args = append(args, analysis.Symbol, analysis.FinalScore, analysis.Volatility,
				analysis.Volume24hUSDT, analysis.ATRScore, analysis.VolumeScore,
				analysis.CorrelationScore, analysis.RiskLevel, "active", time.Now(), time.Now())
		}

		query += strings.Join(values, ", ")
		query += ` ON CONFLICT (symbol) DO UPDATE SET
            selection_score = EXCLUDED.selection_score,
            volatility_24h = EXCLUDED.volatility_24h,
            volume_24h_usdt = EXCLUDED.volume_24h_usdt,
            atr_score = EXCLUDED.atr_score,
            volume_score = EXCLUDED.volume_score,
            correlation_score = EXCLUDED.correlation_score,
            risk_level = EXCLUDED.risk_level,
            status = EXCLUDED.status,
            selected_at = EXCLUDED.selected_at,
            last_evaluated = EXCLUDED.last_evaluated`

		_, err = tx.ExecContext(ctx, query, args...)
		if err != nil {
			return fmt.Errorf("failed to insert selected pairs: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	r.logger.WithField("selected_pairs", len(analyses)).Info("Successfully updated selected pairs")
	return nil
}

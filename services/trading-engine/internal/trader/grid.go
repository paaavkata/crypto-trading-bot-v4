package trader

import (
	"context"
	"math"

	"github.com/paaavkata/crypto-trading-bot-v4/trading-engine/pkg/models"
	"github.com/sirupsen/logrus"
)

type GridStrategy struct {
	logger *logrus.Logger
}

func NewGridStrategy(logger *logrus.Logger) *GridStrategy {
	return &GridStrategy{logger: logger}
}

func (g *GridStrategy) Execute(ctx context.Context, pair models.SelectedPair, config models.TradingConfig,
	signal models.Signal, positions []models.Position, currentPrice float64) error {

	g.logger.WithFields(logrus.Fields{
		"symbol":         pair.Symbol,
		"current_price":  currentPrice,
		"grid_levels":    config.GridLevels,
		"open_positions": len(positions),
	}).Debug("Executing grid strategy")

	// Calculate grid levels if not set
	if config.PriceRangeMin == 0 || config.PriceRangeMax == 0 {
		config.PriceRangeMin = currentPrice * 0.95 // 5% below
		config.PriceRangeMax = currentPrice * 1.05 // 5% above
	}

	gridLevels := g.calculateGridLevels(config, currentPrice)

	// Check for grid opportunities
	for _, level := range gridLevels {
		if g.shouldPlaceOrder(level, currentPrice, positions) {
			if level.Type == "buy" && currentPrice <= level.Price {
				// Place buy order at grid level
				g.logger.WithFields(logrus.Fields{
					"symbol": pair.Symbol,
					"price":  level.Price,
					"type":   "buy",
				}).Info("Placing grid buy order")
				// Implementation would go here
			} else if level.Type == "sell" && currentPrice >= level.Price {
				// Place sell order at grid level
				g.logger.WithFields(logrus.Fields{
					"symbol": pair.Symbol,
					"price":  level.Price,
					"type":   "sell",
				}).Info("Placing grid sell order")
				// Implementation would go here
			}
		}
	}

	return nil
}

func (g *GridStrategy) calculateGridLevels(config models.TradingConfig, currentPrice float64) []models.GridLevel {
	levels := make([]models.GridLevel, 0, config.GridLevels)

	priceRange := config.PriceRangeMax - config.PriceRangeMin
	stepSize := priceRange / float64(config.GridLevels)

	for i := 0; i < config.GridLevels; i++ {
		price := config.PriceRangeMin + (float64(i) * stepSize)
		quantity := config.PositionSizeUSDT / price

		var orderType string
		if price < currentPrice {
			orderType = "buy"
		} else {
			orderType = "sell"
		}

		levels = append(levels, models.GridLevel{
			Price:    price,
			Quantity: quantity,
			Type:     orderType,
			IsActive: true,
		})
	}

	return levels
}

func (g *GridStrategy) shouldPlaceOrder(level models.GridLevel, currentPrice float64, positions []models.Position) bool {
	// Check if we already have an order at this level
	tolerance := 0.001 // 0.1% price tolerance

	for _, position := range positions {
		if math.Abs(position.EntryPrice-level.Price)/level.Price < tolerance {
			return false // Already have position at this level
		}
	}

	return true
}

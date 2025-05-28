package trader

import (
	"github.com/paaavkata/crypto-trading-bot-v4/trading-engine/pkg/models"
	"github.com/shopspring/decimal"
)

// Grid logic now uses decimal.Decimal everywhere
func (g *GridStrategy) calculateGridLevels(config models.TradingConfig, currentPrice decimal.Decimal) []models.GridLevel {
	levels := make([]models.GridLevel, 0, config.GridLevels)

	priceRange := config.PriceRangeMax.Decimal.Sub(config.PriceRangeMin.Decimal)
	stepSize := priceRange.Div(decimal.NewFromInt(int64(config.GridLevels)))

	for i := 0; i < config.GridLevels; i++ {
		price := config.PriceRangeMin.Decimal.Add(stepSize.Mul(decimal.NewFromInt(int64(i))))
		quantity := config.PositionSizeUSDT.Decimal.Div(price)

		var orderType string
		if price.LessThan(currentPrice) {
			orderType = "buy"
		} else {
			orderType = "sell"
		}

		levels = append(levels, models.GridLevel{
			Price:    models.DatabaseDecimalFromDecimal(price),
			Quantity: models.DatabaseDecimalFromDecimal(quantity),
			Type:     orderType,
			IsActive: true,
		})
	}

	return levels
}

func (g *GridStrategy) shouldPlaceOrder(level models.GridLevel, currentPrice decimal.Decimal, positions []models.Position) bool {
	tolerance := decimal.NewFromFloat(0.001) // 0.1% price tolerance

	for _, position := range positions {
		if position.EntryPrice.Decimal.Sub(level.Price.Decimal).Abs().Div(level.Price.Decimal).LessThan(tolerance) {
			return false // Already have position at this level
		}
	}
	return true
}

package signals

import (
	"context"
	"math/rand"
	"time"

	"github.com/paaavkata/crypto-trading-bot-v4/trading-engine/pkg/models"
	"github.com/sirupsen/logrus"
)

type Generator struct {
	logger *logrus.Logger
}

func NewGenerator(logger *logrus.Logger) *Generator {
	return &Generator{logger: logger}
}

func (g *Generator) GenerateSignal(ctx context.Context, symbol string, currentPrice float64) models.Signal {
	// Simple signal generation logic - in practice, this would use technical indicators
	// For now, we'll use a basic random walk with bias towards mean reversion

	action := "HOLD"
	strength := 0.5
	reason := "neutral market conditions"

	// Simple random signal for demonstration
	// In production, this would analyze price patterns, volume, etc.
	randomValue := rand.Float64()

	if randomValue < 0.3 {
		action = "BUY"
		strength = randomValue + 0.5
		reason = "price below moving average"
	} else if randomValue > 0.7 {
		action = "SELL"
		strength = (1.0 - randomValue) + 0.3
		reason = "price above moving average"
	}

	signal := models.Signal{
		Symbol:    symbol,
		Action:    action,
		Price:     currentPrice,
		Strength:  strength,
		Timestamp: time.Now(),
		Reason:    reason,
	}

	g.logger.WithFields(logrus.Fields{
		"symbol":   symbol,
		"action":   action,
		"strength": strength,
		"price":    currentPrice,
		"reason":   reason,
	}).Debug("Generated trading signal")

	return signal

}

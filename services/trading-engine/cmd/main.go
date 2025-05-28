package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	tradeDB "github.com/paaavkata/crypto-trading-bot-v4/shared/pkg/database"
	"github.com/paaavkata/crypto-trading-bot-v4/shared/pkg/kucoin"
	"github.com/paaavkata/crypto-trading-bot-v4/shared/pkg/utils"

	"github.com/paaavkata/crypto-trading-bot-v4/trading-engine/internal/config"
	"github.com/paaavkata/crypto-trading-bot-v4/trading-engine/internal/database"
	"github.com/paaavkata/crypto-trading-bot-v4/trading-engine/internal/exchange"
	"github.com/paaavkata/crypto-trading-bot-v4/trading-engine/internal/signals"
	"github.com/paaavkata/crypto-trading-bot-v4/trading-engine/internal/trader"

	"github.com/sirupsen/logrus"
)

func main() {
	// Initialize logger
	logger := utils.NewLogger("trading-engine")

	// Load configuration
	cfg := config.Load()
	logger.WithFields(logrus.Fields{
		"db_url":                 cfg.Database.DbUri,
		"trading_interval":       cfg.TradingInterval,
		"max_positions_per_pair": cfg.MaxPositionsPerPair,
		"default_position_size":  cfg.DefaultPositionSize,
	}).Info("Configuration loaded")

	// Initialize database connection
	db, err := tradeDB.NewConnection(cfg.Database.DbUri, logger)
	if err != nil {
		logger.WithError(err).Fatal("Failed to connect to database")
	}
	defer db.Close()

	// Initialize KuCoin client
	kucoinClient := kucoin.NewClient(cfg.KuCoin, logger)

	// Initialize services
	repo := database.NewRepository(db, logger)
	kucoinExchange := exchange.NewKuCoinExchange(kucoinClient, logger)
	signalGenerator := signals.NewGenerator(logger)

	// Initialize trading engine
	engineConfig := trader.EngineConfig{
		MaxPositionsPerPair: cfg.MaxPositionsPerPair,
		DefaultPositionSize: cfg.DefaultPositionSize,
		StopLossPercent:     cfg.StopLossPercent,
		TakeProfitPercent:   cfg.TakeProfitPercent,
	}

	engine := trader.NewEngine(repo, kucoinExchange, signalGenerator, engineConfig, logger)

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start the trading engine
	go func() {
		if err := engine.Run(ctx); err != nil {
			logger.WithError(err).Error("Trading engine stopped with error")
		}
	}()

	logger.Info("Trading engine service started successfully")

	// Wait for interrupt signal to gracefully shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down trading engine service...")

	// Cancel context to stop trading engine
	cancel()

	// Give some time for graceful shutdown
	time.Sleep(2 * time.Second)

	logger.Info("Trading engine service stopped")
}

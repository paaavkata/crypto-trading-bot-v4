package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/paaavkata/crypto-trading-bot-v4/shared/pkg/database"
	"github.com/paaavkata/crypto-trading-bot-v4/shared/pkg/kucoin"
	"github.com/paaavkata/crypto-trading-bot-v4/shared/pkg/utils"

	"github.com/paaavkata/crypto-trading-bot-v4/price-collector/internal/collector"
	"github.com/paaavkata/crypto-trading-bot-v4/price-collector/internal/config"
	priceDB "github.com/paaavkata/crypto-trading-bot-v4/price-collector/internal/database"
	"github.com/paaavkata/crypto-trading-bot-v4/price-collector/internal/health"

	"github.com/sirupsen/logrus"
)

func main() {
	// Initialize logger
	logger := utils.NewLogger("price-collector")

	// Load configuration
	cfg := config.Load()
	logger.WithFields(logrus.Fields{
		"db_uri":              cfg.Database.DbUri,
		"collection_interval": cfg.CollectionInterval,
		"batch_size":          cfg.BatchSize,
	}).Info("Configuration loaded")

	// Initialize database connection
	db, err := database.NewConnection(cfg.Database.DbUri, logger)
	if err != nil {
		logger.WithError(err).Fatal("Failed to connect to database")
	}
	defer db.Close()

	// Initialize KuCoin client
	kucoinClient := kucoin.NewClient(cfg.KuCoin, logger)

	// Initialize repositories and services
	repo := priceDB.NewRepository(db, logger)
	fetcher := collector.NewFetcher(kucoinClient, logger)
	processor := collector.NewProcessor(repo, logger, cfg.DataRetentionDays)
	scheduler := collector.NewScheduler(fetcher, processor, cfg.CollectionInterval, logger)

	// Initialize health checker
	healthChecker := health.NewHealthChecker(db, logger)
	healthServer := healthChecker.StartServer(cfg.MetricsPort)

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start the scheduler
	if err := scheduler.Start(ctx); err != nil {
		logger.WithError(err).Fatal("Failed to start scheduler")
	}

	logger.Info("Price collector service started successfully")

	// Wait for interrupt signal to gracefully shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down price collector service...")

	// Stop scheduler
	scheduler.Stop()

	// Shutdown health server
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := healthServer.Shutdown(shutdownCtx); err != nil {
		logger.WithError(err).Error("Failed to shutdown health server gracefully")
	}

	// Cancel context
	cancel()

	logger.Info("Price collector service stopped")
}

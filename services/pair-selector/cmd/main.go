package main

import (
	"context"
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/paaavkata/crypto-trading-bot-v4/shared/pkg/database"
	"github.com/paaavkata/crypto-trading-bot-v4/shared/pkg/utils"

	"github.com/paaavkata/crypto-trading-bot-v4/pair-selector/internal/config"
	pairDB "github.com/paaavkata/crypto-trading-bot-v4/pair-selector/internal/database"
	"github.com/paaavkata/crypto-trading-bot-v4/pair-selector/internal/health" // Import health package
	"github.com/paaavkata/crypto-trading-bot-v4/pair-selector/internal/scheduler"
	"github.com/paaavkata/crypto-trading-bot-v4/pair-selector/internal/selector"

	"github.com/sirupsen/logrus"
)

func main() {
	// Initialize logger
	logger := utils.NewLogger("pair-selector")

	// Load configuration
	cfg := config.Load()
	logger.WithFields(logrus.Fields{
		"db_uri":              cfg.Database.DbUri,
		"evaluation_interval": cfg.EvaluationInterval,
		"min_volume_usdt":     cfg.SelectionCriteria.MinVolumeUSDT,
		"max_active_pairs":    cfg.SelectionCriteria.MaxActivesPairs,
	}).Info("Configuration loaded")

	// Initialize database connection
	db, err := database.NewConnection(cfg.Database.DbUri, logger)
	if err != nil {
		logger.WithError(err).Fatal("Failed to connect to database")
	}
	defer db.Close()

	// Initialize repositories and services
	repo := pairDB.NewRepository(db, logger)
	analyzer := selector.NewAnalyzer(repo, logger)
	pairScheduler := scheduler.NewScheduler(analyzer, repo, cfg.SelectionCriteria, cfg.EvaluationInterval, logger)

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start the scheduler
	if err := pairScheduler.Start(ctx); err != nil {
		logger.WithError(err).Fatal("Failed to start scheduler")
	}

	// Initialize and start health checker server
	healthChecker := health.NewHealthChecker(logger) // Pass logger, and db if needed later
	healthServer := healthChecker.StartServer("8082") // Port for pair-selector health checks

	logger.Info("Pair selector service started successfully")

	// Wait for interrupt signal to gracefully shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down pair selector service...")

	// Stop scheduler
	pairScheduler.Stop()

	// Shutdown health server
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := healthServer.Shutdown(shutdownCtx); err != nil {
		logger.WithError(err).Error("Failed to shutdown health server gracefully for pair-selector")
	}

	// Cancel context
	cancel()

	logger.Info("Pair selector service stopped")
}

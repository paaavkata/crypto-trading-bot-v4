package scheduler

import (
	"context"
	"time"

	"github.com/paaavkata/crypto-trading-bot-v4/pair-selector/internal/database"
	"github.com/paaavkata/crypto-trading-bot-v4/pair-selector/internal/selector"
	"github.com/paaavkata/crypto-trading-bot-v4/pair-selector/pkg/models"
	"github.com/robfig/cron/v3"
	"github.com/sirupsen/logrus"
)

type Scheduler struct {
	analyzer *selector.Analyzer
	repo     *database.Repository
	cron     *cron.Cron
	criteria models.SelectionCriteria
	logger   *logrus.Logger
	interval time.Duration
}

func NewScheduler(analyzer *selector.Analyzer, repo *database.Repository, criteria models.SelectionCriteria, interval time.Duration, logger *logrus.Logger) *Scheduler {
	cronScheduler := cron.New(cron.WithSeconds())

	return &Scheduler{
		analyzer: analyzer,
		repo:     repo,
		cron:     cronScheduler,
		criteria: criteria,
		logger:   logger,
		interval: interval,
	}
}

func (s *Scheduler) Start(ctx context.Context) error {
	s.logger.WithField("interval", s.interval).Info("Starting pair selection scheduler")

	// Schedule pair selection every N hours
	cronExpr := "0 0 */4 * * *" // Every 4 hours
	if s.interval == 6*time.Hour {
		cronExpr = "0 0 */6 * * *" // Every 6 hours
	}

	_, err := s.cron.AddFunc(cronExpr, func() {
		s.selectPairs(ctx)
	})
	if err != nil {
		return err
	}

	s.cron.Start()

	// Run initial selection
	go s.selectPairs(ctx)

	s.logger.Info("Pair selection scheduler started successfully")
	return nil
}

func (s *Scheduler) Stop() {
	s.logger.Info("Stopping pair selection scheduler")
	s.cron.Stop()
}

func (s *Scheduler) selectPairs(ctx context.Context) {
	start := time.Now()
	s.logger.Info("Starting pair selection cycle")

	// Analyze all pairs
	analyses, err := s.analyzer.AnalyzePairs(ctx, s.criteria)
	if err != nil {
		s.logger.WithError(err).Error("Failed to analyze pairs")
		return
	}

	// Select top pairs for active trading
	selectedPairs := s.analyzer.SelectTopPairs(analyses, s.criteria.MaxActivesPairs)

	// Update selected pairs in database
	if err := s.repo.UpdateSelectedPairs(ctx, selectedPairs, s.criteria); err != nil {
		s.logger.WithError(err).Error("Failed to update selected pairs")
		return
	}

	duration := time.Since(start)
	s.logger.WithFields(logrus.Fields{
		"duration_ms":      duration.Milliseconds(),
		"analyzed_pairs":   len(analyses),
		"selected_pairs":   len(selectedPairs),
		"watchlist_size":   s.criteria.WatchlistSize,
		"max_active_pairs": s.criteria.MaxActivesPairs,
	}).Info("Pair selection cycle completed successfully")

	// Log selected pairs for monitoring
	for i, pair := range selectedPairs {
		s.logger.WithFields(logrus.Fields{
			"rank":            i + 1,
			"symbol":          pair.Symbol,
			"final_score":     pair.FinalScore,
			"volume_24h_usdt": pair.Volume24hUSDT,
			"volatility":      pair.Volatility,
			"risk_level":      pair.RiskLevel,
		}).Info("Selected trading pair")
	}
}

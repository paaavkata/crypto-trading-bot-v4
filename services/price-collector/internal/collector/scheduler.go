package collector

import (
	"context"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/sirupsen/logrus"
)

type Scheduler struct {
	fetcher   *Fetcher
	processor *Processor
	cron      *cron.Cron
	logger    *logrus.Logger
	interval  time.Duration
}

func NewScheduler(fetcher *Fetcher, processor *Processor, interval time.Duration, logger *logrus.Logger) *Scheduler {
	cronScheduler := cron.New(cron.WithSeconds())

	return &Scheduler{
		fetcher:   fetcher,
		processor: processor,
		cron:      cronScheduler,
		logger:    logger,
		interval:  interval,
	}
}

func (s *Scheduler) Start(ctx context.Context) error {
	s.logger.WithField("interval", s.interval).Info("Starting price collection scheduler")

	// Schedule price collection
	_, err := s.cron.AddFunc("0 * * * * *", func() {
		s.collectPrices(ctx)
	})
	if err != nil {
		return err
	}

	// Schedule cleanup daily at 2 AM
	_, err = s.cron.AddFunc("0 0 2 * * *", func() {
		s.cleanupData(ctx)
	})
	if err != nil {
		return err
	}

	s.cron.Start()

	// Run initial collection
	go s.collectPrices(ctx)

	s.logger.Info("Price collection scheduler started successfully")
	return nil
}

func (s *Scheduler) Stop() {
	s.logger.Info("Stopping price collection scheduler")
	s.cron.Stop()
}

func (s *Scheduler) collectPrices(ctx context.Context) {
	start := time.Now()
	s.logger.Info("Starting price collection cycle")

	// Fetch all tickers
	tickers, err := s.fetcher.FetchAllTickers(ctx)
	if err != nil {
		s.logger.WithError(err).Error("Failed to fetch tickers")
		return
	}

	// Process and store tickers
	if err := s.processor.ProcessTickers(ctx, tickers); err != nil {
		s.logger.WithError(err).Error("Failed to process tickers")
		return
	}

	duration := time.Since(start)
	s.logger.WithFields(logrus.Fields{
		"duration_ms":   duration.Milliseconds(),
		"tickers_count": len(tickers),
	}).Info("Price collection cycle completed successfully")
}

func (s *Scheduler) cleanupData(ctx context.Context) {
	s.logger.Info("Starting data cleanup cycle")

	if err := s.processor.CleanupOldData(ctx); err != nil {
		s.logger.WithError(err).Error("Failed to cleanup old data")
		return
	}

	s.logger.Info("Data cleanup cycle completed successfully")
}

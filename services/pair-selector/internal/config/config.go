package config

import (
	"os"
	"strconv"
	"time"

	"github.com/paaavkata/crypto-trading-bot-v4/pair-selector/pkg/models"
	"github.com/paaavkata/crypto-trading-bot-v4/shared/pkg/database"
)

type Config struct {
	Database           database.Config
	SelectionCriteria  models.SelectionCriteria
	EvaluationInterval time.Duration
	MetricsPort        string
}

func Load() *Config {
	return &Config{
		Database: database.Config{
			DbUri: getEnv("DB_URI", "localhost"),
		},
		SelectionCriteria: models.SelectionCriteria{
			MinVolumeUSDT:     getEnvFloat("MIN_VOLUME_USDT", 1000000), // $1M
			MaxVolatility:     getEnvFloat("MAX_VOLATILITY", 0.08),     // 8%
			MinVolatility:     getEnvFloat("MIN_VOLATILITY", 0.03),     // 3%
			MaxActivesPairs:   getEnvInt("MAX_ACTIVE_PAIRS", 8),
			WatchlistSize:     getEnvInt("WATCHLIST_SIZE", 20),
			VolumeWeight:      getEnvFloat("VOLUME_WEIGHT", 0.30),
			VolatilityWeight:  getEnvFloat("VOLATILITY_WEIGHT", 0.25),
			ATRWeight:         getEnvFloat("ATR_WEIGHT", 0.25),
			CorrelationWeight: getEnvFloat("CORRELATION_WEIGHT", 0.20),
		},
		EvaluationInterval: time.Duration(getEnvInt("EVALUATION_INTERVAL_HOURS", 4)) * time.Hour,
		MetricsPort:        getEnv("METRICS_PORT", "8081"),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvFloat(key string, defaultValue float64) float64 {
	if value := os.Getenv(key); value != "" {
		if floatValue, err := strconv.ParseFloat(value, 64); err == nil {
			return floatValue
		}
	}
	return defaultValue
}

package config

import (
	"os"
	"strconv"
	"time"

	"github.com/paaavkata/crypto-trading-bot-v4/shared/pkg/database"
	"github.com/paaavkata/crypto-trading-bot-v4/shared/pkg/kucoin"
)

type Config struct {
	Database           database.Config
	KuCoin             kucoin.Config
	CollectionInterval time.Duration
	BatchSize          int
	MetricsPort        string
}

func Load() *Config {
	return &Config{
		Database: database.Config{
			DbUri: getEnv("DB_URI", "localhost"),
		},
		KuCoin: kucoin.Config{
			APIKey:     getEnv("KUCOIN_API_KEY", ""),
			APISecret:  getEnv("KUCOIN_API_SECRET", ""),
			Passphrase: getEnv("KUCOIN_PASSPHRASE", ""),
			Sandbox:    getEnvBool("KUCOIN_SANDBOX", false),
		},
		CollectionInterval: time.Duration(getEnvInt("COLLECTION_INTERVAL_SECONDS", 60)) * time.Second,
		BatchSize:          getEnvInt("BATCH_SIZE", 1000),
		MetricsPort:        getEnv("METRICS_PORT", "8080"),
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

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}

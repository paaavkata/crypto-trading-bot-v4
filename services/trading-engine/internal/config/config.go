package config

import (
	"os"
	"strconv"
	"time"

	"github.com/paaavkata/crypto-trading-bot-v4/shared/pkg/database"
	"github.com/paaavkata/crypto-trading-bot-v4/shared/pkg/kucoin"
)

type Config struct {
	Database            database.Config
	KuCoin              kucoin.Config
	TradingInterval     time.Duration
	MaxPositionsPerPair int
	DefaultPositionSize float64
	StopLossPercent     float64
	TakeProfitPercent   float64
	MetricsPort         string
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
		TradingInterval:     time.Duration(getEnvInt("TRADING_INTERVAL_SECONDS", 30)) * time.Second,
		MaxPositionsPerPair: getEnvInt("MAX_POSITIONS_PER_PAIR", 5),
		DefaultPositionSize: getEnvFloat("DEFAULT_POSITION_SIZE_USDT", 100.0),
		StopLossPercent:     getEnvFloat("STOP_LOSS_PERCENT", 0.05),   // 5%
		TakeProfitPercent:   getEnvFloat("TAKE_PROFIT_PERCENT", 0.03), // 3%
		MetricsPort:         getEnv("METRICS_PORT", "8082"),
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

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}

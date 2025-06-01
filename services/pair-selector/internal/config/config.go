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
			// MinVolumeUSDT defines the minimum 24-hour trading volume in USDT for a pair to be considered.
			// Default: 1,000,000 ($1M). Pairs below this are generally considered too illiquid.
			MinVolumeUSDT: getEnvFloat("MIN_VOLUME_USDT", 1000000),

			// MaxVolatility defines the maximum allowed volatility for a pair.
			// Volatility is calculated as the standard deviation of 1-minute price returns over the past 24 hours.
			// Default: 0.08 (8%).
			// Purpose: Aims to cap risk by filtering out pairs that are excessively erratic or experiencing
			// extreme price swings. Such pairs might be harder to trade predictably or could lead to larger losses.
			// Note: This value is sensitive and may need adjustment based on market conditions (e.g., bull/bear)
			// and trading strategy's risk tolerance. Empirical testing is recommended.
			MaxVolatility: getEnvFloat("MAX_VOLATILITY", 0.08),

			// MinVolatility defines the minimum required volatility for a pair.
			// Volatility is calculated as the standard deviation of 1-minute price returns over the past 24 hours.
			// Default: 0.03 (3%).
			// Purpose: Aims to filter out pairs that are too stagnant or have very low price movement,
			// as they might offer fewer trading opportunities.
			// Note: Similar to MaxVolatility, this is sensitive and may need tuning based on market conditions
			// and strategy. Backtesting or empirical observation can help find optimal values.
			MinVolatility: getEnvFloat("MIN_VOLATILITY", 0.03),

			MaxActivesPairs: getEnvInt("MAX_ACTIVE_PAIRS", 8),
			WatchlistSize:   getEnvInt("WATCHLIST_SIZE", 20),
			VolumeWeight:      getEnvFloat("VOLUME_WEIGHT", 0.30),
			VolatilityWeight:  getEnvFloat("VOLATILITY_WEIGHT", 0.25),
			ATRWeight:         getEnvFloat("ATR_WEIGHT", 0.25),
			CorrelationWeight: getEnvFloat("CORRELATION_WEIGHT", 0.20),
			ATRPeriod:         getEnvInt("ATR_PERIOD", 60),
			RiskThresholds: models.RiskThresholdsConfig{
				VolatilityRisk: models.VolatilityRiskConfig{
					Weight: getEnvFloat("RISK_VOL_WEIGHT", 0.35),
					Band1:  getEnvFloat("RISK_VOL_BAND1", 0.12),
					Score1: getEnvFloat("RISK_VOL_SCORE1", 4.0),
					Band2:  getEnvFloat("RISK_VOL_BAND2", 0.08),
					Score2: getEnvFloat("RISK_VOL_SCORE2", 3.0),
					Band3:  getEnvFloat("RISK_VOL_BAND3", 0.05),
					Score3: getEnvFloat("RISK_VOL_SCORE3", 2.0),
					Band4:  getEnvFloat("RISK_VOL_BAND4", 0.03),
					Score4: getEnvFloat("RISK_VOL_SCORE4", 1.5),
					Score5: getEnvFloat("RISK_VOL_SCORE5", 1.0),
				},
				CorrelationRisk: models.CorrelationRiskConfig{
					Weight: getEnvFloat("RISK_CORR_WEIGHT", 0.25),
					Band1:  getEnvFloat("RISK_CORR_BAND1", 0.2),
					Score1: getEnvFloat("RISK_CORR_SCORE1", 4.0),
					Band2:  getEnvFloat("RISK_CORR_BAND2", 0.4),
					Score2: getEnvFloat("RISK_CORR_SCORE2", 3.0),
					Band3:  getEnvFloat("RISK_CORR_BAND3", 0.6),
					Score3: getEnvFloat("RISK_CORR_SCORE3", 2.0),
					Score4: getEnvFloat("RISK_CORR_SCORE4", 1.0),
				},
				VolumeRisk: models.VolumeRiskConfig{
					Weight: getEnvFloat("RISK_VOLU_WEIGHT", 0.20),
					Band1:  getEnvFloat("RISK_VOLU_BAND1", 1000000),
					Score1: getEnvFloat("RISK_VOLU_SCORE1", 4.0),
					Band2:  getEnvFloat("RISK_VOLU_BAND2", 3000000),
					Score2: getEnvFloat("RISK_VOLU_SCORE2", 3.0),
					Band3:  getEnvFloat("RISK_VOLU_BAND3", 10000000),
					Score3: getEnvFloat("RISK_VOLU_SCORE3", 2.0),
					Score4: getEnvFloat("RISK_VOLU_SCORE4", 1.0),
				},
				ATRRisk: models.ATRRiskConfig{
					Weight:            getEnvFloat("RISK_ATR_WEIGHT", 0.10),
					DefaultScoreNoVol: getEnvFloat("RISK_ATR_DEF_SCORE_NO_VOL", 2.0),
					RatioBand1:        getEnvFloat("RISK_ATR_RATIO_BAND1", 2.0),
					Score1:            getEnvFloat("RISK_ATR_SCORE1", 3.0),
					RatioBand2:        getEnvFloat("RISK_ATR_RATIO_BAND2", 1.5),
					Score2:            getEnvFloat("RISK_ATR_SCORE2", 2.0),
					Score3:            getEnvFloat("RISK_ATR_SCORE3", 1.0),
				},
				MomentumRisk: models.MomentumRiskConfig{
					Weight:              getEnvFloat("RISK_MOM_WEIGHT", 0.10),
					MinDataPoints:       getEnvInt("RISK_MOM_MIN_POINTS", 10),
					DefaultScoreForSafe: getEnvFloat("RISK_MOM_DEF_SCORE_SAFE", 2.0),
					RecentPeriods:       getEnvInt("RISK_MOM_RECENT_PERIODS", 5),
					OlderPeriodsStart:   getEnvInt("RISK_MOM_OLDER_START", 5), // Note: This is an index, typically. Or count from recent.
					OlderPeriodsEnd:     getEnvInt("RISK_MOM_OLDER_END", 10),   // Note: This is an exclusive end index or count.
					ChangeBand1:         getEnvFloat("RISK_MOM_CHANGE_BAND1", 0.1),
					Score1:              getEnvFloat("RISK_MOM_SCORE1", 3.0),
					ChangeBand2:         getEnvFloat("RISK_MOM_CHANGE_BAND2", 0.05),
					Score2:              getEnvFloat("RISK_MOM_SCORE2", 2.0),
					Score3:              getEnvFloat("RISK_MOM_SCORE3", 1.0),
				},
				OverallRisk: models.OverallRiskConfig{
					HighThreshold:   getEnvFloat("RISK_OVERALL_HIGH_THRESH", 0.75),
					MediumThreshold: getEnvFloat("RISK_OVERALL_MED_THRESH", 0.5),
				},
			},
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

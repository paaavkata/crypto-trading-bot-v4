package utils

import (
	"os"

	"github.com/sirupsen/logrus"
)

func NewLogger(service string) *logrus.Logger {
	logger := logrus.New()

	// Set log level based on environment
	level := os.Getenv("LOG_LEVEL")
	switch level {
	case "debug":
		logger.SetLevel(logrus.DebugLevel)
	case "info":
		logger.SetLevel(logrus.InfoLevel)
	case "warn":
		logger.SetLevel(logrus.WarnLevel)
	case "error":
		logger.SetLevel(logrus.ErrorLevel)
	default:
		logger.SetLevel(logrus.InfoLevel)
	}

	// Set JSON formatter for production
	if os.Getenv("ENVIRONMENT") == "production" {
		logger.SetFormatter(&logrus.JSONFormatter{})
	} else {
		logger.SetFormatter(&logrus.TextFormatter{
			FullTimestamp: true,
			ForceColors:   true,
		})
	}

	// Add service name to all log entries
	logger = logger.WithField("service", service).Logger

	return logger
}

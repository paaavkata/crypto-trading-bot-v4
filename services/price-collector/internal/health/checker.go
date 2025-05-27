package health

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/paaavkata/crypto-trading-bot-v4/shared/pkg/database"
	"github.com/sirupsen/logrus"
)

type HealthChecker struct {
	db     *database.DB
	logger *logrus.Logger
}

type HealthStatus struct {
	Status    string            `json:"status"`
	Timestamp time.Time         `json:"timestamp"`
	Services  map[string]string `json:"services"`
}

func NewHealthChecker(db *database.DB, logger *logrus.Logger) *HealthChecker {
	return &HealthChecker{
		db:     db,
		logger: logger,
	}
}

func (h *HealthChecker) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		status := h.CheckHealth(ctx)

		w.Header().Set("Content-Type", "application/json")
		if status.Status == "healthy" {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
		}

		json.NewEncoder(w).Encode(status)
	}
}

func (h *HealthChecker) CheckHealth(ctx context.Context) HealthStatus {
	services := make(map[string]string)
	overallStatus := "healthy"

	// Check database
	if err := h.db.HealthCheck(); err != nil {
		services["database"] = "unhealthy: " + err.Error()
		overallStatus = "unhealthy"
		h.logger.WithError(err).Error("Database health check failed")
	} else {
		services["database"] = "healthy"
	}

	return HealthStatus{
		Status:    overallStatus,
		Timestamp: time.Now(),
		Services:  services,
	}
}

func (h *HealthChecker) StartServer(port string) *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", h.Handler())
	mux.HandleFunc("/ready", h.Handler()) // Kubernetes readiness probe

	server := &http.Server{
		Addr:         ":" + port,
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
	}

	go func() {
		h.logger.WithField("port", port).Info("Starting health check server")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			h.logger.WithError(err).Error("Health check server failed")
		}
	}()

	return server
}

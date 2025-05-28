package health

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
)

// HealthChecker defines a basic health checker.
// For pair-selector, it's currently a simple OK response.
// It could be expanded to check DB connectivity if needed, similar to price-collector.
type HealthChecker struct {
	logger *logrus.Logger
	// db *database.DB // Example: if DB check is needed later
}

// HealthStatus defines the structure for health check responses.
type HealthStatus struct {
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
	// Services  map[string]string `json:"services,omitempty"` // Example
}

// NewHealthChecker creates a new HealthChecker.
func NewHealthChecker(logger *logrus.Logger) *HealthChecker {
	return &HealthChecker{
		logger: logger,
	}
}

// Handler returns an http.HandlerFunc for the health check endpoint.
func (h *HealthChecker) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		status := HealthStatus{
			Status:    "healthy", // Pair-selector basic health check
			Timestamp: time.Now(),
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(status)
	}
}

// StartServer starts the HTTP server for health checks.
func (h *HealthChecker) StartServer(port string) *http.Server {
	mux := http.NewServeMux()
	handler := h.Handler()
	mux.HandleFunc("/healthz", handler) // Standard /healthz endpoint
	// Optionally, add /health and /ready if needed for other compatibility
	mux.HandleFunc("/health", handler)
	mux.HandleFunc("/ready", handler)


	server := &http.Server{
		Addr:         ":" + port,
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
	}

	go func() {
		h.logger.WithField("port", port).Info("Starting health check server for pair-selector")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			h.logger.WithError(err).Error("Health check server failed for pair-selector")
		}
	}()

	return server
}

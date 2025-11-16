package monitoring

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

// HealthStatus represents the health status of the application
type HealthStatus string

const (
	StatusHealthy   HealthStatus = "healthy"
	StatusDegraded  HealthStatus = "degraded"
	StatusUnhealthy HealthStatus = "unhealthy"
)

// ComponentHealth represents the health of a component
type ComponentHealth struct {
	Status  HealthStatus           `json:"status"`
	Message string                 `json:"message,omitempty"`
	Details map[string]interface{} `json:"details,omitempty"`
}

// HealthCheck represents the overall health check response
type HealthCheck struct {
	Status     HealthStatus               `json:"status"`
	Timestamp  time.Time                  `json:"timestamp"`
	Uptime     time.Duration              `json:"uptime"`
	Version    string                     `json:"version"`
	Components map[string]ComponentHealth `json:"components"`
}

// HealthChecker manages health checks for the application
type HealthChecker struct {
	mu         sync.RWMutex
	components map[string]ComponentHealth
	startTime  time.Time
	version    string
}

// NewHealthChecker creates a new health checker
func NewHealthChecker(version string) *HealthChecker {
	return &HealthChecker{
		components: make(map[string]ComponentHealth),
		startTime:  time.Now(),
		version:    version,
	}
}

// RegisterComponent registers a component for health checking
func (h *HealthChecker) RegisterComponent(name string, status HealthStatus, message string, details map[string]interface{}) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.components[name] = ComponentHealth{
		Status:  status,
		Message: message,
		Details: details,
	}
}

// UpdateComponent updates a component's health status
func (h *HealthChecker) UpdateComponent(name string, status HealthStatus, message string, details map[string]interface{}) {
	h.RegisterComponent(name, status, message, details)
}

// GetHealth returns the current health status
func (h *HealthChecker) GetHealth() HealthCheck {
	h.mu.RLock()
	defer h.mu.RUnlock()

	// Determine overall status
	overallStatus := StatusHealthy
	for _, comp := range h.components {
		if comp.Status == StatusUnhealthy {
			overallStatus = StatusUnhealthy
			break
		}
		if comp.Status == StatusDegraded && overallStatus == StatusHealthy {
			overallStatus = StatusDegraded
		}
	}

	// Copy components
	components := make(map[string]ComponentHealth)
	for k, v := range h.components {
		components[k] = v
	}

	return HealthCheck{
		Status:     overallStatus,
		Timestamp:  time.Now().UTC(),
		Uptime:     time.Since(h.startTime),
		Version:    h.version,
		Components: components,
	}
}

// HTTPHandler returns an HTTP handler for health checks
func (h *HealthChecker) HTTPHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		health := h.GetHealth()

		w.Header().Set("Content-Type", "application/json")

		// Set status code based on health
		switch health.Status {
		case StatusHealthy:
			w.WriteHeader(http.StatusOK)
		case StatusDegraded:
			w.WriteHeader(http.StatusOK) // Still 200 but degraded
		case StatusUnhealthy:
			w.WriteHeader(http.StatusServiceUnavailable)
		}

		json.NewEncoder(w).Encode(health)
	}
}

// ReadinessHandler returns an HTTP handler for readiness checks
func (h *HealthChecker) ReadinessHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		health := h.GetHealth()

		w.Header().Set("Content-Type", "application/json")

		// Ready only if all components are healthy
		if health.Status == StatusHealthy {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]string{"status": "ready"})
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(map[string]string{"status": "not ready"})
		}
	}
}

// LivenessHandler returns an HTTP handler for liveness checks
func (h *HealthChecker) LivenessHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "alive"})
	}
}

// Global health checker
var globalHealthChecker *HealthChecker

// InitHealthChecker initializes the global health checker
func InitHealthChecker(version string) {
	globalHealthChecker = NewHealthChecker(version)
}

// GetHealthChecker returns the global health checker
func GetHealthChecker() *HealthChecker {
	if globalHealthChecker == nil {
		globalHealthChecker = NewHealthChecker("unknown")
	}
	return globalHealthChecker
}

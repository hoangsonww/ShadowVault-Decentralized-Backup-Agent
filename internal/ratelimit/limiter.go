package ratelimit

import (
	"context"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/hoangsonww/backupagent/internal/monitoring"
	"golang.org/x/time/rate"
)

// Limiter manages rate limiting for different resources
type Limiter struct {
	mu             sync.RWMutex
	limiters       map[string]*rate.Limiter
	requestsPerSec int
	burst          int
	ipWhitelist    map[string]bool
	enabled        bool
}

// NewLimiter creates a new rate limiter
func NewLimiter(requestsPerSec, burst int, whitelist []string, enabled bool) *Limiter {
	ipMap := make(map[string]bool)
	for _, ip := range whitelist {
		ipMap[ip] = true
	}

	return &Limiter{
		limiters:       make(map[string]*rate.Limiter),
		requestsPerSec: requestsPerSec,
		burst:          burst,
		ipWhitelist:    ipMap,
		enabled:        enabled,
	}
}

// GetLimiter returns a rate limiter for a specific identifier (IP address, user ID, etc.)
func (l *Limiter) GetLimiter(identifier string) *rate.Limiter {
	l.mu.Lock()
	defer l.mu.Unlock()

	limiter, exists := l.limiters[identifier]
	if !exists {
		limiter = rate.NewLimiter(rate.Limit(l.requestsPerSec), l.burst)
		l.limiters[identifier] = limiter
	}

	return limiter
}

// Allow checks if a request should be allowed
func (l *Limiter) Allow(identifier string) bool {
	if !l.enabled {
		return true
	}

	// Check whitelist
	if l.ipWhitelist[identifier] {
		return true
	}

	return l.GetLimiter(identifier).Allow()
}

// Wait blocks until the request can proceed
func (l *Limiter) Wait(ctx context.Context, identifier string) error {
	if !l.enabled {
		return nil
	}

	// Check whitelist
	if l.ipWhitelist[identifier] {
		return nil
	}

	return l.GetLimiter(identifier).Wait(ctx)
}

// Middleware returns an HTTP middleware that applies rate limiting
func (l *Limiter) Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !l.enabled {
				next.ServeHTTP(w, r)
				return
			}

			// Extract IP address
			ip := getIP(r)

			// Check if whitelisted
			if l.ipWhitelist[ip] {
				next.ServeHTTP(w, r)
				return
			}

			// Check rate limit
			limiter := l.GetLimiter(ip)
			if !limiter.Allow() {
				monitoring.GetLogger().WithField("ip", ip).Warn("Rate limit exceeded")
				http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// CleanupOldLimiters removes limiters that haven't been used recently
func (l *Limiter) CleanupOldLimiters() {
	l.mu.Lock()
	defer l.mu.Unlock()

	// In production, you'd track last access time and remove old entries
	// For now, just clear if we have too many
	if len(l.limiters) > 10000 {
		l.limiters = make(map[string]*rate.Limiter)
	}
}

// StartCleanup starts a background goroutine to clean up old limiters
func (l *Limiter) StartCleanup(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				l.CleanupOldLimiters()
			}
		}
	}()
}

// getIP extracts the real IP address from the request
func getIP(r *http.Request) string {
	// Check X-Forwarded-For header
	xff := r.Header.Get("X-Forwarded-For")
	if xff != "" {
		ips := parseXFF(xff)
		if len(ips) > 0 {
			return ips[0]
		}
	}

	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}

	// Fall back to RemoteAddr
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

// parseXFF parses the X-Forwarded-For header
func parseXFF(xff string) []string {
	var ips []string
	for _, ip := range splitAndTrim(xff, ",") {
		if ip != "" {
			ips = append(ips, ip)
		}
	}
	return ips
}

// splitAndTrim splits a string and trims whitespace
func splitAndTrim(s, sep string) []string {
	parts := []string{}
	for i := 0; i < len(s); {
		j := i
		for j < len(s) && s[j] != sep[0] {
			j++
		}
		parts = append(parts, trim(s[i:j]))
		i = j + 1
	}
	return parts
}

// trim removes leading and trailing whitespace
func trim(s string) string {
	start := 0
	end := len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	for start < end && (s[end-1] == ' ' || s[end-1] == '\t') {
		end--
	}
	return s[start:end]
}

// ResourceLimiter limits resource usage (memory, disk, CPU)
type ResourceLimiter struct {
	maxMemoryBytes int64
	maxDiskBytes   int64
	maxGoroutines  int
	mu             sync.RWMutex
	currentMemory  int64
	currentDisk    int64
	goroutineCount int
}

// NewResourceLimiter creates a new resource limiter
func NewResourceLimiter(maxMemoryMB, maxDiskGB, maxGoroutines int) *ResourceLimiter {
	return &ResourceLimiter{
		maxMemoryBytes: int64(maxMemoryMB) * 1024 * 1024,
		maxDiskBytes:   int64(maxDiskGB) * 1024 * 1024 * 1024,
		maxGoroutines:  maxGoroutines,
	}
}

// CheckMemory checks if memory allocation is allowed
func (r *ResourceLimiter) CheckMemory(bytes int64) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.currentMemory+bytes <= r.maxMemoryBytes
}

// AllocateMemory records memory allocation
func (r *ResourceLimiter) AllocateMemory(bytes int64) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.currentMemory+bytes > r.maxMemoryBytes {
		return false
	}
	r.currentMemory += bytes
	return true
}

// ReleaseMemory records memory release
func (r *ResourceLimiter) ReleaseMemory(bytes int64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.currentMemory -= bytes
	if r.currentMemory < 0 {
		r.currentMemory = 0
	}
}

// CheckGoroutine checks if a new goroutine can be started
func (r *ResourceLimiter) CheckGoroutine() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.goroutineCount < r.maxGoroutines
}

// StartGoroutine records a goroutine start
func (r *ResourceLimiter) StartGoroutine() bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.goroutineCount >= r.maxGoroutines {
		return false
	}
	r.goroutineCount++
	return true
}

// EndGoroutine records a goroutine end
func (r *ResourceLimiter) EndGoroutine() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.goroutineCount--
	if r.goroutineCount < 0 {
		r.goroutineCount = 0
	}
}

// GetStats returns current resource usage
func (r *ResourceLimiter) GetStats() map[string]interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return map[string]interface{}{
		"memory_used_mb":   r.currentMemory / (1024 * 1024),
		"memory_limit_mb":  r.maxMemoryBytes / (1024 * 1024),
		"goroutines":       r.goroutineCount,
		"goroutine_limit":  r.maxGoroutines,
		"memory_usage_pct": float64(r.currentMemory) / float64(r.maxMemoryBytes) * 100,
	}
}

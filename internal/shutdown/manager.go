package shutdown

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/hoangsonww/backupagent/internal/monitoring"
)

// Hook represents a cleanup function to run on shutdown
type Hook struct {
	Name     string
	Priority int // Lower priority runs first
	Func     func(context.Context) error
	Timeout  time.Duration
}

// Manager manages graceful shutdown
type Manager struct {
	mu          sync.RWMutex
	hooks       []*Hook
	signals     chan os.Signal
	shutdown    chan struct{}
	done        chan struct{}
	timeout     time.Duration
	logger      *monitoring.Logger
	healthCheck *monitoring.HealthChecker
}

// NewManager creates a new shutdown manager
func NewManager(timeout time.Duration) *Manager {
	return &Manager{
		hooks:       make([]*Hook, 0),
		signals:     make(chan os.Signal, 1),
		shutdown:    make(chan struct{}),
		done:        make(chan struct{}),
		timeout:     timeout,
		logger:      monitoring.GetLogger(),
		healthCheck: monitoring.GetHealthChecker(),
	}
}

// RegisterHook registers a shutdown hook
func (m *Manager) RegisterHook(name string, priority int, timeout time.Duration, fn func(context.Context) error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	hook := &Hook{
		Name:     name,
		Priority: priority,
		Func:     fn,
		Timeout:  timeout,
	}

	// Insert in priority order (lower priority first)
	inserted := false
	for i, h := range m.hooks {
		if priority < h.Priority {
			m.hooks = append(m.hooks[:i], append([]*Hook{hook}, m.hooks[i:]...)...)
			inserted = true
			break
		}
	}

	if !inserted {
		m.hooks = append(m.hooks, hook)
	}

	m.logger.WithFields(map[string]interface{}{
		"hook":     name,
		"priority": priority,
	}).Debug("Shutdown hook registered")
}

// ListenAndWait listens for shutdown signals and waits
func (m *Manager) ListenAndWait() {
	signal.Notify(m.signals, syscall.SIGINT, syscall.SIGTERM)

	sig := <-m.signals
	m.logger.WithField("signal", sig.String()).Info("Received shutdown signal")

	// Update health status
	m.healthCheck.UpdateComponent("shutdown", monitoring.StatusDegraded,
		"Graceful shutdown in progress", nil)

	m.Shutdown()
	<-m.done
}

// Shutdown initiates the shutdown process
func (m *Manager) Shutdown() {
	close(m.shutdown)

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), m.timeout)
	defer cancel()

	m.logger.Info("Starting graceful shutdown")

	m.mu.RLock()
	hooks := make([]*Hook, len(m.hooks))
	copy(hooks, m.hooks)
	m.mu.RUnlock()

	// Execute hooks in priority order
	errors := make([]error, 0)
	for _, hook := range hooks {
		logger := m.logger.WithField("hook", hook.Name)
		logger.Info("Executing shutdown hook")

		// Create hook-specific context with timeout
		hookCtx, hookCancel := context.WithTimeout(ctx, hook.Timeout)

		done := make(chan error, 1)
		go func() {
			done <- hook.Func(hookCtx)
		}()

		select {
		case err := <-done:
			if err != nil {
				logger.WithError(err).Error("Shutdown hook failed")
				errors = append(errors, fmt.Errorf("%s: %w", hook.Name, err))
			} else {
				logger.Info("Shutdown hook completed successfully")
			}
		case <-hookCtx.Done():
			logger.Warn("Shutdown hook timeout")
			errors = append(errors, fmt.Errorf("%s: timeout", hook.Name))
		}

		hookCancel()
	}

	if len(errors) > 0 {
		m.logger.Errorf("Shutdown completed with %d errors", len(errors))
		for _, err := range errors {
			m.logger.Error(err.Error())
		}
	} else {
		m.logger.Info("Graceful shutdown completed successfully")
	}

	close(m.done)
}

// WaitForShutdown blocks until shutdown is initiated
func (m *Manager) WaitForShutdown() {
	<-m.shutdown
}

// Done returns a channel that is closed when shutdown is complete
func (m *Manager) Done() <-chan struct{} {
	return m.done
}

// DefaultHooks returns a set of default shutdown hooks
func DefaultHooks() []*Hook {
	return []*Hook{
		{
			Name:     "stop-accepting-requests",
			Priority: 0,
			Timeout:  5 * time.Second,
			Func: func(ctx context.Context) error {
				monitoring.GetLogger().Info("Stopped accepting new requests")
				return nil
			},
		},
		{
			Name:     "drain-connections",
			Priority: 10,
			Timeout:  30 * time.Second,
			Func: func(ctx context.Context) error {
				monitoring.GetLogger().Info("Draining active connections")
				time.Sleep(5 * time.Second) // Simulate draining
				return nil
			},
		},
		{
			Name:     "flush-metrics",
			Priority: 20,
			Timeout:  10 * time.Second,
			Func: func(ctx context.Context) error {
				monitoring.GetLogger().Info("Flushing metrics")
				return nil
			},
		},
		{
			Name:     "close-database",
			Priority: 90,
			Timeout:  15 * time.Second,
			Func: func(ctx context.Context) error {
				monitoring.GetLogger().Info("Closing database connections")
				return nil
			},
		},
	}
}

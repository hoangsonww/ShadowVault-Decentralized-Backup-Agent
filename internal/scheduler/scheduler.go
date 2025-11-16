package scheduler

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/hoangsonww/backupagent/internal/monitoring"
)

// BackupTask represents a scheduled backup task
type BackupTask struct {
	ID         string
	Path       string
	Interval   time.Duration
	LastRun    time.Time
	NextRun    time.Time
	Enabled    bool
	MaxRetries int
	RetryCount int
}

// Scheduler manages automated backup scheduling
type Scheduler struct {
	mu         sync.RWMutex
	tasks      map[string]*BackupTask
	backupFunc func(string) error
	ctx        context.Context
	cancel     context.CancelFunc
	running    bool
	metrics    *monitoring.Metrics
	logger     *monitoring.Logger
}

// NewScheduler creates a new backup scheduler
func NewScheduler(backupFunc func(string) error) *Scheduler {
	ctx, cancel := context.WithCancel(context.Background())
	return &Scheduler{
		tasks:      make(map[string]*BackupTask),
		backupFunc: backupFunc,
		ctx:        ctx,
		cancel:     cancel,
		metrics:    monitoring.GetMetrics(),
		logger:     monitoring.GetLogger(),
	}
}

// AddTask adds a new scheduled backup task
func (s *Scheduler) AddTask(id, path string, interval time.Duration, maxRetries int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.tasks[id]; exists {
		return fmt.Errorf("task %s already exists", id)
	}

	now := time.Now()
	task := &BackupTask{
		ID:         id,
		Path:       path,
		Interval:   interval,
		LastRun:    time.Time{},
		NextRun:    now.Add(interval),
		Enabled:    true,
		MaxRetries: maxRetries,
		RetryCount: 0,
	}

	s.tasks[id] = task
	s.logger.WithFields(map[string]interface{}{
		"task_id":  id,
		"path":     path,
		"interval": interval.String(),
	}).Info("Backup task added")

	return nil
}

// RemoveTask removes a scheduled backup task
func (s *Scheduler) RemoveTask(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.tasks[id]; !exists {
		return fmt.Errorf("task %s not found", id)
	}

	delete(s.tasks, id)
	s.logger.WithField("task_id", id).Info("Backup task removed")
	return nil
}

// EnableTask enables a scheduled task
func (s *Scheduler) EnableTask(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, exists := s.tasks[id]
	if !exists {
		return fmt.Errorf("task %s not found", id)
	}

	task.Enabled = true
	s.logger.WithField("task_id", id).Info("Backup task enabled")
	return nil
}

// DisableTask disables a scheduled task
func (s *Scheduler) DisableTask(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, exists := s.tasks[id]
	if !exists {
		return fmt.Errorf("task %s not found", id)
	}

	task.Enabled = false
	s.logger.WithField("task_id", id).Info("Backup task disabled")
	return nil
}

// Start starts the scheduler
func (s *Scheduler) Start() {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.mu.Unlock()

	s.logger.Info("Backup scheduler started")

	go s.run()
}

// Stop stops the scheduler
func (s *Scheduler) Stop() {
	s.cancel()
	s.mu.Lock()
	s.running = false
	s.mu.Unlock()
	s.logger.Info("Backup scheduler stopped")
}

// run is the main scheduler loop
func (s *Scheduler) run() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case now := <-ticker.C:
			s.checkAndRunTasks(now)
		}
	}
}

// checkAndRunTasks checks for tasks that need to run
func (s *Scheduler) checkAndRunTasks(now time.Time) {
	s.mu.Lock()
	tasksToRun := make([]*BackupTask, 0)

	for _, task := range s.tasks {
		if task.Enabled && now.After(task.NextRun) {
			tasksToRun = append(tasksToRun, task)
		}
	}
	s.mu.Unlock()

	// Run tasks outside the lock
	for _, task := range tasksToRun {
		go s.runTask(task)
	}
}

// runTask executes a backup task
func (s *Scheduler) runTask(task *BackupTask) {
	logger := s.logger.WithFields(map[string]interface{}{
		"task_id": task.ID,
		"path":    task.Path,
	})

	logger.Info("Running scheduled backup")

	err := s.backupFunc(task.Path)

	s.mu.Lock()
	defer s.mu.Unlock()

	if err != nil {
		task.RetryCount++
		logger.WithError(err).Errorf("Scheduled backup failed (retry %d/%d)",
			task.RetryCount, task.MaxRetries)

		if task.RetryCount < task.MaxRetries {
			// Retry after a delay
			task.NextRun = time.Now().Add(5 * time.Minute)
		} else {
			// Max retries reached, schedule next regular run
			task.LastRun = time.Now()
			task.NextRun = task.LastRun.Add(task.Interval)
			task.RetryCount = 0
			logger.Error("Max retries reached for scheduled backup")
		}
	} else {
		task.LastRun = time.Now()
		task.NextRun = task.LastRun.Add(task.Interval)
		task.RetryCount = 0
		logger.WithField("next_run", task.NextRun.Format(time.RFC3339)).Info("Scheduled backup completed")
	}
}

// GetTasks returns all scheduled tasks
func (s *Scheduler) GetTasks() map[string]*BackupTask {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tasks := make(map[string]*BackupTask)
	for id, task := range s.tasks {
		// Create a copy
		taskCopy := *task
		tasks[id] = &taskCopy
	}
	return tasks
}

// GetTask returns a specific task
func (s *Scheduler) GetTask(id string) (*BackupTask, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	task, exists := s.tasks[id]
	if !exists {
		return nil, fmt.Errorf("task %s not found", id)
	}

	taskCopy := *task
	return &taskCopy, nil
}

// LoadFromConfig loads tasks from configuration
func (s *Scheduler) LoadFromConfig(paths []string, interval time.Duration, maxRetries int) error {
	for i, path := range paths {
		id := fmt.Sprintf("config-task-%d", i)
		if err := s.AddTask(id, path, interval, maxRetries); err != nil {
			return err
		}
	}
	return nil
}

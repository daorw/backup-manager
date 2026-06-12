package scheduler

import (
	"fmt"
	"log"
	"sync"

	"github.com/robfig/cron/v3"
)

// BackupJobFunc is the signature for a backup job execution function.
type BackupJobFunc func(repoID string) error

// Scheduler wraps robfig/cron/v3 to provide per-repo backup scheduling.
type Scheduler struct {
	cron     *cron.Cron
	entries  map[string]cron.EntryID
	backupFn BackupJobFunc
	mu       sync.RWMutex
	started  bool
}

// NewScheduler creates a new Scheduler. If backupFn is nil, jobs will be
// registered as no-ops.
func NewScheduler(backupFn BackupJobFunc) *Scheduler {
	return &Scheduler{
		cron:     cron.New(cron.WithSeconds()),
		entries:  make(map[string]cron.EntryID),
		backupFn: backupFn,
	}
}

// Start starts the cron scheduler. Safe to call multiple times.
func (s *Scheduler) Start() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started {
		return
	}
	s.cron.Start()
	s.started = true
}

// Stop gracefully stops the cron scheduler, waiting for running jobs to finish.
// Safe to call multiple times.
func (s *Scheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.started {
		return
	}
	ctx := s.cron.Stop()
	<-ctx.Done()
	s.started = false
}

// Register adds a cron job for the given repoID. If a job already exists for
// this repoID, it is replaced.
func (s *Scheduler) Register(repoID, cronExpr string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Remove existing entry if any
	if entryID, ok := s.entries[repoID]; ok {
		s.cron.Remove(entryID)
	}

	// Create a new job that calls backupFn with error logging and panic recovery
	repoIDCopy := repoID
	entryID, err := s.cron.AddFunc(cronExpr, func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[scheduler] panic in backup job for repo %s: %v", repoIDCopy, r)
			}
		}()
		if s.backupFn != nil {
			if err := s.backupFn(repoIDCopy); err != nil {
				log.Printf("[scheduler] backup job failed for repo %s: %v", repoIDCopy, err)
			}
		}
	})
	if err != nil {
		return fmt.Errorf("failed to register cron job: %w", err)
	}

	s.entries[repoID] = entryID
	return nil
}

// Unregister removes the cron job for the given repoID, if any.
func (s *Scheduler) Unregister(repoID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if entryID, ok := s.entries[repoID]; ok {
		s.cron.Remove(entryID)
		delete(s.entries, repoID)
	}
}

// IsRegistered returns true if a job is registered for the given repoID.
func (s *Scheduler) IsRegistered(repoID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.entries[repoID]
	return ok
}

// Len returns the number of registered jobs.
func (s *Scheduler) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.entries)
}

package service

import "sync"

// RepoMutexManager provides per-repository mutexes for concurrency control.
// Both BackupService and RollbackService share the same instance to ensure
// backup and rollback operations on the same repo are mutually exclusive.
type RepoMutexManager struct {
	mu sync.Map // repoID -> *sync.Mutex
}

// NewRepoMutexManager creates a new RepoMutexManager.
func NewRepoMutexManager() *RepoMutexManager {
	return &RepoMutexManager{}
}

// Get returns or creates a per-repo mutex for the given repo ID.
func (m *RepoMutexManager) Get(repoID string) *sync.Mutex {
	mu, _ := m.mu.LoadOrStore(repoID, &sync.Mutex{})
	return mu.(*sync.Mutex)
}

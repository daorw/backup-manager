package util

import "sync"

// RepoMutexManager 提供按仓库粒度的互斥锁。
// 备份、回滚以及所有会改动文件系统的链接操作共享同一个实例，
// 保证同一仓库上的这些操作串行执行。
type RepoMutexManager struct {
	mu sync.Map // repoID -> *sync.Mutex
}

// NewRepoMutexManager 创建管理器。
func NewRepoMutexManager() *RepoMutexManager {
	return &RepoMutexManager{}
}

// Get 返回指定仓库的互斥锁，不存在则创建。
func (m *RepoMutexManager) Get(repoID string) *sync.Mutex {
	mu, _ := m.mu.LoadOrStore(repoID, &sync.Mutex{})
	return mu.(*sync.Mutex)
}

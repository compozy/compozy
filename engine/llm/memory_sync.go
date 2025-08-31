package llm

import (
	"sort"
	"sync"
)

// MemorySync provides thread-safe synchronization for memory operations
type MemorySync struct {
	mu     sync.RWMutex
	locks  map[string]*sync.Mutex
	refCnt map[string]int
}

// NewMemorySync creates a new memory synchronization manager
func NewMemorySync() *MemorySync {
	return &MemorySync{
		locks:  make(map[string]*sync.Mutex),
		refCnt: make(map[string]int),
	}
}

// WithLock runs fn while holding the per-memory mutex and ensures proper release.
func (ms *MemorySync) WithLock(memoryID string, fn func()) {
	l := ms.GetLock(memoryID)
	l.Lock()
	defer func() {
		l.Unlock()
		ms.ReleaseLock(memoryID)
	}()
	fn()
}

// WithMultipleLocks runs fn while holding multiple per-memory mutexes and ensures proper release.
// Memory IDs are sorted to prevent deadlocks when multiple goroutines request the same set of locks.
func (ms *MemorySync) WithMultipleLocks(memoryIDs []string, fn func()) {
	if len(memoryIDs) == 0 {
		fn()
		return
	}
	if len(memoryIDs) == 1 {
		ms.WithLock(memoryIDs[0], fn)
		return
	}
	// Sort memory IDs to prevent deadlocks
	sortedIDs := make([]string, len(memoryIDs))
	copy(sortedIDs, memoryIDs)
	sort.Strings(sortedIDs)
	// Remove duplicates while preserving order
	uniqueIDs := make([]string, 0, len(sortedIDs))
	seen := make(map[string]bool)
	for _, id := range sortedIDs {
		if !seen[id] {
			uniqueIDs = append(uniqueIDs, id)
			seen[id] = true
		}
	}
	// Acquire locks in sorted order
	var locks []*sync.Mutex
	for _, id := range uniqueIDs {
		lock := ms.GetLock(id)
		locks = append(locks, lock)
		lock.Lock()
	}
	// Release locks in reverse order when done
	defer func() {
		for i := len(locks) - 1; i >= 0; i-- {
			locks[i].Unlock()
			ms.ReleaseLock(uniqueIDs[i])
		}
	}()
	fn()
}

// GetLock returns a mutex for the given memory ID, creating it if necessary.
// Usage contract:
//
//	l := ms.GetLock(id); l.Lock(); defer func(){ l.Unlock(); ms.ReleaseLock(id) }()
//
// Always ReleaseLock AFTER Unlock. Releasing while locked may create a new mutex
// for the same ID, breaking mutual exclusion guarantees across callers.
func (ms *MemorySync) GetLock(memoryID string) *sync.Mutex {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	if lock, exists := ms.locks[memoryID]; exists {
		ms.refCnt[memoryID]++
		return lock
	}
	lock := &sync.Mutex{}
	ms.locks[memoryID] = lock
	ms.refCnt[memoryID] = 1
	return lock
}

// ReleaseLock decrements the reference count and removes the lock if no longer needed
func (ms *MemorySync) ReleaseLock(memoryID string) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	if cnt, exists := ms.refCnt[memoryID]; exists {
		cnt--
		if cnt <= 0 {
			delete(ms.locks, memoryID)
			delete(ms.refCnt, memoryID)
		} else {
			ms.refCnt[memoryID] = cnt
		}
	}
}

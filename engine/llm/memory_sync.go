package llm

import (
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

// GetLock returns a mutex for the given memory ID, creating it if necessary
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

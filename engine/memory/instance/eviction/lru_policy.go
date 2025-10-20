package eviction

import (
	"context"
	"fmt"
	"hash/fnv"
	"sort"
	"sync"
	"time"

	"github.com/compozy/compozy/engine/llm"
)

// LRUEvictionPolicy implements a Least Recently Used eviction strategy
type LRUEvictionPolicy struct {
	accessTimes sync.Map // messageID -> last access time
	name        string
	mu          sync.RWMutex
}

// messageWithAccess holds a message with its access time information
type messageWithAccess struct {
	message    llm.Message
	accessTime time.Time
	index      int
}

// NewLRUEvictionPolicy creates a new LRU eviction policy
func NewLRUEvictionPolicy(_ context.Context) *LRUEvictionPolicy {
	return &LRUEvictionPolicy{
		name: "lru",
	}
}

// SelectMessagesToEvict selects the least recently used messages for eviction
func (p *LRUEvictionPolicy) SelectMessagesToEvict(
	_ context.Context,
	messages []llm.Message,
	targetCount int,
) []llm.Message {
	if len(messages) <= targetCount || targetCount < 0 {
		return nil
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	// Create array of messages with their access times
	messageAccess := make([]messageWithAccess, len(messages))
	for i, msg := range messages {
		accessTime := p.getLastAccessTime(msg)
		messageAccess[i] = messageWithAccess{
			message:    msg,
			accessTime: accessTime,
			index:      i,
		}
	}
	// Sort by access time (oldest first)
	sort.Slice(messageAccess, func(i, j int) bool {
		// If both have zero time, maintain original order
		if messageAccess[i].accessTime.IsZero() && messageAccess[j].accessTime.IsZero() {
			return messageAccess[i].index < messageAccess[j].index
		}
		// Zero time means never accessed, evict first
		if messageAccess[i].accessTime.IsZero() {
			return true
		}
		if messageAccess[j].accessTime.IsZero() {
			return false
		}
		// Otherwise sort by access time
		return messageAccess[i].accessTime.Before(messageAccess[j].accessTime)
	})
	// Select messages to evict
	evictCount := len(messages) - targetCount
	evicted := make([]llm.Message, evictCount)
	for i := range evictCount {
		evicted[i] = messageAccess[i].message
	}
	return evicted
}

// UpdateAccess updates the last access time for a message
func (p *LRUEvictionPolicy) UpdateAccess(msg llm.Message) {
	msgID := p.generateMessageID(msg)
	p.accessTimes.Store(msgID, time.Now())
}

// UpdateAccessBatch updates access times for multiple messages
func (p *LRUEvictionPolicy) UpdateAccessBatch(messages []llm.Message) {
	now := time.Now()
	for _, msg := range messages {
		msgID := p.generateMessageID(msg)
		p.accessTimes.Store(msgID, now)
	}
}

// getLastAccessTime retrieves the last access time for a message
func (p *LRUEvictionPolicy) getLastAccessTime(msg llm.Message) time.Time {
	msgID := p.generateMessageID(msg)
	if accessTime, ok := p.accessTimes.Load(msgID); ok {
		if t, ok := accessTime.(time.Time); ok {
			return t
		}
	}
	return time.Time{} // Never accessed, evict first
}

// generateMessageID creates a unique identifier for a message
func (p *LRUEvictionPolicy) generateMessageID(msg llm.Message) string {
	// Use content hash for consistent ID generation
	h := fnv.New64a()
	fmt.Fprintf(h, "%v:%s", msg.Role, msg.Content)
	return fmt.Sprintf("%d", h.Sum64())
}

// ClearAccessHistory clears all access time records
func (p *LRUEvictionPolicy) ClearAccessHistory() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.accessTimes.Range(func(key, _ any) bool {
		p.accessTimes.Delete(key)
		return true
	})
}

// GetType returns the policy type
func (p *LRUEvictionPolicy) GetType() string {
	return p.name
}

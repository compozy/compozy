package bridgesdk

import (
	"strings"
	"sync"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
)

const (
	// ParentMessageCacheLimit bounds provider-local reply context retained in memory.
	ParentMessageCacheLimit = 4000
	// ParentMessageTextLimit bounds the text copied into one reply-context entry.
	ParentMessageTextLimit = 500
)

// ParentMessageKey isolates one provider message inside its workspace conversation.
type ParentMessageKey struct {
	WorkspaceID      string
	BridgeInstanceID string
	ConversationID   string
	MessageID        string
}

// ParentMessage is the bounded reply context retained for one inbound message.
type ParentMessage struct {
	Text       string
	AuthorID   string
	AuthorName string
}

type parentMessageCacheEntry struct {
	key      ParentMessageKey
	message  ParentMessage
	previous *parentMessageCacheEntry
	next     *parentMessageCacheEntry
}

// ParentMessageCache retains a deterministic least-recently-used reply-context window.
type ParentMessageCache struct {
	mu       sync.Mutex
	capacity int
	entries  map[ParentMessageKey]*parentMessageCacheEntry
	newest   *parentMessageCacheEntry
	oldest   *parentMessageCacheEntry
}

// NewParentMessageCache creates a cache capped at ParentMessageCacheLimit entries.
func NewParentMessageCache(capacity int) *ParentMessageCache {
	if capacity <= 0 || capacity > ParentMessageCacheLimit {
		capacity = ParentMessageCacheLimit
	}
	return &ParentMessageCache{
		capacity: capacity,
		entries:  make(map[ParentMessageKey]*parentMessageCacheEntry, capacity),
	}
}

// Put records one normalized parent message and promotes it to most recently used.
func (c *ParentMessageCache) Put(key ParentMessageKey, message ParentMessage) {
	if c == nil {
		return
	}
	key = normalizeParentMessageKey(key)
	if !validParentMessageKey(key) {
		return
	}
	message = normalizeParentMessage(message)

	c.mu.Lock()
	defer c.mu.Unlock()

	if existing, ok := c.entries[key]; ok {
		existing.message = message
		c.promoteLocked(existing)
		return
	}

	entry := &parentMessageCacheEntry{key: key, message: message}
	c.entries[key] = entry
	c.insertNewestLocked(entry)
	if len(c.entries) <= c.capacity {
		return
	}
	if c.oldest == nil {
		return
	}
	c.removeLocked(c.oldest)
}

// Get returns one parent message and promotes it to most recently used.
func (c *ParentMessageCache) Get(key ParentMessageKey) (ParentMessage, bool) {
	if c == nil {
		return ParentMessage{}, false
	}
	key = normalizeParentMessageKey(key)
	if !validParentMessageKey(key) {
		return ParentMessage{}, false
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	entry, ok := c.entries[key]
	if !ok {
		return ParentMessage{}, false
	}
	c.promoteLocked(entry)
	return entry.message, true
}

// Delete removes one parent message from the cache.
func (c *ParentMessageCache) Delete(key ParentMessageKey) {
	if c == nil {
		return
	}
	key = normalizeParentMessageKey(key)
	if !validParentMessageKey(key) {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	entry, ok := c.entries[key]
	if !ok {
		return
	}
	c.removeLocked(entry)
}

// RememberEnvelope records the reply context exposed by one accepted inbound envelope.
func (c *ParentMessageCache) RememberEnvelope(envelope bridgepkg.InboundMessageEnvelope) {
	if c == nil {
		return
	}
	switch envelope.EventFamily.Normalize() {
	case bridgepkg.InboundEventFamilyMessage:
		c.Put(parentMessageKeyForEnvelope(envelope, envelope.PlatformMessageID), parentMessageFromEnvelope(
			envelope,
			envelope.Content.Text,
		))
	case bridgepkg.InboundEventFamilyEdit:
		if envelope.Edit == nil {
			return
		}
		key := parentMessageKeyForEnvelope(envelope, envelope.Edit.MessageID)
		switch envelope.Edit.Operation.Normalize() {
		case bridgepkg.InboundEditOperationUpdated:
			c.Put(key, parentMessageFromEnvelope(envelope, envelope.Edit.NewText))
		case bridgepkg.InboundEditOperationDeleted:
			c.Delete(key)
		}
	}
}

// EnrichReply fills reply context from a provider-local cache without network access.
func (c *ParentMessageCache) EnrichReply(
	envelope *bridgepkg.InboundMessageEnvelope,
	parentMessageID string,
) bool {
	if c == nil || envelope == nil {
		return false
	}
	parent, ok := c.Get(parentMessageKeyForEnvelope(*envelope, parentMessageID))
	if !ok {
		return false
	}
	ApplyParentMessage(envelope, parent)
	return true
}

// RememberParent records provider-supplied parent context after ingress authorization succeeds.
func (c *ParentMessageCache) RememberParent(
	envelope bridgepkg.InboundMessageEnvelope,
	parentMessageID string,
	message ParentMessage,
) {
	message = normalizeParentMessage(message)
	if message.Text == "" && message.AuthorID == "" && message.AuthorName == "" {
		return
	}
	c.Put(parentMessageKeyForEnvelope(envelope, parentMessageID), message)
}

// ApplyParentMessage merges bounded provider-supplied parent context into an envelope.
func ApplyParentMessage(envelope *bridgepkg.InboundMessageEnvelope, message ParentMessage) {
	if envelope == nil {
		return
	}
	message = normalizeParentMessage(message)
	if message.Text != "" {
		envelope.ReplyToText = message.Text
	}
	if message.AuthorID != "" {
		envelope.ReplyToAuthorID = message.AuthorID
	}
	if message.AuthorName != "" {
		envelope.ReplyToAuthorName = message.AuthorName
	}
}

func normalizeParentMessageKey(key ParentMessageKey) ParentMessageKey {
	key.WorkspaceID = strings.TrimSpace(key.WorkspaceID)
	key.BridgeInstanceID = strings.TrimSpace(key.BridgeInstanceID)
	key.ConversationID = strings.TrimSpace(key.ConversationID)
	key.MessageID = strings.TrimSpace(key.MessageID)
	return key
}

func parentMessageKeyForEnvelope(
	envelope bridgepkg.InboundMessageEnvelope,
	messageID string,
) ParentMessageKey {
	return ParentMessageKey{
		WorkspaceID:      envelope.WorkspaceID,
		BridgeInstanceID: envelope.BridgeInstanceID,
		ConversationID:   firstParentConversationID(envelope.PeerID, envelope.GroupID),
		MessageID:        messageID,
	}
}

func parentMessageFromEnvelope(
	envelope bridgepkg.InboundMessageEnvelope,
	text string,
) ParentMessage {
	return ParentMessage{
		Text:       text,
		AuthorID:   envelope.Sender.ID,
		AuthorName: firstParentAuthorName(envelope.Sender.DisplayName, envelope.Sender.Username),
	}
}

func firstParentConversationID(peerID string, groupID string) string {
	if trimmed := strings.TrimSpace(peerID); trimmed != "" {
		return trimmed
	}
	return strings.TrimSpace(groupID)
}

func firstParentAuthorName(displayName string, username string) string {
	if trimmed := strings.TrimSpace(displayName); trimmed != "" {
		return trimmed
	}
	return strings.TrimSpace(username)
}

func validParentMessageKey(key ParentMessageKey) bool {
	return key.BridgeInstanceID != "" && key.ConversationID != "" && key.MessageID != ""
}

func normalizeParentMessage(message ParentMessage) ParentMessage {
	message.Text = truncateParentMessageText(strings.TrimSpace(message.Text))
	message.AuthorID = strings.TrimSpace(message.AuthorID)
	message.AuthorName = strings.TrimSpace(message.AuthorName)
	return message
}

func truncateParentMessageText(value string) string {
	runes := []rune(value)
	if len(runes) <= ParentMessageTextLimit {
		return value
	}
	return string(runes[:ParentMessageTextLimit])
}

func (c *ParentMessageCache) promoteLocked(entry *parentMessageCacheEntry) {
	if entry == nil || c.newest == entry {
		return
	}
	c.detachLocked(entry)
	c.insertNewestLocked(entry)
}

func (c *ParentMessageCache) insertNewestLocked(entry *parentMessageCacheEntry) {
	entry.previous = nil
	entry.next = c.newest
	if c.newest != nil {
		c.newest.previous = entry
	}
	c.newest = entry
	if c.oldest == nil {
		c.oldest = entry
	}
}

func (c *ParentMessageCache) removeLocked(entry *parentMessageCacheEntry) {
	c.detachLocked(entry)
	delete(c.entries, entry.key)
}

func (c *ParentMessageCache) detachLocked(entry *parentMessageCacheEntry) {
	if entry.previous != nil {
		entry.previous.next = entry.next
	} else {
		c.newest = entry.next
	}
	if entry.next != nil {
		entry.next.previous = entry.previous
	} else {
		c.oldest = entry.previous
	}
	entry.previous = nil
	entry.next = nil
}

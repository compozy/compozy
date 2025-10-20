package store

import (
	"strings"
)

// Redis key management constants
const (
	// Metadata fields stored in Redis hash
	metadataTokenCountField   = "token_count"
	metadataMessageCountField = "message_count"

	// Key suffixes
	metadataSuffix     = ":metadata"
	flushPendingSuffix = ":flush_pending"
)

// KeyManager handles Redis key generation and management for memory store
type KeyManager struct {
	keyPrefix string
}

// NewKeyManager creates a new key manager with the given prefix
func NewKeyManager(keyPrefix string) *KeyManager {
	return &KeyManager{
		keyPrefix: keyPrefix,
	}
}

// FullKey returns the full Redis key for a given memory key
func (km *KeyManager) FullKey(key string) string {
	if km.keyPrefix == "" {
		return key
	}
	if strings.HasSuffix(km.keyPrefix, ":") {
		return km.keyPrefix + key
	}
	return km.keyPrefix + ":" + key
}

// MetadataKey returns the Redis key for metadata storage
func (km *KeyManager) MetadataKey(key string) string {
	if km.keyPrefix == "" {
		return key + metadataSuffix
	}
	if strings.HasSuffix(km.keyPrefix, ":") {
		return km.keyPrefix + key + metadataSuffix
	}
	return km.keyPrefix + ":" + key + metadataSuffix
}

// FlushPendingKey returns the Redis key for flush pending flag
func (km *KeyManager) FlushPendingKey(key string) string {
	return km.FullKey(key) + flushPendingSuffix
}

// LastFlushedKey returns the Redis key for last flushed timestamp
func (km *KeyManager) LastFlushedKey(key string) string {
	return km.FullKey(key) + ":last_flushed"
}

// ExpirationKey returns the Redis key for expiration timestamp
func (km *KeyManager) ExpirationKey(key string) string {
	return km.FullKey(key) + ":expiration"
}

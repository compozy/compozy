package bridgesdk

import "crypto/sha256"

// DeliveryChunkMode distinguishes a multi-chunk create from an update that
// reuses an existing remote message as its first chunk.
type DeliveryChunkMode string

const (
	DeliveryChunkModeCreate DeliveryChunkMode = "create"
	DeliveryChunkModeUpdate DeliveryChunkMode = "update"
)

// DeliveryChunkCursor records the successfully materialized prefix of one
// provider delivery attempt. Providers retain it when an explicit later-chunk
// failure is returned so the daemon's resume request starts at the first
// unsent chunk instead of duplicating the prefix.
type DeliveryChunkCursor struct {
	eventSeq               int64
	mode                   DeliveryChunkMode
	chunkCount             int
	nextChunk              int
	contentDigest          [sha256.Size]byte
	lastRemoteMessageID    string
	replaceRemoteMessageID string
}

// BeginDeliveryChunks reuses a matching partial cursor or starts a fresh
// cursor for a different event, delivery mode, content, or chunk layout.
func BeginDeliveryChunks(
	current DeliveryChunkCursor,
	eventSeq int64,
	mode DeliveryChunkMode,
	chunkCount int,
	content string,
	lastRemoteMessageID string,
	replaceRemoteMessageID string,
) DeliveryChunkCursor {
	digest := sha256.Sum256([]byte(content))
	if current.eventSeq == eventSeq &&
		current.mode == mode &&
		current.chunkCount == chunkCount &&
		current.contentDigest == digest &&
		current.nextChunk >= 0 &&
		current.nextChunk <= chunkCount {
		return current
	}
	return DeliveryChunkCursor{
		eventSeq:               eventSeq,
		mode:                   mode,
		chunkCount:             chunkCount,
		contentDigest:          digest,
		lastRemoteMessageID:    lastRemoteMessageID,
		replaceRemoteMessageID: replaceRemoteMessageID,
	}
}

// Active reports whether the cursor owns an in-progress chunk sequence.
func (c DeliveryChunkCursor) Active() bool {
	return c.chunkCount > 0 && c.nextChunk < c.chunkCount
}

// NextChunk returns the first chunk index that has not been acknowledged by
// the provider API.
func (c DeliveryChunkCursor) NextChunk() int {
	return c.nextChunk
}

// Advance records one successfully materialized chunk.
func (c DeliveryChunkCursor) Advance(remoteMessageID string) DeliveryChunkCursor {
	if c.nextChunk < c.chunkCount {
		c.nextChunk++
	}
	if remoteMessageID != "" {
		c.lastRemoteMessageID = remoteMessageID
	}
	return c
}

// LastRemoteMessageID returns the last remote handle materialized so far.
func (c DeliveryChunkCursor) LastRemoteMessageID() string {
	return c.lastRemoteMessageID
}

// ReplaceRemoteMessageID returns the original remote handle replaced by this
// chunk sequence, when the delivery began as an update.
func (c DeliveryChunkCursor) ReplaceRemoteMessageID() string {
	return c.replaceRemoteMessageID
}

package store

import "errors"

var (
	// ErrNetworkConversationNotFound reports a missing network conversation container.
	ErrNetworkConversationNotFound = errors.New("store: network conversation not found")
	// ErrNetworkDirectRoomCollision reports a direct_id bound to another peer pair.
	ErrNetworkDirectRoomCollision = errors.New("store: network direct room collision")
	// ErrNetworkWorkContainerMismatch reports a work_id used outside its bound container.
	ErrNetworkWorkContainerMismatch = errors.New("store: network work container mismatch")
	// ErrNetworkWorkClosed reports a non-duplicate message for terminal work.
	ErrNetworkWorkClosed = errors.New("store: network work closed")
	// ErrNetworkCursorInvalid reports a malformed or query-mismatched network list cursor.
	ErrNetworkCursorInvalid = errors.New("store: network cursor invalid")
	// ErrNetworkChannelExists reports an attempted create for an existing workspace channel.
	ErrNetworkChannelExists = errors.New("store: network channel already exists")
)

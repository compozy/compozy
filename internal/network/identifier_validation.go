package network

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/compozy/compozy/internal/network/identifier"
)

const (
	// PeerIDMaxLength is the maximum length accepted for a Network peer id.
	PeerIDMaxLength = identifier.PeerIDMaxLength
	// PeerIDPattern is the Network peer identifier grammar.
	PeerIDPattern = identifier.PeerIDPattern

	validateDirectIDKey = "direct_id"
	validateThreadIDKey = "thread_id"
)

const (
	// DirectIDPattern is the public direct-room identifier grammar.
	DirectIDPattern = `^direct_[a-f0-9]{32}$`
	// ThreadIDPattern is the public thread identifier grammar.
	ThreadIDPattern = `^thread_[a-z0-9][a-z0-9_-]{2,95}$`
)

var (
	threadIDPattern = regexp.MustCompile(ThreadIDPattern)
	directIDPattern = regexp.MustCompile(DirectIDPattern)
)

// ValidatePeerID reports whether the peer identifier matches the RFC grammar.
func ValidatePeerID(peerID string) error {
	return identifier.ValidatePeerID(peerID)
}

// ValidateConversationID reports whether a container identifier matches its field grammar.
func ValidateConversationID(id string, field string) error {
	trimmed := strings.TrimSpace(id)
	if trimmed == "" {
		return fmt.Errorf("%w: %s is required", ErrMissingField, field)
	}
	switch field {
	case validateThreadIDKey:
		if !threadIDPattern.MatchString(trimmed) {
			return fmt.Errorf("%w: thread_id=%q", ErrInvalidField, id)
		}
	case validateDirectIDKey:
		if !directIDPattern.MatchString(trimmed) {
			return fmt.Errorf("%w: direct_id=%q", ErrInvalidField, id)
		}
	default:
		if strings.ContainsAny(trimmed, `/\`) || containsControlCharacter(trimmed) || len(trimmed) > 128 {
			return fmt.Errorf("%w: %s=%q", ErrInvalidField, field, id)
		}
	}
	return nil
}

// ValidateWorkID reports whether a work id can safely cross the network boundary.
func ValidateWorkID(id string) error {
	return ValidateConversationID(id, "work_id")
}

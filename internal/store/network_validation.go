package store

import (
	"fmt"
	"slices"
	"strings"
)

func validateOptionalNetworkConversation(
	workspaceID string,
	channel string,
	surface string,
	threadID string,
	directID string,
	label string,
) error {
	if strings.TrimSpace(surface) == "" && strings.TrimSpace(threadID) == "" && strings.TrimSpace(directID) == "" {
		return nil
	}
	if err := (NetworkConversationRef{
		WorkspaceID: workspaceID,
		Channel:     channel,
		Surface:     surface,
		ThreadID:    threadID,
		DirectID:    directID,
	}).Validate(); err != nil {
		return fmt.Errorf("store: invalid %s: %w", label, err)
	}
	return nil
}

func validateNetworkDirectRoom(
	workspaceID string,
	channel string,
	directID string,
	sessionA string,
	sessionB string,
) error {
	ref := NetworkConversationRef{
		WorkspaceID: workspaceID,
		Channel:     channel,
		Surface:     NetworkSurfaceDirect,
		DirectID:    directID,
	}
	if err := ref.Validate(); err != nil {
		return err
	}
	normalizedA, normalizedB, err := NormalizeNetworkDirectRoomSessions(sessionA, sessionB)
	if err != nil {
		return err
	}
	if normalizedA != strings.TrimSpace(sessionA) || normalizedB != strings.TrimSpace(sessionB) {
		return fmt.Errorf("store: network direct room sessions must be stored in lexicographic order")
	}
	return nil
}

func validateNetworkMessageKind(kind string) error {
	switch strings.TrimSpace(kind) {
	case NetworkKindGreet,
		NetworkKindWhois,
		NetworkKindSay,
		NetworkKindCapability,
		NetworkKindReceipt,
		NetworkKindTrace:
		return nil
	default:
		return fmt.Errorf("store: unsupported network message kind %q", kind)
	}
}

func validateNetworkWorkState(state string) error {
	switch strings.TrimSpace(state) {
	case NetworkWorkStateSubmitted,
		NetworkWorkStateWorking,
		NetworkWorkStateNeedsInput,
		NetworkWorkStateCompleted,
		NetworkWorkStateFailed,
		NetworkWorkStateCanceled:
		return nil
	default:
		return fmt.Errorf("store: unsupported network work state %q", state)
	}
}

func networkWorkStateIsTerminal(state string) bool {
	switch strings.TrimSpace(state) {
	case NetworkWorkStateCompleted, NetworkWorkStateFailed, NetworkWorkStateCanceled:
		return true
	default:
		return false
	}
}

func validateNetworkConversationID(id string, field string) error {
	trimmed := strings.TrimSpace(id)
	if trimmed == "" {
		return fmt.Errorf("store: network %s is required", field)
	}
	switch field {
	case typesThreadIDKey:
		if !networkThreadIDPattern.MatchString(trimmed) {
			return fmt.Errorf("store: invalid network thread_id %q", id)
		}
	case typesDirectIDKey:
		if !networkDirectIDPattern.MatchString(trimmed) {
			return fmt.Errorf("store: invalid network direct_id %q", id)
		}
	default:
		if len(trimmed) > 128 || strings.ContainsAny(trimmed, `/\`) || containsControlCharacter(trimmed) {
			return fmt.Errorf("store: invalid network %s %q", field, id)
		}
	}
	return nil
}

func validateNetworkPeerID(peerID string, field string) error {
	trimmed := strings.TrimSpace(peerID)
	if !networkPeerIDPattern.MatchString(trimmed) {
		return fmt.Errorf("store: invalid network %s %q", field, peerID)
	}
	return nil
}

// NormalizeNetworkPeerIDs trims, validates, deduplicates, and sorts peer ids.
func NormalizeNetworkPeerIDs(values []string, field string) ([]string, error) {
	seen := make(map[string]struct{}, len(values))
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if err := validateNetworkPeerID(trimmed, field); err != nil {
			return nil, err
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		normalized = append(normalized, trimmed)
	}
	slices.Sort(normalized)
	return normalized, nil
}

func containsControlCharacter(value string) bool {
	for _, r := range value {
		if r < 0x20 || r == 0x7f {
			return true
		}
	}
	return false
}

package store

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

// NormalizeNetworkDirectRoomSessions validates and orders a two-party room pair.
func NormalizeNetworkDirectRoomSessions(sessionA string, sessionB string) (string, string, error) {
	first := strings.TrimSpace(sessionA)
	second := strings.TrimSpace(sessionB)
	if err := requireField(first, "network direct room session_a"); err != nil {
		return "", "", err
	}
	if err := requireField(second, "network direct room session_b"); err != nil {
		return "", "", err
	}
	if first == second {
		return "", "", fmt.Errorf("store: network direct room sessions must differ")
	}
	if second < first {
		first, second = second, first
	}
	return first, second, nil
}

// NetworkDirectRoomIdentity derives the stable direct-room id for one ordered session pair.
func NetworkDirectRoomIdentity(
	workspaceID string,
	channel string,
	sessionA string,
	sessionB string,
) (string, string, string, error) {
	trimmedWorkspaceID := strings.TrimSpace(workspaceID)
	if err := requireField(trimmedWorkspaceID, "network direct room workspace_id"); err != nil {
		return "", "", "", err
	}
	trimmedChannel := strings.TrimSpace(channel)
	if err := requireField(trimmedChannel, "network direct room channel"); err != nil {
		return "", "", "", err
	}
	normalizedA, normalizedB, err := NormalizeNetworkDirectRoomSessions(sessionA, sessionB)
	if err != nil {
		return "", "", "", err
	}
	sum := sha256.Sum256([]byte(
		"agh-network/direct-room/v1\x00" + trimmedWorkspaceID + "\x00" + trimmedChannel + "\x00" +
			normalizedA + "\x00" + normalizedB,
	))
	return "direct_" + hex.EncodeToString(sum[:])[:32], normalizedA, normalizedB, nil
}

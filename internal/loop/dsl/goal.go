package dsl

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
)

const (
	// SessionModeContinuous reuses one durable binding across Goal turns.
	SessionModeContinuous = "continuous"
	// RetryOnFailureFreshSession rotates a proven-not-started Goal binding attempt.
	RetryOnFailureFreshSession = "fresh_session"
	// GoalOnExhaustedHalt terminates a Goal at its effective turn limit.
	GoalOnExhaustedHalt = "halt"
	// GoalOnExhaustedEscalate requests approval at the effective turn limit.
	GoalOnExhaustedEscalate = "escalate"
)

// DeriveGoalHandle returns the fixed persisted binding identity for one Goal node item.
func DeriveGoalHandle(nodeID NodeID, label string, itemIndex int) (string, error) {
	normalizedNodeID := strings.TrimSpace(string(nodeID))
	if normalizedNodeID == "" {
		return "", fmt.Errorf("goal handle node_id is required")
	}
	if itemIndex < 0 {
		return "", fmt.Errorf("goal handle item_index must be zero or positive")
	}
	normalizedLabel := strings.TrimSpace(label)
	if normalizedLabel == "" {
		normalizedLabel = normalizedNodeID
	}
	identity := normalizedNodeID + "\x00" + normalizedLabel + "\x00" + strconv.Itoa(itemIndex)
	sum := sha256.Sum256([]byte(identity))
	return "goal:" + hex.EncodeToString(sum[:]), nil
}

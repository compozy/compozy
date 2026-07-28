package store

import (
	"encoding/json"
	"fmt"
	"strings"
)

// SessionSpawnBudget captures durable spawn limits attached to a session.
type SessionSpawnBudget struct {
	MaxChildren           int   `json:"max_children"`
	MaxDepth              int   `json:"max_depth"`
	TTLSeconds            int64 `json:"ttl_seconds"`
	MaxActivePerWorkspace int   `json:"max_active_per_workspace,omitempty"`
}

func validateSessionSpawnBudget(budget SessionSpawnBudget) error {
	switch {
	case budget.MaxChildren < 0:
		return fmt.Errorf("store: session spawn budget max_children cannot be negative")
	case budget.MaxDepth < 0:
		return fmt.Errorf("store: session spawn budget max_depth cannot be negative")
	case budget.TTLSeconds < 0:
		return fmt.Errorf("store: session spawn budget ttl_seconds cannot be negative")
	case budget.MaxActivePerWorkspace < 0:
		return fmt.Errorf("store: session spawn budget max_active_per_workspace cannot be negative")
	default:
		return nil
	}
}

// EncodeSessionSpawnBudget marshals budget metadata for the global session catalog.
func EncodeSessionSpawnBudget(budget SessionSpawnBudget) (string, error) {
	if err := validateSessionSpawnBudget(budget); err != nil {
		return "", err
	}
	data, err := json.Marshal(budget)
	if err != nil {
		return "", fmt.Errorf("store: marshal session spawn budget: %w", err)
	}
	return string(data), nil
}

// DecodeSessionSpawnBudget unmarshals budget metadata from the global session catalog.
func DecodeSessionSpawnBudget(raw string) (SessionSpawnBudget, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return SessionSpawnBudget{}, nil
	}
	var budget SessionSpawnBudget
	if err := json.Unmarshal([]byte(trimmed), &budget); err != nil {
		return SessionSpawnBudget{}, fmt.Errorf("store: parse session spawn budget json: %w", err)
	}
	if err := validateSessionSpawnBudget(budget); err != nil {
		return SessionSpawnBudget{}, err
	}
	return budget, nil
}

package task

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/network/participation"
)

// QueueRunReservation captures the canonical store-level queue reservation input.
type QueueRunReservation struct {
	TaskID               string             `json:"task_id"`
	RunID                string             `json:"run_id"`
	RunKind              RunKind            `json:"run_kind,omitempty"`
	LoopRunID            string             `json:"loop_run_id,omitempty"`
	IdempotencyKey       string             `json:"idempotency_key,omitempty"`
	Origin               Origin             `json:"origin"`
	NetworkSpec          participation.Spec `json:"network_spec"`
	DesignationGroupID   string             `json:"designation_group_id,omitempty"`
	ResolvedWorktreeMode WorktreeMode       `json:"resolved_worktree_mode"`
	ResolvedWorktreeRef  string             `json:"resolved_worktree_ref,omitempty"`
	Metadata             json.RawMessage    `json:"metadata,omitempty"`
	QueuedAt             time.Time          `json:"queued_at"`
}

// Validate reports whether the store-level queue reservation input is internally consistent.
func (r QueueRunReservation) Validate(path string) error {
	if strings.TrimSpace(r.TaskID) == "" {
		return fmt.Errorf("%w: %s is required", ErrValidation, nestedPath(path, "task_id"))
	}
	if strings.TrimSpace(r.RunID) == "" {
		return fmt.Errorf("%w: %s is required", ErrValidation, nestedPath(path, "run_id"))
	}
	runKind := normalizeRunKindOrDefault(r.RunKind)
	if err := runKind.Validate(nestedPath(path, "run_kind")); err != nil {
		return err
	}
	if runKind == RunKindCoordinator && strings.TrimSpace(r.LoopRunID) == "" {
		return fmt.Errorf(
			"%w: %s is required for coordinator runs",
			ErrValidation,
			nestedPath(path, "loop_run_id"),
		)
	}
	if err := r.Origin.Validate(nestedPath(path, "origin")); err != nil {
		return err
	}
	if r.NetworkSpec != (participation.Spec{}) {
		if err := participation.ValidateSpec(r.NetworkSpec); err != nil {
			return fmt.Errorf("%s: %w", nestedPath(path, "network_spec"), err)
		}
	}
	if err := validateResolvedWorktreePolicy(WorktreePolicy{
		Mode:        r.ResolvedWorktreeMode,
		WorktreeRef: r.ResolvedWorktreeRef,
	}); err != nil {
		return err
	}
	if err := ValidateMetadataSize(r.Metadata, nestedPath(path, "metadata")); err != nil {
		return err
	}
	return nil
}

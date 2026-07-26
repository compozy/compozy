package globaldb

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	looppkg "github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/loop/dsl"
)

func normalizeLoopRunForCreate(run looppkg.Run) (looppkg.Run, error) {
	if err := normalizeLoopRunIdentity(&run); err != nil {
		return looppkg.Run{}, err
	}
	run.DefinitionDigest = strings.TrimSpace(run.DefinitionDigest)
	if run.DefinitionDigest == "" {
		return looppkg.Run{}, fmt.Errorf("%w: definition_digest is required", looppkg.ErrValidation)
	}
	if len(run.DefinitionSnapshot) == 0 || !json.Valid(run.DefinitionSnapshot) {
		return looppkg.Run{}, fmt.Errorf("%w: definition snapshot must be valid JSON", looppkg.ErrValidation)
	}
	resolved, err := looppkg.LoadExecutedDefinitionSnapshot(
		run.DefinitionSnapshot,
		run.DefinitionDigest,
	)
	if err != nil {
		return looppkg.Run{}, err
	}
	if resolved.DefinitionVersion != run.DefinitionVersion {
		return looppkg.Run{}, fmt.Errorf("%w: definition snapshot version changed", looppkg.ErrValidation)
	}
	run.ActiveGateID = looppkg.NodeID(strings.TrimSpace(string(run.ActiveGateID)))
	if len(run.ActiveHumanCriteria) == 0 {
		run.ActiveHumanCriteria = json.RawMessage(`[]`)
	}
	if !json.Valid(run.ActiveHumanCriteria) {
		return looppkg.Run{}, fmt.Errorf("%w: active_human_criteria_json must be valid JSON", looppkg.ErrValidation)
	}
	if run.ReattemptStrategy == "" {
		run.ReattemptStrategy = looppkg.ReattemptFailedOnly
	}
	if run.BudgetOnExceeded == "" {
		run.BudgetOnExceeded = dsl.BudgetExceededHalt
	}
	if run.Inputs == nil {
		run.Inputs = map[string]any{}
	}
	if run.StartMetadata == nil {
		run.StartMetadata = map[string]any{}
	}
	if err := (looppkg.GoalRunPolicy{ContextNudgeRatio: run.GoalContextNudgeRatio}).Validate(); err != nil {
		return looppkg.Run{}, err
	}
	origin := looppkg.RunOrigin{}
	if run.Origin != nil {
		origin = *run.Origin
	}
	origin = origin.Normalize()
	if err := origin.Validate(); err != nil {
		return looppkg.Run{}, err
	}
	run.Origin = &origin
	return run, nil
}

func normalizeLoopRunIdentity(run *looppkg.Run) error {
	if strings.TrimSpace(string(run.ID)) == "" {
		return fmt.Errorf("%w: loop run id is required", looppkg.ErrValidation)
	}
	if strings.TrimSpace(string(run.WorkspaceID)) == "" {
		return fmt.Errorf("%w: workspace_id is required", looppkg.ErrValidation)
	}
	run.LoopName = strings.TrimSpace(run.LoopName)
	if run.LoopName == "" {
		return fmt.Errorf("%w: loop_name is required", looppkg.ErrValidation)
	}
	if !run.Status.Valid() {
		return fmt.Errorf("%w: loop status is invalid: %q", looppkg.ErrValidation, run.Status)
	}
	if run.CreatedAt.IsZero() {
		return fmt.Errorf("%w: created_at is required", looppkg.ErrValidation)
	}
	if run.StartedAt.IsZero() {
		run.StartedAt = run.CreatedAt
	}
	if run.LastProgressAt.IsZero() {
		run.LastProgressAt = run.CreatedAt
	}
	return nil
}

func normalizeLoopConfigForStore(cfg looppkg.LoopConfig) (looppkg.LoopConfig, error) {
	normalized := looppkg.ClampLoopConfig(cfg)
	if len(normalized.EnabledChecks) == 0 {
		normalized.EnabledChecks = json.RawMessage(`{}`)
	}
	if !json.Valid(normalized.EnabledChecks) {
		return looppkg.LoopConfig{}, fmt.Errorf(
			"%w: enabled_checks_json must be valid JSON",
			looppkg.ErrValidation,
		)
	}
	return normalized, nil
}

func enabledChecksForStore(raw json.RawMessage) string {
	if len(raw) == 0 {
		return "{}"
	}
	return string(raw)
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func boolPtrToInt(value *bool) int {
	if value == nil {
		return 0
	}
	return boolToInt(*value)
}

func nullString(value string) sql.NullString {
	if strings.TrimSpace(value) == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: value, Valid: true}
}

func nullIntPtr(value *int) sql.NullInt64 {
	if value == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(*value), Valid: true}
}

func nullStringPtr[T ~string](value *T) sql.NullString {
	if value == nil || strings.TrimSpace(string(*value)) == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: string(*value), Valid: true}
}

func modelDefaultNullString(defaults *looppkg.ModelDefaults, worker bool) sql.NullString {
	if defaults == nil {
		return sql.NullString{}
	}
	var value *string
	if worker {
		value = defaults.Worker
	} else {
		value = defaults.Judge
	}
	if value == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: strings.TrimSpace(*value), Valid: true}
}

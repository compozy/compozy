package globaldb

import (
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/store"
)

func buildUpdateSessionStateStatement(update store.SessionStateUpdate, updatedAt time.Time) (string, []any, error) {
	assignments := []string{"state = ?"}
	args := []any{update.State}

	if update.ACPSessionID != nil {
		assignments = append(assignments, "acp_session_id = ?")
		args = append(args, store.NullableStringPointer(update.ACPSessionID))
	}

	var err error
	assignments, args, err = appendSessionRuntimeAssignments(assignments, args, update)
	if err != nil {
		return "", nil, err
	}
	assignments, args, err = appendSelectedRuntimeAssignments(assignments, args, update)
	if err != nil {
		return "", nil, err
	}
	if update.StopReasonSet {
		assignments = append(assignments, "stop_reason = ?", "stop_detail = ?")
		args = append(args, store.NullableStringPointer(update.StopReason), store.NullableString(update.StopDetail))
	}
	if update.FailureSet {
		assignments = append(assignments, "failure_kind = ?", "failure_summary = ?", "crash_bundle_path = ?")
		args = append(
			args,
			sessionFailureKind(update.Failure),
			sessionFailureSummary(update.Failure),
			sessionCrashBundlePath(update.Failure),
		)
	}
	assignments, args, err = appendSessionLivenessAssignments(assignments, args, update)
	if err != nil {
		return "", nil, err
	}
	assignments, args = appendSessionSandboxAssignments(assignments, args, update)
	if update.AttentionTransition {
		assignments = append(
			assignments,
			"attention_revision = attention_revision + 1",
			"attention_changed_at = ?",
		)
		args = append(args, store.FormatTimestamp(updatedAt))
	}

	if strings.TrimSpace(update.State) == globalDBSessionStateStopped {
		assignments = append(assignments, "attached_to = ''", "attach_expires_at = NULL")
	}
	assignments = append(assignments, "updated_at = ?")
	args = append(args, store.FormatTimestamp(updatedAt), update.ID)
	return fmt.Sprintf("UPDATE sessions SET %s WHERE id = ?", strings.Join(assignments, ", ")), args, nil
}

func appendSelectedRuntimeAssignments(
	assignments []string,
	args []any,
	update store.SessionStateUpdate,
) ([]string, []any, error) {
	if !update.SelectedRuntimeSet {
		return assignments, args, nil
	}
	selection := store.NormalizeSessionRuntimeSelection(update.SelectedRuntime)
	provider, model, reasoningEffort, speed := "", "", "", ""
	optionsJSON, err := encodeCanonicalSelectedRuntimeACPOptions(selection)
	if err != nil {
		return nil, nil, err
	}
	if selection != nil {
		provider = selection.Provider
		model = selection.Model
		reasoningEffort = selection.ReasoningEffort
		speed = string(selection.Speed)
	}
	assignments = append(assignments,
		"selected_provider = ?",
		"selected_model = ?",
		"selected_reasoning_effort = ?",
		"selected_speed = ?",
		"selected_acp_options_json = ?",
		"runtime_selection_revision = ?",
	)
	args = append(args, provider, model, reasoningEffort, speed, optionsJSON, update.RuntimeSelectionRevision)
	return assignments, args, nil
}

func appendSessionRuntimeAssignments(
	assignments []string,
	args []any,
	update store.SessionStateUpdate,
) ([]string, []any, error) {
	if !update.RuntimeSet {
		return assignments, args, nil
	}
	speedResolutionJSON, err := encodeSessionSpeedResolution(update.SpeedResolution)
	if err != nil {
		return nil, nil, err
	}
	acpOptionsJSON, err := encodeSessionACPOptions(update.ACPOptions, "session ACP options")
	if err != nil {
		return nil, nil, err
	}
	assignments = append(
		assignments,
		"provider = ?",
		"model = ?",
		"reasoning_effort = ?",
		"speed = ?",
		"acp_options_json = ?",
		"speed_resolution_json = ?",
		"runtime_status = ?",
		"runtime_transition = ?",
		"runtime_failure = ?",
		"runtime_generation = ?",
		"runtime_recovery_json = ?",
	)
	runtimeRecoveryJSON, err := encodeSessionRuntimeRecovery(update.RuntimeRecovery)
	if err != nil {
		return nil, nil, err
	}
	args = append(
		args,
		strings.TrimSpace(update.Provider),
		strings.TrimSpace(update.Model),
		strings.TrimSpace(update.ReasoningEffort),
		string(update.Speed),
		acpOptionsJSON,
		speedResolutionJSON,
		string(update.RuntimeStatus),
		string(update.RuntimeTransition),
		strings.TrimSpace(update.RuntimeFailure),
		update.RuntimeGeneration,
		runtimeRecoveryJSON,
	)
	return assignments, args, nil
}

func appendSessionLivenessAssignments(
	assignments []string,
	args []any,
	update store.SessionStateUpdate,
) ([]string, []any, error) {
	if update.Liveness == nil {
		return assignments, args, nil
	}
	activityJSON, err := sessionLivenessActivityJSON(update.Liveness)
	if err != nil {
		return nil, nil, err
	}
	assignments = append(
		assignments,
		"subprocess_pid = ?",
		"subprocess_started_at = ?",
		"last_update_at = ?",
		"stall_state = ?",
		"stall_reason = ?",
		"activity_json = ?",
	)
	args = append(
		args,
		sessionLivenessPID(update.Liveness),
		sessionLivenessStartedAt(update.Liveness),
		sessionLivenessLastUpdateAt(update.Liveness),
		sessionLivenessStallState(update.Liveness),
		sessionLivenessStallReason(update.Liveness),
		activityJSON,
	)
	return assignments, args, nil
}

func appendSessionSandboxAssignments(
	assignments []string,
	args []any,
	update store.SessionStateUpdate,
) ([]string, []any) {
	if update.Sandbox == nil {
		return assignments, args
	}
	assignments = append(
		assignments,
		"sandbox_id = ?",
		"sandbox_backend = ?",
		"sandbox_profile = ?",
		"sandbox_instance_id = ?",
		"sandbox_state = ?",
		"sandbox_provider_state_json = ?",
		"sandbox_last_sync_at = ?",
		"sandbox_last_sync_error = ?",
	)
	args = append(
		args,
		sessionSandboxID(update.Sandbox),
		sessionSandboxBackend(update.Sandbox),
		sessionSandboxProfile(update.Sandbox),
		sessionSandboxInstanceID(update.Sandbox),
		sessionSandboxState(update.Sandbox),
		sessionSandboxProviderStateJSON(update.Sandbox),
		sessionSandboxLastSyncAt(update.Sandbox),
		sessionSandboxLastSyncError(update.Sandbox),
	)
	return assignments, args
}

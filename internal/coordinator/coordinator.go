package coordinator

import (
	"encoding/json"
	"slices"
	"strings"
	"time"

	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/network/participation"
	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/store"
	taskpkg "github.com/compozy/agh/internal/task"
	toolspkg "github.com/compozy/agh/internal/tools"
)

const (
	ReasonRunEnqueued        = "task_run_enqueued"
	ReasonRecovery           = "recovery"
	ReasonCoordinatorStopped = "coordinator_stopped"

	DecisionBootstrap        = "bootstrap"
	DecisionDisabled         = "disabled"
	DecisionGlobalScope      = "global_scope"
	DecisionMissingWorkspace = "missing_workspace"
	DecisionNonExecutableRun = "non_executable_run"
	DecisionTaskRunMismatch  = "task_run_mismatch"
	DecisionExisting         = "existing_coordinator"
	DecisionDenied           = "denied"
	DecisionFailed           = "failed"
)

var (
	// operationalMessageKinds are the coordination-channel message kinds a
	// coordinator may use for worker conversation. Task ownership remains in
	// the task lease API.
	operationalMessageKinds = [...]string{
		"status",
		"request",
		"blocker",
		"handoff",
		"result",
		"review_request",
	}

	// taskToolAllowlist is the orchestration-safe surface granted to coordinator
	// sessions. Operator lifecycle verbs and coordinator-to-coordinator spawn
	// are intentionally absent. These must stay aligned with canonical builtin
	// ToolIDs because lineage permission policies validate concrete tool atoms.
	taskToolAllowlist = [...]string{
		toolspkg.ToolIDSessionDescribe.String(),
		toolspkg.ToolIDTaskRunClaimNext.String(),
		toolspkg.ToolIDTaskRunHeartbeat.String(),
		toolspkg.ToolIDTaskRunComplete.String(),
		toolspkg.ToolIDTaskRunFail.String(),
		toolspkg.ToolIDTaskRunRelease.String(),
		toolspkg.ToolIDTaskCreate.String(),
	}
	networkToolAllowlist = [...]string{
		toolspkg.ToolIDNetworkChannels.String(),
		toolspkg.ToolIDNetworkInbox.String(),
		toolspkg.ToolIDNetworkSend.String(),
	}
)

// OperationalMessageKinds returns the coordination-channel message kinds a
// coordinator may use for worker conversation.
func OperationalMessageKinds() []string {
	return slices.Clone(operationalMessageKinds[:])
}

// ToolAllowlist returns the orchestration-safe tool surface granted to
// coordinator sessions.
func ToolAllowlist(spec participation.Spec) []string {
	tools := slices.Clone(taskToolAllowlist[:])
	if spec.Mode == participation.ModeLive {
		tools = append(tools, networkToolAllowlist[:]...)
	}
	return tools
}

// Decision describes whether a task run is eligible to bootstrap a workspace
// coordinator.
type Decision struct {
	ShouldBootstrap      bool
	Reason               string
	WorkspaceID          string
	TaskID               string
	RunID                string
	WorkflowID           string
	NetworkParticipation participation.Spec
}

// PromptInput captures the first-run situation given to a coordinator session.
type PromptInput struct {
	WorkspaceID          string
	TaskID               string
	RunID                string
	WorkflowID           string
	NetworkParticipation participation.Spec
}

// DecideBootstrap evaluates the mechanical coordinator bootstrap rules. It
// does not check for already-running coordinator sessions; that singleton check
// belongs to the daemon runtime.
func DecideBootstrap(task taskpkg.Task, run taskpkg.Run, cfg aghconfig.ResolvedCoordinatorRole) Decision {
	decision := Decision{
		WorkspaceID:          strings.TrimSpace(task.WorkspaceID),
		TaskID:               strings.TrimSpace(task.ID),
		RunID:                strings.TrimSpace(run.ID),
		WorkflowID:           workflowIDFromMetadata(run.Metadata),
		NetworkParticipation: run.NetworkSpecSnapshot(),
	}
	if !cfg.Enabled {
		decision.Reason = DecisionDisabled
		return decision
	}
	if strings.TrimSpace(task.ID) == "" ||
		strings.TrimSpace(run.TaskID) == "" ||
		strings.TrimSpace(task.ID) != strings.TrimSpace(run.TaskID) {
		decision.Reason = DecisionTaskRunMismatch
		return decision
	}
	switch task.Scope.Normalize() {
	case taskpkg.ScopeGlobal:
		decision.Reason = DecisionGlobalScope
		return decision
	case taskpkg.ScopeWorkspace:
	default:
		decision.Reason = DecisionMissingWorkspace
		return decision
	}
	if decision.WorkspaceID == "" {
		decision.Reason = DecisionMissingWorkspace
		return decision
	}
	if !IsExecutableRunStatus(run.Status) {
		decision.Reason = DecisionNonExecutableRun
		return decision
	}
	decision.ShouldBootstrap = true
	decision.Reason = DecisionBootstrap
	return decision
}

// IsExecutableRunStatus reports whether a run still represents executable work
// that may need coordinator orchestration.
func IsExecutableRunStatus(status taskpkg.RunStatus) bool {
	switch status.Normalize() {
	case taskpkg.TaskRunStatusQueued,
		taskpkg.TaskRunStatusClaimed,
		taskpkg.TaskRunStatusStarting,
		taskpkg.TaskRunStatusRunning:
		return true
	default:
		return false
	}
}

// ExecutableRunStatuses returns every open run state considered by recovery.
func ExecutableRunStatuses() []taskpkg.RunStatus {
	return []taskpkg.RunStatus{
		taskpkg.TaskRunStatusQueued,
		taskpkg.TaskRunStatusClaimed,
		taskpkg.TaskRunStatusStarting,
		taskpkg.TaskRunStatusRunning,
	}
}

// PermissionPolicy returns the restricted coordinator root permission policy.
func PermissionPolicy(spec participation.Spec) store.SessionPermissionPolicy {
	channelIDs := []string(nil)
	if spec.Mode == participation.ModeLive {
		channelIDs = nonEmptyAtoms(spec.ChannelID)
	}
	policy := store.SessionPermissionPolicy{
		Tools:           ToolAllowlist(spec),
		NetworkChannels: channelIDs,
	}
	return store.NormalizeSessionPermissionPolicy(policy)
}

// ToolAllowed reports whether a concrete tool/action is coordinator-safe.
func ToolAllowed(spec participation.Spec, tool string) bool {
	return slices.Contains(ToolAllowlist(spec), strings.TrimSpace(tool))
}

// SpawnRoleAllowed reports whether a coordinator may request the given child
// role through the public safe-spawn API.
func SpawnRoleAllowed(role string) bool {
	return strings.TrimSpace(strings.ToLower(role)) != string(session.SessionTypeCoordinator)
}

// Lineage builds root lineage metadata for a managed coordinator session.
func Lineage(
	now time.Time,
	cfg aghconfig.ResolvedCoordinatorRole,
	policy store.SessionPermissionPolicy,
) *store.SessionLineage {
	ttl := now.UTC().Add(cfg.TTL)
	return &store.SessionLineage{
		SpawnRole:    string(session.SessionTypeCoordinator),
		TTLExpiresAt: &ttl,
		SpawnBudget: store.SessionSpawnBudget{
			MaxChildren: cfg.MaxChildren,
			MaxDepth:    session.DefaultSpawnMaxDepth,
			TTLSeconds:  int64(cfg.TTL.Seconds()),
			// Coordinator uniqueness is enforced by the daemon singleton; this caps managed sessions.
			MaxActivePerWorkspace: cfg.MaxActiveSessionsPerWorkspace,
		},
		PermissionPolicy: store.NormalizeSessionPermissionPolicy(policy),
	}
}

// HealthySession reports whether a session snapshot is an active coordinator
// for the workspace.
func HealthySession(info *session.Info, workspaceID string, now time.Time) bool {
	if info == nil {
		return false
	}
	if info.Type != session.SessionTypeCoordinator {
		return false
	}
	if strings.TrimSpace(info.WorkspaceID) != strings.TrimSpace(workspaceID) {
		return false
	}
	switch info.State {
	case session.StateStarting, session.StateActive:
	default:
		return false
	}
	lineage := store.NormalizeSessionLineage(info.ID, info.Lineage)
	if lineage.TTLExpiresAt != nil && !lineage.TTLExpiresAt.After(now.UTC()) {
		return false
	}
	return true
}

// PromptOverlay assembles the coordinator's first-run situation and available
// public API surface.
func PromptOverlay(input PromptInput) string {
	var b strings.Builder
	b.WriteString("You are the AGH workspace coordinator for executable task runs.\n\n")
	b.WriteString("Current run context:\n")
	writePromptLine(&b, "workspace_id", input.WorkspaceID)
	writePromptLine(&b, "task_id", input.TaskID)
	writePromptLine(&b, "run_id", input.RunID)
	writePromptLine(&b, "workflow_id", input.WorkflowID)
	if input.NetworkParticipation.Mode == participation.ModeLive {
		writePromptLine(&b, "participation_channel", input.NetworkParticipation.ChannelID)
	}
	b.WriteString("\nUse public AGH agent APIs only:\n")
	b.WriteString("- `agh me context` for the Situation Surface.\n")
	b.WriteString("- `agh task create` to persist follow-up task intent.\n")
	b.WriteString("- `agh task next|heartbeat|complete|fail|release` for task ownership and terminal status.\n")
	if input.NetworkParticipation.Mode == participation.ModeLive {
		b.WriteString("- `agh ch list|recv|send|reply` for operational worker communication.\n")
	}
	b.WriteString("- `agh spawn` for bounded worker delegation.\n")
	b.WriteString("\nCreating a task only records follow-up intent. The current coordinator run is the active ")
	b.WriteString("execution boundary, and child work must stay within the allowed task-run and spawn surfaces.\n")
	if input.NetworkParticipation.Mode == participation.ModeLive {
		b.WriteString("\nChannel communication is operational only. Use the run coordination channel for ")
		b.WriteString(strings.Join(operationalMessageKinds[:], ", "))
		b.WriteString(" messages when conversation is useful. Do not use channel messages as task ownership state.\n")
	}
	b.WriteString("Never spawn another coordinator. ")
	b.WriteString("Worker delegation must stay inside safe-spawn permissions and task approvals.\n")
	return strings.TrimSpace(b.String())
}

func writePromptLine(b *strings.Builder, key string, value string) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return
	}
	b.WriteString("- ")
	b.WriteString(key)
	b.WriteString(": ")
	b.WriteString(trimmed)
	b.WriteByte('\n')
}

func nonEmptyAtoms(values ...string) []string {
	atoms := make([]string, 0, len(values))
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			atoms = append(atoms, trimmed)
		}
	}
	return atoms
}

func workflowIDFromMetadata(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var metadata map[string]any
	if err := json.Unmarshal(raw, &metadata); err != nil {
		return ""
	}
	value, ok := metadata["workflow_id"].(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(value)
}

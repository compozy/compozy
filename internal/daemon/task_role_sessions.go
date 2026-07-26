package daemon

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	hookspkg "github.com/compozy/agh/internal/hooks"
	"github.com/compozy/agh/internal/network/participation"
	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/store"
	taskpkg "github.com/compozy/agh/internal/task"
)

func (r *taskRoleRuntime) activeRoleSession(
	ctx context.Context,
	activation taskRoleActivation,
) (*session.Info, error) {
	infos, err := r.sessions.ListAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("daemon: list sessions for task role activation: %w", err)
	}
	for _, info := range infos {
		if taskRoleSessionMatches(info, activation) {
			return info, nil
		}
	}
	return nil, nil
}

func (r *taskRoleRuntime) startRoleSession(
	ctx context.Context,
	activation taskRoleActivation,
) (*session.Info, error) {
	opts, err := taskRoleCreateOpts(activation)
	if err != nil {
		return nil, err
	}
	return r.createRoleSession(ctx, opts)
}

func taskRoleCreateOpts(activation taskRoleActivation) (session.CreateOpts, error) {
	opts := session.CreateOpts{
		AgentName:                    activation.AgentName,
		Provider:                     activation.Provider,
		Model:                        activation.Model,
		Name:                         taskRoleSessionName(activation),
		ResolvedNetworkParticipation: activation.NetworkParticipation,
		NetworkOwnerKey: participation.OwnerKey(participation.OwnerRef{
			Kind: participation.OwnerKindTaskRun,
			ID:   activation.RunID,
		}),
		PromptOverlay: taskRolePromptOverlay(activation),
		Type:          session.SessionTypeSystem,
	}
	policy := sessionPolicyFromTaskExecutionProfile(activation.Profile)
	applySessionSandboxPolicy(&opts, policy)
	applySessionPermissionPolicy(&opts, policy)
	switch activation.Scope {
	case taskpkg.ScopeWorkspace:
		opts.Workspace = activation.WorkspaceID
	case taskpkg.ScopeGlobal:
		opts.WorkspacePath = activation.WorkspacePath
	default:
		return session.CreateOpts{}, fmt.Errorf(
			"%w: unsupported task scope %q for role session start",
			taskpkg.ErrValidation,
			activation.Scope,
		)
	}
	return opts, nil
}

func (r *taskRoleRuntime) createRoleSession(
	ctx context.Context,
	opts session.CreateOpts,
) (*session.Info, error) {
	created, err := r.sessions.Create(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("daemon: create task role session: %w", err)
	}
	if created == nil {
		return nil, errors.New("daemon: task role session create returned nil")
	}
	info := created.Info()
	if info == nil {
		return nil, errors.New("daemon: task role session create returned nil info")
	}
	return info, nil
}

// activateForStarvation spawns a capability-matched worker for a starved run that no agent has
// claimed. The worker self-claims via `agh task next`; the scheduler never claims. It carries a
// TTL + spawn budget so the reaper bounds its lifetime. Dedup on (agent, channel, scope) keeps the
// effective per-workspace cap at one active worker per role.
func (r *taskRoleRuntime) activateForStarvation(
	ctx context.Context,
	taskRecord taskpkg.Task,
	run taskpkg.Run,
	spawner starvationSpawner,
) error {
	if ctx == nil {
		return errors.New("daemon: starvation activation context is required")
	}
	agentName, err := r.resolveStarvationAgent(ctx, taskRecord, run, spawner)
	if err != nil {
		return err
	}
	activation, ok, err := r.starvationActivation(taskRecord, run, agentName)
	if err != nil || !ok {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	result, err := r.activateRoleSession(
		ctx,
		activation,
		taskRoleActivationReasonStarvation,
		r.startStarvationSession,
	)
	if err != nil {
		return err
	}
	if !result.created {
		r.logger.Info(
			"daemon: starvation worker already active",
			"session_id", result.info.ID,
			taskRoleRuntimeTaskIDKey, activation.TaskID,
			daemonLogRunIDKey, activation.RunID,
			"agent_name", activation.AgentName,
			"channel", activation.NetworkParticipation.ChannelID,
		)
		return nil
	}
	r.logger.Info(
		"daemon: starvation worker spawned",
		"session_id", result.info.ID,
		taskRoleRuntimeTaskIDKey, activation.TaskID,
		daemonLogRunIDKey, activation.RunID,
		"agent_name", activation.AgentName,
		"channel", activation.NetworkParticipation.ChannelID,
	)
	return nil
}

func (r *taskRoleRuntime) resolveStarvationAgent(
	ctx context.Context,
	taskRecord taskpkg.Task,
	run taskpkg.Run,
	spawner starvationSpawner,
) (string, error) {
	required := trimmedNonEmptyStrings(run.RequiredCapabilities)
	if len(required) == 0 &&
		taskRecord.Owner != nil && !taskRecord.Owner.IsZero() &&
		taskRecord.Owner.Kind.Normalize() == taskpkg.OwnerKindPool {
		if name := strings.TrimSpace(taskRecord.Owner.Ref); name != "" {
			return name, nil
		}
	}
	name, ok, err := spawner.resolveAgent(ctx, strings.TrimSpace(taskRecord.WorkspaceID), required)
	if err != nil {
		return "", fmt.Errorf("daemon: resolve starvation agent: %w", err)
	}
	if ok {
		return name, nil
	}
	return "", errStarvationSpawnUnresolvable
}

func (r *taskRoleRuntime) starvationActivation(
	taskRecord taskpkg.Task,
	run taskpkg.Run,
	agentName string,
) (taskRoleActivation, bool, error) {
	agentName = strings.TrimSpace(agentName)
	if agentName == "" {
		return taskRoleActivation{}, false, nil
	}
	if run.Status.Normalize() != taskpkg.TaskRunStatusQueued {
		return taskRoleActivation{}, false, nil
	}
	switch taskRecord.Status.Normalize() {
	case taskpkg.TaskStatusDraft, taskpkg.TaskStatusBlocked, taskpkg.TaskStatusCanceled:
		return taskRoleActivation{}, false, nil
	default:
	}
	activation := taskRoleActivation{
		TaskID:               strings.TrimSpace(taskRecord.ID),
		RunID:                strings.TrimSpace(run.ID),
		Scope:                taskRecord.Scope.Normalize(),
		WorkspaceID:          strings.TrimSpace(taskRecord.WorkspaceID),
		AgentName:            agentName,
		NetworkParticipation: new(run.NetworkSpecSnapshot()),
		Title:                strings.TrimSpace(taskRecord.Title),
	}
	applyTaskRoleDesignation(&activation, run)
	if err := r.applyActivationScope(&activation, taskRecord.ID); err != nil {
		return taskRoleActivation{}, false, err
	}
	return activation, true, nil
}

func (r *taskRoleRuntime) startStarvationSession(
	ctx context.Context,
	activation taskRoleActivation,
) (*session.Info, error) {
	opts, err := taskRoleCreateOpts(activation)
	if err != nil {
		return nil, err
	}
	ttlExpiresAt := r.now().UTC().Add(defaultStarvationWorkerTTL)
	opts.Lineage = &store.SessionLineage{
		SpawnRole:    session.DefaultSpawnRole,
		TTLExpiresAt: &ttlExpiresAt,
		SpawnBudget: store.SessionSpawnBudget{
			MaxChildren:           session.DefaultSpawnMaxChildren,
			MaxDepth:              session.DefaultSpawnMaxDepth,
			TTLSeconds:            int64(defaultStarvationWorkerTTL / time.Second),
			MaxActivePerWorkspace: defaultStarvationMaxActivePerWorkspace,
		},
	}
	return r.createRoleSession(ctx, opts)
}

func taskRoleSessionMatches(info *session.Info, activation taskRoleActivation) bool {
	if info == nil {
		return false
	}
	if !taskRoleSessionStateReusable(info.State) {
		return false
	}
	if strings.TrimSpace(info.AgentName) != activation.AgentName {
		return false
	}
	if activation.NetworkParticipation == nil || info.NetworkParticipation != *activation.NetworkParticipation {
		return false
	}
	if strings.TrimSpace(info.Name) != taskRoleSessionName(activation) {
		return false
	}
	switch activation.Scope {
	case taskpkg.ScopeWorkspace:
		return strings.TrimSpace(info.WorkspaceID) == activation.WorkspaceID
	case taskpkg.ScopeGlobal:
		return strings.TrimSpace(info.WorkspaceID) == "" &&
			strings.TrimSpace(info.Workspace) == activation.WorkspacePath
	default:
		return false
	}
}

func taskRoleSessionStateReusable(state session.State) bool {
	switch state {
	case session.StateStarting, session.StateActive:
		return true
	default:
		return false
	}
}

func taskRoleSessionName(activation taskRoleActivation) string {
	base := fmt.Sprintf(
		"task-role:%s:%s",
		activation.AgentName,
		strings.TrimSpace(activation.RunID),
	)
	fingerprint := taskRoleProfileFingerprint(activation)
	if fingerprint == "" {
		return base
	}
	return base + ":" + fingerprint
}

func taskRoleProfileFingerprint(activation taskRoleActivation) string {
	if activation.Profile == nil {
		return ""
	}
	profile := activation.Profile
	parts := []string{
		strings.TrimSpace(activation.Provider),
		strings.TrimSpace(activation.Model),
		string(profile.Sandbox.Mode.Normalize()),
		strings.TrimSpace(profile.Sandbox.SandboxRef),
		string(profile.Runtime.Mode.Normalize()),
		strings.Join(uniqueNonEmptyStrings(profile.Worker.RequiredCapabilities), "\x1f"),
	}
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(sum[:8])
}

func taskRolePromptOverlay(activation taskRoleActivation) string {
	title := firstNonEmpty(activation.Title, activation.TaskID)
	claimCommand := taskRoleClaimCommand(activation.RunID, activation.Capabilities)
	designation := taskRoleDesignationOverlay(activation)
	channelLine := ""
	if activation.NetworkParticipation != nil &&
		activation.NetworkParticipation.Mode == participation.ModeLive {
		channelLine = fmt.Sprintf("Coordination channel: %s", activation.NetworkParticipation.ChannelID)
	}
	return fmt.Sprintf(`A queued AGH task run is assigned to this agent.

Task: %s
Run: %s
%s
%s
Use `+"`%s`"+` once to claim work for this session before changing files. Complete or fail the claimed run through the AGH task lease commands from this same session.`,
		title,
		activation.RunID,
		channelLine,
		designation,
		claimCommand,
	)
}

func taskRoleDesignationOverlay(activation taskRoleActivation) string {
	if !activation.HasDesignation {
		return ""
	}
	parts := []string{
		fmt.Sprintf("group %s", activation.Designation.GroupID),
		fmt.Sprintf("index %d", activation.Designation.Index),
	}
	if brief := strings.TrimSpace(activation.Designation.Brief); brief != "" {
		parts = append(parts, fmt.Sprintf("brief %q", brief))
	}
	return "Designated fan-out assignment: " + strings.Join(parts, ", ") + "."
}

func applyTaskRoleDesignation(activation *taskRoleActivation, run taskpkg.Run) {
	if activation == nil {
		return
	}
	designation, ok := taskpkg.DesignationFromRun(run)
	if !ok {
		return
	}
	activation.Designation = designation
	activation.HasDesignation = true
}

func taskRoleClaimCommand(runID string, capabilities []string) string {
	args := []string{"agh task next --run-id " + shellQuoteSimple(strings.TrimSpace(runID)) + " --wait -o json"}
	for _, capability := range uniqueNonEmptyStrings(capabilities) {
		args = append(args, "--capability "+shellQuoteSimple(capability))
	}
	return strings.Join(args, " ")
}

func profileRequiredWorkerCapabilities(profile *taskpkg.ExecutionProfile) []string {
	if profile == nil {
		return nil
	}
	return append([]string(nil), profile.Worker.RequiredCapabilities...)
}

func uniqueNonEmptyStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}
	return result
}

func shellQuoteSimple(value string) string {
	if value == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func (r *taskRoleRuntime) logTaskRoleError(
	message string,
	err error,
	payload hookspkg.TaskRunEnqueuedPayload,
) {
	if r == nil {
		return
	}
	args := []any{
		taskRoleRuntimeTaskIDKey, strings.TrimSpace(payload.TaskID),
		daemonLogRunIDKey, strings.TrimSpace(payload.RunID),
		taskRoleRuntimeWorkspaceIDKey, strings.TrimSpace(payload.WorkspaceID),
		daemonNetworkChannelKey, strings.TrimSpace(payload.NetworkSpecSnapshot().ChannelID),
	}
	if err != nil {
		args = append(args, "error", err)
	}
	r.logger.Warn(message, args...)
}

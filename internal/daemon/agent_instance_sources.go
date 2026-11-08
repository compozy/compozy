package daemon

import (
	"context"
	"path/filepath"
	"time"

	"github.com/compozy/compozy/internal/agentinstance"
	"github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/heartbeat"
	"github.com/compozy/compozy/internal/heartbeat/source"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

func (r instanceDefinitions) projectInstanceSources(
	ctx context.Context,
	instance agentinstance.Instance,
	agent compozyconfig.AgentDef,
	artifacts *session.AgentArtifacts,
	workspace *workspacepkg.ResolvedWorkspace,
	payload *contract.AgentInstancePayload,
	reader instanceAttentionReader,
) {
	statesReader, ok := reader.(interface {
		SourceStates(context.Context, string, []source.Ref, bool, time.Time) ([]heartbeat.SourceState, error)
	})
	if !ok || agent.SourcePath == "" {
		return
	}
	refs, err := r.sourceDeclarations.resolve(ctx, r.agents, artifacts, heartbeat.AuthoringTarget{
		CheckInLimits: &heartbeat.CheckInLimits{
			MaxToolCalls:  workspace.Config.Agents.AlwaysOn.CheckInMaxToolCalls,
			MaxWriteCalls: workspace.Config.Agents.AlwaysOn.CheckInMaxWriteCalls,
		},
		ProfileID: instance.ProfileID, ProfileName: workspace.ProfileName,
		WorkspaceID: instance.WorkspaceID, WorkspaceRoot: workspace.RootDir,
		AgentName: instance.AgentName, AgentPath: agent.SourcePath, Config: workspace.Config.Agents.Heartbeat,
	})
	if err != nil {
		return
	}
	scripts := newInstanceScriptApprovals(instance, agent, artifacts, r.scriptsEnabled, reader)
	out := make([]heartbeat.HeartbeatSourceStatus, 0, len(refs))
	if len(refs) == 0 {
		payload.Sources = out
		return
	}
	states, err := statesReader.SourceStates(ctx, instance.ID, refs, true, time.Time{})
	if err != nil {
		return
	}
	for i, ref := range refs {
		state := states[i]
		status := heartbeat.HeartbeatSourceStatus{
			ID: ref.ID, Kind: ref.Kind, Script: ref.Script, Status: "pending_first_poll",
			LastPolledAt: state.LastPolledAt, LastSuccessAt: state.LastSuccessAt,
			LastItemCount: state.LastItemCount, FailureStreak: state.FailureStreak, Error: state.Error,
		}
		if state.LastPolledAt != "" {
			status.Status = heartbeat.SourceResultStatus(state.Error)
		}
		if !state.NextPollAt.IsZero() {
			status.NextPollAt = store.FormatTimestamp(state.NextPollAt)
		}
		if scripts.ready && ref.Script != "" {
			projectScriptApproval(ctx, scripts, ref.Script, &status)
		}
		out = append(out, status)
	}
	payload.Sources = out
}

// instanceScriptApprovals is what an instance read needs to tell whether a script waits for the
// operator: approvals keyed like the poller's, and only where the poller would run the script.
type instanceScriptApprovals struct {
	approvals source.ApprovalReader
	profileID string
	dir       string
	ready     bool
}

func newInstanceScriptApprovals(
	instance agentinstance.Instance,
	agent compozyconfig.AgentDef,
	artifacts *session.AgentArtifacts,
	scriptsEnabled bool,
	reader instanceAttentionReader,
) instanceScriptApprovals {
	approvals, ok := reader.(source.ApprovalReader)
	packageOwned := artifacts != nil && artifacts.PackageOwned
	return instanceScriptApprovals{
		approvals: approvals,
		profileID: instance.ProfileID,
		dir:       filepath.Dir(agent.SourcePath),
		ready:     ok && !packageOwned && scriptsEnabled,
	}
}

// projectScriptApproval reads the script's bytes without running it, so approval shows before the
// first check and on a paused instance. A script that can't run (missing, not executable) has
// nothing to approve; its poll status says why.
func projectScriptApproval(
	ctx context.Context,
	scripts instanceScriptApprovals,
	name string,
	status *heartbeat.HeartbeatSourceStatus,
) {
	_, hash, approved, err := source.ApprovedScript(ctx, scripts.approvals, scripts.profileID, scripts.dir, name)
	if err != nil {
		return
	}
	// The resolved path is the approval's identity: two copies of a definition need two approvals.
	if dir, err := filepath.EvalSymlinks(scripts.dir); err == nil {
		status.ScriptPath = filepath.Join(dir, "sources", name)
	}
	status.ScriptSHA256 = hash
	status.AwaitingApproval = !approved
}

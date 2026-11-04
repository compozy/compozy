package daemon

import (
	"context"
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
	workspace workspacepkg.ResolvedWorkspace,
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
		ProfileID: instance.ProfileID, ProfileName: workspace.ProfileName,
		WorkspaceID: instance.WorkspaceID, WorkspaceRoot: workspace.RootDir,
		AgentName: instance.AgentName, AgentPath: agent.SourcePath, Config: workspace.Config.Agents.Heartbeat,
	})
	if err != nil {
		return
	}
	out := make([]heartbeat.SourceStatus, 0, len(refs))
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
		status := heartbeat.SourceStatus{
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
		out = append(out, status)
	}
	payload.Sources = out
}

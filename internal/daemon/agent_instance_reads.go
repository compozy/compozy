package daemon

import (
	"context"
	"errors"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"strings"

	"github.com/compozy/compozy/internal/agentinstance"
	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/session"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

type instanceAttentionReader interface {
	NeedsAttention(context.Context, string) (bool, error)
}

func (r instanceDefinitions) instancePayload(ctx context.Context, instance agentinstance.Instance, attention instanceAttentionReader) (contract.AgentInstancePayload, error) {
	payload := core.AgentInstancePayload(instance)
	var resolved workspacepkg.ResolvedWorkspace
	var err error
	if instance.WorkspaceID == "" {
		resolved, err = r.workspaces.ResolveUser(ctx, instance.ProfileID)
		payload.Scope = "Global"
	} else {
		name, lookupErr := r.profiles.ProfileName(ctx, instance.ProfileID)
		if lookupErr != nil {
			return payload, lookupErr
		}
		resolved, err = r.workspaces.ResolveForProfile(ctx, instance.WorkspaceID, name)
		payload.Scope = resolved.Name
	}
	if err != nil {
		return payload, err
	}
	agent, err := r.agents.ResolveAgent(instance.AgentName, &resolved)
	if errors.Is(err, workspacepkg.ErrAgentNotAvailable) {
		agent = compozyconfig.AgentDef{Name: instance.AgentName}
	} else if err != nil {
		return payload, err
	}
	payload.Label = agent.Label()
	payload.ResolvedAppearance, err = agent.ResolveAppearance()
	if err != nil {
		return payload, err
	}
	if agent.Provider != "" {
		runtime, err := resolved.Config.ResolveAgent(agent)
		if err != nil {
			return payload, err
		}
		payload.Provider = runtime.Provider
	}
	if instance.MainSessionID != "" {
		info, err := r.sessions.Status(ctx, instance.MainSessionID)
		if err != nil && !errors.Is(err, session.ErrSessionNotFound) {
			return payload, err
		}
		if info != nil && info.ProfileID == instance.ProfileID {
			projectInstanceMain(&payload, info)
		}
	}
	payload.NeedsAttention, err = attention.NeedsAttention(ctx, instance.ID)
	if err != nil {
		return payload, err
	}
	return payload, nil
}

func projectInstanceMain(payload *contract.AgentInstancePayload, info *session.Info) {
	payload.Provider = info.Provider
	payload.Activity = "idle"
	if info.Liveness != nil && info.Liveness.Activity != nil {
		activity := info.Liveness.Activity
		payload.Now = strings.Join(strings.Fields(activity.LastActivityDetail), " ")
		chars := []rune(payload.Now)
		if len(chars) > 80 {
			payload.Now = string(chars[:80])
		}
		if activity.TurnID != "" && info.State == session.StateActive {
			payload.Activity = "working"
		}
	}
	if info.State == session.StateStarting {
		payload.Activity = "unknown"
	}
	if info.Failure != nil {
		payload.Activity = "failed"
	}
	if info.PendingPermissionCount > 0 || info.PendingClarifyCount > 0 {
		payload.Activity = "waiting"
	}
}

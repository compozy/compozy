package daemon

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/monitor"

	compozyconfig "github.com/compozy/compozy/internal/config"

	"github.com/compozy/compozy/internal/agentinstance"
	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

type instanceAttentionReader interface {
	HasLiveInteractions(context.Context, string) (bool, error)
	NeedsAttention(context.Context, string) (bool, error)
}

func (r instanceDefinitions) instancePayload(
	ctx context.Context,
	instance agentinstance.Instance,
	attention instanceAttentionReader,
) (contract.AgentInstancePayload, error) {
	payload := core.AgentInstancePayload(instance)
	if err := r.projectInstanceDefinition(ctx, instance, &payload, attention); err != nil {
		return payload, err
	}
	if instance.MainSessionID != "" {
		info, err := r.sessions.Status(ctx, instance.MainSessionID)
		if err != nil && !errors.Is(err, session.ErrSessionNotFound) {
			return payload, err
		}
		if info != nil && info.ProfileID == instance.ProfileID {
			projectInstanceMain(&payload, info)
			if r.monitors != nil {
				projectInstanceMonitors(&payload, r.monitors.List(instance.MainSessionID))
			}
		}
	}
	needsAttention, err := attention.NeedsAttention(ctx, instance.ID)
	if err != nil {
		return payload, err
	}
	payload.NeedsAttention = needsAttention
	if needsAttention {
		live, err := attention.HasLiveInteractions(ctx, instance.ID)
		if err != nil {
			return payload, err
		}
		payload.AttentionDismissible = !live
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
		payload.Activity = daemonUnknownValue
	}
	// A stop the operator or daemon shutdown asked for is not a failure of the agent.
	if info.Failure != nil && info.Failure.Kind != store.FailureCanceled {
		payload.Activity = "failed"
	}
	if info.PendingPermissionCount > 0 || info.PendingClarifyCount > 0 {
		payload.Activity = "waiting"
	}
	if payload.Activity != "working" {
		payload.Now = ""
	}
}

func projectInstanceMonitors(payload *contract.AgentInstancePayload, monitors []monitor.Monitor) {
	for _, item := range monitors {
		if item.Status != monitor.StatusActive && item.Status != monitor.StatusDelivering {
			continue
		}
		candidate := &contract.AgentInstanceNextWake{Kind: string(item.Kind), Description: item.Description}
		if item.Kind == monitor.KindTimer && !item.DeadlineAt.IsZero() {
			candidate.At = item.DeadlineAt.UTC().Format(time.RFC3339Nano)
		}
		current := payload.NextWake
		if current == nil || (candidate.At != "" && (current.At == "" || earlierWake(candidate.At, current.At))) {
			payload.NextWake = candidate
		}
	}
}

func earlierWake(a, b string) bool {
	first, firstErr := time.Parse(time.RFC3339Nano, a)
	second, secondErr := time.Parse(time.RFC3339Nano, b)
	return firstErr == nil && (secondErr != nil || first.Before(second))
}

func (r instanceDefinitions) projectInstanceDefinition(
	ctx context.Context,
	instance agentinstance.Instance,
	payload *contract.AgentInstancePayload,
	reader instanceAttentionReader,
) error {
	var resolved workspacepkg.ResolvedWorkspace
	var err error
	if instance.WorkspaceID == "" {
		resolved, err = r.workspaces.ResolveUser(ctx, instance.ProfileID)
		payload.Scope = daemonGlobalLabel
	} else {
		name, lookupErr := r.profiles.ProfileName(ctx, instance.ProfileID)
		if lookupErr != nil {
			return lookupErr
		}
		resolved, err = r.workspaces.ResolveForProfile(ctx, instance.WorkspaceID, name)
		payload.Scope = resolved.Name
	}
	if errors.Is(err, workspacepkg.ErrWorkspaceRootMissing) {
		registered, lookupErr := r.workspaces.Get(ctx, instance.WorkspaceID)
		if lookupErr != nil {
			return lookupErr
		}
		payload.Scope = registered.Name
		fallback := compozyconfig.AgentDef{Name: instance.AgentName}
		payload.Label = fallback.Label()
		payload.ResolvedAppearance = fallback.AppearanceOrDefault()
		return nil
	}
	if err != nil {
		return err
	}
	var artifacts *session.AgentArtifacts
	var agent compozyconfig.AgentDef
	if resolver, ok := r.agents.(session.AgentArtifactResolver); ok {
		result, resolveErr := resolver.ResolveAgentArtifacts(instance.AgentName, &resolved)
		agent, err = result.Agent, resolveErr
		if errors.Is(resolveErr, workspacepkg.ErrAgentNotAvailable) {
			agent, err = r.agents.ResolveAgent(instance.AgentName, &resolved)
		} else {
			artifacts = &result
		}
	} else {
		agent, err = r.agents.ResolveAgent(instance.AgentName, &resolved)
	}
	if errors.Is(err, workspacepkg.ErrAgentNotAvailable) {
		agent = compozyconfig.AgentDef{Name: instance.AgentName}
	} else if err != nil {
		return err
	}
	r.projectInstanceSources(ctx, instance, agent, artifacts, &resolved, payload, reader)
	projectInstanceAuthority(ctx, instance, agent, artifacts, &resolved, payload, reader)
	payload.Label = agent.Label()
	// An unsupported saved appearance resolves to the default face; the definition read
	// carries the agent_appearance_invalid diagnostic, so the instance read never fails on it.
	payload.ResolvedAppearance = agent.AppearanceOrDefault()
	if agent.Provider != "" {
		runtime, err := resolved.Config.ResolveAgent(agent)
		if err != nil {
			return err
		}
		payload.Provider = runtime.Provider
	}
	return nil
}

func (r instanceDefinitions) instancePeer(
	ctx context.Context,
	instance agentinstance.Instance,
) (contract.AgentInstancePeer, error) {
	peer := contract.AgentInstancePeer{
		Name:  instance.AgentName,
		Scope: extensionInstallationScopeGlobal,
		State: instance.State,
	}
	var resolved workspacepkg.ResolvedWorkspace
	var err error
	if instance.WorkspaceID == "" {
		resolved, err = r.workspaces.ResolveUser(ctx, instance.ProfileID)
	} else {
		name, lookupErr := r.profiles.ProfileName(ctx, instance.ProfileID)
		if lookupErr != nil {
			return peer, lookupErr
		}
		resolved, err = r.workspaces.ResolveForProfile(ctx, instance.WorkspaceID, name)
		peer.Scope = resolved.Name
	}
	if err != nil {
		return peer, err
	}
	agent, err := r.agents.ResolveAgent(instance.AgentName, &resolved)
	if errors.Is(err, workspacepkg.ErrAgentNotAvailable) {
		return peer, nil
	}
	if err != nil {
		return peer, err
	}
	peer.Description = agent.Description
	return peer, nil
}

package daemon

import (
	"context"
	"time"

	automationpkg "github.com/compozy/compozy/internal/automation"
	"github.com/compozy/compozy/internal/heartbeat"
	"github.com/compozy/compozy/internal/store/globaldb"
)

type automationInstanceEvents struct {
	sink        automationpkg.InstanceEventSink
	enabled     bool
	definitions instanceDefinitions
	policies    heartbeat.PolicyResolver
}

var _ automationpkg.InstanceEventSink = (*automationInstanceEvents)(nil)

func automationInstanceEventSink(state *bootState) automationpkg.InstanceEventSink {
	provider, ok := state.registry.(interface {
		InstanceEvents(time.Duration) *globaldb.InstanceEventRepo
	})
	if !ok {
		return nil
	}
	catalog := agentCatalogDependency(
		state.agentCatalog,
		agentSidecarCatalogs{soul: state.soulCatalog, heartbeat: state.heartbeatCatalog},
	)
	return &automationInstanceEvents{
		definitions: instanceDefinitions{
			workspaces: state.workspaceResolver,
			profiles:   state.profiles,
			agents:     catalog,
		},
		policies: catalog,
		sink:     provider.InstanceEvents(state.cfg.Agents.Heartbeat.MinInterval),
		enabled:  state.cfg.Agents.Heartbeat.Enabled,
	}
}

func (s *automationInstanceEvents) Deliver(
	ctx context.Context,
	target automationpkg.InstanceTarget,
	event automationpkg.InstanceEvent,
) (automationpkg.InstanceEventDelivery, error) {
	reason := ""
	if !s.enabled {
		reason = "heartbeat_disabled"
	} else if target.ChecksSupported {
		var err error
		reason, err = s.checkUnavailableReason(ctx, target)
		if err != nil {
			reason = "heartbeat_policy_unavailable"
		}
	}
	target.ChecksSupported = target.ChecksSupported && reason == ""
	result, err := s.sink.Deliver(ctx, target, event)
	if err == nil && reason != "" {
		result.CheckScheduled, result.Reason = false, reason
	}
	return result, err
}

func (s *automationInstanceEvents) checkUnavailableReason(
	ctx context.Context,
	target automationpkg.InstanceTarget,
) (string, error) {
	workspace, err := s.definitions.resolveWorkspace(ctx, target.ProfileID, target.WorkspaceID)
	if err != nil {
		return "", err
	}
	agent, err := s.definitions.agents.ResolveAgent(target.AgentName, &workspace)
	if err != nil {
		return "", err
	}
	policy, err := resolveTimerPolicy(ctx, s.policies, heartbeat.AuthoringTarget{
		ProfileID: target.ProfileID, ProfileName: workspace.ProfileName, WorkspaceID: target.WorkspaceID,
		WorkspaceRoot: workspace.RootDir, AgentName: target.AgentName, AgentPath: agent.SourcePath,
		Config: workspace.Config.Agents.Heartbeat,
	})
	if err != nil {
		return "", err
	}
	if !policy.Enabled {
		return "policy_disabled", nil
	}
	return "", nil
}

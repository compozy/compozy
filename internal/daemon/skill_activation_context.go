package daemon

import (
	"context"

	skillspkg "github.com/compozy/agh/internal/skills"
	toolspkg "github.com/compozy/agh/internal/tools"
)

func newSkillActivationContextProvider(state *bootState) skillspkg.ActivationContextProvider {
	return func(ctx context.Context, target skillspkg.ActivationTarget) (skillspkg.ActivationContext, error) {
		if !target.NeedsTools || state == nil || state.toolRegistry == nil {
			return skillspkg.ActivationContext{}, nil
		}

		views, err := state.toolRegistry.List(ctx, toolspkg.Scope{
			WorkspaceID: target.WorkspaceID,
			SessionID:   target.SessionID,
			AgentName:   target.AgentName,
		})
		if err != nil {
			return skillspkg.ActivationContext{}, err
		}
		toolIDs := make([]string, 0, len(views))
		for idx := range views {
			toolIDs = append(toolIDs, string(views[idx].Descriptor.ID))
		}
		return skillspkg.ActivationContext{Tools: toolIDs}, nil
	}
}

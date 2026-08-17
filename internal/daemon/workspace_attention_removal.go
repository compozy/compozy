package daemon

import (
	"context"
	"errors"
	"fmt"

	settingspkg "github.com/compozy/compozy/internal/settings"
)

type attentionWorkspaceMuteMutator interface {
	SetAttentionWorkspaceMuted(ctx context.Context, workspaceID string, muted bool) (bool, error)
}

type attentionWorkspaceRemovalPreparation struct {
	mutator     attentionWorkspaceMuteMutator
	workspaceID string
	pruned      bool
}

func prepareAttentionWorkspaceRemoval(
	state *bootState,
	workspaceID string,
) (*attentionWorkspaceRemovalPreparation, error) {
	if state == nil || state.attentionMuteMutator == nil {
		return nil, errors.New("daemon: settings service is required for workspace attention cleanup")
	}
	return &attentionWorkspaceRemovalPreparation{
		mutator: state.attentionMuteMutator, workspaceID: workspaceID,
	}, nil
}

func (p *attentionWorkspaceRemovalPreparation) BeforeDelete(ctx context.Context) error {
	changed, err := p.mutator.SetAttentionWorkspaceMuted(
		settingspkg.WithMutationSource(ctx, "workspace.unregister"),
		p.workspaceID,
		false,
	)
	p.pruned = changed
	if err != nil {
		return fmt.Errorf("daemon: prune workspace attention mute: %w", err)
	}
	return nil
}

func (*attentionWorkspaceRemovalPreparation) Commit(context.Context) error { return nil }

func (p *attentionWorkspaceRemovalPreparation) Rollback(ctx context.Context) error {
	if !p.pruned {
		return nil
	}
	_, err := p.mutator.SetAttentionWorkspaceMuted(
		settingspkg.WithMutationSource(ctx, "workspace.unregister.rollback"),
		p.workspaceID,
		true,
	)
	if err != nil {
		return fmt.Errorf("daemon: restore workspace attention mute: %w", err)
	}
	p.pruned = false
	return nil
}

package session

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/acp"
	"github.com/compozy/agh/internal/network/participation"
)

// PromptNetwork sends one network-originated prompt turn to an active session.
func (m *Manager) PromptNetwork(
	ctx context.Context,
	id string,
	msg string,
	meta ...acp.PromptNetworkMeta,
) (<-chan acp.AgentEvent, error) {
	if len(meta) > 1 {
		return nil, errors.New("session: network prompt accepts at most one metadata value")
	}
	target := strings.TrimSpace(id)
	if target == "" {
		return nil, errors.New("session: session id is required")
	}
	targetSession, err := m.lookup(target)
	if err != nil {
		return nil, err
	}
	if targetSession.NetworkParticipation.Mode != participation.ModeLive {
		return nil, fmt.Errorf("session: %w: %s", participation.ErrNotParticipating, target)
	}

	var promptMeta acp.PromptMeta
	if len(meta) > 0 {
		promptMeta.Network = &meta[0]
	}
	return m.PromptWithOpts(ctx, id, PromptOpts{
		Message:    msg,
		TurnSource: TurnSourceNetwork,
		PromptMeta: promptMeta,
	})
}

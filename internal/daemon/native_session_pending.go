package daemon

import (
	"context"
	"errors"
	"fmt"
	"github.com/compozy/compozy/internal/api/contract"
	core "github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/store"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

func (n *daemonNativeTools) sessionPending(ctx context.Context, scope toolspkg.Scope, req toolspkg.CallRequest) (toolspkg.ToolResult, error) {
	var input struct {
		Workspace   string `json:"workspace,omitempty"`
		SessionID   string `json:"session_id"`
		Descendants bool   `json:"descendants,omitempty"`
	}
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	id, err := n.nativeSessionInputScope(ctx, scope, req.ToolID, input.Workspace, input.SessionID)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	var rows []store.PendingInteraction
	if input.Descendants {
		reader, ok := n.deps.Sessions.(core.SessionDescendantInteractionManager)
		if !ok {
			return toolspkg.ToolResult{}, errors.New("daemon: descendant interactions are required")
		}
		rows, err = reader.DescendantPendingInteractions(ctx, id, nil)
	} else {
		reader, ok := n.deps.Sessions.(core.SessionAttentionManager)
		if !ok {
			return toolspkg.ToolResult{}, errors.New("daemon: session attention is required")
		}
		rows, err = reader.PendingInteractions(ctx, id, nil)
	}
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	return structuredResult(contract.SessionInteractionsResponse{Interactions: core.PendingInteractionPayloadsFromStore(rows)}, fmt.Sprintf("%d pending interactions", len(rows)))
}

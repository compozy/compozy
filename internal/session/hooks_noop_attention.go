package session

import (
	"context"

	hookspkg "github.com/compozy/compozy/internal/hooks"
)

type noopAttentionHooks struct{}

func (noopAttentionHooks) DispatchSessionAttentionChanged(
	_ context.Context,
	payload hookspkg.SessionAttentionChangedPayload,
) (hookspkg.SessionAttentionChangedPayload, error) {
	return payload, nil
}

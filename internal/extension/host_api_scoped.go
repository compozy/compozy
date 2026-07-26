package extensionpkg

import (
	"context"
	"encoding/json"

	"github.com/compozy/agh/internal/resources"
)

// HandleWithResourceActor dispatches a Host API call with an authenticated resource-session actor.
// Capability checks and rate limiting still run through Handle.
func (h *HostAPIHandler) HandleWithResourceActor(
	ctx context.Context,
	principal string,
	method string,
	params json.RawMessage,
	actor resources.MutationActor,
) (any, error) {
	return h.Handle(
		withHostAPIResourceSession(ctx, &hostAPIResourceSession{Actor: actor}),
		principal,
		method,
		params,
	)
}

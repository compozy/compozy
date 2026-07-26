package daemon

import (
	"context"
	"strings"

	"github.com/compozy/agh/internal/network/participation"
	toolspkg "github.com/compozy/agh/internal/tools"
)

func (n *daemonNativeTools) networkParticipationAvailability(
	ready func() bool,
) toolspkg.NativeAvailabilityFunc {
	return func(ctx context.Context, scope toolspkg.Scope) toolspkg.Availability {
		if ready == nil || !ready() || n == nil || n.deps == nil {
			return toolspkg.Unavailable(toolspkg.ReasonDependencyMissing)
		}
		sessionID := strings.TrimSpace(scope.SessionID)
		if sessionID == "" {
			return toolspkg.Available()
		}
		if n.deps.Sessions == nil {
			return toolspkg.Unavailable(toolspkg.ReasonDependencyMissing)
		}
		info, err := n.deps.Sessions.Status(ctx, sessionID)
		if err != nil || info == nil {
			return toolspkg.Unavailable(toolspkg.ReasonBackendUnhealthy)
		}
		if info.NetworkParticipation.Mode != participation.ModeLive {
			return toolspkg.Unavailable(toolspkg.ReasonNetworkNotParticipating)
		}
		return toolspkg.Available()
	}
}

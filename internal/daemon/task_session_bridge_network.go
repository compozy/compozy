package daemon

import (
	"context"
	"errors"
	"strings"

	"github.com/compozy/compozy/internal/network/participation"
	"github.com/compozy/compozy/internal/session"
	taskpkg "github.com/compozy/compozy/internal/task"
)

type taskBridgeNetworkPeerBinder interface {
	BindNetworkPeer(
		ctx context.Context,
		sessionID string,
		spec participation.Spec,
		ownerKey string,
	) error
	RestoreNetworkPeer(ctx context.Context, sessionID string) error
}

var _ taskBridgeNetworkPeerBinder = (*session.Manager)(nil)
var _ taskpkg.RunNetworkSessionBinder = (*taskSessionBridge)(nil)

func (b *taskSessionBridge) BindTaskRunNetwork(
	ctx context.Context,
	sessionID string,
	run taskpkg.Run,
) error {
	if ctx == nil {
		return errors.New("daemon: bind task-run network context is required")
	}
	spec := run.NetworkSpecSnapshot()
	if spec.Mode != participation.ModeLive {
		return nil
	}
	binder, ok := b.sessions.(taskBridgeNetworkPeerBinder)
	if !ok {
		return nil
	}
	owner := participation.OwnerRef{
		WorkspaceID: strings.TrimSpace(spec.WorkspaceID),
		Kind:        participation.OwnerKindTaskRun,
		ID:          strings.TrimSpace(run.ID),
	}
	if loopRunID := strings.TrimSpace(run.LoopRunID); loopRunID != "" {
		owner.Kind = participation.OwnerKindLoopRun
		owner.ID = loopRunID
	}
	return binder.BindNetworkPeer(ctx, sessionID, spec, participation.OwnerKey(owner))
}

func (b *taskSessionBridge) RestoreTaskRunNetwork(ctx context.Context, sessionID string) error {
	if ctx == nil {
		return errors.New("daemon: restore task-run network context is required")
	}
	binder, ok := b.sessions.(taskBridgeNetworkPeerBinder)
	if !ok {
		return nil
	}
	return binder.RestoreNetworkPeer(ctx, sessionID)
}

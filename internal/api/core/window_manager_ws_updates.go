package core

import (
	"errors"
	"fmt"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/windowmanager"
)

type encodedWindowManagerUpdate struct {
	snapshot *contract.WindowManagerSnapshotFrame
	event    *contract.WindowManagerEventFrame
	client   *contract.WindowManagerClientFrame
}

func encodeWindowManagerUpdate(
	workspaceID windowmanager.WorkspaceID,
	update windowmanager.SubscriptionUpdate,
) (encodedWindowManagerUpdate, error) {
	switch {
	case update.Snapshot != nil && update.Event == nil && update.Client == nil:
		payload, err := contract.WindowManagerSnapshotFromDomain(*update.Snapshot)
		if err != nil {
			return encodedWindowManagerUpdate{}, fmt.Errorf("encode window-manager snapshot update: %w", err)
		}
		return encodedWindowManagerUpdate{
			snapshot: &contract.WindowManagerSnapshotFrame{
				Type: contract.WindowManagerFrameSnapshot, WorkspaceID: workspaceID,
				Revision: payload.Revision, Snapshot: payload,
			},
		}, nil
	case update.Event != nil && update.Client == nil && update.Snapshot == nil:
		payload, err := contract.WindowManagerEventFromDomain(*update.Event)
		if err != nil {
			return encodedWindowManagerUpdate{}, fmt.Errorf("encode window-manager event update: %w", err)
		}
		return encodedWindowManagerUpdate{
			event: &contract.WindowManagerEventFrame{
				Type: contract.WindowManagerFrameEvent, WorkspaceID: workspaceID,
				Revision: payload.Revision, Event: payload,
			},
		}, nil
	case update.Client != nil && update.Event == nil && update.Snapshot == nil:
		payload, err := contract.WindowManagerClientFromDomain(*update.Client)
		if err != nil {
			return encodedWindowManagerUpdate{}, fmt.Errorf("encode window-manager client update: %w", err)
		}
		return encodedWindowManagerUpdate{
			client: &contract.WindowManagerClientFrame{
				Type: contract.WindowManagerFrameClient, WorkspaceID: workspaceID,
				Revision: payload.PresentationRevision, Client: payload,
			},
		}, nil
	default:
		return encodedWindowManagerUpdate{}, errors.New(
			"window-manager subscription update must contain exactly one payload",
		)
	}
}

func (update encodedWindowManagerUpdate) write(socket *windowManagerSocket) error {
	if update.snapshot != nil {
		socket.revision = update.snapshot.Revision
		return socket.writeJSON(*update.snapshot)
	}
	if update.event != nil {
		socket.revision = max(socket.revision, update.event.Revision)
		return socket.writeJSON(*update.event)
	}
	if update.client != nil {
		return socket.writeJSON(*update.client)
	}
	return errors.New("window-manager encoded update has no payload")
}

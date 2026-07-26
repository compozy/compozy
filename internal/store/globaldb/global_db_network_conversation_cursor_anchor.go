package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
)

func lookupNetworkConversationCursorMessageID(
	ctx context.Context,
	exec networkSQLExecutor,
	ref store.NetworkChannelRef,
	surface string,
	containerID string,
	sequence int64,
) (string, error) {
	if sequence == 0 {
		return "", nil
	}
	messageID, err := sqlcgen.New(exec).GetNetworkCursorMessageID(
		ctx,
		sqlcgen.GetNetworkCursorMessageIDParams{
			WorkspaceID: ref.WorkspaceID, Channel: ref.Channel,
			Surface: nullableNetworkString(surface), Sequence: sequence,
			ContainerID: nullableNetworkString(containerID),
		},
	)
	if err != nil {
		return "", fmt.Errorf("store: lookup network cursor message anchor: %w", err)
	}
	return messageID, nil
}

func lookupNetworkConversationCursorSequence(
	ctx context.Context,
	exec networkSQLExecutor,
	ref store.NetworkChannelRef,
	surface string,
	containerID string,
	messageID string,
) (int64, error) {
	sequence, err := sqlcgen.New(exec).GetNetworkCursorSequence(
		ctx,
		sqlcgen.GetNetworkCursorSequenceParams{
			WorkspaceID: ref.WorkspaceID, Channel: ref.Channel,
			Surface: nullableNetworkString(surface), MessageID: messageID,
			ContainerID: nullableNetworkString(containerID),
		},
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, fmt.Errorf("%w: network cursor message anchor is missing", store.ErrNetworkCursorInvalid)
		}
		return 0, fmt.Errorf("store: lookup network cursor sequence anchor: %w", err)
	}
	return sequence, nil
}

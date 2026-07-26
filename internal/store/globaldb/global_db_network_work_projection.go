package globaldb

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
)

func networkWorkProjectionForState(
	current string,
	entry store.NetworkConversationMessage,
) (networkWorkMutation, error) {
	if strings.TrimSpace(entry.WorkID) == "" {
		return networkWorkMutation{}, nil
	}
	current = strings.TrimSpace(current)
	if current == "" {
		if networkMessageOpensWork(entry) {
			return networkWorkMutation{opened: true, state: store.NetworkWorkStateSubmitted}, nil
		}
		return networkWorkMutation{}, nil
	}
	next, transitioned, err := nextNetworkWorkState(current, entry)
	if err != nil {
		return networkWorkMutation{}, err
	}
	return networkWorkMutation{transitioned: transitioned, state: next}, nil
}

func deriveNetworkTimelineWorkProjection(
	ctx context.Context,
	exec networkSQLExecutor,
	entry store.NetworkConversationMessage,
) (networkWorkMutation, error) {
	if strings.TrimSpace(entry.WorkID) == "" {
		return networkWorkMutation{}, nil
	}
	currentValue, err := sqlcgen.New(exec).GetPriorNetworkTimelineWorkState(
		ctx,
		sqlcgen.GetPriorNetworkTimelineWorkStateParams{
			WorkspaceID: entry.WorkspaceID, MessageID: entry.MessageID,
		},
	)
	if err != nil {
		return networkWorkMutation{}, fmt.Errorf(
			"store: read prior network work projection for message %q: %w",
			entry.MessageID,
			err,
		)
	}
	current, err := networkTextValue(currentValue)
	if err != nil {
		return networkWorkMutation{}, fmt.Errorf(
			"store: read prior network work projection for message %q: %w",
			entry.MessageID,
			err,
		)
	}
	projection, err := networkWorkProjectionForState(current, entry)
	if err != nil {
		return networkWorkMutation{}, fmt.Errorf(
			"store: derive network work projection for message %q: %w",
			entry.MessageID,
			err,
		)
	}
	return projection, nil
}

func persistNetworkTimelineWorkProjection(
	ctx context.Context,
	exec networkSQLExecutor,
	entry store.NetworkConversationMessage,
	projection networkWorkMutation,
) error {
	if err := sqlcgen.New(exec).UpdateNetworkTimelineWorkProjection(
		ctx,
		sqlcgen.UpdateNetworkTimelineWorkProjectionParams{
			WorkOpened: networkBoolInt64(
				projection.opened,
			),
			WorkTransitioned: networkBoolInt64(projection.transitioned),
			WorkState:        strings.TrimSpace(projection.state),
			WorkspaceID:      entry.WorkspaceID,
			MessageID:        entry.MessageID,
		},
	); err != nil {
		return fmt.Errorf(
			"store: persist network work projection for message %q: %w",
			entry.MessageID,
			err,
		)
	}
	return nil
}

func networkTextValue(value any) (string, error) {
	switch typed := value.(type) {
	case nil:
		return "", nil
	case string:
		return typed, nil
	case []byte:
		return string(typed), nil
	default:
		return "", fmt.Errorf("unexpected SQLite text type %T", value)
	}
}

func networkBoolInt64(value bool) int64 {
	if value {
		return 1
	}
	return 0
}

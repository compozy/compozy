package globaldb

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/agh/internal/store"
	taskpkg "github.com/compozy/agh/internal/task"
)

const (
	networkWakeEventAdmitted  = "admitted"
	networkWakeEventClaimed   = "claimed"
	networkWakeEventHeartbeat = "heartbeat"
	networkWakeEventReleased  = "released"
	networkWakeEventSettled   = "settled"
	networkWakeEventStateKey  = "state"
)

type networkWakeEvent struct {
	workspaceID     string
	wakeID          string
	taskRunID       string
	ownerKey        string
	targetSessionID string
	eventType       string
	state           string
	claimTokenHash  string
	usageState      string
	actualWallMS    *int64
	inputTokens     *int64
	outputTokens    *int64
	reason          string
	actor           taskpkg.ActorIdentity
	at              time.Time
}

func appendNetworkWakeEventWithExecutor(
	ctx context.Context,
	exec globalSQLExecutor,
	event networkWakeEvent,
) error {
	if err := validateNetworkWakeEvent(event); err != nil {
		return err
	}
	// dynamic-sql: Network wake lifecycle events share transactions owned by Task and Network repositories.
	const query = `
INSERT INTO network_wake_events (
  workspace_id, wake_id, task_run_id, owner_key, target_session_id,
  event_type, state, claim_token_hash, usage_state, actual_wall_ms,
  input_tokens, output_tokens, reason, actor_kind, actor_ref, timestamp
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	if _, err := exec.ExecContext(
		ctx,
		query,
		event.workspaceID,
		event.wakeID,
		event.taskRunID,
		event.ownerKey,
		event.targetSessionID,
		event.eventType,
		event.state,
		event.claimTokenHash,
		event.usageState,
		nullableNetworkInt64(event.actualWallMS),
		nullableNetworkInt64(event.inputTokens),
		nullableNetworkInt64(event.outputTokens),
		event.reason,
		string(event.actor.Kind),
		event.actor.Ref,
		store.FormatTimestamp(event.at.UTC()),
	); err != nil {
		return fmt.Errorf("store: append network wake %s event: %w", event.eventType, err)
	}
	return nil
}

func validateNetworkWakeEvent(event networkWakeEvent) error {
	for field, value := range map[string]string{
		watchEventsContentWorkspaceIDKey: event.workspaceID,
		"wake_id":                        event.wakeID,
		loopRunEventPayloadKeyTaskRunID:  event.taskRunID,
		"owner_key":                      event.ownerKey,
		"target_session_id":              event.targetSessionID,
		"event_type":                     event.eventType,
		networkWakeEventStateKey:         event.state,
	} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("store: network wake event %s is required", field)
		}
	}
	if err := event.actor.Validate("network wake event actor"); err != nil {
		return err
	}
	if event.at.IsZero() {
		return fmt.Errorf("store: network wake event timestamp is required")
	}
	return nil
}

func nullableNetworkInt64(value *int64) any {
	if value == nil {
		return nil
	}
	return *value
}

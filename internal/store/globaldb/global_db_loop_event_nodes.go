package globaldb

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	looppkg "github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
	taskpkg "github.com/compozy/agh/internal/task"
)

func appendLoopNodeRunningEventWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	run taskpkg.Run,
	at time.Time,
) error {
	if !run.IsLoopWorker() {
		return nil
	}
	loopRun, metadata, ok, err := loopRunAndNodeMetadataForTaskRun(ctx, exec, run)
	if err != nil || !ok {
		return err
	}
	payload := map[string]any{
		loopRunEventPayloadKeyNodeID:     metadata.NodeID,
		loopRunEventPayloadKeyGeneration: metadata.Generation,
		loopRunEventPayloadKeyItemIndex:  metadata.ItemIndex,
		loopRunEventPayloadKeyTaskRunID:  run.ID,
		loopRunEventPayloadKeyTaskID:     run.TaskID,
		loopRunEventPayloadKeyStatus:     loopRunNodeOutputRunning,
	}
	return appendLoopRunEventWithExecutor(
		ctx,
		exec,
		loopRun.ID,
		loopRun.WorkspaceID,
		loopRunEventNodeRunning,
		payload,
		at,
	)
}

func appendLoopNodeTerminalEventsWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	run taskpkg.Run,
	outcome string,
	outputRef string,
	resultPayload json.RawMessage,
	tokensUsed int64,
	at time.Time,
) error {
	if !run.IsLoopWorker() {
		return nil
	}
	loopRun, metadata, ok, err := loopRunAndNodeMetadataForTaskRun(ctx, exec, run)
	if err != nil || !ok {
		return err
	}
	kind := loopRunEventNodeSucceeded
	status := loopNodeOutputSucceeded
	if outcome == loopNodeOutcomeFailure {
		kind = loopRunEventNodeFailed
		status = loopNodeOutputFailed
	}
	if err := appendLoopRunEventWithExecutor(ctx, exec, loopRun.ID, loopRun.WorkspaceID, kind, map[string]any{
		loopRunEventPayloadKeyNodeID:     metadata.NodeID,
		loopRunEventPayloadKeyGeneration: metadata.Generation,
		loopRunEventPayloadKeyItemIndex:  metadata.ItemIndex,
		loopRunEventPayloadKeyTaskRunID:  run.ID,
		loopRunEventPayloadKeyTaskID:     run.TaskID,
		loopRunEventPayloadKeyStatus:     status,
		"output_ref":                     strings.TrimSpace(outputRef),
	}, at); err != nil {
		return err
	}
	if payload, ok := loopChannelMessagePayload(ctx, run, metadata, resultPayload); ok {
		if err := appendLoopRunEventWithExecutor(
			ctx,
			exec,
			loopRun.ID,
			loopRun.WorkspaceID,
			loopRunEventChannelMsg,
			payload,
			at,
		); err != nil {
			return err
		}
	}
	return appendLoopTokenTickEventWithExecutor(
		ctx,
		exec,
		loopRun.ID,
		loopRun.WorkspaceID,
		run.ID,
		tokensUsed,
		true,
		at,
	)
}

func loopRunAndNodeMetadataForTaskRun(
	ctx context.Context,
	exec taskSQLExecutor,
	run taskpkg.Run,
) (looppkg.Run, loopNodeRunMetadata, bool, error) {
	metadata, ok, err := loopNodeMetadataFromTaskRun(run.Metadata)
	if err != nil || !ok {
		return looppkg.Run{}, loopNodeRunMetadata{}, false, err
	}
	loopRun, err := getLoopRunByIDWithExecutor(ctx, exec, looppkg.RunID(strings.TrimSpace(run.LoopRunID)))
	if err != nil {
		return looppkg.Run{}, loopNodeRunMetadata{}, false, err
	}
	return loopRun, metadata, true, nil
}

func loopChannelMessagePayload(
	ctx context.Context,
	run taskpkg.Run,
	metadata loopNodeRunMetadata,
	resultPayload json.RawMessage,
) (map[string]any, bool) {
	text := channelMessageText(ctx, run.ID, resultPayload)
	if strings.TrimSpace(text) == "" {
		return nil, false
	}
	author := loopCoordinatorActorRef
	if run.ClaimedBy != nil && strings.TrimSpace(run.ClaimedBy.Ref) != "" {
		author = strings.TrimSpace(run.ClaimedBy.Ref)
	}
	return map[string]any{
		"id":                             run.ID + ":result",
		"author":                         author,
		loopRunEventPayloadKeyRole:       string(taskpkg.ActorKindAgentSession),
		loopRunEventPayloadKeyText:       text,
		"is_result":                      true,
		loopRunEventPayloadKeyNodeID:     metadata.NodeID,
		loopRunEventPayloadKeyGeneration: metadata.Generation,
		loopRunEventPayloadKeyItemIndex:  metadata.ItemIndex,
		loopRunEventPayloadKeyTaskRunID:  run.ID,
	}, true
}

func channelMessageText(ctx context.Context, taskRunID string, raw json.RawMessage) string {
	if len(bytes.TrimSpace(raw)) == 0 {
		return ""
	}
	var envelope map[string]any
	if err := json.Unmarshal(raw, &envelope); err != nil {
		slog.Default().DebugContext(
			ctx,
			"store: decode loop channel message payload",
			loopRunEventPayloadKeyTaskRunID,
			strings.TrimSpace(taskRunID),
			"error",
			err,
		)
		return ""
	}
	for _, key := range []string{"message", loopRunEventPayloadKeyText, loopRunEventPayloadKeySummary} {
		if value, ok := envelope[key].(string); ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func appendLoopTokenTickEventWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	runID looppkg.RunID,
	ws looppkg.WorkspaceID,
	taskRunID string,
	tokensUsed int64,
	terminal bool,
	at time.Time,
) error {
	if !terminal {
		ok, err := shouldPersistLoopTokenTick(ctx, exec, runID, tokensUsed, at)
		if err != nil || !ok {
			return err
		}
	}
	return appendLoopRunEventWithExecutor(ctx, exec, runID, ws, loopRunEventTokenTick, map[string]any{
		loopRunEventPayloadKeyTaskRunID: strings.TrimSpace(taskRunID),
		columnTokensUsed:                tokensUsed,
		loopRunEventPayloadKeyTerminal:  terminal,
	}, at)
}

func shouldPersistLoopTokenTick(
	ctx context.Context,
	exec taskSQLExecutor,
	runID looppkg.RunID,
	tokensUsed int64,
	at time.Time,
) (bool, error) {
	if at.IsZero() {
		at = time.Now().UTC()
	}
	row, err := sqlcgen.New(exec).GetLastLoopTokenTick(ctx, sqlcgen.GetLastLoopTokenTickParams{
		LoopRunID: string(runID), Kind: loopRunEventTokenTick,
	})
	if errorsIsNoRows(err) {
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("store: load last loop token tick: %w", err)
	}
	var last map[string]json.RawMessage
	if err := json.Unmarshal([]byte(row.PayloadJson), &last); err != nil {
		return false, fmt.Errorf("store: decode last loop token tick: %w", err)
	}
	var lastTokensUsed int64
	if err := json.Unmarshal(last[columnTokensUsed], &lastTokensUsed); err != nil {
		return false, fmt.Errorf("store: decode last loop token tick tokens_used: %w", err)
	}
	lastAt := row.At.UTC()
	if at.Sub(lastAt) < loopTokenTickMinInterval {
		return false, nil
	}
	delta := tokensUsed - lastTokensUsed
	if delta < 0 {
		delta = -delta
	}
	return delta >= loopTokenTickMinDelta, nil
}

package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"strings"
	"time"

	"github.com/compozy/agh/internal/api/contract"

	aghconfig "github.com/compozy/agh/internal/config"

	"github.com/compozy/agh/internal/network"
	"github.com/compozy/agh/internal/network/participation"

	"github.com/compozy/agh/internal/store"
	taskpkg "github.com/compozy/agh/internal/task"
	toolspkg "github.com/compozy/agh/internal/tools"
)

func (n *daemonNativeTools) rollbackPromotedThreadTask(
	ctx context.Context,
	taskID string,
	actor taskpkg.ActorContext,
	cause error,
) error {
	if err := n.deps.Tasks.DeleteTask(ctx, taskID, actor); err != nil {
		return errors.Join(cause, fmt.Errorf("rollback promoted network task %q: %w", taskID, err))
	}
	return cause
}

type nativeThreadPromotionSource struct {
	workspaceID      string
	channel          string
	threadID         string
	originMessageID  string
	digest           string
	sourceMessageIDs []string
	thread           store.NetworkThreadSummary
}

func (n *daemonNativeTools) nativeThreadPromotionSource(
	ctx context.Context,
	scope toolspkg.Scope,
	toolID toolspkg.ToolID,
	input taskPromoteFromThreadInput,
) (nativeThreadPromotionSource, error) {
	if n.deps.NetworkStore == nil {
		return nativeThreadPromotionSource{}, nativeUnavailableError(toolID, "network store is unavailable")
	}
	channel, err := nativeNetworkChannel(toolID, input.Channel)
	if err != nil {
		return nativeThreadPromotionSource{}, err
	}
	threadID := strings.TrimSpace(input.ThreadID)
	if err := network.ValidateConversationID(threadID, "thread_id"); err != nil {
		return nativeThreadPromotionSource{}, nativeNetworkInputError(toolID, err)
	}
	originMessageID, err := requiredNativeString(toolID, "origin_message_id", input.OriginMessageID)
	if err != nil {
		return nativeThreadPromotionSource{}, err
	}
	workspaceID, err := n.nativeNetworkWorkspaceID(ctx, toolID, input.WorkspaceID, scope)
	if err != nil {
		return nativeThreadPromotionSource{}, err
	}
	messages, err := n.deps.NetworkStore.ListConversationMessages(
		ctx,
		store.NetworkConversationRef{
			WorkspaceID: workspaceID,
			Channel:     channel,
			Surface:     store.NetworkSurfaceThread,
			ThreadID:    threadID,
		},
		store.NetworkConversationMessageQuery{Limit: 200},
	)
	if err != nil {
		return nativeThreadPromotionSource{}, nativeNetworkInputError(toolID, err)
	}
	digest, sourceMessageIDs, found := nativeNetworkThreadPromotionDigest(messages, originMessageID)
	if !found {
		return nativeThreadPromotionSource{}, nativeNetworkInputError(
			toolID,
			fmt.Errorf("origin_message_id %q is not part of thread %q", originMessageID, threadID),
		)
	}
	thread, err := n.deps.NetworkStore.GetThread(
		ctx,
		store.NetworkChannelRef{WorkspaceID: workspaceID, Channel: channel},
		threadID,
	)
	if err != nil {
		return nativeThreadPromotionSource{}, nativeNetworkInputError(toolID, err)
	}
	return nativeThreadPromotionSource{
		workspaceID:      workspaceID,
		channel:          channel,
		threadID:         threadID,
		originMessageID:  originMessageID,
		digest:           digest,
		sourceMessageIDs: sourceMessageIDs,
		thread:           thread,
	}, nil
}

func nativePromotedThreadTaskTitle(
	input taskPromoteFromThreadInput,
	source nativeThreadPromotionSource,
) string {
	return firstNonEmpty(
		strings.TrimSpace(input.Title),
		strings.TrimSpace(source.thread.Title),
		"Network thread "+source.threadID,
	)
}

func (n *daemonNativeTools) taskFanOutRuns(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input taskFanOutRunsInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	if n.deps.NetworkStore == nil {
		return toolspkg.ToolResult{}, nativeUnavailableError(req.ToolID, "network store is unavailable")
	}
	taskID, err := requiredNativeString(req.ToolID, "task_id", input.TaskID)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	maxDesignations := n.deps.Config.Task.Orchestration.DesignatedRunMax
	if maxDesignations <= 0 {
		maxDesignations = aghconfig.DefaultTaskDesignatedRunMax
	}
	if len(input.Designations) == 0 {
		return toolspkg.ToolResult{}, nativeRequiredInputError(req.ToolID, "designations")
	}
	prepared, err := prepareNativeFanOutDesignations(input, maxDesignations)
	if err != nil {
		return toolspkg.ToolResult{}, nativeNetworkInputError(req.ToolID, err)
	}
	actor, err := actorContextFromScope(scope)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	groupID := store.NewID("tdg")
	runs := make([]taskpkg.Run, 0, len(input.Designations))
	for index := range input.Designations {
		run, enqueueErr := n.deps.Tasks.EnqueueRun(ctx, taskpkg.EnqueueRun{
			TaskID:         taskID,
			IdempotencyKey: prepared[index].idempotencyKey,
			NetworkParticipation: participation.CloneRequest(
				input.NetworkParticipation,
			),
			DesignationGroupID: groupID,
			Metadata:           prepared[index].metadata,
		}, actor)
		if enqueueErr != nil {
			return toolspkg.ToolResult{}, enqueueErr
		}
		runs = append(runs, *run)
	}
	if err := n.deps.NetworkStore.PutTaskDesignationRollup(ctx, store.TaskDesignationRollup{
		DesignationGroupID: groupID,
		TaskID:             taskID,
		SummaryJSON:        nativeFanOutDesignationRollupJSON(runs),
		CreatedAt:          time.Now().UTC(),
	}); err != nil {
		return toolspkg.ToolResult{}, nativeNetworkInputError(req.ToolID, err)
	}
	return structuredResult(
		map[string]any{"designation_group_id": groupID, nativeToolsRunsKey: runs},
		fmt.Sprintf("%d runs", len(runs)),
	)
}

type nativePreparedFanOutDesignation struct {
	idempotencyKey string
	metadata       json.RawMessage
}

func prepareNativeFanOutDesignations(
	input taskFanOutRunsInput,
	maxDesignations int,
) ([]nativePreparedFanOutDesignation, error) {
	if len(input.Designations) == 0 {
		return nil, errors.New("designations are required")
	}
	if len(input.Designations) > maxDesignations {
		return nil, fmt.Errorf("designations cannot exceed %d", maxDesignations)
	}
	prepared := make([]nativePreparedFanOutDesignation, 0, len(input.Designations))
	for index, designation := range input.Designations {
		metadata, err := nativeFanOutDesignationMetadata(index, designation)
		if err != nil {
			return nil, err
		}
		idempotencyKey := nativeFanOutDesignationIdempotencyKey(input.IdempotencyKey, designation, index)
		if idempotencyKey == "" {
			return nil, fmt.Errorf(
				"designations[%d].idempotency_key is required when task fan-out idempotency_key is empty",
				index,
			)
		}
		prepared = append(prepared, nativePreparedFanOutDesignation{
			idempotencyKey: idempotencyKey,
			metadata:       metadata,
		})
	}
	return prepared, nil
}

func nativeNetworkThreadPromotionDigest(
	messages []store.NetworkConversationMessage,
	originMessageID string,
) (string, []string, bool) {
	target := strings.TrimSpace(originMessageID)
	sourceIDs := make([]string, 0, len(messages))
	var builder strings.Builder
	found := false
	for _, message := range messages {
		messageID := strings.TrimSpace(message.MessageID)
		if messageID == "" {
			continue
		}
		sourceIDs = append(sourceIDs, messageID)
		if messageID == target {
			found = true
		}
		if builder.Len() >= 4000 {
			continue
		}
		line := nativeNetworkThreadPromotionMessageLine(message)
		if line == "" {
			continue
		}
		if builder.Len() > 0 {
			builder.WriteByte('\n')
		}
		builder.WriteString(line)
	}
	digest := strings.TrimSpace(builder.String())
	if len(digest) > 4000 {
		digest = truncateUTF8Bytes(digest, 4000)
	}
	return digest, sourceIDs, found
}

func nativeNetworkThreadPromotionMessageLine(message store.NetworkConversationMessage) string {
	preview := strings.TrimSpace(message.PreviewText)
	if preview == "" {
		preview = strings.TrimSpace(message.Text)
	}
	if preview == "" {
		return ""
	}
	peer := firstNonEmpty(strings.TrimSpace(message.PeerFrom), strings.TrimSpace(message.SessionID), "unknown")
	return peer + ": " + preview
}

type nativeFanOutDesignationMetadataPayload struct {
	Designation nativeFanOutDesignationMetadataDetail `json:"designation"`
	Metadata    json.RawMessage                       `json:"metadata,omitempty"`
}

type nativeFanOutDesignationMetadataDetail struct {
	Index int    `json:"index"`
	Brief string `json:"brief"`
}

func nativeFanOutDesignationMetadata(
	index int,
	designation contract.TaskFanOutRunDesignationRequest,
) (json.RawMessage, error) {
	brief := strings.TrimSpace(designation.Brief)
	if brief == "" {
		return nil, fmt.Errorf("designations[%d].brief is required", index)
	}
	metadata := cloneJSON(designation.Metadata)
	if len(metadata) > 0 && !json.Valid(metadata) {
		return nil, fmt.Errorf("designations[%d].metadata must be valid JSON", index)
	}
	encoded, err := json.Marshal(nativeFanOutDesignationMetadataPayload{
		Designation: nativeFanOutDesignationMetadataDetail{Index: index, Brief: brief},
		Metadata:    metadata,
	})
	if err != nil {
		return nil, fmt.Errorf("task fan-out metadata: %w", err)
	}
	return encoded, nil
}

func nativeFanOutDesignationIdempotencyKey(
	requestKey string,
	designation contract.TaskFanOutRunDesignationRequest,
	index int,
) string {
	if key := strings.TrimSpace(designation.IdempotencyKey); key != "" {
		return key
	}
	if key := strings.TrimSpace(requestKey); key != "" {
		return fmt.Sprintf("%s:%d", key, index)
	}
	return ""
}

func nativeFanOutDesignationRollupJSON(runs []taskpkg.Run) json.RawMessage {
	runIDs := make([]string, 0, len(runs))
	for _, run := range runs {
		if id := strings.TrimSpace(run.ID); id != "" {
			runIDs = append(runIDs, id)
		}
	}
	encoded, err := json.Marshal(map[string]any{
		"run_ids": runIDs,
		"count":   len(runIDs),
	})
	if err != nil {
		return json.RawMessage(`{"run_ids":[],"count":0}`)
	}
	return encoded
}

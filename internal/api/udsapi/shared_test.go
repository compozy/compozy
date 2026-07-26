package udsapi

import (
	"context"
	"encoding/json"
	"time"

	"github.com/compozy/agh/internal/acp"
	"github.com/compozy/agh/internal/api/contract"
	core "github.com/compozy/agh/internal/api/core"
	memcontract "github.com/compozy/agh/internal/memory/contract"
	"github.com/compozy/agh/internal/network/participation"
	"github.com/compozy/agh/internal/store"
)

type sessionPayload = contract.SessionPayload
type sessionEventPayload = contract.SessionEventPayload
type turnHistoryPayload = contract.TurnHistoryPayload
type agentPayload = contract.AgentPayload
type logEventPayload = contract.LogEventPayload
type logsCursor = core.LogsCursor
type memoryWriteRequest = contract.MemoryWriteRequest
type memoryListResponse = contract.MemoryListResponse
type memoryEntryResponse = contract.MemoryEntryResponse
type memoryMutationDecisionResponse = contract.MemoryMutationDecisionResponse
type memorySearchResponse = contract.MemorySearchResponse
type memoryReindexResponse = contract.MemoryReindexResponse
type memoryDreamTriggerResponse = contract.MemoryDreamTriggerResponse
type memoryHealthPayload = contract.MemoryHealthPayload
type memoryLocation = core.MemoryLocation
type workspacePayload = contract.WorkspacePayload

func udsTestLiveParticipation(workspaceID, channelID string) participation.Spec {
	return participation.Spec{
		Version:         participation.SpecVersion,
		Mode:            participation.ModeLive,
		WorkspaceID:     workspaceID,
		ChannelStrategy: participation.StrategyNamed,
		ChannelID:       channelID,
		Source:          participation.SourceExplicitRequest,
		Bounds: participation.Bounds{
			MaxWakes:         1,
			MaxWakeWallTime:  "1m",
			MaxTotalWallTime: "5m",
			MaxInputTokens:   1_000,
			MaxOutputTokens:  1_000,
			MaxWakeDepth:     1,
			CoalesceWindow:   "1s",
		},
	}
}

func statusForMemoryError(err error) int {
	return core.StatusForMemoryError(err)
}

func newMemoryValidationError(err error) error {
	return core.NewMemoryValidationError(err)
}

func payloadJSON(raw string) json.RawMessage {
	return core.PayloadJSON(raw)
}

func observeEventAfterCursor(event store.EventSummary, cursor logsCursor) bool {
	return core.LogEventAfterCursor(event, cursor)
}

func acpCapsPayloadFromInfo(caps acp.Caps) *contract.ACPCapsPayload {
	return core.ACPCapsPayloadFromInfo(caps)
}

func tokenUsagePayloadFromUsage(usage *acp.TokenUsage) *contract.TokenUsagePayload {
	return core.TokenUsagePayloadFromUsage(usage)
}

func resolveMemoryWriteScope(req memoryWriteRequest) (memcontract.Scope, string, error) {
	return core.ResolveMemoryWriteScope(req)
}

func parseOptionalMemoryScope(raw string) (memcontract.Scope, error) {
	return core.ParseOptionalMemoryScope(raw)
}

func resolveMemoryWorkspace(raw string) (string, error) {
	return core.ResolveMemoryWorkspace(raw)
}

func parseLogsCursor(raw string) (logsCursor, error) {
	return core.ParseLogsCursor(raw)
}

func parseOptionalTime(raw string) (time.Time, error) {
	return core.ParseOptionalTime(raw)
}

func parseOptionalInt(raw string) (int, error) {
	return core.ParseOptionalInt(raw)
}

func parseOptionalInt64(raw string) (int64, error) {
	return core.ParseOptionalInt64(raw)
}

func (h *Handlers) resolveMemoryLocation(
	filename string,
	rawScope string,
	rawWorkspace string,
) (memoryLocation, error) {
	return h.ResolveMemoryLocation(filename, rawScope, rawWorkspace)
}

func (h *Handlers) memoryHealthWorkspaces(ctx context.Context, rawWorkspace string) ([]string, error) {
	return h.MemoryHealthWorkspaces(ctx, rawWorkspace)
}

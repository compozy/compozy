package udsapi

import (
	"encoding/json"
	"time"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/api/contract"
	core "github.com/compozy/compozy/internal/api/core"
)

type sessionPayload = contract.SessionPayload
type sessionEventPayload = contract.SessionEventPayload
type turnHistoryPayload = contract.TurnHistoryPayload
type agentPayload = contract.AgentPayload
type logEventPayload = contract.LogEventPayload
type logsCursor = core.LogsCursor
type workspacePayload = contract.WorkspacePayload

func payloadJSON(raw string) json.RawMessage {
	return core.PayloadJSON(raw)
}

func tokenUsagePayloadFromUsage(usage *acp.TokenUsage) *contract.TokenUsagePayload {
	return core.TokenUsagePayloadFromUsage(usage)
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

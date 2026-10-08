package httpapi

import (
	"encoding/json"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/api/contract"
	core "github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/store"
)

type sessionPayload = contract.SessionPayload
type sessionEventPayload = contract.SessionEventPayload
type agentPayload = contract.AgentPayload
type logEventPayload = contract.LogEventPayload
type logsCursor = core.LogsCursor
type workspacePayload = contract.WorkspacePayload

func statusForWorkspaceError(err error) int {
	return core.StatusForWorkspaceError(err)
}

func payloadJSON(raw string) json.RawMessage {
	return core.PayloadJSON(raw)
}

func observeEventAfterCursor(event store.EventSummary, cursor logsCursor) bool {
	return core.LogEventAfterCursor(event, cursor)
}

func acpCapsPayloadFromInfo(caps acp.Caps) *contract.ACPCapsPayload {
	return contract.ACPCapsPayloadFromACP(caps, true)
}

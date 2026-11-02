package builtin

import (
	"encoding/json"

	toolspkg "github.com/compozy/compozy/internal/tools"
)

func sessionPendingInteractionsDescriptor() toolspkg.Descriptor {
	descriptor := nativeDescriptor(
		toolspkg.ToolIDSessionPendingInteractions,
		"session_pending_interactions",
		"Session Pending Interactions",
		"Read pending questions and approvals, optionally across descendants. "+
			"Answer using the original session_id and provider_request_id; expires_at is the approval deadline.",
		`{"type":"object","required":["session_id"],"properties":{"workspace":{"type":"string"},"session_id":{"type":"string"},"descendants":{"type":"boolean"}},"additionalProperties":false}`,
		toolspkg.RiskRead,
		true,
		false,
		false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDSessions},
		[]string{sessionsSessionsKey, "approvals"},
		[]string{"pending interactions", "approval deadline", "relay"},
	)
	descriptor.OutputSchema = json.RawMessage(
		`{"type":"object","required":["interactions"],"properties":{"interactions":{"type":"array","items":{"type":"object","properties":{"session_id":{"type":"string"},"provider_request_id":{"type":"string"},"expires_at":{"type":"string","format":"date-time"}},"additionalProperties":true}}},"additionalProperties":false}`,
	)
	return descriptor
}

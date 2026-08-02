package task

import (
	"encoding/json"

	redactpkg "github.com/compozy/compozy/internal/redact"
)

// RedactClaimTokenJSON removes raw claim-token fields and values from a JSON payload.
func RedactClaimTokenJSON(raw json.RawMessage) json.RawMessage {
	return redactpkg.ClaimTokensJSON(raw)
}

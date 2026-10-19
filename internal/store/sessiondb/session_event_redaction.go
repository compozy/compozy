package sessiondb

import (
	"encoding/json"

	"github.com/compozy/compozy/internal/redact"
)

var persistedEventContentFields = []string{
	"authored_text", "body", "command", "content", "description", "detail", sessionEventPayloadErrorKey,
	"message", "output", "payload", "raw", "raw_input", "raw_output", "reason",
	"result", "stderr", "stdout", "summary", sessionEventPayloadTextKey, sessionEventPayloadTitleKey, "tool_input",
	sessionEventPayloadToolResultKey,
}

const sessionEventPayloadTitleKey = "title"

// persistedDirectiveTokenParents are the input-event lists whose entries carry a `token`: the raw
// `/skill` or `@mention` text of the authored message at its offsets. The transcript renders a chip
// only where that token still matches the text, so it is display text, not a credential.
var persistedDirectiveTokenParents = map[string]struct{}{
	"mentions":          {},
	"skill_invocations": {},
}

func redactSessionEventContent(content string) string {
	if !json.Valid([]byte(content)) {
		return redact.String(content)
	}
	engine := redact.New(redact.Options{Disabled: !redact.Enabled()})
	return string(engine.RedactJSONWithProtection(
		json.RawMessage(content),
		persistedEventContentFields,
		nil,
		persistedDirectiveToken,
	))
}

// persistedDirectiveToken exempts a top-level directive list's `token` string from the key rule that
// blanks every field named like a token. The value still gets exact secret redaction, as the authored
// text it was cut from does.
func persistedDirectiveToken(path []string, value any) bool {
	if _, ok := value.(string); !ok || len(path) != 2 || path[1] != "token" {
		return false
	}
	_, ok := persistedDirectiveTokenParents[path[0]]
	return ok
}

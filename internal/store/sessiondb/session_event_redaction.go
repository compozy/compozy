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

func redactSessionEventContent(content string) string {
	if !json.Valid([]byte(content)) {
		return redact.String(content)
	}
	engine := redact.New(redact.Options{Disabled: !redact.Enabled()})
	return string(engine.RedactJSON(json.RawMessage(content), persistedEventContentFields))
}

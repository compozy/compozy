package sessiondb

import (
	"encoding/json"

	"github.com/compozy/agh/internal/redact"
)

var persistedEventContentFields = []string{
	"authored_text", "body", "command", "content", "description", "detail", sessionEventPayloadErrorKey,
	"message", "output", "payload", "raw", "raw_input", "raw_output", "reason",
	"result", "stderr", "stdout", "summary", sessionEventPayloadTextKey, "title", "tool_input",
	sessionEventPayloadToolResultKey,
}

func redactSessionEventContent(content string) string {
	if !json.Valid([]byte(content)) {
		return redact.String(content)
	}
	engine := redact.New(redact.Options{Disabled: !redact.Enabled()})
	return string(engine.RedactJSON(json.RawMessage(content), persistedEventContentFields))
}

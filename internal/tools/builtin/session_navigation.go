package builtin

import (
	"encoding/json"

	toolspkg "github.com/compozy/compozy/internal/tools"
)

func sessionSearchDescriptor() toolspkg.Descriptor {
	descriptor := nativeDescriptor(toolspkg.ToolIDSessionSearch, "session_search", "Session Search",
		"Search retained projected messages with literal case-insensitive text; bounded results report truncation.",
		sessionSearchInputSchema, toolspkg.RiskRead, true, false, false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDSessions}, []string{sessionsSessionsKey, "search"},
		[]string{"session search", "find in conversation"})
	descriptor.OutputSchema = json.RawMessage(sessionSearchOutputSchema)
	return descriptor
}

func sessionOutlineDescriptor() toolspkg.Descriptor {
	descriptor := nativeDescriptor(toolspkg.ToolIDSessionOutline, "session_outline", "Session Outline",
		"Read every retained operator message with stable sequence, turn, timestamp and reply preview.",
		sessionIDInputSchema, toolspkg.RiskRead, true, false, false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDSessions}, []string{sessionsSessionsKey, "outline"},
		[]string{"session outline", "message trail"})
	descriptor.OutputSchema = json.RawMessage(sessionOutlineOutputSchema)
	return descriptor
}

const sessionSearchInputSchema = `{
 "type":"object","required":["session_id","q"],
 "properties":{
  "workspace":{"type":"string"},"session_id":{"type":"string"},
  "q":{"type":"string","minLength":1,"maxLength":4096},
  "limit":{"type":"integer","minimum":0,"maximum":1000,"default":200}
 },"additionalProperties":false
}`

const sessionSearchOutputSchema = `{
 "type":"object","required":["matches","truncated"],
 "properties":{
  "matches":{"type":"array","items":{
   "type":"object","required":["sequence","turn_id","role","snippet"],
   "properties":{"sequence":{"type":"integer"},"turn_id":{"type":"string"},
    "role":{"type":"string"},"snippet":{"type":"string"},
    "part_index":{"type":"integer","minimum":0},"field":{"type":"string"}},"additionalProperties":false
  }},"truncated":{"type":"boolean"}
 },"additionalProperties":false
}`

const sessionOutlineOutputSchema = `{
 "type":"object","required":["entries"],
 "properties":{"entries":{"type":"array","items":{
  "type":"object","required":["sequence","turn_id","preview","reply_preview","at"],
  "properties":{"sequence":{"type":"integer"},"turn_id":{"type":"string"},
   "preview":{"type":"string"},"reply_preview":{"type":"string"},
   "at":{"type":"string","format":"date-time"}},"additionalProperties":false
 }}},"additionalProperties":false
}`

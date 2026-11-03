package redact

import "encoding/json"

// crossScopeEngine always applies the additive heuristics. Content read from
// another workspace or profile is redacted even when the operator turned the
// heuristics off for their own data, because the reader is not that data's owner.
var crossScopeEngine = New(Options{})

// CrossScopeString redacts free text read across a workspace or profile boundary.
func CrossScopeString(value string) string {
	return crossScopeEngine.RedactString(value)
}

// CrossScopeJSON redacts every string of a JSON document read across a
// workspace or profile boundary. Structural identifiers (`*_id`, hashes,
// cursors) stay readable; sensitive field names are blanked wholesale.
func CrossScopeJSON(raw json.RawMessage) json.RawMessage {
	return crossScopeEngine.RedactJSONEverywhere(raw)
}

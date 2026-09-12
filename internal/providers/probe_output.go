package providers

import (
	"encoding/json"
	"strings"

	"github.com/compozy/compozy/internal/diagnostics"
)

const (
	probeJSONAuthenticated = `{"loggedIn":true}`
	probeJSONNeedsLogin    = `{"loggedIn":false}`
	probeJSONUnknown       = "{}"
)

// RedactAuthProbeOutput retains only the authentication verdict from structured status output.
func RedactAuthProbeOutput(output string, limit int) string {
	trimmed := strings.TrimSpace(output)
	if !strings.HasPrefix(trimmed, "{") && !strings.HasPrefix(trimmed, "[") {
		return diagnostics.RedactAndBound(output, limit)
	}
	loggedIn, valid := probeLoggedIn(trimmed)
	if !valid {
		return probeJSONUnknown
	}
	if loggedIn {
		return probeJSONAuthenticated
	}
	return probeJSONNeedsLogin
}

func probeLoggedIn(output string) (bool, bool) {
	if !json.Valid([]byte(output)) {
		return false, false
	}
	decoder := json.NewDecoder(strings.NewReader(output))
	opening, err := decoder.Token()
	if err != nil || opening != json.Delim('{') {
		return false, false
	}
	var loggedIn bool
	found := false
	for decoder.More() {
		key, err := decoder.Token()
		if err != nil {
			return false, false
		}
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return false, false
		}
		if key == "error" && string(value) != "null" && string(value) != `""` {
			return false, false
		}
		if key != "loggedIn" {
			continue
		}
		if found {
			return false, false
		}
		found = true
		switch string(value) {
		case "true":
			loggedIn = true
		case "false":
			loggedIn = false
		default:
			return false, false
		}
	}
	return loggedIn, found
}

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
	probeJSONTrueLiteral   = "true"
	probeJSONFalseLiteral  = "false"
)

// RedactAuthProbeOutput retains only the authentication verdict from structured status output.
func RedactAuthProbeOutput(output string, limit int) string {
	payload, prefix, structured := structuredProbeOutput(output)
	if !structured {
		return diagnostics.RedactAndBound(output, limit)
	}
	if failure, classified := classifyProbeOutput(strings.ToLower(prefix)); classified {
		switch failure.State {
		case ProviderAuthStateRateLimited:
			return "rate limit"
		case ProviderAuthStatePermissionDenied:
			return "permission denied"
		case ProviderAuthStateTransient:
			return "temporary failure"
		case ProviderAuthStateNeedsLogin:
			return "not authenticated"
		}
	}
	loggedIn, valid := probeLoggedIn(payload)
	if !valid {
		return probeJSONUnknown
	}
	if loggedIn {
		return probeJSONAuthenticated
	}
	return probeJSONNeedsLogin
}

func structuredProbeOutput(output string) (string, string, bool) {
	trimmed := strings.TrimSpace(output)
	if strings.HasPrefix(trimmed, "[") && !probeTextLabel(trimmed) {
		return trimmed, "", true
	}
	if start := strings.IndexByte(trimmed, '{'); start >= 0 {
		return trimmed[start:], trimmed[:start], true
	}
	if strings.Contains(trimmed, `"loggedIn"`) {
		return trimmed, "", true
	}
	return "", "", false
}

func probeTextLabel(output string) bool {
	end := strings.IndexByte(output, ']')
	if end <= 1 {
		return false
	}
	label := output[1:end]
	if label == probeJSONTrueLiteral || label == probeJSONFalseLiteral || label == "null" {
		return false
	}
	for _, char := range label {
		if (char < 'a' || char > 'z') && (char < 'A' || char > 'Z') && char != ' ' && char != '_' && char != '-' {
			return false
		}
	}
	return true
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
		case probeJSONTrueLiteral:
			loggedIn = true
		case probeJSONFalseLiteral:
			loggedIn = false
		default:
			return false, false
		}
	}
	return loggedIn, found
}

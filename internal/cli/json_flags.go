package cli

import (
	"encoding/json"
	"errors"
	"strings"
)

var errEmptyJSONFlag = errors.New("empty JSON flag")

func parseRequiredJSONRawMessage(raw string) (json.RawMessage, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, errEmptyJSONFlag
	}
	var decoded any
	if err := json.Unmarshal([]byte(trimmed), &decoded); err != nil {
		return nil, err
	}
	return json.RawMessage(trimmed), nil
}

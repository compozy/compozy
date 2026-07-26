package globaldb

import (
	"encoding/json"
	"strings"
)

func loopNodeTerminalOutputRef(outputRef string, resultPayload json.RawMessage) string {
	if strings.TrimSpace(outputRef) != "" || len(resultPayload) == 0 {
		return outputRef
	}
	return string(resultPayload)
}

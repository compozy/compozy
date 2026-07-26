package cli

import (
	"encoding/json"

	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/compozy/agh/internal/api/contract"
)

func renderJSONPreview(value any) (string, error) {
	content, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return "", fmt.Errorf("cli: render JSON preview: %w", err)
	}
	return string(content), nil
}

func formatAgentTime(value time.Time) string {
	if value.IsZero() {
		return "-"
	}
	return strconv.FormatInt(value.UTC().Unix(), 10)
}

func firstCLIValue(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func zeroCLIAgentCoordinationMetadata(metadata contract.CoordinationMessageMetadataPayload) bool {
	return strings.TrimSpace(metadata.TaskID) == "" &&
		strings.TrimSpace(metadata.RunID) == "" &&
		strings.TrimSpace(metadata.WorkflowID) == "" &&
		strings.TrimSpace(metadata.ChannelID) == "" &&
		strings.TrimSpace(string(metadata.MessageKind)) == "" &&
		strings.TrimSpace(metadata.CorrelationID) == "" &&
		len(metadata.Ext) == 0
}

package memory

import (
	"encoding/json"

	"fmt"

	"strings"

	"time"

	"github.com/compozy/agh/internal/diagnostics"

	memcontract "github.com/compozy/agh/internal/memory/contract"
)

func canonicalEventOp(record memcontract.OperationRecord) string {
	switch record.Operation.Normalize() {
	case "":
		return ""
	case memcontract.OperationSearch:
		return memoryEventRecallExecuted
	case memcontract.OperationReindex:
		return memoryEventWriteReindex
	case memcontract.OperationDelete:
		return memoryEventWriteCommitted
	default:
		return memoryEventWriteCommitted
	}
}

func operationFromEventOp(op string, metadata map[string]string) memcontract.Operation {
	switch strings.TrimSpace(op) {
	case memoryEventRecallExecuted, memoryEventRecallSkipped:
		return memcontract.OperationSearch
	case memoryEventWriteReindex:
		return memcontract.OperationReindex
	case memoryEventWriteCommitted:
		if metadata[memoryEventMetadataActionKey] == string(memcontract.OperationDelete) {
			return memcontract.OperationDelete
		}
		return memcontract.OperationWrite
	case memoryEventWriteReverted:
		return memcontract.OperationDelete
	default:
		return memcontract.Operation(op).Normalize()
	}
}

func eventMetadata(record memcontract.OperationRecord) map[string]string {
	metadata := map[string]string{}
	if summary := diagnostics.RedactAndBound(
		record.Summary,
		maxOperationSummaryBytes,
	); strings.TrimSpace(
		summary,
	) != "" {
		metadata[memoryEventMetadataSummaryKey] = summary
	}
	if filename := strings.TrimSpace(record.Filename); filename != "" {
		metadata[memoryEventMetadataFilenameKey] = filename
	}
	if action := strings.TrimSpace(string(record.Operation.Normalize())); action != "" {
		metadata[memoryEventMetadataActionKey] = action
	}
	return metadata
}

func mustEventMetadata(record memcontract.OperationRecord) string {
	payload, err := json.Marshal(eventMetadata(record))
	if err != nil {
		return "{}"
	}
	return string(payload)
}

func parseEventMetadata(raw string) (map[string]string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return map[string]string{}, nil
	}
	var metadata map[string]string
	if err := json.Unmarshal([]byte(trimmed), &metadata); err != nil {
		return nil, fmt.Errorf("memory: parse memory event metadata: %w", err)
	}
	if metadata == nil {
		metadata = map[string]string{}
	}
	return metadata, nil
}

func nullStringForEmpty(value any) any {
	switch typed := value.(type) {
	case memcontract.Scope:
		trimmed := strings.TrimSpace(string(typed))
		if trimmed == "" {
			return nil
		}
		return trimmed
	case string:
		trimmed := strings.TrimSpace(typed)
		if trimmed == "" {
			return nil
		}
		return trimmed
	default:
		return value
	}
}

func timeToUnixMillis(value time.Time) int64 {
	if value.IsZero() {
		value = time.Now().UTC()
	}
	return value.UTC().UnixNano() / int64(time.Millisecond)
}

func timeFromUnixMillis(value int64) time.Time {
	return time.Unix(0, value*int64(time.Millisecond)).UTC()
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func catalogSlug(filename string) string {
	base := strings.TrimSpace(filename)
	base = strings.TrimSuffix(base, ".md")
	base = strings.TrimSpace(base)
	if base == "" {
		return strings.TrimSpace(filename)
	}
	return base
}

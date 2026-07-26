package e2e

import (
	"bufio"
	"bytes"

	"encoding/json"

	"fmt"
	"io"

	"strings"
)

func readSSERecords(reader io.Reader, limit int) ([]SSEEvent, error) {
	return readSSERecordsWithCallback(reader, limit, nil)
}

func readSSERecordsWithCallback(
	reader io.Reader,
	limit int,
	onRecord func(SSEEvent) error,
) ([]SSEEvent, error) {
	return readSSERecordsMatching(reader, limit, onRecord, nil)
}

func readSSERecordsUntil(
	reader io.Reader,
	predicate func(SSEEvent) bool,
) ([]SSEEvent, error) {
	if err := validateSSEPredicate(predicate); err != nil {
		return nil, err
	}
	return readSSERecordsMatching(reader, 0, nil, predicate)
}

func validateSSEPredicate(predicate func(SSEEvent) bool) error {
	if predicate == nil {
		return errSSEPredicateRequired
	}
	return nil
}

func readSSERecordsMatching(
	reader io.Reader,
	limit int,
	onRecord func(SSEEvent) error,
	predicate func(SSEEvent) bool,
) ([]SSEEvent, error) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 16*1024), 1024*1024)

	records := make([]SSEEvent, 0, maxInt(limit, 1))
	current := SSEEvent{}
	for scanner.Scan() {
		line := scanner.Text()
		if line != "" {
			applySSELine(&current, line)
			continue
		}
		var matched bool
		var err error
		records, matched, err = appendSSERecord(records, current, onRecord, predicate)
		if err != nil {
			return nil, err
		}
		current = SSEEvent{}
		if matched || (limit > 0 && len(records) >= limit) {
			return records, nil
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan SSE stream: %w", err)
	}
	var err error
	records, _, err = appendSSERecord(records, current, onRecord, predicate)
	if err != nil {
		return nil, err
	}
	return records, nil
}

func applySSELine(record *SSEEvent, line string) {
	switch {
	case strings.HasPrefix(line, "id: "):
		record.ID = strings.TrimPrefix(line, "id: ")
	case strings.HasPrefix(line, "event: "):
		record.Event = strings.TrimPrefix(line, "event: ")
	case strings.HasPrefix(line, "data: "):
		if len(record.Data) > 0 {
			record.Data = append(record.Data, '\n')
		}
		record.Data = append(record.Data, strings.TrimPrefix(line, "data: ")...)
	}
}

func appendSSERecord(
	records []SSEEvent,
	record SSEEvent,
	onRecord func(SSEEvent) error,
	predicate func(SSEEvent) bool,
) ([]SSEEvent, bool, error) {
	if record.ID == "" && record.Event == "" && len(record.Data) == 0 {
		return records, false, nil
	}
	normalizeSSEEvent(&record)
	records = append(records, record)
	if onRecord != nil {
		if err := onRecord(record); err != nil {
			return nil, false, fmt.Errorf("handle SSE record: %w", err)
		}
	}
	return records, predicate != nil && predicate(record), nil
}

func normalizeSSEEvent(record *SSEEvent) {
	if record == nil || strings.TrimSpace(record.Event) != "" {
		return
	}
	record.Event = inferSSEEventName(record.Data)
}

func inferSSEEventName(data []byte) string {
	trimmed := bytes.TrimSpace(data)
	switch {
	case len(trimmed) == 0:
		return ""
	case bytes.Equal(trimmed, []byte("[DONE]")):
		return runtimeHarnessDoneKey
	}

	var envelope struct {
		Type string          `json:"type"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(trimmed, &envelope); err != nil {
		return ""
	}

	switch strings.TrimSpace(envelope.Type) {
	case "data-agh-permission":
		return runtimeHarnessPermissionKey
	case "data-agh-event":
		var payload struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(envelope.Data, &payload); err != nil {
			return runtimeHarnessEventKey
		}
		if eventType := strings.TrimSpace(payload.Type); eventType != "" {
			return eventType
		}
		return runtimeHarnessEventKey
	case "text-start", "text-delta", "text-end":
		return transportParityEventAgentMessage
	case "reasoning-start", "reasoning-delta", "reasoning-end":
		return "reasoning"
	case "tool-input-start", "tool-input-available":
		return "tool_call"
	case "tool-output-available":
		return "tool_result"
	default:
		return strings.TrimSpace(envelope.Type)
	}
}

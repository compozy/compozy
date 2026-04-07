package runs

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/compozy/compozy/pkg/compozy/events"
)

func deriveRunState(paths runPaths) (derivedRunState, error) {
	state := derivedRunState{}

	status, err := loadResultStatus(paths.resultPath)
	if err != nil {
		return state, err
	}
	if status != "" {
		state.status = status
	}

	eventState := bestEffortRunStateFromEvents(paths.eventsPath)
	if state.status == "" {
		state.status = eventState.status
	}
	if state.endedAt == nil {
		state.endedAt = eventState.endedAt
	}
	return state, nil
}

func loadResultStatus(resultPath string) (string, error) {
	if strings.TrimSpace(resultPath) == "" {
		return "", nil
	}
	payload, err := os.ReadFile(resultPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", fmt.Errorf("read run result: %w", err)
	}

	var record resultRecord
	if err := json.Unmarshal(payload, &record); err != nil {
		return "", fmt.Errorf("decode run result: %w", err)
	}
	return normalizeStatus(record.Status), nil
}

func bestEffortRunStateFromEvents(eventsPath string) derivedRunState {
	file, err := os.Open(eventsPath)
	if err != nil {
		return derivedRunState{}
	}
	defer func() {
		_ = file.Close()
	}()

	state := derivedRunState{}
	if err := forEachEventLine(file, func(rawLine []byte, lineNumber int) error {
		line := bytesTrimSpace(rawLine)
		if len(line) == 0 {
			return nil
		}
		ev, err := decodeEventLine(line, lineNumber)
		if err != nil {
			return err
		}
		switch ev.Kind {
		case events.EventKindRunStarted, events.EventKindRunQueued:
			if state.status == "" {
				state.status = defaultRunStatus()
			}
		case events.EventKindRunCompleted:
			state = derivedRunState{status: publicRunStatusCompleted, endedAt: timePointer(ev.Timestamp)}
		case events.EventKindRunFailed:
			state = derivedRunState{status: publicRunStatusFailed, endedAt: timePointer(ev.Timestamp)}
		case events.EventKindRunCancelled:
			state = derivedRunState{status: publicRunStatusCancelled, endedAt: timePointer(ev.Timestamp)}
		}
		return nil
	}); err != nil {
		return state
	}
	return state
}

func validateSchemaVersion(version string) error {
	major, _, ok := strings.Cut(strings.TrimSpace(version), ".")
	if !ok || major == "" {
		return &SchemaVersionError{Version: version}
	}
	expectedMajor, _, _ := strings.Cut(events.SchemaVersion, ".")
	if major != expectedMajor {
		return &SchemaVersionError{Version: version}
	}
	return nil
}

func normalizeStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "":
		return ""
	case "succeeded", publicRunStatusCompleted:
		return publicRunStatusCompleted
	case "canceled", publicRunStatusCancelled:
		return publicRunStatusCancelled
	default:
		return strings.TrimSpace(status)
	}
}

func defaultRunStatus() string {
	return publicRunStatusRunning
}

func isTerminalStatus(status string) bool {
	switch normalizeStatus(status) {
	case publicRunStatusCompleted, publicRunStatusFailed, publicRunStatusCancelled:
		return true
	default:
		return false
	}
}

func timePointer(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	copyValue := value
	return &copyValue
}

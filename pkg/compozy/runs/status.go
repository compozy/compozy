package runs

import (
	"strings"
	"time"

	"github.com/compozy/compozy/pkg/compozy/events"
)

func normalizeStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "":
		return ""
	case "succeeded", publicRunStatusCompleted:
		return publicRunStatusCompleted
	case "canceled", publicRunStatusCancelled:
		return publicRunStatusCancelled
	case publicRunStatusCrashed:
		return publicRunStatusCrashed
	default:
		return strings.TrimSpace(status)
	}
}

func defaultRunStatus() string {
	return publicRunStatusRunning
}

func isTerminalStatus(status string) bool {
	switch normalizeStatus(status) {
	case publicRunStatusCompleted, publicRunStatusFailed, publicRunStatusCancelled, publicRunStatusCrashed:
		return true
	default:
		return false
	}
}

func timePointer(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	copyValue := value.UTC()
	return &copyValue
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

package observe

import (
	"errors"
	"fmt"

	"strings"

	"github.com/compozy/agh/internal/acp"

	"github.com/compozy/agh/internal/store"
)

func summarizeEvent(event acp.AgentEvent) string {
	if strings.TrimSpace(event.Type) == acp.EventTypePermission {
		if summary := firstNonEmptySummary(
			event.Title,
			event.Resource,
			event.Decision,
			event.Text,
			event.Error,
			event.StopReason,
			event.ToolCallID,
		); summary != "" {
			return truncateSummary(summary)
		}
	}
	if summary := firstNonEmptySummary(
		event.Text,
		event.Title,
		event.Error,
		event.Resource,
		event.StopReason,
		event.ToolCallID,
	); summary != "" {
		return truncateSummary(summary)
	}

	if len(event.Raw) > 0 {
		return truncateSummary(string(event.Raw))
	}
	return ""
}

func firstNonEmptySummary(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func truncateSummary(summary string) string {
	const maxRunes = 240

	clean := strings.TrimSpace(summary)
	if clean == "" {
		return ""
	}
	if len(clean) <= maxRunes {
		return clean
	}

	runes := []rune(clean)
	if len(runes) <= maxRunes {
		return clean
	}

	return string(runes[:maxRunes-3]) + "..."
}

func sanitizeHookSessionID(sessionID string) (string, error) {
	target := strings.TrimSpace(sessionID)
	if target == "" {
		return "", errors.New("observe: session id is required")
	}
	if target == "." || target == ".." || strings.ContainsAny(target, `/\`) {
		return "", fmt.Errorf("observe: invalid session id %q", sessionID)
	}
	return target, nil
}

func cloneSessionSandboxMeta(meta *store.SessionSandboxMeta) *store.SessionSandboxMeta {
	if meta == nil {
		return nil
	}
	cloned := *meta
	cloned.RuntimeAdditionalDirs = append([]string(nil), meta.RuntimeAdditionalDirs...)
	if meta.ProviderState != nil {
		cloned.ProviderState = append([]byte(nil), meta.ProviderState...)
	}
	if meta.SSHAccessExpiresAt != nil {
		expiresAt := *meta.SSHAccessExpiresAt
		cloned.SSHAccessExpiresAt = &expiresAt
	}
	if meta.LastSyncAt != nil {
		lastSyncAt := *meta.LastSyncAt
		cloned.LastSyncAt = &lastSyncAt
	}
	return &cloned
}

func shouldAggregateUsage(event acp.AgentEvent) bool {
	return strings.TrimSpace(event.Type) == acp.EventTypeDone && event.Usage != nil && !event.Usage.IsZero()
}

func stringPointer(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}

	copyValue := value
	return &copyValue
}

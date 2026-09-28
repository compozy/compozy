package situation

import (
	"context"

	"errors"

	"strings"
	"time"
	"unicode/utf8"

	"github.com/compozy/compozy/internal/api/contract"

	taskpkg "github.com/compozy/compozy/internal/task"
)

func sectionMeta(total int, limit int) contract.AgentContextSectionMetaPayload {
	normalizedLimit := limit
	if normalizedLimit <= 0 {
		normalizedLimit = DefaultSectionLimit
	}
	return contract.AgentContextSectionMetaPayload{
		Limit:     normalizedLimit,
		Returned:  min(total, normalizedLimit),
		Truncated: total > normalizedLimit,
	}
}

func boundedCapabilities(
	values []contract.AgentCapabilityPayload,
	limit int,
) []contract.AgentCapabilityPayload {
	if len(values) == 0 {
		return []contract.AgentCapabilityPayload{}
	}
	if limit <= 0 || len(values) <= limit {
		return values
	}
	return values[:limit]
}

func cloneActorIdentity(value *taskpkg.ActorIdentity) *taskpkg.ActorIdentity {
	if value == nil {
		return nil
	}
	clone := *value
	return &clone
}

func cloneOwnership(value *taskpkg.Ownership) *taskpkg.Ownership {
	if value == nil {
		return nil
	}
	clone := *value
	return &clone
}

func latestTime(values ...time.Time) time.Time {
	var latest time.Time
	for _, value := range values {
		if value.IsZero() {
			continue
		}
		value = value.UTC()
		if latest.IsZero() || value.After(latest) {
			latest = value
		}
	}
	return latest
}

func optionalTimePtr(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	clone := value.UTC()
	return &clone
}

func firstTrimmed(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func checkContext(ctx context.Context) error {
	if ctx == nil {
		return errors.New("situation: context is required")
	}
	return ctx.Err()
}

func isContextError(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

func singleLine(value string) string {
	fields := strings.Fields(strings.TrimSpace(value))
	return strings.Join(fields, " ")
}

func truncateRunes(value string, limit int) string {
	if limit <= 0 || value == "" || utf8.RuneCountInString(value) <= limit {
		return value
	}
	if limit <= len("...") {
		return strings.Repeat(".", limit)
	}
	var builder strings.Builder
	builder.Grow(len(value))
	count := 0
	for _, r := range value {
		if count == limit-len("...") {
			break
		}
		builder.WriteRune(r)
		count++
	}
	return strings.TrimSpace(builder.String()) + "..."
}

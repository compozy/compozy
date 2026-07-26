package cli

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/agh/internal/session"
)

func waitForSessionStartup(
	ctx context.Context,
	client sessionClientAPI,
	created SessionRecord,
	pollInterval time.Duration,
) (SessionRecord, error) {
	if created.State != session.StateStarting && created.State != session.StateStopping {
		return validateSessionStartupResult(created)
	}
	if pollInterval <= 0 {
		pollInterval = defaultPollInterval
	}

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	current := created
	for current.State == session.StateStarting || current.State == session.StateStopping {
		select {
		case <-ctx.Done():
			return SessionRecord{}, fmt.Errorf("cli: wait for session %q startup: %w", created.ID, ctx.Err())
		case <-ticker.C:
			var err error
			current, err = client.GetSession(ctx, created.ID)
			if err != nil {
				return SessionRecord{}, fmt.Errorf("cli: read session %q startup status: %w", created.ID, err)
			}
		}
	}
	return validateSessionStartupResult(current)
}

func validateSessionStartupResult(info SessionRecord) (SessionRecord, error) {
	if info.State == session.StateActive {
		return info, nil
	}
	if info.Failure != nil {
		summary := strings.TrimSpace(info.Failure.Summary)
		if summary == "" {
			summary = string(info.Failure.Kind)
		}
		return SessionRecord{}, fmt.Errorf("cli: session %q startup failed: %s", info.ID, summary)
	}
	return SessionRecord{}, fmt.Errorf("cli: session %q stopped before startup completed", info.ID)
}

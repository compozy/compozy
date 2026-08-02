package cli

import (
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type sessionWorkspaceClient interface {
	workspaceLookupClient
	agentSessionClient
}

func sessionEventsBundle(events []SessionEventRecord) outputBundle {
	return listBundle(
		events,
		events,
		"Session Events",
		[]string{"Seq", sessionTypeValue, sessionAgentValue, "Turn", sessionTimestampValue, "Content"},
		"events",
		[]string{
			sessionSequenceKey,
			extensionTypeKey,
			sessionAgentNameKey,
			sessionTurnIDKey,
			networkTimestampKey,
			memoryContentKey,
		},
		func(event SessionEventRecord) []string {
			return []string{
				strconv.FormatInt(event.Sequence, 10),
				stringOrDash(event.Type),
				stringOrDash(event.AgentName),
				stringOrDash(event.TurnID),
				stringOrDash(formatTime(event.Timestamp)),
				stringOrDash(compactJSON(event.Content)),
			}
		},
		func(event SessionEventRecord) []string {
			return []string{
				strconv.FormatInt(event.Sequence, 10),
				event.Type,
				event.AgentName,
				event.TurnID,
				formatTime(event.Timestamp),
				compactJSON(event.Content),
			}
		},
	)
}

func sessionHistoryBundle(history []TurnHistoryRecord) outputBundle {
	flattened := flattenHistory(history)
	return listBundle(
		history,
		flattened,
		"Session History",
		[]string{"Turn", "Seq", sessionTypeValue, sessionAgentValue, sessionTimestampValue, "Content"},
		"history",
		[]string{
			sessionTurnIDKey,
			sessionSequenceKey,
			extensionTypeKey,
			sessionAgentNameKey,
			networkTimestampKey,
			memoryContentKey,
		},
		func(event SessionEventRecord) []string {
			return []string{
				stringOrDash(event.TurnID),
				strconv.FormatInt(event.Sequence, 10),
				stringOrDash(event.Type),
				stringOrDash(event.AgentName),
				stringOrDash(formatTime(event.Timestamp)),
				stringOrDash(compactJSON(event.Content)),
			}
		},
		func(event SessionEventRecord) []string {
			return []string{
				event.TurnID,
				strconv.FormatInt(event.Sequence, 10),
				event.Type,
				event.AgentName,
				formatTime(event.Timestamp),
				compactJSON(event.Content),
			}
		},
	)
}

func agentEventsBundle(events []AgentEventRecord) outputBundle {
	return listBundle(
		events,
		events,
		"Prompt Events",
		[]string{sessionTimestampValue, sessionTypeValue, "Detail", "Stop"},
		"prompt_events",
		[]string{networkTimestampKey, extensionTypeKey, "detail", "stop_reason"},
		func(event AgentEventRecord) []string {
			return []string{
				stringOrDash(formatTime(event.Timestamp)),
				stringOrDash(event.Type),
				stringOrDash(firstNonEmpty(event.Text, event.Title, event.Error, compactJSON(event.Raw))),
				stringOrDash(event.StopReason),
			}
		},
		func(event AgentEventRecord) []string {
			return []string{
				formatTime(event.Timestamp),
				event.Type,
				firstNonEmpty(event.Text, event.Title, event.Error, compactJSON(event.Raw)),
				event.StopReason,
			}
		},
	)
}

func displaySessionWorkspace(info SessionRecord) string {
	return firstNonEmpty(strings.TrimSpace(info.WorkspacePath), strings.TrimSpace(info.WorkspaceID))
}

func resolveSessionCreateWorkspace(
	cmd *cobra.Command,
	deps commandDeps,
	client sessionWorkspaceClient,
	workspaceRef string,
	cwd string,
) (string, string, error) {
	trimmedWorkspace := strings.TrimSpace(workspaceRef)
	trimmedCWD := strings.TrimSpace(cwd)

	switch {
	case trimmedWorkspace != "" && trimmedCWD != "":
		return "", "", errors.New("cli: --workspace and --cwd are mutually exclusive")
	case trimmedCWD != "":
		if !filepath.IsAbs(trimmedCWD) {
			return "", "", fmt.Errorf("cli: --cwd must be an absolute path: %q", trimmedCWD)
		}
		cleaned := filepath.Clean(trimmedCWD)
		resolution, err := resolveWorkspaceCandidate(
			cmd.Context(),
			cmd,
			client,
			cleaned,
			workspaceResolutionCWD,
		)
		if err == nil {
			identityRef, _, identityErr := workspaceRefFromSessionIdentity(
				cmd.Context(),
				cmd,
				deps,
				client,
			)
			if identityErr != nil {
				return "", "", identityErr
			}
			resolution = markCrossWorkspaceAttempt(cmd, resolution, identityRef)
			return resolution.ID, "", nil
		}
		if !workspaceResolutionAllowsGlobalFallback(err) {
			return "", "", err
		}
		recordWorkspaceResolution(cmd, workspaceResolution{Root: cleaned, Source: workspaceResolutionCWD})
		return "", cleaned, nil
	}

	resolution, err := resolveCommandWorkspace(
		cmd.Context(),
		cmd,
		deps,
		client,
		workspaceResolutionRequest{FlagRef: trimmedWorkspace},
	)
	if err == nil {
		return resolution.ID, "", nil
	}
	discoveryErr, ok := errors.AsType[*workspaceResolutionError](err)
	if !ok || strings.TrimSpace(discoveryErr.CWD) == "" {
		return "", "", err
	}
	recordWorkspaceResolution(cmd, workspaceResolution{
		Root:   discoveryErr.CWD,
		Source: workspaceResolutionCWD,
	})
	return "", discoveryErr.CWD, nil
}

func flattenHistory(history []TurnHistoryRecord) []SessionEventRecord {
	flattened := make([]SessionEventRecord, 0)
	for _, turn := range history {
		for _, event := range turn.Events {
			if strings.TrimSpace(event.TurnID) == "" {
				event.TurnID = turn.TurnID
			}
			flattened = append(flattened, event)
		}
	}
	return flattened
}

func parseSinceFlag(raw string, now func() time.Time) (time.Time, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return time.Time{}, nil
	}

	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed.UTC(), nil
		}
	}

	duration, err := time.ParseDuration(value)
	if err != nil {
		return time.Time{}, fmt.Errorf("cli: invalid since value %q", value)
	}
	if duration < 0 {
		return time.Time{}, fmt.Errorf("cli: relative since must be positive: %q", value)
	}

	current := time.Now().UTC()
	if now != nil {
		current = now().UTC()
	}
	return current.Add(-duration), nil
}

func validateSessionEventCursorFlags(last int, afterSequence int64) error {
	if last < 0 {
		return fmt.Errorf("cli: --last must be zero or positive: %d", last)
	}
	if afterSequence < 0 {
		return fmt.Errorf("cli: --after must be zero or positive: %d", afterSequence)
	}
	return nil
}

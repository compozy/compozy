package session

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	envpkg "github.com/compozy/agh/internal/sandbox"
	"github.com/compozy/agh/internal/store"
)

func syncResultErrors(result envpkg.SyncResult, err error) []string {
	if err == nil && len(result.Errors) == 0 {
		return nil
	}
	errorsList := make([]string, 0, len(result.Errors)+1)
	seen := make(map[string]struct{}, len(result.Errors)+1)
	for _, item := range result.Errors {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		errorsList = append(errorsList, trimmed)
	}
	if err != nil {
		trimmed := strings.TrimSpace(err.Error())
		if trimmed != "" {
			if _, ok := seen[trimmed]; !ok {
				errorsList = append(errorsList, trimmed)
			}
		}
	}
	return errorsList
}

func sandboxEventFromMeta(
	meta *store.SessionSandboxMeta,
	sessionID string,
	workspaceID string,
	name string,
	reason string,
) SandboxLifecycleEvent {
	event := SandboxLifecycleEvent{
		Name:        strings.TrimSpace(name),
		SessionID:   strings.TrimSpace(sessionID),
		WorkspaceID: strings.TrimSpace(workspaceID),
		Reason:      strings.TrimSpace(reason),
	}
	if meta != nil {
		event.SandboxID = strings.TrimSpace(meta.SandboxID)
		event.Backend = strings.TrimSpace(meta.Backend)
		event.Profile = strings.TrimSpace(meta.Profile)
		event.InstanceID = strings.TrimSpace(meta.InstanceID)
	}
	return event
}

func attachSandboxError(event *SandboxLifecycleEvent, err error) {
	if event == nil || err == nil {
		return
	}
	event.Error = err.Error()
	event.ErrorKind = sandboxErrorKind(err)
}

func sandboxErrorKind(err error) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, context.Canceled):
		return "context_canceled"
	case errors.Is(err, context.DeadlineExceeded):
		return "context_deadline_exceeded"
	default:
		return fmt.Sprintf("%T", err)
	}
}

func sandboxSpanForEvent(name string, reason string) string {
	switch name {
	case sandboxEventPrepareStart, sandboxEventPrepareComplete, sandboxEventPrepareError:
		return sandboxSandboxPreparePath
	case sandboxEventDestroyStart, sandboxEventDestroyComplete, sandboxEventDestroyError:
		return "sandbox.destroy"
	case sandboxEventSyncStart, sandboxEventSyncComplete, sandboxEventSyncError:
		if reason == string(envpkg.SyncReasonStart) || reason == string(envpkg.SyncReasonTurn) {
			return "sandbox.sync.to_runtime"
		}
		return "sandbox.sync.from_runtime"
	default:
		return strings.TrimSpace(name)
	}
}

func cloneSessionSandboxMeta(meta *store.SessionSandboxMeta) *store.SessionSandboxMeta {
	if meta == nil {
		return nil
	}
	cloned := *meta
	cloned.RuntimeAdditionalDirs = append([]string(nil), meta.RuntimeAdditionalDirs...)
	cloned.ProviderState = cloneRawMessage(meta.ProviderState)
	cloned.SSHAccessExpiresAt = cloneTimePointer(meta.SSHAccessExpiresAt)
	cloned.LastSyncAt = cloneTimePointer(meta.LastSyncAt)
	return &cloned
}

func cloneRawMessage(value json.RawMessage) json.RawMessage {
	if value == nil {
		return nil
	}
	cloned := make(json.RawMessage, len(value))
	copy(cloned, value)
	return cloned
}

func cloneTimePointer(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

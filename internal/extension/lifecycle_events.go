package extensionpkg

import (
	"context"
	"errors"
	"fmt"
	"strings"

	eventspkg "github.com/compozy/compozy/internal/events"
)

const (
	lifecycleEventExtensionNameKey       = "extension_name"
	lifecycleEventSourceKindKey          = "source_kind"
	lifecycleEventDigestMatchedKey       = "digest_matched"
	lifecycleEventWorkspaceIDKey         = "workspace_id"
	lifecycleEventExtensionGenerationKey = "extension_generation"
	lifecycleEventDigestKey              = "digest"
	lifecycleEventConfirmedByKey         = "confirmed_by"
	lifecycleEventBoundCountKey          = "bound_count"
	lifecycleEventAutomationCountKey     = "automation_started_count"
	lifecycleEventProfileIDKey           = "profile_id"
	lifecycleEventProfileNameKey         = "profile_name"
)

// LifecycleEventSink records one closed-shape extension lifecycle event.
type LifecycleEventSink interface {
	RecordExtensionLifecycleEvent(context.Context, LifecycleEvent) error
}

// LifecycleEvent carries only the correlation values allowed by the public event matrix.
type LifecycleEvent struct {
	Type                string
	ExtensionName       string
	SourceKind          string
	DigestMatched       bool
	WorkspaceID         string
	ExtensionGeneration string
	Digest              string
	ConfirmedBy         string
	BoundCount          *int
	AutomationCount     *int
	ProfileID           string
	ProfileName         string
}

// RequiredFields returns the exact payload shape for the event type.
func (e LifecycleEvent) RequiredFields() (map[string]any, error) {
	name := strings.TrimSpace(e.ExtensionName)
	if name == "" {
		return nil, errors.New("extension: lifecycle event extension name is required")
	}
	fields := map[string]any{lifecycleEventExtensionNameKey: name}
	switch strings.TrimSpace(e.Type) {
	case eventspkg.ExtensionInstallCompleted, eventspkg.ExtensionInstallFailed:
		if source := strings.TrimSpace(e.SourceKind); source != "" {
			fields[lifecycleEventSourceKindKey] = source
		} else {
			return nil, errors.New("extension: install lifecycle event source kind is required")
		}
		fields[lifecycleEventDigestMatchedKey] = e.DigestMatched
	case eventspkg.ExtensionUpdateCompleted, eventspkg.ExtensionUpdateFailed:
		if source := strings.TrimSpace(e.SourceKind); source != "" {
			fields[lifecycleEventSourceKindKey] = source
		} else {
			return nil, errors.New("extension: update lifecycle event source kind is required")
		}
	case eventspkg.ExtensionDevLinked, eventspkg.ExtensionDevUnlinked,
		eventspkg.ExtensionReloadCompleted, eventspkg.ExtensionReloadFailed:
		if workspaceID := strings.TrimSpace(e.WorkspaceID); workspaceID != "" {
			fields[lifecycleEventWorkspaceIDKey] = workspaceID
		} else {
			return nil, errors.New("extension: development lifecycle event workspace id is required")
		}
		if generation := strings.TrimSpace(e.ExtensionGeneration); generation != "" {
			fields[lifecycleEventExtensionGenerationKey] = generation
		} else {
			return nil, errors.New("extension: development lifecycle event extension generation is required")
		}
	case eventspkg.ExtensionPublishCompleted, eventspkg.ExtensionPublishFailed:
	case eventspkg.ExtensionCrashLoopBackoff:
		if workspaceID := strings.TrimSpace(e.WorkspaceID); workspaceID != "" {
			fields[lifecycleEventWorkspaceIDKey] = workspaceID
		}
	case eventspkg.ExtensionNetworkConfirmed:
		fields[lifecycleEventWorkspaceIDKey] = strings.TrimSpace(e.WorkspaceID)
		if digest := strings.TrimSpace(e.Digest); digest != "" {
			fields[lifecycleEventDigestKey] = digest
		} else {
			return nil, errors.New("extension: network confirmation digest is required")
		}
		if actor := strings.TrimSpace(e.ConfirmedBy); actor != "" {
			fields[lifecycleEventConfirmedByKey] = actor
		} else {
			return nil, errors.New("extension: network confirmation actor is required")
		}
	case eventspkg.ExtensionSecretsUpdated:
		fields[lifecycleEventWorkspaceIDKey] = strings.TrimSpace(e.WorkspaceID)
		if e.BoundCount == nil {
			return nil, errors.New("extension: secrets updated bound count is required")
		}
		fields[lifecycleEventBoundCountKey] = *e.BoundCount
	case eventspkg.ExtensionSecretsUpdateFailed:
		fields[lifecycleEventWorkspaceIDKey] = strings.TrimSpace(e.WorkspaceID)
	case eventspkg.ExtensionEnabled:
		if e.AutomationCount == nil {
			return nil, errors.New("extension: enabled automation count is required")
		}
		fields[lifecycleEventAutomationCountKey] = *e.AutomationCount
	case eventspkg.ExtensionProfileCreated:
		profileID := strings.TrimSpace(e.ProfileID)
		profileName := strings.TrimSpace(e.ProfileName)
		if profileID == "" || profileName == "" {
			return nil, errors.New("extension: profile created event profile id and name are required")
		}
		fields[lifecycleEventProfileIDKey] = profileID
		fields[lifecycleEventProfileNameKey] = profileName
	default:
		return nil, fmt.Errorf("extension: unsupported lifecycle event type %q", e.Type)
	}
	return fields, nil
}

func recordExtensionLifecycleEvent(
	ctx context.Context,
	sink LifecycleEventSink,
	event LifecycleEvent,
) error {
	if sink == nil {
		return nil
	}
	if ctx == nil {
		return ErrContextRequired
	}
	if _, err := event.RequiredFields(); err != nil {
		return err
	}
	return sink.RecordExtensionLifecycleEvent(ctx, event)
}

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
		if err := requireLifecycleStringField(
			fields, lifecycleEventSourceKindKey, e.SourceKind,
			"extension: install lifecycle event source kind is required",
		); err != nil {
			return nil, err
		}
		fields[lifecycleEventDigestMatchedKey] = e.DigestMatched
	case eventspkg.ExtensionUpdateCompleted, eventspkg.ExtensionUpdateFailed:
		if err := requireLifecycleStringField(
			fields, lifecycleEventSourceKindKey, e.SourceKind,
			"extension: update lifecycle event source kind is required",
		); err != nil {
			return nil, err
		}
	case eventspkg.ExtensionDevLinked, eventspkg.ExtensionDevUnlinked,
		eventspkg.ExtensionReloadCompleted, eventspkg.ExtensionReloadFailed:
		if err := addDevelopmentLifecycleFields(fields, e); err != nil {
			return nil, err
		}
	case eventspkg.ExtensionPublishCompleted, eventspkg.ExtensionPublishFailed:
	case eventspkg.ExtensionCrashLoopBackoff:
		if workspaceID := strings.TrimSpace(e.WorkspaceID); workspaceID != "" {
			fields[lifecycleEventWorkspaceIDKey] = workspaceID
		}
	case eventspkg.ExtensionNetworkConfirmed:
		if err := addNetworkConfirmationLifecycleFields(fields, e); err != nil {
			return nil, err
		}
	case eventspkg.ExtensionSecretsUpdated:
		if err := addSecretsUpdatedLifecycleFields(fields, e); err != nil {
			return nil, err
		}
	case eventspkg.ExtensionSecretsUpdateFailed:
		fields[lifecycleEventWorkspaceIDKey] = strings.TrimSpace(e.WorkspaceID)
	case eventspkg.ExtensionEnabled:
		if err := addExtensionEnabledLifecycleFields(fields, e); err != nil {
			return nil, err
		}
	case eventspkg.ExtensionProfileCreated:
		if err := addProfileCreatedLifecycleFields(fields, e); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("extension: unsupported lifecycle event type %q", e.Type)
	}
	return fields, nil
}

func requireLifecycleStringField(fields map[string]any, key string, value string, message string) error {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return errors.New(message)
	}
	fields[key] = trimmed
	return nil
}

func addDevelopmentLifecycleFields(fields map[string]any, event LifecycleEvent) error {
	if err := requireLifecycleStringField(
		fields, lifecycleEventWorkspaceIDKey, event.WorkspaceID,
		"extension: development lifecycle event workspace id is required",
	); err != nil {
		return err
	}
	return requireLifecycleStringField(
		fields, lifecycleEventExtensionGenerationKey, event.ExtensionGeneration,
		"extension: development lifecycle event extension generation is required",
	)
}

func addNetworkConfirmationLifecycleFields(fields map[string]any, event LifecycleEvent) error {
	fields[lifecycleEventWorkspaceIDKey] = strings.TrimSpace(event.WorkspaceID)
	if err := requireLifecycleStringField(
		fields, lifecycleEventDigestKey, event.Digest, "extension: network confirmation digest is required",
	); err != nil {
		return err
	}
	return requireLifecycleStringField(
		fields, lifecycleEventConfirmedByKey, event.ConfirmedBy,
		"extension: network confirmation actor is required",
	)
}

func addSecretsUpdatedLifecycleFields(fields map[string]any, event LifecycleEvent) error {
	fields[lifecycleEventWorkspaceIDKey] = strings.TrimSpace(event.WorkspaceID)
	if event.BoundCount == nil {
		return errors.New("extension: secrets updated bound count is required")
	}
	fields[lifecycleEventBoundCountKey] = *event.BoundCount
	return nil
}

func addExtensionEnabledLifecycleFields(fields map[string]any, event LifecycleEvent) error {
	if event.AutomationCount == nil {
		return errors.New("extension: enabled automation count is required")
	}
	fields[lifecycleEventAutomationCountKey] = *event.AutomationCount
	return nil
}

func addProfileCreatedLifecycleFields(fields map[string]any, event LifecycleEvent) error {
	profileID := strings.TrimSpace(event.ProfileID)
	profileName := strings.TrimSpace(event.ProfileName)
	if profileID == "" || profileName == "" {
		return errors.New("extension: profile created event profile id and name are required")
	}
	fields[lifecycleEventProfileIDKey] = profileID
	fields[lifecycleEventProfileNameKey] = profileName
	return nil
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

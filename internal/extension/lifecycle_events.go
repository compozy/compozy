package extensionpkg

import (
	"context"
	"errors"
	"fmt"
	"strings"

	eventspkg "github.com/compozy/compozy/internal/events"
)

const (
	lifecycleEventExtensionNameKey    = "extension_name"
	lifecycleEventSourceKindKey       = "source_kind"
	lifecycleEventDigestMatchedKey    = "digest_matched"
	lifecycleEventWorkspaceIDKey      = "workspace_id"
	lifecycleEventBundleGenerationKey = "bundle_generation"
)

// LifecycleEventSink records one closed-shape extension lifecycle event.
type LifecycleEventSink interface {
	RecordExtensionLifecycleEvent(context.Context, LifecycleEvent) error
}

// LifecycleEvent carries only the correlation values allowed by the public event matrix.
type LifecycleEvent struct {
	Type             string
	ExtensionName    string
	SourceKind       string
	DigestMatched    bool
	WorkspaceID      string
	BundleGeneration string
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
		if generation := strings.TrimSpace(e.BundleGeneration); generation != "" {
			fields[lifecycleEventBundleGenerationKey] = generation
		} else {
			return nil, errors.New("extension: development lifecycle event bundle generation is required")
		}
	case eventspkg.ExtensionPublishCompleted, eventspkg.ExtensionPublishFailed:
	case eventspkg.ExtensionCrashLoopBackoff:
		if workspaceID := strings.TrimSpace(e.WorkspaceID); workspaceID != "" {
			fields[lifecycleEventWorkspaceIDKey] = workspaceID
		}
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

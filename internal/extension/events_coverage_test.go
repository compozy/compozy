// Suite: extension lifecycle event coverage matrix
// Invariant: every canonical lifecycle event emits only its required correlation keys and manager crash backoff reaches the sink.
// Boundary IN: extension lifecycle event model and manager-owned crash call site.
// Boundary OUT: append-only event persistence, owned by daemon store adapter suites.
package extensionpkg

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"

	eventspkg "github.com/compozy/compozy/internal/events"
)

func TestExtensionLifecycleEventCoverageMatrix(t *testing.T) {
	t.Parallel()

	secret := "super-secret-must-not-appear"
	boundCount := 2
	automationCount := 3
	tests := []struct {
		name  string
		event LifecycleEvent
		want  map[string]any
	}{
		{
			name: "Should emit install completed keys",
			event: LifecycleEvent{Type: eventspkg.ExtensionInstallCompleted, ExtensionName: "alpha",
				SourceKind: "github", DigestMatched: true, WorkspaceID: secret, BundleGeneration: secret},
			want: map[string]any{"extension_name": "alpha", "source_kind": "github", "digest_matched": true},
		},
		{
			name: "Should emit install failed keys",
			event: LifecycleEvent{Type: eventspkg.ExtensionInstallFailed, ExtensionName: "alpha",
				SourceKind: "local_path", WorkspaceID: secret, BundleGeneration: secret},
			want: map[string]any{"extension_name": "alpha", "source_kind": "local_path", "digest_matched": false},
		},
		{
			name: "Should emit update completed keys",
			event: LifecycleEvent{Type: eventspkg.ExtensionUpdateCompleted, ExtensionName: "alpha",
				SourceKind: "github", WorkspaceID: secret, BundleGeneration: secret},
			want: map[string]any{"extension_name": "alpha", "source_kind": "github"},
		},
		{
			name: "Should emit update failed keys",
			event: LifecycleEvent{Type: eventspkg.ExtensionUpdateFailed, ExtensionName: "alpha",
				SourceKind: "git", WorkspaceID: secret, BundleGeneration: secret},
			want: map[string]any{"extension_name": "alpha", "source_kind": "git"},
		},
		{
			name: "Should emit development linked keys",
			event: LifecycleEvent{Type: eventspkg.ExtensionDevLinked, ExtensionName: "alpha",
				WorkspaceID: "workspace-1", BundleGeneration: "abc123", SourceKind: secret},
			want: map[string]any{
				"extension_name": "alpha", "workspace_id": "workspace-1", "bundle_generation": "abc123",
			},
		},
		{
			name: "Should emit development unlinked keys",
			event: LifecycleEvent{Type: eventspkg.ExtensionDevUnlinked, ExtensionName: "alpha",
				WorkspaceID: "workspace-1", BundleGeneration: "abc123", SourceKind: secret},
			want: map[string]any{
				"extension_name": "alpha", "workspace_id": "workspace-1", "bundle_generation": "abc123",
			},
		},
		{
			name: "Should emit reload completed keys",
			event: LifecycleEvent{Type: eventspkg.ExtensionReloadCompleted, ExtensionName: "alpha",
				WorkspaceID: "workspace-1", BundleGeneration: "abc123", SourceKind: secret},
			want: map[string]any{
				"extension_name": "alpha", "workspace_id": "workspace-1", "bundle_generation": "abc123",
			},
		},
		{
			name: "Should emit reload failed keys",
			event: LifecycleEvent{Type: eventspkg.ExtensionReloadFailed, ExtensionName: "alpha",
				WorkspaceID: "workspace-1", BundleGeneration: "abc123", SourceKind: secret},
			want: map[string]any{
				"extension_name": "alpha", "workspace_id": "workspace-1", "bundle_generation": "abc123",
			},
		},
		{
			name: "Should emit publish completed keys",
			event: LifecycleEvent{Type: eventspkg.ExtensionPublishCompleted, ExtensionName: "alpha",
				SourceKind: secret, WorkspaceID: secret, BundleGeneration: secret},
			want: map[string]any{"extension_name": "alpha"},
		},
		{
			name: "Should emit publish failed keys",
			event: LifecycleEvent{Type: eventspkg.ExtensionPublishFailed, ExtensionName: "alpha",
				SourceKind: secret, WorkspaceID: secret, BundleGeneration: secret},
			want: map[string]any{"extension_name": "alpha"},
		},
		{
			name: "Should omit workspace for a global crash backoff",
			event: LifecycleEvent{Type: eventspkg.ExtensionCrashLoopBackoff, ExtensionName: "alpha",
				SourceKind: secret, BundleGeneration: secret},
			want: map[string]any{"extension_name": "alpha"},
		},
		{
			name: "Should include workspace for a development crash backoff",
			event: LifecycleEvent{Type: eventspkg.ExtensionCrashLoopBackoff, ExtensionName: "alpha",
				WorkspaceID: "workspace-1", SourceKind: secret, BundleGeneration: secret},
			want: map[string]any{"extension_name": "alpha", "workspace_id": "workspace-1"},
		},
		{
			name: "Should emit network confirmation keys",
			event: LifecycleEvent{
				Type: eventspkg.ExtensionNetworkConfirmed, ExtensionName: "alpha", WorkspaceID: "workspace-1",
				Digest: "digest-1", ConfirmedBy: "agent:session-1", SourceKind: secret,
			},
			want: map[string]any{
				"extension_name": "alpha", "workspace_id": "workspace-1", "digest": "digest-1",
				"confirmed_by": "agent:session-1",
			},
		},
		{
			name: "Should emit secrets updated keys",
			event: LifecycleEvent{
				Type: eventspkg.ExtensionSecretsUpdated, ExtensionName: "alpha", BoundCount: &boundCount,
				Digest: secret, ConfirmedBy: secret,
			},
			want: map[string]any{"extension_name": "alpha", "workspace_id": "", "bound_count": 2},
		},
		{
			name: "Should emit secrets failure keys",
			event: LifecycleEvent{
				Type: eventspkg.ExtensionSecretsUpdateFailed, ExtensionName: "alpha", WorkspaceID: "workspace-1",
				Digest: secret, ConfirmedBy: secret,
			},
			want: map[string]any{"extension_name": "alpha", "workspace_id": "workspace-1"},
		},
		{
			name: "Should emit enabled automation count keys",
			event: LifecycleEvent{
				Type: eventspkg.ExtensionEnabled, ExtensionName: "alpha", AutomationCount: &automationCount,
				Digest: secret, ConfirmedBy: secret,
			},
			want: map[string]any{"extension_name": "alpha", "automation_started_count": 3},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			sink := &recordingLifecycleEventSink{}
			if err := recordExtensionLifecycleEvent(t.Context(), sink, test.event); err != nil {
				t.Fatalf("recordExtensionLifecycleEvent() error = %v", err)
			}
			events := sink.snapshot()
			if len(events) != 1 {
				t.Fatalf("recorded events = %#v, want one", events)
			}
			fields, err := events[0].RequiredFields()
			if err != nil {
				t.Fatalf("RequiredFields() error = %v", err)
			}
			if !reflect.DeepEqual(fields, test.want) {
				t.Fatalf("RequiredFields() = %#v, want %#v", fields, test.want)
			}
			encoded, err := json.Marshal(fields)
			if err != nil {
				t.Fatalf("json.Marshal(fields) error = %v", err)
			}
			if strings.Contains(string(encoded), secret) {
				t.Fatalf("event payload = %s, leaked excluded secret", encoded)
			}
		})
	}
}

func TestManagerCrashBackoffEmitsLifecycleEvent(t *testing.T) {
	t.Parallel()

	t.Run("Should emit workspace backoff from the manager call site", func(t *testing.T) {
		t.Parallel()

		sink := &recordingLifecycleEventSink{}
		manager := NewManager(nil, WithLifecycleEventSink(sink))
		key := InstanceKey{Name: "alpha", WorkspaceID: "workspace-1"}
		process := newFakeProcess(9001)
		managed := &managedExtension{
			key: key, info: ExtensionInfo{Name: key.Name, Enabled: true}, process: process, active: true,
		}
		manager.mu.Lock()
		manager.devExtensions[key] = managed
		manager.mu.Unlock()

		backoff, disabled, owned := manager.recordOwnedInstanceFailure(
			key,
			managed,
			process,
			errors.New("subprocess exited"),
		)
		if backoff <= 0 || disabled || !owned {
			t.Fatalf("recordOwnedInstanceFailure() = (%s, %t, %t), want backoff/false/true", backoff, disabled, owned)
		}
		events := sink.snapshot()
		if len(events) != 1 || events[0].Type != eventspkg.ExtensionCrashLoopBackoff ||
			events[0].ExtensionName != key.Name || events[0].WorkspaceID != key.WorkspaceID {
			t.Fatalf("crash lifecycle events = %#v, want workspace backoff event", events)
		}
	})
}

type recordingLifecycleEventSink struct {
	mu     sync.Mutex
	events []LifecycleEvent
}

func (s *recordingLifecycleEventSink) RecordExtensionLifecycleEvent(
	_ context.Context,
	event LifecycleEvent,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, event)
	return nil
}

func (s *recordingLifecycleEventSink) snapshot() []LifecycleEvent {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]LifecycleEvent(nil), s.events...)
}

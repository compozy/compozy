package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/cmdpalette"
	eventspkg "github.com/compozy/compozy/internal/events"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	"github.com/compozy/compozy/internal/store"
	taskpkg "github.com/compozy/compozy/internal/task"
	"github.com/compozy/compozy/internal/testutil"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

func TestExtensionEventCallSitesUseCanonicalSafePayloads(t *testing.T) {
	t.Parallel()

	t.Run("Should emit exact secret and network keys plus the enable result count", func(t *testing.T) {
		t.Parallel()

		secret := "vault:extensions/global/kit/env/secret-material-must-not-leak"
		cases := []struct {
			name      string
			eventType string
			emit      func(*daemonExtensionService, taskpkg.ActorContext) error
			want      map[string]any
		}{
			{
				name:      "Should emit the secrets-updated projection",
				eventType: eventspkg.ExtensionSecretsUpdated,
				emit: func(service *daemonExtensionService, actor taskpkg.ActorContext) error {
					return service.recordExtensionSecretsUpdatedEvent(
						t.Context(),
						actor,
						extensionpkg.GlobalInstanceKey("kit"),
						contract.ExtensionSecretsPayload{BoundEnvKeys: []string{secret}},
					)
				},
				want: map[string]any{
					"extension_name": "kit", "workspace_id": "", "bound_count": float64(1),
				},
			},
			{
				name:      "Should emit the secrets-failed projection",
				eventType: eventspkg.ExtensionSecretsUpdateFailed,
				emit: func(service *daemonExtensionService, actor taskpkg.ActorContext) error {
					return service.recordExtensionSecretsFailedEvent(
						t.Context(), actor, extensionpkg.InstanceKey{Name: "kit", WorkspaceID: "ws-1"},
					)
				},
				want: map[string]any{"extension_name": "kit", "workspace_id": "ws-1"},
			},
			{
				name:      "Should emit the network-confirmed projection",
				eventType: eventspkg.ExtensionNetworkConfirmed,
				emit: func(service *daemonExtensionService, actor taskpkg.ActorContext) error {
					return service.recordExtensionNetworkConfirmedEvent(
						t.Context(),
						actor,
						extensionpkg.InstanceKey{Name: "kit", WorkspaceID: "ws-1"},
						extensionpkg.NetworkConfirmation{Digest: "digest-1", ConfirmedBy: "operator"},
					)
				},
				want: map[string]any{
					"extension_name": "kit", "workspace_id": "ws-1", "digest": "digest-1", "confirmed_by": "operator",
				},
			},
			{
				name:      "Should emit the enable-result projection",
				eventType: eventspkg.ExtensionEnabled,
				emit: func(service *daemonExtensionService, actor taskpkg.ActorContext) error {
					return service.recordExtensionEnableEvents(
						t.Context(),
						actor,
						extensionpkg.GlobalInstanceKey("kit"),
						nil,
						contract.ExtensionEnableResult{
							Extension:         contract.ExtensionPayload{Name: "kit", LastError: secret},
							AutomationStarted: []string{"kit/alpha", "kit/beta"},
						},
					)
				},
				want: map[string]any{
					"extension_name": "kit", "automation_started_count": float64(2),
				},
			},
		}
		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()
				writer := &extensionEventRecorder{}
				service := &daemonExtensionService{
					eventWriter: writer,
					now:         func() time.Time { return time.Date(2026, 8, 2, 16, 0, 0, 0, time.UTC) },
				}
				actor, err := taskpkg.DeriveHumanActorContext("operator-1", taskpkg.OriginKindCLI, "cli")
				if err != nil {
					t.Fatalf("DeriveHumanActorContext() error = %v", err)
				}
				if err := testCase.emit(service, actor); err != nil {
					t.Fatalf("emit() error = %v", err)
				}
				got := writer.snapshot()
				if len(got) != 1 {
					t.Fatalf("event count = %d, want 1", len(got))
				}
				summary := got[0]
				if summary.Type != testCase.eventType {
					t.Fatalf("event type = %q, want %q", summary.Type, testCase.eventType)
				}
				if strings.Contains(string(summary.Content), secret) {
					t.Fatalf("event %s leaked secret material: %s", summary.Type, summary.Content)
				}
				var fields map[string]any
				if err := json.Unmarshal(summary.Content, &fields); err != nil {
					t.Fatalf("json.Unmarshal(%s) error = %v", summary.Type, err)
				}
				if !reflect.DeepEqual(fields, testCase.want) {
					t.Fatalf("event %s fields = %#v, want %#v", summary.Type, fields, testCase.want)
				}
			})
		}
	})

	t.Run("Should not emit a failure event for a deterministic validation refusal", func(t *testing.T) {
		t.Parallel()

		service, _ := newExtensionSecretsTestService(t, []string{"API_KEY"}, newExtensionSecretVaultFake())
		writer := &extensionEventRecorder{}
		service.eventWriter = writer
		actor, err := taskpkg.DeriveHumanActorContext("operator-1", taskpkg.OriginKindHTTP, "http")
		if err != nil {
			t.Fatalf("DeriveHumanActorContext() error = %v", err)
		}
		value := "secret-value"
		_, err = service.SetExtensionSecrets(
			testutil.Context(t),
			"kit",
			contract.SetExtensionSecretsRequest{Bindings: []contract.ExtensionSecretBindingInput{
				{EnvName: "UNDECLARED", Value: &value},
			}},
			actor,
		)
		if err == nil {
			t.Fatal("SetExtensionSecrets() error = nil, want validation refusal")
		}
		if !errors.Is(err, extensionpkg.ErrExtensionEnvBindingUndeclared) {
			t.Fatalf("SetExtensionSecrets() error = %v, want ErrExtensionEnvBindingUndeclared", err)
		}
		if events := writer.snapshot(); len(events) != 0 {
			t.Fatalf("validation refusal events = %#v, want none", events)
		}
	})

	t.Run("Should persist enable consent and completion as one event batch", func(t *testing.T) {
		t.Parallel()

		writer := &extensionEventRecorder{}
		service := &daemonExtensionService{eventWriter: writer, now: time.Now}
		actor, err := taskpkg.DeriveHumanActorContext("operator-1", taskpkg.OriginKindCLI, "cli")
		if err != nil {
			t.Fatalf("DeriveHumanActorContext() error = %v", err)
		}
		confirmation := &extensionpkg.NetworkConfirmation{Digest: "digest-1", ConfirmedBy: "operator"}
		result := contract.ExtensionEnableResult{
			Extension: contract.ExtensionPayload{Name: "kit"}, AutomationStarted: []string{"kit/job"},
		}
		if err := service.recordExtensionEnableEvents(
			t.Context(), actor, extensionpkg.GlobalInstanceKey("kit"), confirmation, result,
		); err != nil {
			t.Fatalf("recordExtensionEnableEvents() error = %v", err)
		}
		if writer.batchCalls != 1 || writer.writeCalls != 0 {
			t.Fatalf("event writes = batch:%d single:%d, want one atomic batch", writer.batchCalls, writer.writeCalls)
		}
		got := writer.snapshot()
		if len(got) != 2 || got[0].Type != eventspkg.ExtensionNetworkConfirmed ||
			got[1].Type != eventspkg.ExtensionEnabled {
			t.Fatalf("event batch = %#v, want network confirmation then enable completion", got)
		}
	})

	t.Run("Should invalidate every workspace catalog after a global enable", func(t *testing.T) {
		t.Parallel()

		catalog := &recordingExtensionPaletteCatalog{}
		views := &recordingExtensionViewSessions{}
		service := &daemonExtensionService{
			eventWriter: &extensionEventRecorder{},
			paletteNotifier: &extensionPaletteNotifier{
				catalog: func() cmdpalette.Registry { return catalog },
				views:   func() cmdpalette.ViewSessionService { return views },
				workspaces: func(context.Context) ([]workspacepkg.Workspace, error) {
					return []workspacepkg.Workspace{{ID: "ws-a"}, {ID: "ws-b"}}, nil
				},
			},
		}
		actor, err := taskpkg.DeriveHumanActorContext("operator-1", taskpkg.OriginKindCLI, "cli")
		if err != nil {
			t.Fatalf("DeriveHumanActorContext() error = %v", err)
		}
		if err := service.recordExtensionEnableEvents(
			t.Context(), actor, extensionpkg.GlobalInstanceKey("notes"), nil,
			contract.ExtensionEnableResult{Extension: contract.ExtensionPayload{Name: "notes"}},
		); err != nil {
			t.Fatalf("recordExtensionEnableEvents() error = %v", err)
		}
		if !reflect.DeepEqual(catalog.workspaces, []cmdpalette.WorkspaceID{"ws-a", "ws-b"}) {
			t.Fatalf("notified workspaces = %#v, want ws-a and ws-b", catalog.workspaces)
		}
		if !reflect.DeepEqual(views.invalidations, []viewSessionInvalidation{
			{workspace: "ws-a", extension: "notes"},
			{workspace: "ws-b", extension: "notes"},
		}) {
			t.Fatalf("view invalidations = %#v, want notes in both workspaces", views.invalidations)
		}
	})

	t.Run("Should invalidate workspace view sessions on extension crash", func(t *testing.T) {
		t.Parallel()

		catalog := &recordingExtensionPaletteCatalog{}
		views := &recordingExtensionViewSessions{}
		sink := extensionPaletteLifecycleEventSink{notifier: &extensionPaletteNotifier{
			catalog: func() cmdpalette.Registry { return catalog },
			views:   func() cmdpalette.ViewSessionService { return views },
		}}
		if err := sink.RecordExtensionLifecycleEvent(t.Context(), extensionpkg.LifecycleEvent{
			Type: eventspkg.ExtensionCrashLoopBackoff, ExtensionName: "notes", WorkspaceID: "ws-a",
		}); err != nil {
			t.Fatalf("RecordExtensionLifecycleEvent(crash) error = %v", err)
		}
		if !reflect.DeepEqual(catalog.workspaces, []cmdpalette.WorkspaceID{"ws-a"}) {
			t.Fatalf("notified workspaces = %#v, want ws-a", catalog.workspaces)
		}
		if !reflect.DeepEqual(views.invalidations, []viewSessionInvalidation{{
			workspace: "ws-a", extension: "notes",
		}}) {
			t.Fatalf("view invalidations = %#v, want notes in ws-a", views.invalidations)
		}
	})
}

type recordingExtensionPaletteCatalog struct {
	cmdpalette.Registry
	workspaces []cmdpalette.WorkspaceID
}

type viewSessionInvalidation struct {
	profileLens cmdpalette.ProfileLens
	workspace   cmdpalette.WorkspaceID
	extension   string
}

type recordingExtensionViewSessions struct {
	cmdpalette.ViewSessionService
	invalidations []viewSessionInvalidation
}

func (s *recordingExtensionViewSessions) InvalidateInstance(
	_ context.Context,
	profileLens cmdpalette.ProfileLens,
	workspace cmdpalette.WorkspaceID,
	extension string,
	_ uint64,
) error {
	s.invalidations = append(s.invalidations, viewSessionInvalidation{
		profileLens: profileLens,
		workspace:   workspace,
		extension:   extension,
	})
	return nil
}

func (c *recordingExtensionPaletteCatalog) NotifyCatalogChanged(
	_ context.Context,
	_ cmdpalette.ProfileLens,
	workspaceID cmdpalette.WorkspaceID,
) error {
	c.workspaces = append(c.workspaces, workspaceID)
	return nil
}

type extensionEventRecorder struct {
	events     []store.EventSummary
	writeCalls int
	batchCalls int
}

func (r *extensionEventRecorder) WriteEventSummary(_ context.Context, summary store.EventSummary) error {
	r.writeCalls++
	summary.Content = append([]byte(nil), summary.Content...)
	r.events = append(r.events, summary)
	return nil
}

func (r *extensionEventRecorder) WriteEventSummaries(
	_ context.Context,
	summaries []store.EventSummary,
) error {
	r.batchCalls++
	for _, summary := range summaries {
		summary.Content = append([]byte(nil), summary.Content...)
		r.events = append(r.events, summary)
	}
	return nil
}

func (r *extensionEventRecorder) ListEventSummaries(
	context.Context,
	store.EventSummaryQuery,
) ([]store.EventSummary, error) {
	return r.snapshot(), nil
}

func (r *extensionEventRecorder) snapshot() []store.EventSummary {
	return append([]store.EventSummary(nil), r.events...)
}

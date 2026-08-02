package daemon

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	eventspkg "github.com/compozy/compozy/internal/events"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	"github.com/compozy/compozy/internal/store"
	taskpkg "github.com/compozy/compozy/internal/task"
	"github.com/compozy/compozy/internal/testutil"
)

func TestExtensionEventCallSitesUseCanonicalSafePayloads(t *testing.T) {
	t.Parallel()

	t.Run("Should emit exact secret and network keys plus the enable result count", func(t *testing.T) {
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
		secret := "vault:extensions/global/kit/env/secret-material-must-not-leak"
		if err := service.recordExtensionSecretsUpdatedEvent(
			context.Background(),
			actor,
			extensionpkg.GlobalInstanceKey("kit"),
			contract.ExtensionSecretsPayload{BoundEnvKeys: []string{secret}},
		); err != nil {
			t.Fatalf("recordExtensionSecretsUpdatedEvent() error = %v", err)
		}
		if err := service.recordExtensionSecretsFailedEvent(
			context.Background(),
			actor,
			extensionpkg.InstanceKey{Name: "kit", WorkspaceID: "ws-1"},
		); err != nil {
			t.Fatalf("recordExtensionSecretsFailedEvent() error = %v", err)
		}
		if err := service.recordExtensionNetworkConfirmedEvent(
			context.Background(),
			actor,
			extensionpkg.InstanceKey{Name: "kit", WorkspaceID: "ws-1"},
			extensionpkg.NetworkConfirmation{Digest: "digest-1", ConfirmedBy: "operator"},
		); err != nil {
			t.Fatalf("recordExtensionNetworkConfirmedEvent() error = %v", err)
		}
		if err := service.recordExtensionEnabledEvent(
			context.Background(),
			actor,
			contract.ExtensionEnableResult{
				Extension:         contract.ExtensionPayload{Name: "kit", LastError: secret},
				AutomationStarted: []string{"kit/alpha", "kit/beta"},
			},
		); err != nil {
			t.Fatalf("recordExtensionEnabledEvent() error = %v", err)
		}

		got := writer.snapshot()
		if len(got) != 4 {
			t.Fatalf("event count = %d, want 4", len(got))
		}
		wantExact := map[string]map[string]any{
			eventspkg.ExtensionSecretsUpdated: {
				"extension_name": "kit", "workspace_id": "", "bound_count": float64(1),
			},
			eventspkg.ExtensionSecretsUpdateFailed: {
				"extension_name": "kit", "workspace_id": "ws-1",
			},
			eventspkg.ExtensionNetworkConfirmed: {
				"extension_name": "kit", "workspace_id": "ws-1", "digest": "digest-1", "confirmed_by": "operator",
			},
		}
		for _, summary := range got {
			if strings.Contains(string(summary.Content), secret) {
				t.Fatalf("event %s leaked secret material: %s", summary.Type, summary.Content)
			}
			var fields map[string]any
			if err := json.Unmarshal(summary.Content, &fields); err != nil {
				t.Fatalf("json.Unmarshal(%s) error = %v", summary.Type, err)
			}
			if want, exact := wantExact[summary.Type]; exact && !reflect.DeepEqual(fields, want) {
				t.Fatalf("event %s fields = %#v, want %#v", summary.Type, fields, want)
			}
			if summary.Type == eventspkg.ExtensionEnabled {
				if fields["automation_started_count"] != float64(2) {
					t.Fatalf("extension.enabled fields = %#v, want automation_started_count=2", fields)
				}
			}
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
			contract.SetExtensionSecretsRequest{Secrets: map[string]contract.ExtensionSecretInput{
				"UNDECLARED": {Value: &value},
			}},
			actor,
		)
		if err == nil {
			t.Fatal("SetExtensionSecrets() error = nil, want validation refusal")
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

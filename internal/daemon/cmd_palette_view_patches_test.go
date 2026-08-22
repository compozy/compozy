package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/cmdpalette"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

func TestExtensionCmdPaletteViewPatchSubscriber(t *testing.T) {
	t.Parallel()

	t.Run("Should satisfy ViewPatchSubscriber so Service subscribe does not fail closed", func(t *testing.T) {
		t.Parallel()
		provider := newViewPatchTestProvider(t, viewPatchTestProjection())
		service := newViewPatchTestService(t, provider)
		if err := provider.PublishViewPatch(t.Context(), testCmdPaletteProfileLens, "ws-a", "notes", viewPatchTestPatch("vr_1", "vr_2")); err != nil {
			t.Fatalf("PublishViewPatch() error = %v", err)
		}
		var subscriber cmdpalette.ViewPatchSubscriber = provider
		_, cancelProbe, err := subscriber.SubscribeViewPatches(t.Context(), viewPatchSubscribeRequest(0, ""))
		if err != nil {
			t.Fatalf("ViewPatchSubscriber.SubscribeViewPatches() error = %v", err)
		}
		cancelProbe()
		snapshot, events, cancel, err := service.SubscribeViewPatches(
			t.Context(), viewPatchSubscribeRequest(0, ""),
		)
		if err != nil {
			if errors.Is(err, cmdpalette.ErrViewPatchStreamUnavailable) {
				t.Fatalf("Service.SubscribeViewPatches() failed closed: %v", err)
			}
			t.Fatalf("Service.SubscribeViewPatches() error = %v", err)
		}
		t.Cleanup(cancel)
		if snapshot.Revision == "" || snapshot.Descriptor.ID != "ext.notes.recent" {
			t.Fatalf("SubscribeViewPatches() snapshot = %#v", snapshot)
		}
		event := recvViewPatchEvent(t, events)
		if event.Sequence != 1 || event.Patch.From != "vr_1" || event.Patch.To != "vr_2" {
			t.Fatalf("SubscribeViewPatches() event = %#v, want retained vr_1→vr_2", event)
		}
	})

	t.Run("Should replay retained patches after the cursor on the same epoch", func(t *testing.T) {
		t.Parallel()
		provider := newViewPatchTestProvider(t, viewPatchTestProjection())
		if err := provider.PublishViewPatch(t.Context(), testCmdPaletteProfileLens, "ws-a", "notes", viewPatchTestPatch("vr_1", "vr_2")); err != nil {
			t.Fatalf("PublishViewPatch(first) error = %v", err)
		}
		if err := provider.PublishViewPatch(t.Context(), testCmdPaletteProfileLens, "ws-a", "notes", viewPatchTestPatch("vr_2", "vr_3")); err != nil {
			t.Fatalf("PublishViewPatch(second) error = %v", err)
		}
		events, cancel, err := provider.SubscribeViewPatchesAfter(t.Context(), testCmdPaletteProfileLens, "ws-a", "ext.notes.recent", 1, "")
		if err != nil {
			t.Fatalf("SubscribeViewPatchesAfter() error = %v", err)
		}
		t.Cleanup(cancel)
		event := recvViewPatchEvent(t, events)
		if event.Sequence != 2 || event.Patch.From != "vr_2" || event.Patch.To != "vr_3" {
			t.Fatalf("replayed event = %#v, want sequence 2 vr_2→vr_3", event)
		}
	})

	t.Run("Should deliver a live patch published after subscribe", func(t *testing.T) {
		t.Parallel()
		provider := newViewPatchTestProvider(t, viewPatchTestProjection())
		events, cancel, err := provider.SubscribeViewPatches(t.Context(), viewPatchSubscribeRequest(0, ""))
		if err != nil {
			t.Fatalf("SubscribeViewPatches() error = %v", err)
		}
		t.Cleanup(cancel)
		if err := provider.PublishViewPatch(t.Context(), testCmdPaletteProfileLens, "ws-a", "notes", viewPatchTestPatch("vr_1", "vr_2")); err != nil {
			t.Fatalf("PublishViewPatch() error = %v", err)
		}
		event := recvViewPatchEvent(t, events)
		if event.Sequence != 1 || event.StreamEpoch == "" || event.Patch.ViewID != "ext.notes.recent" {
			t.Fatalf("live event = %#v, want sequence 1 with epoch", event)
		}
	})

	t.Run("Should skip replay when the after cursor names a different epoch", func(t *testing.T) {
		t.Parallel()
		provider := newViewPatchTestProvider(t, viewPatchTestProjection())
		if err := provider.PublishViewPatch(t.Context(), testCmdPaletteProfileLens, "ws-a", "notes", viewPatchTestPatch("vr_1", "vr_2")); err != nil {
			t.Fatalf("PublishViewPatch() error = %v", err)
		}
		events, cancel, err := provider.SubscribeViewPatches(
			t.Context(), viewPatchSubscribeRequest(1, "vse_stale"),
		)
		if err != nil {
			t.Fatalf("SubscribeViewPatches() error = %v", err)
		}
		t.Cleanup(cancel)
		select {
		case event := <-events:
			t.Fatalf("replayed %#v, want no stale-epoch replay", event)
		default:
		}
	})

	t.Run("Should isolate patches across workspaces", func(t *testing.T) {
		t.Parallel()
		provider := newViewPatchTestProvider(t, viewPatchTestProjection())
		if err := provider.PublishViewPatch(t.Context(), testCmdPaletteProfileLens, "ws-a", "notes", viewPatchTestPatch("vr_1", "vr_2")); err != nil {
			t.Fatalf("PublishViewPatch(ws-a) error = %v", err)
		}
		events, cancel, err := provider.SubscribeViewPatches(t.Context(), cmdpalette.ViewPatchSubscribeRequest{
			ProfileLens: testCmdPaletteProfileLens,
			Workspace:   "ws-b", ViewID: "ext.notes.recent",
		})
		if err != nil {
			t.Fatalf("SubscribeViewPatches(ws-b) error = %v", err)
		}
		t.Cleanup(cancel)
		select {
		case event := <-events:
			t.Fatalf("ws-b received %#v, want workspace isolation", event)
		default:
		}
	})

	t.Run("Should isolate patches across profile lenses", func(t *testing.T) {
		t.Parallel()
		provider := newViewPatchTestProvider(t, viewPatchTestProjection())
		if err := provider.PublishViewPatch(
			t.Context(), testCmdPaletteProfileLens, "ws-a", "notes", viewPatchTestPatch("vr_1", "vr_2"),
		); err != nil {
			t.Fatalf("PublishViewPatch(profile a) error = %v", err)
		}
		profileB := cmdpalette.ScopedProfileLens("01ARZ3NDEKTSV4RRFFQ69G5FAV", "review")
		events, cancel, err := provider.SubscribeViewPatches(t.Context(), cmdpalette.ViewPatchSubscribeRequest{
			ProfileLens: profileB, Workspace: "ws-a", ViewID: "ext.notes.recent",
		})
		if err != nil {
			t.Fatalf("SubscribeViewPatches(profile b) error = %v", err)
		}
		t.Cleanup(cancel)
		select {
		case event := <-events:
			t.Fatalf("profile b received %#v, want profile isolation", event)
		default:
		}
	})

	t.Run("Should close the subscriber channel on cancel and ignore later publishes", func(t *testing.T) {
		t.Parallel()
		provider := newViewPatchTestProvider(t, viewPatchTestProjection())
		events, cancel, err := provider.SubscribeViewPatches(t.Context(), viewPatchSubscribeRequest(0, ""))
		if err != nil {
			t.Fatalf("SubscribeViewPatches() error = %v", err)
		}
		cancel()
		if _, open := <-events; open {
			t.Fatal("subscriber stayed open after cancel")
		}
		if err := provider.PublishViewPatch(t.Context(), testCmdPaletteProfileLens, "ws-a", "notes", viewPatchTestPatch("vr_1", "vr_2")); err != nil {
			t.Fatalf("PublishViewPatch(after cancel) error = %v", err)
		}
	})

	t.Run("Should close remaining subscribers when the hub shuts down", func(t *testing.T) {
		t.Parallel()
		provider := newViewPatchTestProvider(t, viewPatchTestProjection())
		events, cancel, err := provider.SubscribeViewPatches(t.Context(), viewPatchSubscribeRequest(0, ""))
		if err != nil {
			t.Fatalf("SubscribeViewPatches() error = %v", err)
		}
		t.Cleanup(cancel)
		provider.CloseViewPatches()
		if _, open := <-events; open {
			t.Fatal("subscriber stayed open after CloseViewPatches")
		}
		err = provider.PublishViewPatch(t.Context(), testCmdPaletteProfileLens, "ws-a", "notes", viewPatchTestPatch("vr_1", "vr_2"))
		if err == nil {
			t.Fatal("PublishViewPatch() after close error = nil")
		}
	})

	t.Run("Should reject a missing, programmable, or foreign view", func(t *testing.T) {
		t.Parallel()
		projection := extensionpkg.CmdPaletteProjection{
			Views: []extensionpkg.CmdPaletteProjectedView{
				{ID: "ext.notes.board", Title: "Board", Kind: "list", Program: true, Extension: "notes"},
				{ID: "ext.notes.recent", Title: "Recent", Kind: "list", SourceTool: "notes.list", Extension: "notes"},
			},
		}
		t.Run("Should reject a missing view", func(t *testing.T) {
			t.Parallel()
			provider := newViewPatchTestProvider(t, projection)
			_, _, err := provider.SubscribeViewPatches(t.Context(), cmdpalette.ViewPatchSubscribeRequest{
				ProfileLens: testCmdPaletteProfileLens,
				Workspace:   "ws-a", ViewID: "ext.notes.gone",
			})
			var notFound *cmdpalette.ViewNotFoundError
			if !errors.As(err, &notFound) || notFound.ViewID != "ext.notes.gone" {
				t.Fatalf("SubscribeViewPatches(missing) error = %v, want ViewNotFoundError", err)
			}
		})
		t.Run("Should reject a programmable view", func(t *testing.T) {
			t.Parallel()
			provider := newViewPatchTestProvider(t, projection)
			_, _, err := provider.SubscribeViewPatches(t.Context(), cmdpalette.ViewPatchSubscribeRequest{
				ProfileLens: testCmdPaletteProfileLens,
				Workspace:   "ws-a", ViewID: "ext.notes.board",
			})
			if err == nil {
				t.Fatal("SubscribeViewPatches(program) error = nil")
			}
		})
		t.Run("Should reject a foreign extension publish", func(t *testing.T) {
			t.Parallel()
			provider := newViewPatchTestProvider(t, projection)
			err := provider.PublishViewPatch(t.Context(), testCmdPaletteProfileLens, "ws-a", "other", viewPatchTestPatch("vr_1", "vr_2"))
			if err == nil {
				t.Fatal("PublishViewPatch(foreign) error = nil")
			}
		})
	})
}

func newViewPatchTestProvider(
	t *testing.T,
	projection extensionpkg.CmdPaletteProjection,
) *extensionCmdPaletteProvider {
	t.Helper()
	payload, err := json.Marshal(cmdpalette.ViewPayload{
		View: cmdpalette.ViewContractVersion,
		Sections: []cmdpalette.Section{{Rows: []cmdpalette.Row{{
			ID: "task-1", Title: "Review task",
			Badge: &cmdpalette.ViewBadge{Label: "Queued", Tone: "info"},
		}}}},
	})
	if err != nil {
		t.Fatalf("json.Marshal(view payload) error = %v", err)
	}
	return &extensionCmdPaletteProvider{
		palette: viewPatchRuntimeStub{projection: projection},
		patches: newViewPatchHub(),
		tools:   &recordingCmdPaletteToolRegistry{result: toolspkg.ToolResult{Structured: payload}},
	}
}

func newViewPatchTestService(t *testing.T, provider *extensionCmdPaletteProvider) *cmdpalette.Service {
	t.Helper()
	service, err := cmdpalette.NewRegistry(
		[]cmdpalette.ProviderRegistration{{
			Source:   cmdpalette.Source{Kind: cmdpalette.SourceKindCore},
			Provider: viewPatchTestCommands{},
		}},
		nil, nil, viewPatchTestExecutor{},
		cmdpalette.WithDynamicViewProvider(provider),
	)
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}
	return service
}

func viewPatchTestProjection() extensionpkg.CmdPaletteProjection {
	return extensionpkg.CmdPaletteProjection{
		Views: []extensionpkg.CmdPaletteProjectedView{{
			ID: "ext.notes.recent", Title: "Recent", Kind: "list",
			SourceTool: "notes.list", Extension: "notes",
		}},
	}
}

func viewPatchSubscribeRequest(after int64, epoch string) cmdpalette.ViewPatchSubscribeRequest {
	return cmdpalette.ViewPatchSubscribeRequest{
		ProfileLens: testCmdPaletteProfileLens,
		Workspace:   "ws-a", ViewID: "ext.notes.recent", After: after, StreamEpoch: epoch,
	}
}

func viewPatchTestPatch(from string, to string) cmdpalette.ViewPatch {
	return cmdpalette.ViewPatch{
		ViewID: "ext.notes.recent", From: from, To: to,
		Ops: []cmdpalette.PatchOp{{
			Op: "replace", Path: "/sections/0/rows/0/title", Value: json.RawMessage(`"Changed"`),
		}},
	}
}

func recvViewPatchEvent(t *testing.T, events <-chan cmdpalette.ViewPatchEvent) cmdpalette.ViewPatchEvent {
	t.Helper()
	select {
	case event, open := <-events:
		if !open {
			t.Fatal("view patch channel closed")
		}
		return event
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for a view patch event")
	}
	return cmdpalette.ViewPatchEvent{}
}

type viewPatchRuntimeStub struct {
	projection extensionpkg.CmdPaletteProjection
}

func (s viewPatchRuntimeStub) CmdPalette(
	string,
	extensionpkg.ProfileLens,
) (extensionpkg.CmdPaletteProjection, error) {
	return s.projection, nil
}

type viewPatchTestCommands struct{}

func (viewPatchTestCommands) ProvideCommands(
	context.Context,
	cmdpalette.CatalogRequest,
) ([]cmdpalette.Descriptor, error) {
	return nil, nil
}

type viewPatchTestExecutor struct{}

func (viewPatchTestExecutor) ExecuteAction(
	context.Context,
	cmdpalette.ExecutionRequest,
) (cmdpalette.ExecutionResult, error) {
	return cmdpalette.ExecutionResult{}, nil
}

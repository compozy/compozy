package extensionpkg

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/compozy/compozy/internal/cmdpalette"
	"github.com/compozy/compozy/internal/resources"
)

func TestHostAPIViewPatch(t *testing.T) {
	t.Parallel()

	t.Run("Should publish a programmable frame when view_session is set", func(t *testing.T) {
		t.Parallel()
		views := &hostAPIViewServiceStub{}
		handler := newHostAPIViewPatchHandler(t, views, nil)
		_, err := handler.Handle(t.Context(), "notes", "view/patch", mustViewPatchParams(t, map[string]any{
			"view_session": "vs_1", "revision": "vr_1", "generation": 1, "handlers": []string{},
		}))
		if err != nil {
			t.Fatalf("Handle(view/patch session) error = %v", err)
		}
		if views.publishToken.ViewSession != "vs_1" || views.publishToken.Extension != "notes" {
			t.Fatalf("PublishFrame token = %#v", views.publishToken)
		}
	})

	t.Run("Should publish a declarative patch from a workspace-bound session", func(t *testing.T) {
		t.Parallel()
		publisher := &recordingViewPatchPublisher{}
		handler := newHostAPIViewPatchHandler(t, nil, publisher)
		ctx := withHostAPIResourceSession(t.Context(), &hostAPIResourceSession{
			Actor: resources.MutationActor{
				MaxScope: resources.ResourceScope{Kind: resources.ResourceScopeKindWorkspace, ID: "ws-a"},
			},
		})
		_, err := handler.Handle(ctx, "notes", "view/patch", mustViewPatchParams(t, map[string]any{
			"patch": map[string]any{
				"view_id": "ext.notes.recent", "from": "vr_1", "to": "vr_2",
				"ops": []map[string]any{{
					"op": "replace", "path": "/sections/0/rows/0/title", "value": "Changed",
				}},
			},
		}))
		if err != nil {
			t.Fatalf("Handle(view/patch declarative) error = %v", err)
		}
		if publisher.workspace != "ws-a" || publisher.extension != "notes" ||
			publisher.patch.ViewID != "ext.notes.recent" {
			t.Fatalf("PublishViewPatch = workspace %q extension %q patch %#v",
				publisher.workspace, publisher.extension, publisher.patch)
		}
	})

	t.Run("Should reject a declarative patch without a bound workspace", func(t *testing.T) {
		t.Parallel()
		handler := newHostAPIViewPatchHandler(t, nil, &recordingViewPatchPublisher{})
		_, err := handler.Handle(t.Context(), "notes", "view/patch", mustViewPatchParams(t, map[string]any{
			"patch": map[string]any{"view_id": "ext.notes.recent", "from": "vr_1", "to": "vr_2"},
		}))
		if err == nil {
			t.Fatal("Handle(view/patch unbound) error = nil")
		}
	})

	t.Run("Should reject a frame with neither a session nor a declarative patch", func(t *testing.T) {
		t.Parallel()
		handler := newHostAPIViewPatchHandler(t, &hostAPIViewServiceStub{}, &recordingViewPatchPublisher{})
		_, err := handler.Handle(t.Context(), "notes", "view/patch", mustViewPatchParams(t, map[string]any{
			"revision": "vr_1", "generation": 1, "handlers": []string{},
		}))
		if err == nil {
			t.Fatal("Handle(view/patch empty) error = nil")
		}
	})
}

func newHostAPIViewPatchHandler(
	t *testing.T,
	views cmdpalette.ViewService,
	publisher ViewPatchPublisher,
) *HostAPIHandler {
	t.Helper()
	opts := []HostAPIOption{
		WithHostAPICapabilityChecker(newTestCapabilityChecker("notes", SourceUser, []string{"view/patch"})),
	}
	if views != nil {
		opts = append(opts, WithHostAPIViewService(views))
	}
	if publisher != nil {
		opts = append(opts, WithHostAPIViewPatchPublisher(publisher))
	}
	return NewHostAPIHandler(nil, nil, nil, nil, opts...)
}

func mustViewPatchParams(t *testing.T, payload map[string]any) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("json.Marshal(view/patch) error = %v", err)
	}
	return raw
}

type recordingViewPatchPublisher struct {
	profileLens cmdpalette.ProfileLens
	workspace   cmdpalette.WorkspaceID
	extension   string
	patch       cmdpalette.ViewPatch
}

func (r *recordingViewPatchPublisher) PublishViewPatch(
	_ context.Context,
	profileLens cmdpalette.ProfileLens,
	workspaceID cmdpalette.WorkspaceID,
	extension string,
	patch cmdpalette.ViewPatch,
) error {
	r.profileLens = profileLens
	r.workspace = workspaceID
	r.extension = extension
	r.patch = patch
	return nil
}

type hostAPIViewServiceStub struct {
	publishToken cmdpalette.SessionToken
	publishFrame cmdpalette.ViewFrame
}

func (s *hostAPIViewServiceStub) ResolveView(
	context.Context, cmdpalette.ProfileLens, cmdpalette.WorkspaceID, string,
) (cmdpalette.ViewDescriptor, error) {
	return cmdpalette.ViewDescriptor{}, nil
}

func (s *hostAPIViewServiceStub) OpenSource(
	context.Context, cmdpalette.ProfileLens, cmdpalette.WorkspaceID, string,
) (cmdpalette.ViewSnapshot, error) {
	return cmdpalette.ViewSnapshot{}, nil
}

func (s *hostAPIViewServiceStub) SubscribeViewPatches(
	context.Context, cmdpalette.ViewPatchSubscribeRequest,
) (cmdpalette.ViewSnapshot, <-chan cmdpalette.ViewPatchEvent, func(), error) {
	return cmdpalette.ViewSnapshot{}, nil, func() {}, nil
}

func (s *hostAPIViewServiceStub) OpenSession(
	context.Context, cmdpalette.ViewSessionOpenRequest,
) (cmdpalette.ViewSessionOpenResult, error) {
	return cmdpalette.ViewSessionOpenResult{}, nil
}

func (s *hostAPIViewServiceStub) AdmitEvent(context.Context, cmdpalette.SessionToken, cmdpalette.ViewEvent) error {
	return nil
}

func (s *hostAPIViewServiceStub) PublishFrame(
	_ context.Context,
	token cmdpalette.SessionToken,
	frame cmdpalette.ViewFrame,
) error {
	s.publishToken = token
	s.publishFrame = frame
	return nil
}

func (s *hostAPIViewServiceStub) AckEffects(context.Context, cmdpalette.SessionToken, []string) error {
	return nil
}

func (s *hostAPIViewServiceStub) SubscribeSessionFrames(
	context.Context, cmdpalette.SessionToken,
) (cmdpalette.ViewFrame, <-chan cmdpalette.ViewFrame, func(), error) {
	return cmdpalette.ViewFrame{}, nil, func() {}, nil
}

func (s *hostAPIViewServiceStub) CloseSession(context.Context, cmdpalette.SessionToken, string) error {
	return nil
}

func (s *hostAPIViewServiceStub) CloseClientSessions(
	context.Context,
	cmdpalette.ProfileLens,
	cmdpalette.WorkspaceID,
	cmdpalette.ClientID,
) error {
	return nil
}

func (s *hostAPIViewServiceStub) InvalidateInstance(
	context.Context,
	cmdpalette.ProfileLens,
	cmdpalette.WorkspaceID,
	string,
	uint64,
) error {
	return nil
}

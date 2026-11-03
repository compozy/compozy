package daemon

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	core "github.com/compozy/compozy/internal/api/core"
	apitest "github.com/compozy/compozy/internal/api/testutil"
	profilepkg "github.com/compozy/compozy/internal/profile"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/testutil"
	toolspkg "github.com/compozy/compozy/internal/tools"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
	"github.com/compozy/compozy/internal/workspaceaccess"
)

func TestWorkspaceAccessConsentLifecycle(t *testing.T) {
	t.Parallel()

	t.Run("Should UT-084 clear only the stopped session consent through the lifecycle fanout", func(t *testing.T) {
		t.Parallel()
		ctx := testutil.Context(t)
		cache := newWorkspaceAccessConsentCache()
		fanout := newSessionLifecycleFanout(cache)
		cache.PutConsent(ctx, "sess-a", workspaceaccess.ConsentAllow)
		cache.PutConsent(ctx, "sess-b", workspaceaccess.ConsentReject)

		fanout.OnSessionStopped(ctx, &session.Session{ID: "sess-a", AgentName: "coder"})

		if consent, ok := cache.ConsentFor(ctx, "sess-a"); ok || consent != "" {
			t.Fatalf("ConsentFor(sess-a) = (%q, %t), want miss", consent, ok)
		}
		if consent, ok := cache.ConsentFor(ctx, "sess-b"); !ok || consent != workspaceaccess.ConsentReject {
			t.Fatalf("ConsentFor(sess-b) = (%q, %t), want reject hit", consent, ok)
		}
		fanout.OnSessionCreated(ctx, &session.Session{ID: "sess-c", AgentName: "coder"})
		if consent, ok := cache.ConsentFor(ctx, "sess-c"); ok || consent != "" {
			t.Fatalf("ConsentFor(new same-agent session) = (%q, %t), want miss", consent, ok)
		}
	})
}

func TestNativeSessionTargetAccess(t *testing.T) {
	t.Parallel()

	const (
		homeWorkspace    = "ws_00000000000000a1"
		foreignWorkspace = "ws_00000000000000b2"
		seededSecret     = "sk-proj-AbCdEf0123456789xyzQRS"
	)
	statusDescriptor := toolspkg.Descriptor{
		ID:      toolspkg.ToolIDSessionStatus,
		Backend: toolspkg.BackendRef{Kind: toolspkg.BackendNativeGo, NativeName: "test"},
		InputSchema: json.RawMessage(
			`{"type":"object","properties":{"workspace":{"type":"string"},"session_id":{"type":"string"}}}`,
		),
		ReadOnly: true,
	}
	actorScope := toolspkg.Scope{SessionID: "sess-actor", WorkspaceID: homeWorkspace, AgentName: "coder"}
	sessionsFor := func(mode workspaceaccess.Mode) *apitest.StubSessionManager {
		return &apitest.StubSessionManager{StatusFn: func(_ context.Context, id string) (*session.Info, error) {
			switch id {
			case "sess-actor":
				return &session.Info{
					ID: id, AgentName: "coder", WorkspaceID: homeWorkspace, ProfileID: "p-x",
					State: session.StateActive, EffectivePermissions: string(mode),
				}, nil
			case "sess-own":
				return &session.Info{
					ID: id, AgentName: "coder", WorkspaceID: homeWorkspace, ProfileID: "p-x",
					State: session.StateActive, Name: "own " + seededSecret,
				}, nil
			case "sess-foreign":
				return &session.Info{
					ID: id, AgentName: "coder", WorkspaceID: foreignWorkspace, ProfileID: "p-y",
					State: session.StateActive, Name: "deploy with " + seededSecret,
				}, nil
			case "sess-peer":
				return &session.Info{
					ID: id, AgentName: "coder", WorkspaceID: homeWorkspace, ProfileID: "p-y",
					State: session.StateStopped,
				}, nil
			case "sess-sibling":
				return &session.Info{
					ID: id, AgentName: "coder", WorkspaceID: foreignWorkspace, ProfileID: "p-x",
					State: session.StateActive,
				}, nil
			case "sess-user":
				return &session.Info{ID: id, AgentName: "coder", ProfileID: "p-y", State: session.StateStopped}, nil
			case "sess-user-actor":
				return &session.Info{
					ID: id, AgentName: "coder", ProfileID: "p-x",
					State: session.StateActive, EffectivePermissions: string(mode),
				}, nil
			default:
				return nil, session.ErrSessionNotFound
			}
		}}
	}
	newPolicy := func(t *testing.T, sessions *apitest.StubSessionManager) workspaceaccess.Policy {
		t.Helper()
		facts, err := newWorkspaceAccessModeSource(sessions)
		if err != nil {
			t.Fatalf("newWorkspaceAccessModeSource() error = %v", err)
		}
		policy, err := workspaceaccess.New(workspaceaccess.Deps{
			Facts:   facts,
			Consent: newWorkspaceAccessConsentCache(),
			Audit:   nativeSessionTargetAuditStub{},
			Log:     slog.New(slog.DiscardHandler),
		})
		if err != nil {
			t.Fatalf("workspaceaccess.New() error = %v", err)
		}
		return policy
	}
	authorize := func(
		t *testing.T,
		binder *nativeWorkspaceInputBinder,
		input string,
	) (json.RawMessage, error) {
		t.Helper()
		bound, err := binder.BindCallInput(t.Context(), actorScope, statusDescriptor, json.RawMessage(input))
		if err != nil {
			return nil, err
		}
		return bound, binder.AuthorizeCallInput(t.Context(), actorScope, statusDescriptor, toolspkg.CallRequest{
			ToolID: statusDescriptor.ID, SessionID: actorScope.SessionID, Input: bound,
		})
	}

	t.Run("Should resolve an omitted workspace from a foreign session under approve-all", func(t *testing.T) {
		t.Parallel()
		sessions := sessionsFor(workspaceaccess.ModeApproveAll)
		binder := newNativeWorkspaceInputBinder(nativeTestWorkspaceService(t), sessions, newPolicy(t, sessions), nil)
		for input, wantWorkspace := range map[string]string{
			`{"session_id":"sess-foreign"}`: foreignWorkspace,
			`{"session_id":"sess-user"}`:    workspaceaccess.UserSelector,
		} {
			bound, err := authorize(t, binder, input)
			if err != nil {
				t.Fatalf("authorize(%s) error = %v, want approve-all reach", input, err)
			}
			if !strings.Contains(string(bound), `"workspace":"`+wantWorkspace+`"`) {
				t.Fatalf("bound input = %s, want the session's own workspace %q", bound, wantWorkspace)
			}
		}
	})

	t.Run("Should keep approve-all writes to another workspace of the same profile", func(t *testing.T) {
		t.Parallel()
		renameDescriptor := statusDescriptor
		renameDescriptor.ID, renameDescriptor.ReadOnly = toolspkg.ToolIDSessionRename, false
		write := func(t *testing.T, binder *nativeWorkspaceInputBinder, input string) error {
			t.Helper()
			bound, err := binder.BindCallInput(t.Context(), actorScope, renameDescriptor, json.RawMessage(input))
			if err != nil {
				return err
			}
			return binder.AuthorizeCallInput(t.Context(), actorScope, renameDescriptor, toolspkg.CallRequest{
				ToolID: renameDescriptor.ID, SessionID: actorScope.SessionID, Input: bound,
			})
		}
		sessions := sessionsFor(workspaceaccess.ModeApproveAll)
		binder := newNativeWorkspaceInputBinder(nativeTestWorkspaceService(t), sessions, newPolicy(t, sessions), nil)
		if err := write(t, binder, `{"session_id":"sess-sibling"}`); err != nil {
			t.Fatalf("same-profile cross-workspace write error = %v, want the ADR-007 approve-all reach", err)
		}
		if err := write(t, binder, `{"session_id":"sess-missing"}`); err != nil {
			t.Fatalf("missing-session write error = %v, want the handler to report not found", err)
		}
		err := write(t, binder, `{"session_id":"sess-foreign"}`)
		requireToolReason(t, err, toolspkg.ErrToolDenied, toolspkg.ReasonWorkspaceAccessDenied)
		if !strings.Contains(err.Error(), "workspace_access: unrestricted") {
			t.Fatalf("cross-profile write error = %v, want the grant named", err)
		}
		denySessions := sessionsFor(workspaceaccess.ModeDenyAll)
		denied := newNativeWorkspaceInputBinder(
			nativeTestWorkspaceService(t),
			denySessions,
			newPolicy(t, denySessions),
			nil,
		)
		foreignErr, missingErr := write(
			t,
			denied,
			`{"session_id":"sess-foreign"}`,
		), write(
			t,
			denied,
			`{"session_id":"sess-missing"}`,
		)
		if foreignErr == nil || missingErr == nil || foreignErr.Error() != missingErr.Error() {
			t.Fatalf("deny-all write denials differ:\nforeign: %v\nmissing: %v", foreignErr, missingErr)
		}
	})

	t.Run("Should treat another profile's session in the same project as a crossing for fork", func(t *testing.T) {
		t.Parallel()
		forkDescriptor := statusDescriptor
		forkDescriptor.ID, forkDescriptor.ReadOnly = toolspkg.ToolIDSessionFork, false
		fork := func(t *testing.T, binder *nativeWorkspaceInputBinder, input string) error {
			t.Helper()
			bound, err := binder.BindCallInput(t.Context(), actorScope, forkDescriptor, json.RawMessage(input))
			if err != nil {
				return err
			}
			return binder.AuthorizeCallInput(t.Context(), actorScope, forkDescriptor, toolspkg.CallRequest{
				ToolID: forkDescriptor.ID, SessionID: actorScope.SessionID, Input: bound,
			})
		}
		sessions := sessionsFor(workspaceaccess.ModeApproveAll)
		binder := newNativeWorkspaceInputBinder(nativeTestWorkspaceService(t), sessions, newPolicy(t, sessions), nil)
		if err := fork(t, binder, `{"session_id":"sess-own"}`); err != nil {
			t.Fatalf("same-profile fork error = %v, want the caller's own work to fork", err)
		}
		// The derive manager checks only the workspace of a project source, so this boundary is
		// what keeps one profile from copying another profile's transcript in a shared project.
		err := fork(t, binder, `{"session_id":"sess-peer"}`)
		requireToolReason(t, err, toolspkg.ErrToolDenied, toolspkg.ReasonWorkspaceAccessDenied)
	})

	t.Run("Should answer a denied caller the same for foreign and missing sessions", func(t *testing.T) {
		t.Parallel()
		sessions := sessionsFor(workspaceaccess.ModeDenyAll)
		binder := newNativeWorkspaceInputBinder(nativeTestWorkspaceService(t), sessions, newPolicy(t, sessions), nil)
		_, foreignErr := authorize(t, binder, `{"session_id":"sess-foreign"}`)
		_, missingErr := authorize(t, binder, `{"session_id":"sess-missing"}`)
		requireToolReason(t, foreignErr, toolspkg.ErrToolDenied, toolspkg.ReasonWorkspaceAccessDenied)
		requireToolReason(t, missingErr, toolspkg.ErrToolDenied, toolspkg.ReasonWorkspaceAccessDenied)
		if foreignErr.Error() != missingErr.Error() {
			t.Fatalf("denials differ:\nforeign: %v\nmissing: %v", foreignErr, missingErr)
		}
		if !strings.Contains(foreignErr.Error(), "deny-all") || strings.Contains(foreignErr.Error(), foreignWorkspace) {
			t.Fatalf("denial = %v, want the deny-all axis without the target workspace", foreignErr)
		}
		public := core.ToolErrorResponseForError(foreignErr, http.StatusForbidden, true).Error.Message
		if !strings.Contains(public, "deny-all") {
			t.Fatalf("public denial message = %q, want the axis guidance on every transport", public)
		}
		if _, err := authorize(t, binder, `{"session_id":"sess-own"}`); err != nil {
			t.Fatalf("authorize(own session) error = %v, want own workspace and profile unchanged", err)
		}
	})

	t.Run("Should deny @user probes from User scope like missing sessions", func(t *testing.T) {
		t.Parallel()
		sessions := sessionsFor(workspaceaccess.ModeDenyAll)
		binder := newNativeWorkspaceInputBinder(nativeTestWorkspaceService(t), sessions, newPolicy(t, sessions), nil)
		userScope := toolspkg.Scope{SessionID: "sess-user-actor", AgentName: "coder"}
		call := func(input string) error {
			bound, err := binder.BindCallInput(t.Context(), userScope, statusDescriptor, json.RawMessage(input))
			if err != nil {
				return err
			}
			return binder.AuthorizeCallInput(t.Context(), userScope, statusDescriptor, toolspkg.CallRequest{
				ToolID: statusDescriptor.ID, SessionID: userScope.SessionID, Input: bound,
			})
		}
		foreignErr := call(`{"workspace":"@user","session_id":"sess-foreign"}`)
		missingErr := call(`{"workspace":"@user","session_id":"sess-missing"}`)
		requireToolReason(t, foreignErr, toolspkg.ErrToolDenied, toolspkg.ReasonWorkspaceAccessDenied)
		if missingErr == nil || foreignErr.Error() != missingErr.Error() {
			t.Fatalf("denials differ:\nforeign: %v\nmissing: %v", foreignErr, missingErr)
		}
	})

	t.Run("Should deny a foreign session whose project folder is gone like a missing one", func(t *testing.T) {
		t.Parallel()
		sessions := sessionsFor(workspaceaccess.ModeDenyAll)
		workspaces := nativeTestWorkspaceService(t)
		resolve := workspaces.ResolveFn
		workspaces.ResolveFn = func(ctx context.Context, ref string) (workspacepkg.ResolvedWorkspace, error) {
			if ref == foreignWorkspace {
				return workspacepkg.ResolvedWorkspace{}, workspacepkg.ErrWorkspaceRootMissing
			}
			return resolve(ctx, ref)
		}
		binder := newNativeWorkspaceInputBinder(workspaces, sessions, newPolicy(t, sessions), nil)
		historyDescriptor := statusDescriptor
		historyDescriptor.ID = toolspkg.ToolIDSessionHistory
		call := func(input string) error {
			bound, err := binder.BindCallInput(t.Context(), actorScope, historyDescriptor, json.RawMessage(input))
			if err != nil {
				return err
			}
			return binder.AuthorizeCallInput(t.Context(), actorScope, historyDescriptor, toolspkg.CallRequest{
				ToolID: historyDescriptor.ID, SessionID: actorScope.SessionID, Input: bound,
			})
		}
		foreignErr, missingErr := call(`{"session_id":"sess-foreign"}`), call(`{"session_id":"sess-missing"}`)
		requireToolReason(t, foreignErr, toolspkg.ErrToolDenied, toolspkg.ReasonWorkspaceAccessDenied)
		if missingErr == nil || foreignErr.Error() != missingErr.Error() {
			t.Fatalf("denials differ:\nforeign: %v\nmissing: %v", foreignErr, missingErr)
		}
	})

	t.Run("Should prompt approve-reads identically for foreign and missing sessions", func(t *testing.T) {
		t.Parallel()
		sessions := sessionsFor(workspaceaccess.ModeApproveReads)
		requester := selectedPermissionRequester(workspaceAccessRejectOnceID)
		binder := newNativeWorkspaceInputBinder(
			nativeTestWorkspaceService(t),
			sessions,
			newPolicy(t, sessions),
			newWorkspaceAccessPromptBridge(
				func() sessionPermissionRequester { return requester },
				time.Second,
				newWorkspaceAccessConsentCache(),
			),
		)
		_, foreignErr := authorize(t, binder, `{"session_id":"sess-foreign"}`)
		_, missingErr := authorize(t, binder, `{"session_id":"sess-foreign-missing"}`)
		requireToolReason(t, foreignErr, toolspkg.ErrToolDenied, toolspkg.ReasonWorkspaceAccessDenied)
		if foreignErr.Error() != missingErr.Error() {
			t.Fatalf("rejections differ:\nforeign: %v\nmissing: %v", foreignErr, missingErr)
		}
		if len(requester.requests) != 2 {
			t.Fatalf("permission requests = %d, want one prompt per call", len(requester.requests))
		}
		for _, request := range requester.requests {
			title := *request.ToolCall.Title
			if !strings.HasPrefix(title, "Read session sess-foreign") || strings.Contains(title, foreignWorkspace) {
				t.Fatalf("prompt title = %q, want a session label without its owner", title)
			}
		}

		renameDescriptor := statusDescriptor
		renameDescriptor.ID = toolspkg.ToolIDSessionRename
		renameDescriptor.ReadOnly = false
		bound, err := binder.BindCallInput(
			t.Context(), actorScope, renameDescriptor, json.RawMessage(`{"session_id":"sess-sibling"}`),
		)
		if err != nil {
			t.Fatal(err)
		}
		renameErr := binder.AuthorizeCallInput(t.Context(), actorScope, renameDescriptor, toolspkg.CallRequest{
			ToolID: renameDescriptor.ID, SessionID: actorScope.SessionID, Input: bound,
		})
		requireToolReason(t, renameErr, toolspkg.ErrToolDenied, toolspkg.ReasonWorkspaceAccessDenied)
		if len(requester.requests) != 3 {
			t.Fatalf("permission requests = %d, want the write prompted too", len(requester.requests))
		}
		if title := *requester.requests[2].ToolCall.Title; !strings.HasPrefix(title, "Change session sess-sibling") {
			t.Fatalf("write prompt title = %q, want the change named, not a read", title)
		}
	})

	t.Run("Should redact a seeded secret only when the read crosses the caller scope", func(t *testing.T) {
		t.Parallel()
		sessions := sessionsFor(workspaceaccess.ModeApproveAll)
		registry := newDaemonNativeRegistryWithPolicyResolverAndWorkspaceAccess(
			t,
			&daemonNativeToolsDeps{
				Sessions: sessions, Workspaces: nativeTestWorkspaceService(t),
				Profiles: nativeProfileReaderStub{profiles: []profilepkg.WithCounts{
					{ID: "p-x", Name: "open-design", State: profilepkg.StateActive},
					{ID: "p-y", Name: "default", State: profilepkg.StateActive},
				}},
			},
			toolspkg.NewStaticPolicyInputResolver(nativeApproveAllPolicyInputs()),
			newPolicy(t, sessions),
		)
		read := func(sessionID string) string {
			result, err := registry.Call(t.Context(), toolspkg.Scope{SessionID: "sess-actor"}, toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDSessionStatus,
				Input:  json.RawMessage(`{"session_id":"` + sessionID + `"}`),
			})
			if err != nil {
				t.Fatalf("Registry.Call(session_status %s) error = %v", sessionID, err)
			}
			return string(result.Structured)
		}
		if foreign := read("sess-foreign"); strings.Contains(foreign, seededSecret) ||
			!strings.Contains(foreign, `"id":"sess-foreign"`) {
			t.Fatalf("foreign status = %s, want the session with its secret redacted", foreign)
		}
		if own := read("sess-own"); !strings.Contains(own, seededSecret) {
			t.Fatalf("own status = %s, want own-scope reads unchanged", own)
		}
	})
}

type nativeSessionTargetAuditStub struct{}

func (nativeSessionTargetAuditStub) EmitWorkspaceAccess(context.Context, workspaceaccess.AccessRecord) error {
	return nil
}

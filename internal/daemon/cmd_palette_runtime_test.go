package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/cmdpalette"
	"github.com/compozy/compozy/internal/testutil"
	toolspkg "github.com/compozy/compozy/internal/tools"
	"github.com/compozy/compozy/internal/windowmanager"
)

func TestCmdPaletteActionExecutor(t *testing.T) {
	t.Parallel()

	t.Run("Should dispatch an immediate tool with stable correlation and merged arguments", func(t *testing.T) {
		t.Parallel()
		registry := &recordingCmdPaletteToolRegistry{
			result: toolspkg.ToolResult{Structured: json.RawMessage(`{"note_id":"note-a"}`)},
		}
		executor := &cmdPaletteActionExecutor{
			tools: registry, now: time.Now, approvalTTL: time.Minute,
		}
		result, err := executor.ExecuteAction(t.Context(), cmdPaletteToolExecutionRequest())
		if err != nil {
			t.Fatalf("ExecuteAction() error = %v", err)
		}
		if string(result.Result) != `{"note_id":"note-a"}` {
			t.Fatalf("ExecuteAction() result = %s", result.Result)
		}
		if registry.call.ToolCallID != "invocation-a" || registry.call.CorrelationID != "invocation-a" ||
			registry.scope.ProfileID != string(testCmdPaletteProfileLens.ID) ||
			registry.scope.WorkspaceID != "workspace-a" || registry.scope.SessionID != "cmd-palette:invocation-a" {
			t.Fatalf("tool dispatch = scope %#v call %#v", registry.scope, registry.call)
		}
		var input map[string]any
		if err := json.Unmarshal(registry.call.Input, &input); err != nil {
			t.Fatalf("json.Unmarshal(input) error = %v", err)
		}
		if input["fixed"] != "bound" || input["title"] != "Standup" {
			t.Fatalf("tool input = %#v, want bound and supplied args", input)
		}
	})

	t.Run("Should return a pending ticket whose completion is owned by the coordinator", func(t *testing.T) {
		t.Parallel()
		completion := make(chan struct{})
		coordinator := &recordingCmdPaletteApprovalCoordinator{ticket: toolspkg.ApprovalTicket{
			ApprovalID: "apr_test", InvocationID: "invocation-a", Completion: completion,
		}}
		registry := &recordingCmdPaletteToolRegistry{approvalRequired: true}
		now := time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC)
		executor := &cmdPaletteActionExecutor{
			tools: registry, approvals: coordinator, now: func() time.Time { return now },
			approvalTTL: 2 * time.Minute,
		}
		result, err := executor.ExecuteAction(t.Context(), cmdPaletteToolExecutionRequest())
		if err != nil {
			t.Fatalf("ExecuteAction() error = %v", err)
		}
		if result.ApprovalID != "apr_test" || result.Completion != completion {
			t.Fatalf("ExecuteAction() = %#v, want coordinator ticket", result)
		}
		if coordinator.request.InvocationID != "invocation-a" ||
			coordinator.request.ProfileID != string(testCmdPaletteProfileLens.ID) ||
			coordinator.request.Target.Kind != toolspkg.ApprovalTargetTool ||
			coordinator.request.Target.ToolID != "compozy__notes_capture" ||
			!coordinator.request.ExpiresAt.Equal(now.Add(2*time.Minute)) {
			t.Fatalf("Begin() request = %#v", coordinator.request)
		}
		if registry.call.ToolID != "" {
			t.Fatalf("tool call happened before approval: %#v", registry.call)
		}
	})

	t.Run("Should resume an approved tool through one-use approval authorization", func(t *testing.T) {
		t.Parallel()
		const approvalOwnerProfileID = "01ARZ3NDEKTSV4RRFFQ69G5FAV"
		registry := &recordingCmdPaletteToolRegistry{
			approvalRequired: true,
			result:           toolspkg.ToolResult{Structured: json.RawMessage(`{"ok":true}`)},
		}
		issuer := &recordingCmdPaletteApprovalIssuer{}
		executor := &cmdPaletteActionExecutor{
			tools: registry, approvalTokens: issuer, now: time.Now, approvalTTL: time.Minute,
		}
		target, err := json.Marshal(cmdPaletteDeferredTarget{
			Action: cmdpalette.Action{
				Kind: cmdpalette.ActionKindTool, Tool: "compozy__notes_capture",
				Args: map[string]any{"fixed": "bound"},
			},
		})
		if err != nil {
			t.Fatalf("json.Marshal(target) error = %v", err)
		}
		result, err := executor.DispatchApproval(t.Context(), toolspkg.ApprovalStatus{
			ApprovalID: "apr_test", ProfileID: approvalOwnerProfileID,
			WorkspaceID: "workspace-a", InvocationID: "invocation-a",
			CommandID: "notes.capture", Target: toolspkg.ApprovalTarget{
				Kind: toolspkg.ApprovalTargetTool, ToolID: "compozy__notes_capture", Payload: target,
			},
			Args: json.RawMessage(`{"title":"Standup"}`), ApprovalStatus: toolspkg.ApprovalApproved,
		})
		if err != nil {
			t.Fatalf("DispatchApproval() error = %v", err)
		}
		if string(result) != `{"ok":true}` || issuer.request.SessionID != "cmd-palette:invocation-a" ||
			registry.call.ApprovalToken != "approval-token" || registry.getCalls != 1 ||
			registry.scope.ProfileID != approvalOwnerProfileID || issuer.scope.ProfileID != approvalOwnerProfileID {
			t.Fatalf("resumed dispatch = result %s issuer %#v call %#v", result, issuer.request, registry.call)
		}
		var approvedInput map[string]any
		if err := json.Unmarshal(issuer.request.Input, &approvedInput); err != nil {
			t.Fatalf("json.Unmarshal(approval input) error = %v", err)
		}
		if approvedInput["fixed"] != "bound" || approvedInput["title"] != "Standup" {
			t.Fatalf("approval input = %#v, want the exact dispatched arguments", approvedInput)
		}
	})
}

func TestCmdPaletteClientDirectory(t *testing.T) {
	t.Parallel()

	t.Run(
		"Should project split availability from the current context instead of the retired mirror",
		func(t *testing.T) {
			t.Parallel()
			for _, inSplit := range []bool{false, true} {
				context := cmdPaletteContextSnapshot(windowmanager.ClientView{
					PaletteContext: windowmanager.PaletteContext{WindowInSplit: inSplit, WindowStacked: !inSplit},
				})
				if context.Values[cmdpalette.ContextWindowStacked] != inSplit {
					t.Fatalf("split availability=%v, want %t", context.Values, inSplit)
				}
			}
		},
	)

	t.Run("Should return no-attached-shell when the window manager is missing", func(t *testing.T) {
		t.Parallel()
		directory := &cmdPaletteClientDirectory{}
		_, err := directory.Context(t.Context(), testCmdPaletteProfileLens, "workspace-a", "client-a")
		if !errors.Is(err, cmdpalette.ErrNoAttachedShell) {
			t.Fatalf("Context() error = %v, want ErrNoAttachedShell", err)
		}
	})

	t.Run("Should project empty global shortcut statuses when the window manager is missing", func(t *testing.T) {
		t.Parallel()
		directory := &cmdPaletteClientDirectory{}
		statuses, err := directory.GlobalShortcutStatuses(
			t.Context(),
			testCmdPaletteProfileLens,
			"workspace-a",
			"client-a",
		)
		if err != nil {
			t.Fatalf("GlobalShortcutStatuses() error = %v", err)
		}
		if len(statuses) != 0 {
			t.Fatalf("GlobalShortcutStatuses() = %#v, want empty map", statuses)
		}
	})
}

// Invariant: palette clients are registered per profile, so a profile lens reaches only its
// own profile's clients in a shared workspace partition (#1196); only the aggregate widens reads.
func TestCmdPaletteClientDirectoryProfileIsolation(t *testing.T) {
	t.Parallel()

	const (
		profileX = "01JQ00000000000000000000X1"
		profileY = "01JQ00000000000000000000Y1"
		clientX  = windowmanager.ClientID("client:x")
		clientY  = windowmanager.ClientID("client:y")
	)
	lensX := cmdpalette.ScopedProfileLens(profileX, "x")
	lensY := cmdpalette.ScopedProfileLens(profileY, "y")

	setup := func(t *testing.T) (*cmdPaletteClientDirectory, *windowManagerRegistry, cmdpalette.WorkspaceID, string) {
		t.Helper()
		fixture := newDaemonWindowManagerFixture(t)
		ctx := testutil.Context(t)
		workspaceID := windowmanager.WorkspaceID(fixture.workspace.ID)
		var tokenX string
		for _, attach := range []struct {
			profile string
			client  windowmanager.ClientID
		}{{profileX, clientX}, {profileY, clientY}} {
			view, err := fixture.registry.ClaimClient(ctx, attach.profile, windowmanager.ClientRegistration{
				WorkspaceID: workspaceID, ClientID: attach.client,
			})
			if err != nil {
				t.Fatalf("ClaimClient(%s) error = %v", attach.profile, err)
			}
			if attach.client == clientX {
				tokenX = view.AttachmentToken
			}
			manager, err := fixture.registry.For(attach.profile)
			if err != nil {
				t.Fatalf("For(%s) error = %v", attach.profile, err)
			}
			connection, err := manager.AttachClientCommands(ctx, workspaceID, attach.client)
			if err != nil {
				t.Fatalf("AttachClientCommands(%s) error = %v", attach.client, err)
			}
			t.Cleanup(func() {
				if err := connection.Close(); err != nil {
					t.Errorf("ClientCommandConnection.Close() error = %v", err)
				}
			})
		}
		if tokenX == "" {
			t.Fatal("ClaimClient(X) returned no attachment token")
		}
		return &cmdPaletteClientDirectory{windowManagers: fixture.registry},
			fixture.registry, cmdpalette.WorkspaceID(workspaceID), tokenX
	}

	t.Run("Should list only the invoking profile's clients unless the aggregate is requested", func(t *testing.T) {
		t.Parallel()
		directory, _, workspaceID, _ := setup(t)
		ctx := testutil.Context(t)
		for _, tc := range []struct {
			lens cmdpalette.ProfileLens
			want []cmdpalette.ClientID
		}{
			{lens: lensY, want: []cmdpalette.ClientID{cmdpalette.ClientID(clientY)}},
			{lens: lensX, want: []cmdpalette.ClientID{cmdpalette.ClientID(clientX)}},
			{
				lens: cmdpalette.AggregateProfileLens(),
				want: []cmdpalette.ClientID{cmdpalette.ClientID(clientX), cmdpalette.ClientID(clientY)},
			},
		} {
			for _, list := range []struct {
				name string
				call func(context.Context, cmdpalette.ProfileLens, cmdpalette.WorkspaceID) ([]cmdpalette.Client, error)
			}{
				{name: "AttachedClients", call: directory.AttachedClients},
				{name: "Clients", call: directory.Clients},
			} {
				clients, err := list.call(ctx, tc.lens, workspaceID)
				if err != nil {
					t.Fatalf("%s(%s) error = %v", list.name, tc.lens.ID, err)
				}
				got := make([]cmdpalette.ClientID, 0, len(clients))
				for _, client := range clients {
					got = append(got, client.ID)
				}
				slices.Sort(got)
				if !slices.Equal(got, tc.want) {
					t.Fatalf("%s(%s) = %v, want %v", list.name, tc.lens.ID, got, tc.want)
				}
			}
		}
	})

	t.Run("Should refuse another profile's client for context, shortcuts, and authorization", func(t *testing.T) {
		t.Parallel()
		directory, _, workspaceID, tokenX := setup(t)
		ctx := testutil.Context(t)
		if _, err := directory.Context(ctx, lensY, workspaceID, cmdpalette.ClientID(clientX)); !errors.Is(
			err, cmdpalette.ErrNoAttachedShell,
		) {
			t.Fatalf("Context(Y, X client) error = %v, want ErrNoAttachedShell", err)
		}
		if _, err := directory.GlobalShortcutStatuses(
			ctx, lensY, workspaceID, cmdpalette.ClientID(clientX),
		); !errors.Is(err, cmdpalette.ErrNoAttachedShell) {
			t.Fatalf("GlobalShortcutStatuses(Y, X client) error = %v, want ErrNoAttachedShell", err)
		}
		if err := directory.Authorize(
			ctx, lensY, workspaceID, cmdpalette.ClientID(clientX), tokenX,
		); !errors.Is(err, cmdpalette.ErrClientUnauthorized) {
			t.Fatalf("Authorize(Y, X client) error = %v, want ErrClientUnauthorized", err)
		}
		if err := directory.Authorize(ctx, lensX, workspaceID, cmdpalette.ClientID(clientX), tokenX); err != nil {
			t.Fatalf("Authorize(X, X client) error = %v", err)
		}
	})

	t.Run("Should not dispatch a Y invocation to the X client", func(t *testing.T) {
		t.Parallel()
		_, registry, workspaceID, _ := setup(t)
		executor := &cmdPaletteActionExecutor{windowManagers: registry, now: time.Now, approvalTTL: time.Minute}
		_, err := executor.ExecuteAction(testutil.Context(t), cmdpalette.ExecutionRequest{
			ProfileLens: lensY, WorkspaceID: workspaceID, InvocationID: "invocation-y",
			ClientID: cmdpalette.ClientID(clientX),
			Descriptor: cmdpalette.Descriptor{
				ID:     "window.close",
				Action: cmdpalette.Action{Kind: cmdpalette.ActionKindClientOp, Op: "window.close"},
			},
		})
		if !errors.Is(err, cmdpalette.ErrNoAttachedShell) {
			t.Fatalf("ExecuteAction(Y, X client) error = %v, want ErrNoAttachedShell", err)
		}
	})
}

func cmdPaletteToolExecutionRequest() cmdpalette.ExecutionRequest {
	return cmdpalette.ExecutionRequest{
		ProfileLens: testCmdPaletteProfileLens,
		WorkspaceID: "workspace-a", InvocationID: "invocation-a",
		Descriptor: cmdpalette.Descriptor{
			ID: "notes.capture",
			Action: cmdpalette.Action{
				Kind: cmdpalette.ActionKindTool, Tool: "compozy__notes_capture",
				Args: map[string]any{"fixed": "bound"},
			},
		},
		Args: map[string]any{"title": "Standup"},
	}
}

type recordingCmdPaletteToolRegistry struct {
	approvalRequired bool
	getCalls         int
	result           toolspkg.ToolResult
	scope            toolspkg.Scope
	call             toolspkg.CallRequest
}

func (r *recordingCmdPaletteToolRegistry) List(
	context.Context,
	toolspkg.Scope,
) ([]toolspkg.ToolView, error) {
	return nil, errors.New("unexpected List call")
}

func (r *recordingCmdPaletteToolRegistry) Search(
	context.Context,
	toolspkg.Scope,
	toolspkg.SearchQuery,
) ([]toolspkg.ToolView, error) {
	return nil, errors.New("unexpected Search call")
}

func (r *recordingCmdPaletteToolRegistry) Get(
	_ context.Context,
	_ toolspkg.Scope,
	_ toolspkg.ToolID,
) (toolspkg.ToolView, error) {
	r.getCalls++
	return toolspkg.ToolView{Decision: toolspkg.EffectiveToolDecision{
		ApprovalRequired: r.approvalRequired,
	}}, nil
}

func (r *recordingCmdPaletteToolRegistry) Call(
	_ context.Context,
	scope toolspkg.Scope,
	request toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	r.scope = scope
	r.call = request
	return r.result, nil
}

type recordingCmdPaletteApprovalCoordinator struct {
	ticket  toolspkg.ApprovalTicket
	request toolspkg.ApprovalRequest
}

func (c *recordingCmdPaletteApprovalCoordinator) Begin(
	_ context.Context,
	request toolspkg.ApprovalRequest,
) (toolspkg.ApprovalTicket, error) {
	c.request = request
	return c.ticket, nil
}

func (c *recordingCmdPaletteApprovalCoordinator) Resolve(
	context.Context,
	string,
	toolspkg.ApprovalOutcome,
) error {
	return errors.New("unexpected Resolve call")
}

func (c *recordingCmdPaletteApprovalCoordinator) Status(
	context.Context,
	string,
) (toolspkg.ApprovalStatus, error) {
	return toolspkg.ApprovalStatus{}, errors.New("unexpected Status call")
}

func (c *recordingCmdPaletteApprovalCoordinator) Cancel(context.Context, string) error {
	return errors.New("unexpected Cancel call")
}

func (c *recordingCmdPaletteApprovalCoordinator) Recover(context.Context) error { return nil }
func (c *recordingCmdPaletteApprovalCoordinator) Close() error                  { return nil }

type recordingCmdPaletteApprovalIssuer struct {
	scope   toolspkg.Scope
	request toolspkg.ApprovalTokenRequest
}

func (i *recordingCmdPaletteApprovalIssuer) CreateToolApproval(
	_ context.Context,
	scope toolspkg.Scope,
	request toolspkg.ApprovalTokenRequest,
) (toolspkg.ApprovalTokenGrant, error) {
	i.scope = scope
	i.request = request
	return toolspkg.ApprovalTokenGrant{ApprovalToken: "approval-token"}, nil
}

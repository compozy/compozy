package daemon

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	acpsdk "github.com/coder/acp-go-sdk"
	"github.com/compozy/agh/internal/acp"
	toolspkg "github.com/compozy/agh/internal/tools"
)

func TestToolApprovalBridgeDeterministicErrors(t *testing.T) {
	t.Parallel()

	view := toolApprovalTestView()

	t.Run("Should return approval_unreachable without a permission channel", func(t *testing.T) {
		t.Parallel()

		bridge := newToolApprovalBridge(nil, time.Second, nil, nil, nil)
		err := bridge.RequestToolApproval(
			t.Context(),
			toolspkg.Scope{SessionID: "sess-1"},
			toolspkg.CallRequest{ToolID: view.Descriptor.ID, Input: []byte(`{}`)},
			&view,
		)
		requireToolApprovalReason(t, err, toolspkg.ReasonApprovalUnreachable)
	})

	t.Run("Should return approval_timed_out when ACP permission request exceeds timeout", func(t *testing.T) {
		t.Parallel()

		requester := &recordingPermissionRequester{
			fn: func(ctx context.Context, _ string, _ acp.RequestPermissionRequest) (acp.RequestPermissionResponse, error) {
				<-ctx.Done()
				return acp.RequestPermissionResponse{}, ctx.Err()
			},
		}
		bridge := newToolApprovalBridge(
			func() sessionPermissionRequester { return requester },
			time.Nanosecond,
			nil,
			nil,
			nil,
		)
		err := bridge.RequestToolApproval(
			t.Context(),
			toolspkg.Scope{SessionID: "sess-1"},
			toolspkg.CallRequest{ToolID: view.Descriptor.ID, Input: []byte(`{}`)},
			&view,
		)
		requireToolApprovalReason(t, err, toolspkg.ReasonApprovalTimedOut)
	})

	t.Run("Should return approval_canceled when caller context is canceled", func(t *testing.T) {
		t.Parallel()

		requester := &recordingPermissionRequester{
			fn: func(ctx context.Context, _ string, _ acp.RequestPermissionRequest) (acp.RequestPermissionResponse, error) {
				return acp.RequestPermissionResponse{}, ctx.Err()
			},
		}
		bridge := newToolApprovalBridge(
			func() sessionPermissionRequester { return requester },
			time.Second,
			nil,
			nil,
			nil,
		)
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		err := bridge.RequestToolApproval(
			ctx,
			toolspkg.Scope{SessionID: "sess-1"},
			toolspkg.CallRequest{ToolID: view.Descriptor.ID, Input: []byte(`{}`)},
			&view,
		)
		requireToolApprovalReason(t, err, toolspkg.ReasonApprovalCanceled)
	})

	t.Run("Should return approval_canceled when ACP returns canceled outcome", func(t *testing.T) {
		t.Parallel()

		requester := &recordingPermissionRequester{
			response: acp.RequestPermissionResponse{Outcome: acpsdk.NewRequestPermissionOutcomeCancelled()},
		}
		bridge := newToolApprovalBridge(
			func() sessionPermissionRequester { return requester },
			time.Second,
			nil,
			nil,
			nil,
		)
		err := bridge.RequestToolApproval(
			t.Context(),
			toolspkg.Scope{SessionID: "sess-1"},
			toolspkg.CallRequest{ToolID: view.Descriptor.ID, Input: []byte(`{}`)},
			&view,
		)
		requireToolApprovalReason(t, err, toolspkg.ReasonApprovalCanceled)
	})
}

func TestToolApprovalBridgeRoutesAllowAndRejectOutcomes(t *testing.T) {
	t.Parallel()

	view := toolApprovalTestView()

	t.Run("Should allow selected allow option", func(t *testing.T) {
		t.Parallel()

		requester := &recordingPermissionRequester{
			response: acp.RequestPermissionResponse{
				Outcome: acpsdk.NewRequestPermissionOutcomeSelected(toolApprovalAllowOnceID),
			},
		}
		bridge := newToolApprovalBridge(
			func() sessionPermissionRequester { return requester },
			time.Second,
			nil,
			nil,
			nil,
		)
		err := bridge.RequestToolApproval(
			t.Context(),
			toolspkg.Scope{SessionID: "sess-1"},
			toolspkg.CallRequest{
				ToolID:      view.Descriptor.ID,
				ToolCallID:  "call-1",
				SessionID:   "sess-1",
				WorkspaceID: "ws-1",
				AgentName:   "codex",
				Input:       []byte(`{"message":"hello"}`),
			},
			&view,
		)
		if err != nil {
			t.Fatalf("RequestToolApproval() error = %v, want nil", err)
		}
		request := requester.lastRequest(t)
		if request.ToolCall.ToolCallId != "call-1" || request.SessionId != "sess-1" {
			t.Fatalf("permission request = %#v, want hosted call context", request)
		}
		if got, want := len(request.Options), 4; got != want {
			t.Fatalf("permission options = %#v, want %d options", request.Options, want)
		}
	})

	t.Run("Should reject selected reject option", func(t *testing.T) {
		t.Parallel()

		requester := &recordingPermissionRequester{
			response: acp.RequestPermissionResponse{
				Outcome: acpsdk.NewRequestPermissionOutcomeSelected(toolApprovalRejectOnceID),
			},
		}
		bridge := newToolApprovalBridge(
			func() sessionPermissionRequester { return requester },
			time.Second,
			nil,
			nil,
			nil,
		)
		err := bridge.RequestToolApproval(
			t.Context(),
			toolspkg.Scope{SessionID: "sess-1"},
			toolspkg.CallRequest{ToolID: view.Descriptor.ID, Input: []byte(`{}`)},
			&view,
		)
		requireToolApprovalReason(t, err, toolspkg.ReasonApprovalRequired)
	})
}

func TestToolApprovalBridgePersistsDurableOutcomes(t *testing.T) {
	t.Parallel()

	view := toolApprovalTestView()

	t.Run("Should remember allow always and skip the next prompt", func(t *testing.T) {
		t.Parallel()

		requester := selectedPermissionRequester(toolApprovalAllowAlwaysID)
		grants := &recordingApprovalGrantStore{}
		bridge := newToolApprovalBridge(
			func() sessionPermissionRequester { return requester },
			time.Second,
			nil,
			grants,
			nil,
		)
		call := toolApprovalTestCall(view.Descriptor.ID, "ws-1")
		for attempt := range 2 {
			if err := bridge.RequestToolApproval(t.Context(), toolspkg.Scope{}, call, &view); err != nil {
				t.Fatalf("RequestToolApproval(%d) error = %v, want nil", attempt, err)
			}
		}
		if got := len(requester.requests); got != 1 {
			t.Fatalf("permission requests = %d, want 1", got)
		}
		if got := len(grants.grants); got != 1 || grants.grants[0].Decision != toolspkg.ApprovalGrantAllow {
			t.Fatalf("durable grants = %#v, want one allow", grants.grants)
		}
		if grants.grants[0].WorkspaceID != "ws-1" || grants.grants[0].AgentName != "codex" ||
			grants.grants[0].InputDigest == "" {
			t.Fatalf("durable grant key = %#v, want exact prompt context", grants.grants[0].ApprovalGrantKey)
		}
	})

	t.Run("Should remember reject always and auto deny the next call", func(t *testing.T) {
		t.Parallel()

		requester := selectedPermissionRequester(toolApprovalRejectAlwaysID)
		grants := &recordingApprovalGrantStore{}
		bridge := newToolApprovalBridge(
			func() sessionPermissionRequester { return requester },
			time.Second,
			nil,
			grants,
			nil,
		)
		call := toolApprovalTestCall(view.Descriptor.ID, "ws-1")
		for attempt := range 2 {
			err := bridge.RequestToolApproval(t.Context(), toolspkg.Scope{}, call, &view)
			requireToolApprovalReason(t, err, toolspkg.ReasonApprovalRequired)
			if len(requester.requests) != 1 {
				t.Fatalf("permission requests after attempt %d = %d, want 1", attempt, len(requester.requests))
			}
		}
		if got := len(grants.grants); got != 1 || grants.grants[0].Decision != toolspkg.ApprovalGrantReject {
			t.Fatalf("durable grants = %#v, want one reject", grants.grants)
		}
	})

	t.Run("Should keep allow once one shot and prompt again", func(t *testing.T) {
		t.Parallel()

		requester := selectedPermissionRequester(toolApprovalAllowOnceID)
		grants := &recordingApprovalGrantStore{}
		bridge := newToolApprovalBridge(
			func() sessionPermissionRequester { return requester },
			time.Second,
			nil,
			grants,
			nil,
		)
		call := toolApprovalTestCall(view.Descriptor.ID, "ws-1")
		for attempt := range 2 {
			if err := bridge.RequestToolApproval(t.Context(), toolspkg.Scope{}, call, &view); err != nil {
				t.Fatalf("RequestToolApproval(%d) error = %v, want nil", attempt, err)
			}
		}
		if got := len(requester.requests); got != 2 {
			t.Fatalf("permission requests = %d, want 2", got)
		}
		if len(grants.grants) != 0 {
			t.Fatalf("durable grants = %#v, want none", grants.grants)
		}
	})

	t.Run("Should fail open to the prompt when durable lookup fails", func(t *testing.T) {
		t.Parallel()

		requester := selectedPermissionRequester(toolApprovalAllowOnceID)
		grants := &recordingApprovalGrantStore{lookupErr: errors.New("database unavailable")}
		bridge := newToolApprovalBridge(
			func() sessionPermissionRequester { return requester },
			time.Second,
			nil,
			grants,
			nil,
		)
		if err := bridge.RequestToolApproval(
			t.Context(),
			toolspkg.Scope{},
			toolApprovalTestCall(view.Descriptor.ID, "ws-1"),
			&view,
		); err != nil {
			t.Fatalf("RequestToolApproval() error = %v, want prompt fallback", err)
		}
		if got := len(requester.requests); got != 1 {
			t.Fatalf("permission requests = %d, want 1", got)
		}
	})

	t.Run("Should never reuse a grant across workspaces", func(t *testing.T) {
		t.Parallel()

		requester := selectedPermissionRequester(toolApprovalAllowOnceID)
		grants := &recordingApprovalGrantStore{}
		callA := toolApprovalTestCall(view.Descriptor.ID, "ws-a")
		keyA, err := toolApprovalGrantKey(toolspkg.Scope{}, callA, view.Descriptor.ID)
		if err != nil {
			t.Fatalf("toolApprovalGrantKey() error = %v", err)
		}
		grants.grants = append(grants.grants, materializedApprovalGrant("grant-a", keyA, toolspkg.ApprovalGrantAllow))
		bridge := newToolApprovalBridge(
			func() sessionPermissionRequester { return requester },
			time.Second,
			nil,
			grants,
			nil,
		)
		if err := bridge.RequestToolApproval(
			t.Context(),
			toolspkg.Scope{},
			toolApprovalTestCall(view.Descriptor.ID, "ws-b"),
			&view,
		); err != nil {
			t.Fatalf("RequestToolApproval() error = %v, want workspace B prompt", err)
		}
		if got := len(requester.requests); got != 1 {
			t.Fatalf("permission requests = %d, want 1", got)
		}
	})

	t.Run("Should surface durable write failures as backend failures", func(t *testing.T) {
		t.Parallel()

		requester := selectedPermissionRequester(toolApprovalAllowAlwaysID)
		grants := &recordingApprovalGrantStore{putErr: errors.New("disk full")}
		bridge := newToolApprovalBridge(
			func() sessionPermissionRequester { return requester },
			time.Second,
			nil,
			grants,
			nil,
		)
		err := bridge.RequestToolApproval(
			t.Context(),
			toolspkg.Scope{},
			toolApprovalTestCall(view.Descriptor.ID, "ws-1"),
			&view,
		)
		if !errors.Is(err, toolspkg.ErrToolBackendFailed) {
			t.Fatalf("RequestToolApproval() error = %v, want ErrToolBackendFailed", err)
		}
		var toolErr *toolspkg.ToolError
		if !errors.As(err, &toolErr) || toolErr.Code != toolspkg.ErrorCodeBackendFailed {
			t.Fatalf("RequestToolApproval() error = %#v, want backend failure envelope", err)
		}
	})
}

func requireToolApprovalReason(t *testing.T, err error, want toolspkg.ReasonCode) {
	t.Helper()

	if !errors.Is(err, toolspkg.ErrToolApprovalRequired) {
		t.Fatalf("RequestToolApproval() error = %v, want ErrToolApprovalRequired", err)
	}
	var toolErr *toolspkg.ToolError
	if !errors.As(err, &toolErr) || !slices.Contains(toolErr.ReasonCodes, want) {
		t.Fatalf("approval error = %#v, want reason %q", err, want)
	}
}

func toolApprovalTestView() toolspkg.ToolView {
	return toolspkg.ToolView{
		Descriptor: toolspkg.Descriptor{
			ID:           "agh__approval_probe",
			Backend:      toolspkg.BackendRef{Kind: toolspkg.BackendNativeGo, NativeName: "approval_probe"},
			Description:  "approval probe",
			InputSchema:  []byte(`{"type":"object"}`),
			Source:       toolspkg.SourceRef{Kind: toolspkg.SourceBuiltin, Owner: "daemon"},
			Visibility:   toolspkg.VisibilityModel,
			Risk:         toolspkg.RiskMutating,
			Destructive:  false,
			ReadOnly:     false,
			OpenWorld:    false,
			DisplayTitle: "Approval Probe",
		},
		Decision: toolspkg.EffectiveToolDecision{
			VisibleToSession: true,
			Callable:         true,
			ApprovalRequired: true,
		},
	}
}

func toolApprovalTestCall(toolID toolspkg.ToolID, workspaceID string) toolspkg.CallRequest {
	return toolspkg.CallRequest{
		ToolID:      toolID,
		ToolCallID:  "call-1",
		SessionID:   "sess-1",
		WorkspaceID: workspaceID,
		AgentName:   "codex",
		Input:       []byte(`{"message":"hello"}`),
	}
}

func selectedPermissionRequester(optionID acpsdk.PermissionOptionId) *recordingPermissionRequester {
	return &recordingPermissionRequester{
		response: acp.RequestPermissionResponse{
			Outcome: acpsdk.NewRequestPermissionOutcomeSelected(optionID),
		},
	}
}

type recordingPermissionRequester struct {
	response acp.RequestPermissionResponse
	err      error
	fn       func(context.Context, string, acp.RequestPermissionRequest) (acp.RequestPermissionResponse, error)
	requests []acp.RequestPermissionRequest
}

var _ sessionPermissionRequester = (*recordingPermissionRequester)(nil)

func (r *recordingPermissionRequester) RequestPermission(
	ctx context.Context,
	id string,
	req acp.RequestPermissionRequest,
) (acp.RequestPermissionResponse, error) {
	r.requests = append(r.requests, req)
	if r.fn != nil {
		return r.fn(ctx, id, req)
	}
	return r.response, r.err
}

func (r *recordingPermissionRequester) lastRequest(t *testing.T) acp.RequestPermissionRequest {
	t.Helper()

	if len(r.requests) == 0 {
		t.Fatal("RequestPermission was not invoked")
	}
	return r.requests[len(r.requests)-1]
}

type recordingApprovalGrantStore struct {
	grants    []toolspkg.ApprovalGrant
	lookupErr error
	putErr    error
}

var _ toolspkg.ApprovalGrantStore = (*recordingApprovalGrantStore)(nil)

func (s *recordingApprovalGrantStore) LookupApprovalGrant(
	_ context.Context,
	key toolspkg.ApprovalGrantKey,
) (toolspkg.ApprovalGrant, bool, error) {
	if s.lookupErr != nil {
		return toolspkg.ApprovalGrant{}, false, s.lookupErr
	}
	for _, grant := range s.grants {
		if grant.ApprovalGrantKey == key {
			return grant, true, nil
		}
	}
	return toolspkg.ApprovalGrant{}, false, nil
}

func (s *recordingApprovalGrantStore) PutApprovalGrant(
	_ context.Context,
	grant toolspkg.ApprovalGrant,
) (toolspkg.ApprovalGrant, error) {
	if s.putErr != nil {
		return toolspkg.ApprovalGrant{}, s.putErr
	}
	stored := materializedApprovalGrant("grant-1", grant.ApprovalGrantKey, grant.Decision)
	for index := range s.grants {
		if s.grants[index].ApprovalGrantKey == stored.ApprovalGrantKey {
			s.grants[index] = stored
			return stored, nil
		}
	}
	s.grants = append(s.grants, stored)
	return stored, nil
}

func (s *recordingApprovalGrantStore) ListApprovalGrants(
	_ context.Context,
	workspaceID string,
) ([]toolspkg.ApprovalGrant, error) {
	grants := make([]toolspkg.ApprovalGrant, 0, len(s.grants))
	for _, grant := range s.grants {
		if grant.WorkspaceID == workspaceID {
			grants = append(grants, grant)
		}
	}
	return grants, nil
}

func (s *recordingApprovalGrantStore) RevokeApprovalGrant(
	_ context.Context,
	workspaceID string,
	id string,
) error {
	for index, grant := range s.grants {
		if grant.WorkspaceID == workspaceID && grant.ID == id {
			s.grants = append(s.grants[:index], s.grants[index+1:]...)
			return nil
		}
	}
	return toolspkg.ErrApprovalGrantNotFound
}

func materializedApprovalGrant(
	id string,
	key toolspkg.ApprovalGrantKey,
	decision toolspkg.ApprovalGrantDecision,
) toolspkg.ApprovalGrant {
	now := time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC)
	return toolspkg.ApprovalGrant{
		ID:               id,
		ApprovalGrantKey: key,
		Decision:         decision,
		CreatedAt:        now,
		LastUsedAt:       now,
	}
}

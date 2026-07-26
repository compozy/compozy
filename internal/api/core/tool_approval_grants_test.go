package core

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/compozy/agh/internal/api/contract"
	toolspkg "github.com/compozy/agh/internal/tools"
	workspacepkg "github.com/compozy/agh/internal/workspace"
	"github.com/gin-gonic/gin"
)

func TestToolApprovalGrantHandlers(t *testing.T) {
	t.Parallel()

	t.Run("Should set an agent-wide grant in the resolved workspace", func(t *testing.T) {
		t.Parallel()

		service := &stubToolApprovalGrantService{
			putFn: func(_ context.Context, grant toolspkg.ApprovalGrant) (toolspkg.ApprovalGrant, error) {
				if grant.WorkspaceID != "registry-ws" || grant.AgentName != "codex" || grant.InputDigest != "" {
					t.Fatalf("PutApprovalGrant() key = %#v, want registry-ws/codex agent-wide", grant.ApprovalGrantKey)
				}
				grant.ID = "grant-wide"
				grant.CreatedAt = toolApprovalGrantHandlerFixture().CreatedAt
				grant.LastUsedAt = grant.CreatedAt
				return grant, nil
			},
		}
		engine := newToolApprovalGrantHandlerFixture(service)
		request := httptest.NewRequestWithContext(
			t.Context(),
			http.MethodPut,
			"/tool-approval-grants?workspace_id=alpha",
			strings.NewReader(`{
				"tool_id":"agh__approval_probe",
				"decision":"allow",
				"scope":"agent",
				"agent_name":"codex"
			}`),
		)
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		engine.ServeHTTP(response, request)

		if response.Code != http.StatusOK {
			t.Fatalf("PUT status = %d, want 200; body=%s", response.Code, response.Body.String())
		}
		var payload contract.ToolApprovalGrantResponse
		if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
			t.Fatalf("Unmarshal(PUT body) error = %v", err)
		}
		if payload.Grant.ID != "grant-wide" || payload.Grant.AgentName != "codex" || payload.Grant.InputDigest != "" {
			t.Fatalf("PUT payload = %#v, want stored agent-wide grant", payload)
		}
	})

	t.Run("Should reject an invalid wider scope without calling the store", func(t *testing.T) {
		t.Parallel()

		engine := newToolApprovalGrantHandlerFixture(&stubToolApprovalGrantService{})
		request := httptest.NewRequestWithContext(
			t.Context(),
			http.MethodPut,
			"/tool-approval-grants?workspace_id=alpha",
			strings.NewReader(`{"tool_id":"agh__approval_probe","decision":"allow","scope":"input"}`),
		)
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		engine.ServeHTTP(response, request)

		if response.Code != http.StatusBadRequest || response.Body.Len() == 0 {
			t.Fatalf("PUT invalid status/body = %d/%q, want 400/non-empty", response.Code, response.Body.String())
		}
	})

	t.Run("Should list only the resolved workspace grants", func(t *testing.T) {
		t.Parallel()

		service := &stubToolApprovalGrantService{
			listFn: func(_ context.Context, workspaceID string) ([]toolspkg.ApprovalGrant, error) {
				if workspaceID != "registry-ws" {
					t.Fatalf("ListApprovalGrants workspace_id = %q, want registry-ws", workspaceID)
				}
				return []toolspkg.ApprovalGrant{toolApprovalGrantHandlerFixture()}, nil
			},
		}
		engine := newToolApprovalGrantHandlerFixture(service)
		request := httptest.NewRequestWithContext(
			t.Context(),
			http.MethodGet,
			"/tool-approval-grants?workspace_id=alpha",
			http.NoBody,
		)
		response := httptest.NewRecorder()
		engine.ServeHTTP(response, request)

		if response.Code != http.StatusOK {
			t.Fatalf("GET status = %d, want 200; body=%s", response.Code, response.Body.String())
		}
		var payload contract.ToolApprovalGrantListResponse
		if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
			t.Fatalf("Unmarshal(GET body) error = %v", err)
		}
		if payload.Total != 1 || len(payload.Grants) != 1 || payload.Grants[0].WorkspaceID != "registry-ws" {
			t.Fatalf("GET payload = %#v, want one registry-ws grant", payload)
		}
	})

	t.Run("Should revoke in the resolved workspace and return an empty body", func(t *testing.T) {
		t.Parallel()

		service := &stubToolApprovalGrantService{
			revokeFn: func(_ context.Context, workspaceID, id string) error {
				if workspaceID != "registry-ws" || id != "grant-1" {
					t.Fatalf("RevokeApprovalGrant(%q, %q), want registry-ws/grant-1", workspaceID, id)
				}
				return nil
			},
		}
		engine := newToolApprovalGrantHandlerFixture(service)
		request := httptest.NewRequestWithContext(
			t.Context(),
			http.MethodDelete,
			"/tool-approval-grants/grant-1?workspace_id=alpha",
			http.NoBody,
		)
		response := httptest.NewRecorder()
		engine.ServeHTTP(response, request)

		if response.Code != http.StatusNoContent || response.Body.Len() != 0 {
			t.Fatalf("DELETE status/body = %d/%q, want 204/empty", response.Code, response.Body.String())
		}
	})

	t.Run("Should return the same not found response for any invisible grant id", func(t *testing.T) {
		t.Parallel()

		service := &stubToolApprovalGrantService{
			revokeFn: func(context.Context, string, string) error {
				return toolspkg.ErrApprovalGrantNotFound
			},
		}
		engine := newToolApprovalGrantHandlerFixture(service)
		request := httptest.NewRequestWithContext(
			t.Context(),
			http.MethodDelete,
			"/tool-approval-grants/foreign-grant?workspace_id=alpha",
			http.NoBody,
		)
		response := httptest.NewRecorder()
		engine.ServeHTTP(response, request)

		if response.Code != http.StatusNotFound || response.Body.Len() == 0 {
			t.Fatalf("DELETE invisible status/body = %d/%q, want 404/non-empty", response.Code, response.Body.String())
		}
	})

	t.Run("Should require an explicit workspace query", func(t *testing.T) {
		t.Parallel()

		engine := newToolApprovalGrantHandlerFixture(&stubToolApprovalGrantService{})
		request := httptest.NewRequestWithContext(
			t.Context(),
			http.MethodGet,
			"/tool-approval-grants",
			http.NoBody,
		)
		response := httptest.NewRecorder()
		engine.ServeHTTP(response, request)

		if response.Code != http.StatusBadRequest || response.Body.Len() == 0 {
			t.Fatalf(
				"GET missing workspace status/body = %d/%q, want 400/non-empty",
				response.Code,
				response.Body.String(),
			)
		}
	})
}

func newToolApprovalGrantHandlerFixture(service ToolApprovalGrantService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	resolver := workspaceResolveServiceStub{
		resolve: func(context.Context, string) (workspacepkg.ResolvedWorkspace, error) {
			return workspacepkg.ResolvedWorkspace{
				Workspace:   workspacepkg.Workspace{ID: "registry-ws", Name: "alpha"},
				WorkspaceID: "public-ws",
			}, nil
		},
	}
	handlers := NewBaseHandlers(&BaseHandlerConfig{
		TransportName:  "tool-approval-grants-test",
		ApprovalGrants: service,
		Workspaces:     resolver,
	})
	engine := gin.New()
	engine.PUT("/tool-approval-grants", handlers.SetToolApprovalGrant)
	engine.GET("/tool-approval-grants", handlers.ListToolApprovalGrants)
	engine.DELETE("/tool-approval-grants/:id", handlers.RevokeToolApprovalGrant)
	return engine
}

func toolApprovalGrantHandlerFixture() toolspkg.ApprovalGrant {
	now := time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC)
	return toolspkg.ApprovalGrant{
		ID: "grant-1",
		ApprovalGrantKey: toolspkg.ApprovalGrantKey{
			WorkspaceID: "registry-ws",
			AgentName:   "codex",
			ToolID:      "agh__approval_probe",
			InputDigest: "sha256:abc",
		},
		Decision:   toolspkg.ApprovalGrantAllow,
		CreatedAt:  now,
		LastUsedAt: now,
	}
}

type stubToolApprovalGrantService struct {
	lookupFn func(context.Context, toolspkg.ApprovalGrantKey) (toolspkg.ApprovalGrant, bool, error)
	putFn    func(context.Context, toolspkg.ApprovalGrant) (toolspkg.ApprovalGrant, error)
	listFn   func(context.Context, string) ([]toolspkg.ApprovalGrant, error)
	revokeFn func(context.Context, string, string) error
}

var _ ToolApprovalGrantService = (*stubToolApprovalGrantService)(nil)

func (s *stubToolApprovalGrantService) LookupApprovalGrant(
	ctx context.Context,
	key toolspkg.ApprovalGrantKey,
) (toolspkg.ApprovalGrant, bool, error) {
	if s.lookupFn != nil {
		return s.lookupFn(ctx, key)
	}
	return toolspkg.ApprovalGrant{}, false, nil
}

func (s *stubToolApprovalGrantService) PutApprovalGrant(
	ctx context.Context,
	grant toolspkg.ApprovalGrant,
) (toolspkg.ApprovalGrant, error) {
	if s.putFn != nil {
		return s.putFn(ctx, grant)
	}
	return toolspkg.ApprovalGrant{}, errors.New("unexpected PutApprovalGrant call")
}

func (s *stubToolApprovalGrantService) ListApprovalGrants(
	ctx context.Context,
	workspaceID string,
) ([]toolspkg.ApprovalGrant, error) {
	if s.listFn != nil {
		return s.listFn(ctx, workspaceID)
	}
	return []toolspkg.ApprovalGrant{}, nil
}

func (s *stubToolApprovalGrantService) RevokeApprovalGrant(
	ctx context.Context,
	workspaceID string,
	id string,
) error {
	if s.revokeFn != nil {
		return s.revokeFn(ctx, workspaceID, id)
	}
	return errors.New("unexpected RevokeApprovalGrant call")
}

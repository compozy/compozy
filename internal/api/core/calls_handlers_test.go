package core_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/api/testutil"
	callspkg "github.com/compozy/compozy/internal/calls"
	"github.com/compozy/compozy/internal/contracts"
	"github.com/compozy/compozy/internal/network/participation"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
	"github.com/gin-gonic/gin"
)

type callsServiceStub struct {
	create        func(context.Context, callspkg.CreateInput) (callspkg.CallRecord, error)
	createBatch   func(context.Context, []callspkg.CreateInput) ([]callspkg.BatchOutcome, error)
	list          func(context.Context, callspkg.CallListQuery) (callspkg.CallPage, error)
	get           func(context.Context, callspkg.CallReadQuery, string) (callspkg.CallRecord, error)
	result        func(context.Context, callspkg.CallReadQuery, string) (callspkg.ResultPayload, error)
	await         func(context.Context, callspkg.AwaitInput) (callspkg.AwaitOutcome, error)
	cancel        func(context.Context, string, string, callspkg.Actor) (callspkg.CallRecord, error)
	sendMessage   func(context.Context, callspkg.SendMessageInput) (callspkg.MessageRecord, error)
	publish       func(context.Context, callspkg.PublishInput) (callspkg.PublishReceipt, error)
	message       func(context.Context, callspkg.CallScope, string) (callspkg.MessageRecord, error)
	listMessages  func(context.Context, callspkg.MessageListQuery) (callspkg.MessagePage, error)
	drainSubtree  func(context.Context, string, callspkg.Actor, string) (callspkg.DrainReport, error)
	resolveCaller func(context.Context, callspkg.CallScope, callspkg.Actor) (participation.OwnerRef, error)
}

func (s callsServiceStub) Create(ctx context.Context, input callspkg.CreateInput) (callspkg.CallRecord, error) {
	return s.create(ctx, input)
}

func (s callsServiceStub) CreateBatch(
	ctx context.Context,
	inputs []callspkg.CreateInput,
) ([]callspkg.BatchOutcome, error) {
	return s.createBatch(ctx, inputs)
}

func (callsServiceStub) Return(context.Context, callspkg.ReturnInput) (callspkg.Settlement, error) {
	return callspkg.Settlement{}, nil
}

func (s callsServiceStub) List(ctx context.Context, query callspkg.CallListQuery) (callspkg.CallPage, error) {
	return s.list(ctx, query)
}

func (s callsServiceStub) GetRead(
	ctx context.Context,
	query callspkg.CallReadQuery,
	callID string,
) (callspkg.CallRecord, error) {
	return s.get(ctx, query, callID)
}

func (s callsServiceStub) Result(
	ctx context.Context,
	query callspkg.CallReadQuery,
	callID string,
) (callspkg.ResultPayload, error) {
	return s.result(ctx, query, callID)
}

func (s callsServiceStub) Await(ctx context.Context, input callspkg.AwaitInput) (callspkg.AwaitOutcome, error) {
	return s.await(ctx, input)
}

func (s callsServiceStub) Cancel(
	ctx context.Context,
	callID string,
	reason string,
	actor callspkg.Actor,
) (callspkg.CallRecord, error) {
	return s.cancel(ctx, callID, reason, actor)
}

func (s callsServiceStub) SendMessage(
	ctx context.Context,
	input callspkg.SendMessageInput,
) (callspkg.MessageRecord, error) {
	return s.sendMessage(ctx, input)
}

func (s callsServiceStub) Publish(
	ctx context.Context,
	input callspkg.PublishInput,
) (callspkg.PublishReceipt, error) {
	return s.publish(ctx, input)
}

func (s callsServiceStub) Message(
	ctx context.Context,
	scope callspkg.CallScope,
	messageID string,
) (callspkg.MessageRecord, error) {
	return s.message(ctx, scope, messageID)
}

func (s callsServiceStub) ListMessages(
	ctx context.Context,
	query callspkg.MessageListQuery,
) (callspkg.MessagePage, error) {
	return s.listMessages(ctx, query)
}

func (s callsServiceStub) DrainSubtree(
	ctx context.Context,
	rootSessionID string,
	actor callspkg.Actor,
	reason string,
) (callspkg.DrainReport, error) {
	return s.drainSubtree(ctx, rootSessionID, actor, reason)
}

func (s callsServiceStub) ResolveOperatorCaller(
	ctx context.Context,
	scope callspkg.CallScope,
	actor callspkg.Actor,
) (participation.OwnerRef, error) {
	return s.resolveCaller(ctx, scope, actor)
}

func newCallsHandlerRouter(service core.CallsService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	handlers := &core.BaseHandlers{
		TransportName: "api-core-test",
		Calls:         service,
		Now: func() time.Time {
			return time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
		},
	}
	router := gin.New()
	router.POST("/calls", handlers.CallsCreate)
	router.GET("/calls", handlers.CallsList)
	router.GET("/calls/:call_id", handlers.CallsGet)
	router.GET("/calls/:call_id/result", handlers.CallsResult)
	router.POST("/calls/:call_id/await", handlers.CallsAwait)
	router.POST("/calls/:call_id/cancel", handlers.CallsCancel)
	router.POST("/workspaces/:workspace_id/calls/:call_id/publish", handlers.CallsPublish)
	router.POST("/messages", handlers.CallMessagesCreate)
	router.GET("/messages", handlers.CallMessagesList)
	router.GET("/messages/:message_id", handlers.CallMessagesGet)
	return router
}

func performCallsRequest(t *testing.T, router http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}

func TestCallsHandlers(t *testing.T) {
	t.Parallel()

	t.Run("Should create one call with wire deadline and return the asynchronous receipt", func(t *testing.T) {
		t.Parallel()
		var captured callspkg.CreateInput
		service := callsServiceStub{
			resolveCaller: func(context.Context, callspkg.CallScope, callspkg.Actor) (participation.OwnerRef, error) {
				return participation.OwnerRef{Kind: participation.OwnerKindSession, ID: "operator-session"}, nil
			},
			create: func(_ context.Context, input callspkg.CreateInput) (callspkg.CallRecord, error) {
				captured = input
				return callspkg.CallRecord{CallID: "call-1", ChildSessionID: "child-1", State: callspkg.StateQueued}, nil
			},
		}
		response := performCallsRequest(t, newCallsHandlerRouter(service), http.MethodPost, "/calls", `{
			"target":{"agent":"reviewer"},"prompt":"Review this","deadline_seconds":30
		}`)
		if response.Code != http.StatusCreated {
			t.Fatalf("POST /calls status = %d, body = %s", response.Code, response.Body.String())
		}
		var payload contract.CallCreatePayload
		if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
			t.Fatalf("decode create response: %v", err)
		}
		if payload.CallID != "call-1" || payload.ChildSessionID != "child-1" || payload.State != "queued" {
			t.Fatalf("create payload = %#v", payload)
		}
		wantDeadline := time.Date(2026, 8, 25, 12, 0, 30, 0, time.UTC)
		if captured.Deadline == nil || !captured.Deadline.Equal(wantDeadline) {
			t.Fatalf("Create deadline = %v, want %v", captured.Deadline, wantDeadline)
		}
	})

	t.Run("Should return typed deadline and unknown-agent errors with status and body", func(t *testing.T) {
		t.Parallel()
		service := callsServiceStub{
			resolveCaller: func(context.Context, callspkg.CallScope, callspkg.Actor) (participation.OwnerRef, error) {
				return participation.OwnerRef{Kind: participation.OwnerKindSession, ID: "operator-session"}, nil
			},
			create: func(context.Context, callspkg.CreateInput) (callspkg.CallRecord, error) {
				return callspkg.CallRecord{}, &callspkg.Error{
					Code: callspkg.CodeAgentUnknown, Message: "agent is unavailable",
					Available: []callspkg.AgentRosterEntry{{Name: "reviewer", Description: "Reviews code"}},
				}
			},
		}
		router := newCallsHandlerRouter(service)
		deadline := performCallsRequest(t, router, http.MethodPost, "/calls", `{
			"target":{"agent":"reviewer"},"prompt":"Review","deadline_seconds":"soon"
		}`)
		if deadline.Code != http.StatusUnprocessableEntity || !strings.Contains(deadline.Body.String(), `"code":"call_deadline_invalid"`) {
			t.Fatalf("invalid deadline response = %d %s", deadline.Code, deadline.Body.String())
		}
		unknown := performCallsRequest(t, router, http.MethodPost, "/calls", `{
			"target":{"agent":"missing"},"prompt":"Review"
		}`)
		if unknown.Code != http.StatusNotFound ||
			!strings.Contains(unknown.Body.String(), `"code":"call_agent_unknown"`) ||
			!strings.Contains(unknown.Body.String(), `"name":"reviewer"`) {
			t.Fatalf("unknown agent response = %d %s", unknown.Code, unknown.Body.String())
		}
	})

	t.Run("Should return batch item outcomes under HTTP 200", func(t *testing.T) {
		t.Parallel()
		service := callsServiceStub{
			resolveCaller: func(context.Context, callspkg.CallScope, callspkg.Actor) (participation.OwnerRef, error) {
				return participation.OwnerRef{Kind: participation.OwnerKindSession, ID: "operator-session"}, nil
			},
			createBatch: func(_ context.Context, inputs []callspkg.CreateInput) ([]callspkg.BatchOutcome, error) {
				if len(inputs) != 2 {
					t.Fatalf("CreateBatch inputs = %d, want 2", len(inputs))
				}
				accepted := callspkg.CallRecord{CallID: "call-1", State: callspkg.StateQueued}
				return []callspkg.BatchOutcome{
					{Call: &accepted},
					{Error: &callspkg.Error{Code: callspkg.CodeAgentUnknown, Message: "missing"}},
				}, nil
			},
		}
		response := performCallsRequest(t, newCallsHandlerRouter(service), http.MethodPost, "/calls", `{
			"tasks":[
				{"target":{"agent":"reviewer"},"prompt":"one"},
				{"target":{"agent":"missing"},"prompt":"two"}
			]
		}`)
		if response.Code != http.StatusOK ||
			!strings.Contains(response.Body.String(), `"call_id":"call-1"`) ||
			!strings.Contains(response.Body.String(), `"code":"call_agent_unknown"`) {
			t.Fatalf("batch response = %d %s", response.Code, response.Body.String())
		}
	})

	t.Run("Should preserve pagination await result cancel and publish response contracts", func(t *testing.T) {
		t.Parallel()
		now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
		completed := callspkg.CallRecord{
			CallID: "call-1", ProfileID: store.DefaultProfileID, Scope: callspkg.ScopeGlobal,
			Caller: participation.OwnerRef{Kind: participation.OwnerKindSession, ID: "parent"},
			Actor:  callspkg.Actor{Kind: "human", ID: "operator"}, GovernedRootID: "parent",
			State: callspkg.StateCompleted, ResultBudget: contracts.ByteBudget{MaxBytes: 4096},
			CreatedAt: now, UpdatedAt: now,
		}
		cancelCalls := 0
		service := callsServiceStub{
			list: func(_ context.Context, query callspkg.CallListQuery) (callspkg.CallPage, error) {
				if query.Cursor != "after-0" || query.Limit != 7 || query.ReadScope.ProfileID != store.DefaultProfileID {
					t.Fatalf("List query = %#v", query)
				}
				return callspkg.CallPage{Items: []callspkg.CallRecord{completed}, NextCursor: "after-1"}, nil
			},
			get: func(_ context.Context, _ callspkg.CallReadQuery, callID string) (callspkg.CallRecord, error) {
				if callID != "call-1" {
					t.Fatalf("GetRead call id = %q", callID)
				}
				return completed, nil
			},
			result: func(_ context.Context, _ callspkg.CallReadQuery, callID string) (callspkg.ResultPayload, error) {
				return callspkg.ResultPayload{CallID: callID, Bytes: []byte(`{"score":9}`)}, nil
			},
			await: func(_ context.Context, input callspkg.AwaitInput) (callspkg.AwaitOutcome, error) {
				if len(input.CallIDs) != 1 || input.CallIDs[0] != "call-1" || input.Timeout != 250*time.Millisecond {
					t.Fatalf("Await input = %#v", input)
				}
				return callspkg.AwaitOutcome{
					Settled: []callspkg.CallRecord{completed}, Pending: []string{"call-2"},
					Outcome: "partial", Resume: "resume-1", ClampedTimeout: 250 * time.Millisecond,
				}, nil
			},
			cancel: func(_ context.Context, callID, reason string, _ callspkg.Actor) (callspkg.CallRecord, error) {
				cancelCalls++
				if callID != "call-1" || reason != "stop" {
					t.Fatalf("Cancel = %q %q", callID, reason)
				}
				return callspkg.CallRecord{State: callspkg.StateCanceled}, nil
			},
			publish: func(_ context.Context, input callspkg.PublishInput) (callspkg.PublishReceipt, error) {
				if input.WorkspaceID != "ws-1" || input.Channel != "reviews" || input.ThreadID != "thread-1" {
					t.Fatalf("Publish input = %#v", input)
				}
				return callspkg.PublishReceipt{NetworkMessageID: "network-1", Published: true}, nil
			},
		}
		router := newCallsHandlerRouter(service)
		list := performCallsRequest(t, router, http.MethodGet, "/calls?cursor=after-0&limit=7", "")
		if list.Code != http.StatusOK || !strings.Contains(list.Body.String(), `"next_cursor":"after-1"`) {
			t.Fatalf("list response = %d %s", list.Code, list.Body.String())
		}
		result := performCallsRequest(t, router, http.MethodGet, "/calls/call-1/result", "")
		if result.Code != http.StatusOK || !strings.Contains(result.Body.String(), `"result":{"score":9}`) {
			t.Fatalf("result response = %d %s", result.Code, result.Body.String())
		}
		await := performCallsRequest(t, router, http.MethodPost, "/calls/call-1/await", `{"timeout_ms":250}`)
		if await.Code != http.StatusOK || !strings.Contains(await.Body.String(), `"outcome":"partial"`) ||
			!strings.Contains(await.Body.String(), `"clamped_timeout_ms":250`) {
			t.Fatalf("await response = %d %s", await.Code, await.Body.String())
		}
		for range 2 {
			cancel := performCallsRequest(t, router, http.MethodPost, "/calls/call-1/cancel", `{"reason":"stop"}`)
			if cancel.Code != http.StatusOK || cancel.Body.String() != `{"state":"canceled"}` {
				t.Fatalf("cancel response = %d %s", cancel.Code, cancel.Body.String())
			}
		}
		if cancelCalls != 2 {
			t.Fatalf("Cancel calls = %d, want 2 idempotent requests", cancelCalls)
		}
		publish := performCallsRequest(t, router, http.MethodPost,
			"/workspaces/ws-1/calls/call-1/publish", `{"channel":"reviews","thread_id":"thread-1"}`)
		if publish.Code != http.StatusOK || publish.Body.String() != `{"network_message_id":"network-1","published":true}` {
			t.Fatalf("publish response = %d %s", publish.Code, publish.Body.String())
		}
	})

	t.Run("Should preserve message send list detail and typed errors", func(t *testing.T) {
		t.Parallel()
		now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
		message := callspkg.MessageRecord{
			MessageID: "message-1", ProfileID: store.DefaultProfileID, Scope: callspkg.ScopeGlobal,
			From: callspkg.MessageSender{Kind: "operator", ID: "operator"}, ToSessionID: "child-1",
			Body: "Check this", Delivery: "pending", CreatedAt: now,
		}
		service := callsServiceStub{
			sendMessage: func(_ context.Context, input callspkg.SendMessageInput) (callspkg.MessageRecord, error) {
				if input.To != "child-1" || input.Body != "Check this" {
					t.Fatalf("SendMessage input = %#v", input)
				}
				return message, nil
			},
			listMessages: func(_ context.Context, query callspkg.MessageListQuery) (callspkg.MessagePage, error) {
				if query.SessionID != "child-1" || query.Limit != 5 {
					t.Fatalf("ListMessages query = %#v", query)
				}
				return callspkg.MessagePage{Items: []callspkg.MessageRecord{message}, NextCursor: "next-1"}, nil
			},
			message: func(_ context.Context, _ callspkg.CallScope, id string) (callspkg.MessageRecord, error) {
				if id == "missing" {
					return callspkg.MessageRecord{}, &callspkg.Error{Code: callspkg.CodeMessageNotFound, Message: "missing"}
				}
				return message, nil
			},
		}
		router := newCallsHandlerRouter(service)
		send := performCallsRequest(t, router, http.MethodPost, "/messages", `{
			"to":{"session_id":"child-1"},"text":"Check this"
		}`)
		if send.Code != http.StatusAccepted || send.Body.String() != `{"message_id":"message-1","delivery":"queued"}` {
			t.Fatalf("message send response = %d %s", send.Code, send.Body.String())
		}
		list := performCallsRequest(t, router, http.MethodGet, "/messages?session=child-1&limit=5", "")
		if list.Code != http.StatusOK || !strings.Contains(list.Body.String(), `"next_cursor":"next-1"`) ||
			!strings.Contains(list.Body.String(), `"delivery":"queued"`) {
			t.Fatalf("message list response = %d %s", list.Code, list.Body.String())
		}
		get := performCallsRequest(t, router, http.MethodGet, "/messages/message-1", "")
		if get.Code != http.StatusOK || !strings.Contains(get.Body.String(), `"message_id":"message-1"`) {
			t.Fatalf("message get response = %d %s", get.Code, get.Body.String())
		}
		missing := performCallsRequest(t, router, http.MethodGet, "/messages/missing", "")
		if missing.Code != http.StatusNotFound || !strings.Contains(missing.Body.String(), `"code":"message_not_found"`) {
			t.Fatalf("message error response = %d %s", missing.Code, missing.Body.String())
		}
	})
}

func TestSessionStopSubtreeHandler(t *testing.T) {
	t.Parallel()

	t.Run("Should drain the governed subtree and report preserved results", func(t *testing.T) {
		t.Parallel()
		stopCalled := false
		manager := testutil.StubSessionManager{
			StatusFn: func(context.Context, string) (*session.Info, error) {
				return &session.Info{ID: "root-1", WorkspaceID: "ws-1", ProfileID: store.DefaultProfileID}, nil
			},
			StopWithCauseFn: func(_ context.Context, id string, cause session.StopCause, detail string) error {
				stopCalled = true
				if id != "root-1" || cause != session.CauseUserRequested || detail != "done" {
					t.Fatalf("StopWithCause = %q %v %q", id, cause, detail)
				}
				return nil
			},
		}
		fixture := newHandlerFixture(t, manager, testutil.StubObserver{}, testutil.StubWorkspaceService{
			ResolveFn: func(context.Context, string) (workspacepkg.ResolvedWorkspace, error) {
				return workspacepkg.ResolvedWorkspace{
					Workspace:   workspacepkg.Workspace{ID: "ws-1", Name: "alpha", RootDir: "/workspace"},
					WorkspaceID: "ws-1",
				}, nil
			},
		}, nil, nil)
		fixture.Handlers.Calls = callsServiceStub{
			drainSubtree: func(_ context.Context, root string, actor callspkg.Actor, reason string) (callspkg.DrainReport, error) {
				if root != "root-1" || actor.Kind != "human" || reason != "done" {
					t.Fatalf("DrainSubtree = %q %#v %q", root, actor, reason)
				}
				return callspkg.DrainReport{
					Stopped: []string{"child-1", "child-2"}, CanceledCalls: []string{"call-1"}, PreservedResults: 3,
				}, nil
			},
		}
		response := performCallsRequest(t, fixture.Engine, http.MethodPost,
			"/workspaces/ws-1/sessions/root-1/stop", `{"subtree":true,"reason":"done"}`)
		if response.Code != http.StatusOK ||
			response.Body.String() != `{"stopped_children":2,"closed_calls":1,"preserved_results":3}` {
			t.Fatalf("session stop response = %d %s", response.Code, response.Body.String())
		}
		if !stopCalled {
			t.Fatal("StopWithCause was not called")
		}
	})
}

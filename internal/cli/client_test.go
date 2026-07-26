package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	memcontract "github.com/compozy/agh/internal/memory/contract"

	"github.com/compozy/agh/internal/agentidentity"
	"github.com/compozy/agh/internal/api/contract"
	automationpkg "github.com/compozy/agh/internal/automation"
	bridgepkg "github.com/compozy/agh/internal/bridges"
	diagnosticspkg "github.com/compozy/agh/internal/diagnostics"
	mcppkg "github.com/compozy/agh/internal/mcp"
	taskpkg "github.com/compozy/agh/internal/task"
	toolspkg "github.com/compozy/agh/internal/tools"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestUnixSocketClientSessionPromptShouldDecodeStructuredGoalJSON(t *testing.T) {
	t.Parallel()

	newClient := func(t *testing.T, status int, body string) *unixSocketClient {
		t.Helper()
		transport := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			switch {
			case req.Method == http.MethodGet && req.URL.Path == "/api/sessions/sess-1":
				return newHTTPResponse(
					http.StatusOK,
					`{"session":{"id":"sess-1","workspace_id":"ws-1","state":"active","available_commands":[],"created_at":"2026-07-10T20:00:00Z","updated_at":"2026-07-10T20:00:00Z"}}`,
				), nil
			case req.Method == http.MethodPost &&
				req.URL.Path == "/api/workspaces/ws-1/sessions/sess-1/prompt":
				response := newHTTPResponse(status, body)
				response.Header.Set("Content-Type", "application/json")
				return response, nil
			default:
				t.Fatalf("unexpected request = %s %s", req.Method, req.URL.Path)
				return nil, errors.New("unexpected request")
			}
		})
		client := &unixSocketClient{
			socketPath: "/tmp/agh.sock",
			httpClient: &http.Client{Transport: transport},
		}
		client.streamClient = client.httpClient
		return client
	}

	t.Run("Should decode a successful direct Goal result", func(t *testing.T) {
		t.Parallel()

		client := newClient(
			t,
			http.StatusOK,
			`{"outcome":"status","reason_code":null,"snapshot":null,"replaced_run_id":null}`,
		)
		record, err := client.SendSessionPrompt(
			t.Context(),
			"sess-1",
			SessionPromptRequest{Message: "/goal status"},
		)
		if err != nil {
			t.Fatalf("SendSessionPrompt(Goal status) error = %v", err)
		}
		if record.Goal == nil || record.Goal.Outcome != contract.GoalOutcomeStatus {
			t.Fatalf("SendSessionPrompt(Goal status) = %#v", record)
		}
	})

	t.Run("Should surface one direct Goal result through the streaming JSONL seam", func(t *testing.T) {
		t.Parallel()

		client := newClient(
			t,
			http.StatusAccepted,
			`{"outcome":"started","reason_code":null,"snapshot":null,"replaced_run_id":null}`,
		)
		var events []SSEEvent
		err := client.StreamPromptSession(t.Context(), "sess-1", "/goal ship", func(event SSEEvent) error {
			events = append(events, event)
			return nil
		})
		if err != nil {
			t.Fatalf("StreamPromptSession(Goal start) error = %v", err)
		}
		if len(events) != 1 || events[0].Event != goalResultEventName {
			t.Fatalf("StreamPromptSession(Goal start) events = %#v", events)
		}
		var result contract.GoalCommandResult
		if err := json.Unmarshal(events[0].Data, &result); err != nil {
			t.Fatalf("decode streamed Goal result error = %v", err)
		}
		if result.Outcome != contract.GoalOutcomeStarted {
			t.Fatalf("streamed Goal result = %#v", result)
		}
	})

	t.Run("Should preserve a structured Goal error body", func(t *testing.T) {
		t.Parallel()

		client := newClient(
			t,
			http.StatusConflict,
			`{"outcome":"error","reason_code":"goal_replace_required","snapshot":null,"replaced_run_id":null}`,
		)
		_, err := client.SendSessionPrompt(
			t.Context(),
			"sess-1",
			SessionPromptRequest{Message: "/goal ship"},
		)
		var goalErr *goalCommandAPIError
		if !errors.As(err, &goalErr) {
			t.Fatalf("SendSessionPrompt(Goal conflict) error = %T %[1]v", err)
		}
		if goalErr.statusCode != http.StatusConflict ||
			goalErr.result.ReasonCode == nil ||
			*goalErr.result.ReasonCode != contract.GoalReasonReplaceRequired {
			t.Fatalf("Goal command error = %#v", goalErr)
		}
	})
}

func TestUnixSocketClientAgentDefinitionMethods(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		wantMethod string
		wantPath   string
		wantQuery  string
		wantBody   any
		response   string
		run        func(t *testing.T, client *unixSocketClient)
	}{
		{
			name:       "Should create an agent through the daemon",
			wantMethod: http.MethodPost,
			wantPath:   "/api/agents",
			wantBody: contract.CreateAgentRequest{
				Scope:     contract.AgentCreateScopeWorkspace,
				Workspace: "ws-1",
				Agent: contract.CreateAgentPayload{
					Name: "coder", Provider: "fake", Prompt: "Code.",
				},
			},
			response: `{"agent":{"name":"coder","provider":"fake","origin":"global","definition_digest":"digest-1","prompt":"Code."}}`,
			run: func(t *testing.T, client *unixSocketClient) {
				t.Helper()
				agent, err := client.CreateAgent(t.Context(), contract.CreateAgentRequest{
					Scope:     contract.AgentCreateScopeWorkspace,
					Workspace: "ws-1",
					Agent:     contract.CreateAgentPayload{Name: "coder", Provider: "fake", Prompt: "Code."},
				})
				if err != nil || agent.DefinitionDigest != "digest-1" {
					t.Fatalf("CreateAgent() = %#v, %v", agent, err)
				}
			},
		},
		{
			name:       "Should update an agent with a digest request",
			wantMethod: http.MethodPut,
			wantPath:   "/api/agents/coder",
			wantBody: contract.UpdateAgentRequest{
				Workspace:      "ws-1",
				ExpectedDigest: "digest-1",
				Agent: contract.CreateAgentPayload{
					Name: "coder", Provider: "fake", Prompt: "Review.",
				},
			},
			response: `{"agent":{"name":"coder","provider":"fake","origin":"global","definition_digest":"digest-2","prompt":"Review."}}`,
			run: func(t *testing.T, client *unixSocketClient) {
				t.Helper()
				agent, err := client.UpdateAgent(t.Context(), "coder", contract.UpdateAgentRequest{
					Workspace:      "ws-1",
					ExpectedDigest: "digest-1",
					Agent:          contract.CreateAgentPayload{Name: "coder", Provider: "fake", Prompt: "Review."},
				})
				if err != nil || agent.DefinitionDigest != "digest-2" {
					t.Fatalf("UpdateAgent() = %#v, %v", agent, err)
				}
			},
		},
		{
			name:       "Should delete an agent in workspace scope",
			wantMethod: http.MethodDelete,
			wantPath:   "/api/agents/coder",
			wantQuery:  "workspace=ws-1",
			response:   `{"name":"coder","origin":"workspace","unshadowed_origin":"global"}`,
			run: func(t *testing.T, client *unixSocketClient) {
				t.Helper()
				result, err := client.DeleteAgent(t.Context(), "coder", "ws-1")
				if err != nil || result.UnshadowedOrigin != contract.AgentOriginGlobal {
					t.Fatalf("DeleteAgent() = %#v, %v", result, err)
				}
			},
		},
		{
			name:       "Should duplicate an agent server side",
			wantMethod: http.MethodPost,
			wantPath:   "/api/agents/coder/duplicate",
			wantBody: contract.DuplicateAgentRequest{
				Name:      "reviewer",
				Scope:     contract.AgentCreateScopeWorkspace,
				Workspace: "ws-1",
				Overrides: &contract.DuplicateAgentOverrides{
					Model:  "gpt-5",
					Prompt: "Review.",
				},
			},
			response: `{"agent":{"name":"reviewer","provider":"fake","origin":"global","definition_digest":"digest-copy","prompt":"Review."}}`,
			run: func(t *testing.T, client *unixSocketClient) {
				t.Helper()
				agent, err := client.DuplicateAgent(t.Context(), "coder", contract.DuplicateAgentRequest{
					Name:      "reviewer",
					Scope:     contract.AgentCreateScopeWorkspace,
					Workspace: "ws-1",
					Overrides: &contract.DuplicateAgentOverrides{Model: "gpt-5", Prompt: "Review."},
				})
				if err != nil || agent.Name != "reviewer" {
					t.Fatalf("DuplicateAgent() = %#v, %v", agent, err)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := &unixSocketClient{
				socketPath: "/tmp/agh.sock",
				httpClient: &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
					if req.Method != tt.wantMethod || req.URL.Path != tt.wantPath || req.URL.RawQuery != tt.wantQuery {
						t.Fatalf(
							"request = %s %s?%s, want %s %s?%s",
							req.Method,
							req.URL.Path,
							req.URL.RawQuery,
							tt.wantMethod,
							tt.wantPath,
							tt.wantQuery,
						)
					}
					if tt.wantBody != nil {
						var gotBody any
						if err := json.NewDecoder(req.Body).Decode(&gotBody); err != nil {
							t.Fatalf("decode request body: %v", err)
						}
						wantJSON, err := json.Marshal(tt.wantBody)
						if err != nil {
							t.Fatalf("marshal expected request body: %v", err)
						}
						var wantBody any
						if err := json.Unmarshal(wantJSON, &wantBody); err != nil {
							t.Fatalf("decode expected request body: %v", err)
						}
						if !reflect.DeepEqual(gotBody, wantBody) {
							t.Fatalf("request body = %#v, want %#v", gotBody, wantBody)
						}
					}
					return newHTTPResponse(http.StatusOK, tt.response), nil
				})},
			}
			tt.run(t, client)
		})
	}
}

func TestUnixSocketClientAgentMeSendsIdentityHeaders(t *testing.T) {
	t.Parallel()

	t.Run("Should send identity headers", func(t *testing.T) {
		t.Parallel()

		client := &unixSocketClient{
			socketPath: "/tmp/agh.sock",
			httpClient: &http.Client{
				Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
					if req.Method != http.MethodGet || req.URL.Path != "/api/agent/me" {
						t.Fatalf("request = %s %s, want GET /api/agent/me", req.Method, req.URL.Path)
					}
					if got := req.Header.Get(agentidentity.HeaderSessionID); got != "sess-1" {
						t.Fatalf("%s = %q, want sess-1", agentidentity.HeaderSessionID, got)
					}
					if got := req.Header.Get(agentidentity.HeaderAgent); got != "coder" {
						t.Fatalf("%s = %q, want coder", agentidentity.HeaderAgent, got)
					}
					if got := req.Header.Get(agentidentity.HeaderWorkspaceID); got != "ws-1" {
						t.Fatalf("%s = %q, want ws-1", agentidentity.HeaderWorkspaceID, got)
					}
					return newHTTPResponse(
						http.StatusOK,
						`{"me":{"self":{"session_id":"sess-1","agent_name":"coder","provider":"test"},"workspace":{"id":"ws-1"},"session":{"id":"sess-1","agent_name":"coder","state":"active","created_at":"2026-04-03T12:00:00Z","updated_at":"2026-04-03T12:00:00Z"},"capabilities":[],"channels":[],"active_task_leases":[]}}`,
					), nil
				}),
			},
		}

		me, err := client.AgentMe(context.Background(), agentidentity.Credentials{
			SessionID:   "sess-1",
			AgentName:   "coder",
			WorkspaceID: "ws-1",
		})
		if err != nil {
			t.Fatalf("AgentMe() error = %v", err)
		}
		if me.Self.SessionID != "sess-1" || me.Self.AgentName != "coder" {
			t.Fatalf("AgentMe() = %#v, want validated response", me.Self)
		}
	})
}

func TestUnixSocketClientAgentChannelMethodsSendIdentityHeaders(t *testing.T) {
	t.Parallel()

	credentials := agentidentity.Credentials{
		SessionID:   "sess-1",
		AgentName:   "coder",
		WorkspaceID: "ws-1",
	}
	metadata := contract.CoordinationMessageMetadataPayload{
		TaskID:        "task-1",
		RunID:         "run-1",
		ChannelID:     "builders",
		MessageKind:   contract.CoordinationMessageStatus,
		CorrelationID: "run-1",
	}

	tests := []struct {
		name string
		run  func(t *testing.T, client *unixSocketClient)
	}{
		{
			name: "Should list agent channels",
			run: func(t *testing.T, client *unixSocketClient) {
				t.Helper()

				channels, err := client.AgentChannels(context.Background(), credentials)
				if err != nil {
					t.Fatalf("AgentChannels() error = %v", err)
				}
				if len(channels) != 1 || channels[0].ID != "builders" {
					t.Fatalf("AgentChannels() = %#v, want builders", channels)
				}
			},
		},
		{
			name: "Should receive agent channel messages",
			run: func(t *testing.T, client *unixSocketClient) {
				t.Helper()

				messages, err := client.AgentChannelRecv(
					context.Background(),
					"builders",
					AgentChannelRecvQuery{Wait: true, Limit: 3},
					credentials,
				)
				if err != nil {
					t.Fatalf("AgentChannelRecv() error = %v", err)
				}
				if len(messages) != 1 || messages[0].MessageID != "msg-1" {
					t.Fatalf("AgentChannelRecv() = %#v, want msg-1", messages)
				}
			},
		},
		{
			name: "Should send agent channel messages",
			run: func(t *testing.T, client *unixSocketClient) {
				t.Helper()

				message, err := client.AgentChannelSend(context.Background(), "builders", AgentChannelSendRequest{
					Body:     json.RawMessage(`{"text":"ok"}`),
					Metadata: metadata,
				}, credentials)
				if err != nil {
					t.Fatalf("AgentChannelSend() error = %v", err)
				}
				if message.MessageID != "msg-send" {
					t.Fatalf("AgentChannelSend() = %#v, want msg-send", message)
				}
			},
		},
		{
			name: "Should reply to agent channel messages",
			run: func(t *testing.T, client *unixSocketClient) {
				t.Helper()

				message, err := client.AgentChannelReply(context.Background(), AgentChannelReplyRequest{
					ReplyToMessageID: "msg-1",
					Body:             json.RawMessage(`{"text":"ack"}`),
				}, credentials)
				if err != nil {
					t.Fatalf("AgentChannelReply() error = %v", err)
				}
				if message.MessageID != "msg-reply" {
					t.Fatalf("AgentChannelReply() = %#v, want msg-reply", message)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := &unixSocketClient{
				socketPath: "/tmp/agh.sock",
				httpClient: &http.Client{
					Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
						assertAgentRequestHeaders(t, req, credentials)
						switch {
						case req.Method == http.MethodGet && req.URL.Path == "/api/agent/channels":
							return newHTTPResponse(
								http.StatusOK,
								`{"channels":[{"id":"builders","display_name":"builders","workspace_id":"ws-1","allowed_message_kinds":["status","request","reply","blocker","handoff","result","review_request"]}]}`,
							), nil
						case req.Method == http.MethodGet && req.URL.Path == "/api/agent/channels/builders/recv":
							if req.URL.Query().Get("wait") != "true" || req.URL.Query().Get("limit") != "3" {
								t.Fatalf("recv query = %s, want wait=true&limit=3", req.URL.RawQuery)
							}
							return newHTTPResponse(
								http.StatusOK,
								`{"messages":[{"message_id":"msg-1","channel_id":"builders","from_session_id":"sess-peer","body":{"text":"ok"},"metadata":{"task_id":"task-1","run_id":"run-1","channel_id":"builders","message_kind":"status","correlation_id":"run-1"},"timestamp":"2026-04-03T12:00:00Z"}]}`,
							), nil
						case req.Method == http.MethodPost && req.URL.Path == "/api/agent/channels/builders/send":
							body, err := io.ReadAll(req.Body)
							if err != nil {
								t.Fatalf("io.ReadAll(send body) error = %v", err)
							}
							if !strings.Contains(string(body), `"task_id":"task-1"`) ||
								!strings.Contains(string(body), `"body":{"text":"ok"}`) {
								t.Fatalf("send body = %s, want message and metadata", body)
							}
							return newHTTPResponse(
								http.StatusAccepted,
								`{"message":{"message_id":"msg-send","channel_id":"builders","from_session_id":"sess-1","body":{"text":"ok"},"metadata":{"task_id":"task-1","run_id":"run-1","channel_id":"builders","message_kind":"status","correlation_id":"run-1"},"timestamp":"2026-04-03T12:00:00Z"}}`,
							), nil
						case req.Method == http.MethodPost && req.URL.Path == "/api/agent/channels/reply":
							body, err := io.ReadAll(req.Body)
							if err != nil {
								t.Fatalf("io.ReadAll(reply body) error = %v", err)
							}
							if !strings.Contains(string(body), `"reply_to_message_id":"msg-1"`) {
								t.Fatalf("reply body = %s, want reply_to_message_id", body)
							}
							return newHTTPResponse(
								http.StatusAccepted,
								`{"message":{"message_id":"msg-reply","channel_id":"builders","from_session_id":"sess-1","to_session_id":"sess-peer","body":{"text":"ack"},"metadata":{"task_id":"task-1","run_id":"run-1","channel_id":"builders","message_kind":"reply","correlation_id":"run-1"},"timestamp":"2026-04-03T12:00:00Z"}}`,
							), nil
						default:
							t.Fatalf("unexpected request = %s %s", req.Method, req.URL.Path)
							return nil, nil
						}
					}),
				},
			}
			tt.run(t, client)
		})
	}
}

func TestUnixSocketClientLoopCatalog(t *testing.T) {
	t.Parallel()

	t.Run("Should serialize every bounded catalog query field", func(t *testing.T) {
		t.Parallel()

		client := &unixSocketClient{
			socketPath: "/tmp/agh.sock",
			httpClient: &http.Client{
				Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
					if req.Method != http.MethodGet || req.URL.Path != "/api/workspaces/ws-1/loops" {
						t.Fatalf("request = %s %s, want GET Loop catalog", req.Method, req.URL.Path)
					}
					want := map[string]string{
						"q":        "release plan",
						"kind":     "workspace",
						"category": "delivery",
						"status":   "running",
						"sort":     "name",
						"cursor":   "next/+=?",
						"limit":    "17",
					}
					for name, value := range want {
						if got := req.URL.Query().Get(name); got != value {
							t.Fatalf("query %s = %q, want %q", name, got, value)
						}
					}
					return newHTTPResponse(
						http.StatusOK,
						`{"loops":[],"facets":{"kinds":{},"categories":{},"statuses":{}},"page":{"has_more":false,"total":0,"limit":17}}`,
					), nil
				}),
			},
		}

		response, err := client.ListLoops(t.Context(), "ws-1", LoopListQuery{
			Search:   "release plan",
			Kind:     "workspace",
			Category: "delivery",
			Status:   "running",
			Sort:     "name",
			Cursor:   "next/+=?",
			Limit:    17,
		})
		if err != nil {
			t.Fatalf("ListLoops() error = %v", err)
		}
		if response.Page.Total != 0 || response.Page.Limit != 17 || response.Loops == nil {
			t.Fatalf("ListLoops() = %#v, want explicit empty counted page", response)
		}
	})
}

func TestUnixSocketClientLoopMutationsSendIdentityHeaders(t *testing.T) {
	t.Parallel()

	credentials := agentidentity.Credentials{
		SessionID: "sess-author",
		AgentName: "coder",
	}
	client := &unixSocketClient{
		socketPath: "/tmp/agh.sock",
		httpClient: &http.Client{
			Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				assertAgentRequestHeaders(t, req, credentials)
				switch {
				case req.Method == http.MethodPost && req.URL.Path == "/api/workspaces/ws-1/loops/release/run":
					if got := req.URL.Query().Get("dry"); got != "true" {
						t.Fatalf("run dry query = %q, want true", got)
					}
					return newHTTPResponse(
						http.StatusOK,
						`{"dry_run":{"loop_name":"release","resolved_inputs":{},"generation":1,"nodes":[],"contract":{},"effective_config":{}}}`,
					), nil
				case req.Method == http.MethodPost && req.URL.Path == "/api/workspaces/ws-1/loop-runs/run-1/approve":
					var payload contract.ApproveLoopRunRequest
					if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
						t.Fatalf("json.Decode(approve body) error = %v", err)
					}
					if payload.GateID != "human" || payload.Decision != contract.LoopGateDecisionApprove {
						t.Fatalf("approve body = %#v, want human/approve", payload)
					}
					return newHTTPResponse(http.StatusNoContent, ""), nil
				default:
					t.Fatalf("unexpected request = %s %s", req.Method, req.URL.Path)
					return nil, nil
				}
			}),
		},
	}

	t.Run("Should send identity when starting a Loop run", func(t *testing.T) {
		t.Parallel()

		response, err := client.RunLoop(
			context.Background(),
			"ws-1",
			"release",
			contract.RunLoopRequest{Inputs: map[string]any{"target": "prod"}},
			true,
			credentials,
		)
		if err != nil {
			t.Fatalf("RunLoop() error = %v", err)
		}
		if response.DryRun == nil || response.DryRun.LoopName != "release" {
			t.Fatalf("RunLoop() = %#v, want release dry run", response)
		}
	})

	t.Run("Should send identity when approving a Loop gate", func(t *testing.T) {
		t.Parallel()

		err := client.ApproveLoopRun(
			context.Background(),
			"ws-1",
			"run-1",
			contract.ApproveLoopRunRequest{GateID: "human", Decision: contract.LoopGateDecisionApprove},
			credentials,
		)
		if err != nil {
			t.Fatalf("ApproveLoopRun() error = %v", err)
		}
	})
}

func TestUnixSocketClientTaskMethodsRejectNilPointerRequests(t *testing.T) {
	t.Parallel()

	client := &unixSocketClient{
		socketPath: "/tmp/agh.sock",
		httpClient: &http.Client{
			Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
				t.Fatal("HTTP transport should not be called for nil pointer requests")
				return nil, nil
			}),
		},
	}

	tests := []struct {
		name string
		run  func() error
		want string
	}{
		{
			name: "Should reject nil task execution profile request",
			run: func() error {
				_, err := client.SetTaskExecutionProfile(context.Background(), "task-1", nil)
				return err
			},
			want: "cli: task execution profile request is required",
		},
		{
			name: "Should reject nil bridge notification subscription request",
			run: func() error {
				_, err := client.CreateTaskBridgeNotificationSubscription(context.Background(), "task-1", nil)
				return err
			},
			want: "cli: task bridge notification subscription request is required",
		},
		{
			name: "Should reject nil task run review request",
			run: func() error {
				_, err := client.RequestTaskRunReview(context.Background(), "run-1", nil)
				return err
			},
			want: "cli: task run review request is required",
		},
		{
			name: "Should reject nil task run review verdict request",
			run: func() error {
				_, err := client.SubmitTaskRunReviewVerdict(context.Background(), "review-1", nil)
				return err
			},
			want: "cli: task run review verdict request is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.run()
			if err == nil || err.Error() != tt.want {
				t.Fatalf("error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestUnixSocketClientAgentTaskMethods(t *testing.T) {
	t.Parallel()

	credentials := agentidentity.Credentials{
		SessionID:   "sess-1",
		AgentName:   "coder",
		WorkspaceID: "ws-1",
	}
	var sawClaim bool
	var sawNoWork bool
	var sawHeartbeat bool
	var sawComplete bool
	var sawFail bool
	var sawRelease bool

	client := &unixSocketClient{
		socketPath: "/tmp/agh.sock",
		httpClient: &http.Client{
			Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				assertAgentRequestHeaders(t, req, credentials)
				switch {
				case req.Method == http.MethodPost && req.URL.Path == "/api/agent/tasks/claim-next":
					var payload contract.AgentTaskClaimNextRequest
					if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
						t.Fatalf("json.Decode(claim-next body) error = %v", err)
					}
					if payload.WorkspaceID == "empty" {
						sawNoWork = true
						return newHTTPResponse(http.StatusNoContent, ""), nil
					}
					sawClaim = true
					if payload.WorkspaceID != "ws-1" ||
						payload.PriorityMin != 2 ||
						payload.LeaseSeconds != 120 ||
						!payload.Wait ||
						len(payload.RequiredCapabilities) != 1 ||
						payload.RequiredCapabilities[0] != "go" {
						t.Fatalf("claim-next body = %#v, want parsed request", payload)
					}
					resolved := testLiveResolvedParticipation("builders")
					body := mustJSON(t, contract.AgentTaskClaimResponse{Claim: contract.AgentTaskClaimPayload{
						Task: contract.TaskReferencePayload{
							ID: "task-1", Title: "Run task", Status: taskpkg.TaskStatusInProgress,
							Scope: taskpkg.ScopeWorkspace, WorkspaceID: "ws-1",
						},
						Run: contract.TaskRunPayload{
							ID: "run-1", TaskID: "task-1", Status: taskpkg.TaskRunStatusClaimed,
							Attempt: 1, SessionID: "sess-1", QueuedAt: fixedTestNow,
						},
						Lease: contract.TaskRunLeaseSummaryPayload{
							TaskID: "task-1", RunID: "run-1", Status: taskpkg.TaskRunStatusClaimed,
							SessionID: "sess-1", ResolvedNetworkParticipation: resolved,
							ClaimTokenHash: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
						},
						CoordinationChannel: &contract.CoordinationChannelPayload{
							ID: "builders", DisplayName: "Builders",
							AllowedMessageKinds: []contract.CoordinationMessageKind{contract.CoordinationMessageStatus},
						},
					}})
					return newHTTPResponse(http.StatusOK, string(body)), nil
				case req.Method == http.MethodPost && req.URL.Path == "/api/agent/tasks/run-1/heartbeat":
					sawHeartbeat = true
					var payload contract.AgentTaskHeartbeatRequest
					if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
						t.Fatalf("json.Decode(heartbeat body) error = %v", err)
					}
					if payload.LeaseSeconds != 60 {
						t.Fatalf("heartbeat body = %#v, want lease duration only", payload)
					}
					return agentTaskLeaseHTTPResponse(t, taskpkg.TaskRunStatusClaimed), nil
				case req.Method == http.MethodPost && req.URL.Path == "/api/agent/tasks/run-1/complete":
					sawComplete = true
					var payload contract.AgentTaskCompleteRequest
					if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
						t.Fatalf("json.Decode(complete body) error = %v", err)
					}
					if string(payload.Result) != `{"ok":true}` {
						t.Fatalf("complete body = %#v, want result only", payload)
					}
					return agentTaskLeaseHTTPResponse(t, taskpkg.TaskRunStatusCompleted), nil
				case req.Method == http.MethodPost && req.URL.Path == "/api/agent/tasks/run-1/fail":
					sawFail = true
					var payload contract.AgentTaskFailRequest
					if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
						t.Fatalf("json.Decode(fail body) error = %v", err)
					}
					if payload.Error != "boom" ||
						string(payload.Metadata) != `{"code":"E_TASK"}` {
						t.Fatalf("fail body = %#v, want error metadata only", payload)
					}
					return agentTaskLeaseHTTPResponse(t, taskpkg.TaskRunStatusFailed), nil
				case req.Method == http.MethodPost && req.URL.Path == "/api/agent/tasks/run-1/release":
					sawRelease = true
					var payload contract.AgentTaskReleaseRequest
					if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
						t.Fatalf("json.Decode(release body) error = %v", err)
					}
					if payload.Reason != "handoff" {
						t.Fatalf("release body = %#v, want reason only", payload)
					}
					return agentTaskLeaseHTTPResponse(t, taskpkg.TaskRunStatusQueued), nil
				default:
					t.Fatalf("unexpected request = %s %s", req.Method, req.URL.Path)
					return nil, nil
				}
			}),
		},
	}

	t.Run("Should claim next task", func(t *testing.T) {
		claim, err := client.AgentTaskClaimNext(context.Background(), AgentTaskClaimNextRequest{
			WorkspaceID:          "ws-1",
			RequiredCapabilities: []string{"go"},
			PriorityMin:          2,
			LeaseSeconds:         120,
			Wait:                 true,
		}, credentials)
		if err != nil {
			t.Fatalf("AgentTaskClaimNext() error = %v", err)
		}
		if !claim.Claimed || claim.Claim == nil || claim.Claim.Lease.ClaimTokenHash == "" {
			t.Fatalf("AgentTaskClaimNext() = %#v, want claimed session-bound lease response", claim)
		}
		assertResolvedParticipation(
			t,
			claim.Claim.Lease.ResolvedNetworkParticipation,
			*testLiveResolvedParticipation("builders"),
		)
	})
	t.Run("Should return no work", func(t *testing.T) {
		noWork, err := client.AgentTaskClaimNext(
			context.Background(),
			AgentTaskClaimNextRequest{WorkspaceID: "empty"},
			credentials,
		)
		if err != nil {
			t.Fatalf("AgentTaskClaimNext(no work) error = %v", err)
		}
		if noWork.Claimed || noWork.Claim != nil {
			t.Fatalf("AgentTaskClaimNext(no work) = %#v, want claimed false", noWork)
		}
	})
	t.Run("Should heartbeat claimed task", func(t *testing.T) {
		lease, err := client.AgentTaskHeartbeat(
			context.Background(),
			"run-1",
			AgentTaskHeartbeatRequest{LeaseSeconds: 60},
			credentials,
		)
		if err != nil || lease.Status != taskpkg.TaskRunStatusClaimed {
			t.Fatalf("AgentTaskHeartbeat() = %#v, %v", lease, err)
		}
		assertResolvedParticipation(t, lease.ResolvedNetworkParticipation, *testLiveResolvedParticipation("builders"))
	})
	t.Run("Should complete claimed task", func(t *testing.T) {
		lease, err := client.AgentTaskComplete(
			context.Background(),
			"run-1",
			AgentTaskCompleteRequest{Result: json.RawMessage(`{"ok":true}`)},
			credentials,
		)
		if err != nil || lease.Status != taskpkg.TaskRunStatusCompleted {
			t.Fatalf("AgentTaskComplete() = %#v, %v", lease, err)
		}
		assertResolvedParticipation(t, lease.ResolvedNetworkParticipation, *testLiveResolvedParticipation("builders"))
	})
	t.Run("Should fail claimed task", func(t *testing.T) {
		lease, err := client.AgentTaskFail(
			context.Background(),
			"run-1",
			AgentTaskFailRequest{
				Error:    "boom",
				Metadata: json.RawMessage(`{"code":"E_TASK"}`),
			},
			credentials,
		)
		if err != nil || lease.Status != taskpkg.TaskRunStatusFailed {
			t.Fatalf("AgentTaskFail() = %#v, %v", lease, err)
		}
		assertResolvedParticipation(t, lease.ResolvedNetworkParticipation, *testLiveResolvedParticipation("builders"))
	})
	t.Run("Should release claimed task", func(t *testing.T) {
		lease, err := client.AgentTaskRelease(
			context.Background(),
			"run-1",
			AgentTaskReleaseRequest{Reason: "handoff"},
			credentials,
		)
		if err != nil || lease.Status != taskpkg.TaskRunStatusQueued {
			t.Fatalf("AgentTaskRelease() = %#v, %v", lease, err)
		}
		assertResolvedParticipation(t, lease.ResolvedNetworkParticipation, *testLiveResolvedParticipation("builders"))
	})
	if !sawClaim || !sawNoWork || !sawHeartbeat || !sawComplete || !sawFail || !sawRelease {
		t.Fatalf(
			"method coverage claim=%t no_work=%t heartbeat=%t complete=%t fail=%t release=%t, want all true",
			sawClaim,
			sawNoWork,
			sawHeartbeat,
			sawComplete,
			sawFail,
			sawRelease,
		)
	}
}

func TestUnixSocketClientAgentTaskErrorsRedactClaimTokens(t *testing.T) {
	t.Parallel()

	t.Run("Should redact claim tokens from task errors", func(t *testing.T) {
		t.Parallel()

		rawToken := "agh_claim_CLIENTERRORTOKEN123"
		client := &unixSocketClient{
			socketPath: "/tmp/agh.sock",
			httpClient: &http.Client{
				Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
					return newHTTPResponse(
						http.StatusConflict,
						`{"error":"task: invalid claim token: `+rawToken+`"}`,
					), nil
				}),
			},
		}
		_, err := client.AgentTaskRelease(
			context.Background(),
			"run-1",
			AgentTaskReleaseRequest{},
			agentidentity.Credentials{SessionID: "sess-1", AgentName: "coder"},
		)
		if err == nil {
			t.Fatal("AgentTaskRelease() error = nil, want redacted API error")
		}
		if strings.Contains(err.Error(), rawToken) || !strings.Contains(err.Error(), "agh_claim_[REDACTED]") {
			t.Fatalf("error = %q, want redacted claim token", err.Error())
		}
	})

	t.Run("Should redact MCP OAuth and secret binding material from API errors", func(t *testing.T) {
		t.Parallel()

		err := readAPIErrorBody(
			http.StatusInternalServerError,
			"500 Internal Server Error",
			[]byte(
				`{"error":"mcp_auth_token=raw-mcp access_token=raw-access authorization_code=raw-code code_verifier=raw-verifier secret_binding=raw-binding agh_claim_CLIENTERRORTOKEN456"}`,
			),
		)
		if err == nil {
			t.Fatal("readAPIErrorBody() error = nil, want redacted API error")
		}
		for _, leaked := range []string{
			"raw-mcp",
			"raw-access",
			"raw-code",
			"raw-verifier",
			"raw-binding",
			"agh_claim_CLIENTERRORTOKEN456",
		} {
			if strings.Contains(err.Error(), leaked) {
				t.Fatalf("readAPIErrorBody() error = %q leaked %q", err.Error(), leaked)
			}
		}
		if !strings.Contains(err.Error(), "[REDACTED]") {
			t.Fatalf("readAPIErrorBody() error = %q, want redacted markers", err.Error())
		}
	})

	t.Run("Should redact structured tool API errors", func(t *testing.T) {
		t.Parallel()

		err := readAPIErrorBody(
			http.StatusBadGateway,
			"502 Bad Gateway",
			[]byte(`{"error":{"code":"backend_failed","message":"pkce_verifier=raw-pkce refresh_token=raw-refresh"}}`),
		)
		if err == nil {
			t.Fatal("readAPIErrorBody(tool error) error = nil, want redacted tool API error")
		}
		if strings.Contains(err.Error(), "raw-pkce") || strings.Contains(err.Error(), "raw-refresh") {
			t.Fatalf("readAPIErrorBody(tool error) error = %q, want sensitive values redacted", err.Error())
		}
	})

	t.Run("Should preserve actionable diagnostics from shared API errors", func(t *testing.T) {
		t.Parallel()

		err := readAPIErrorBody(
			http.StatusUnprocessableEntity,
			"422 Unprocessable Entity",
			[]byte(
				`{"error":"provider model not found","diagnostic":{"id":"provider.models.model_not_found","code":"model_not_found","category":"provider","title":"Provider model not found","message":"Choose a current model.","severity":"error","freshness":"live"}}`,
			),
		)
		if err == nil {
			t.Fatal("readAPIErrorBody(diagnostic) error = nil")
		}
		item, ok := diagnosticspkg.ItemFromError(err)
		if !ok || item.Code != contract.CodeModelNotFound {
			t.Fatalf("diagnostic = %#v, %v; want model_not_found", item, ok)
		}
	})
}

func agentTaskLeaseHTTPResponse(t *testing.T, status taskpkg.RunStatus) *http.Response {
	t.Helper()
	body := mustJSON(t, contract.AgentTaskLeaseResponse{Lease: contract.TaskRunLeaseSummaryPayload{
		TaskID:                       "task-1",
		RunID:                        "run-1",
		Status:                       status,
		SessionID:                    "sess-1",
		ResolvedNetworkParticipation: testLiveResolvedParticipation("builders"),
	}})
	return newHTTPResponse(http.StatusOK, string(body))
}

func TestUnixSocketClientToolMethods(t *testing.T) {
	t.Parallel()

	client := &unixSocketClient{
		socketPath: "/tmp/agh.sock",
		httpClient: &http.Client{
			Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				switch {
				case req.Method == http.MethodGet && req.URL.Path == "/api/tools":
					if req.URL.Query().Get("workspace_id") != "ws-1" ||
						req.URL.Query().Get("session_id") != "sess-1" ||
						req.URL.Query().Get("agent_name") != "coder" {
						t.Fatalf("tool list query = %s, want scoped query", req.URL.RawQuery)
					}
					return newHTTPResponse(http.StatusOK, string(mustJSON(t, sampleToolsResponse()))), nil
				case req.Method == http.MethodPost && req.URL.Path == "/api/tools/search":
					var request ToolSearchRequest
					if err := json.NewDecoder(req.Body).Decode(&request); err != nil {
						t.Fatalf("decode search body: %v", err)
					}
					if request.Query != "skill" || request.Limit != 2 || request.WorkspaceID != "ws-1" {
						t.Fatalf("tool search request = %#v, want scoped search", request)
					}
					return newHTTPResponse(http.StatusOK, string(mustJSON(t, sampleToolsResponse()))), nil
				case req.Method == http.MethodGet && req.URL.Path == "/api/tools/agh__skill_view":
					if req.URL.Query().Get("workspace_id") != "ws-1" {
						t.Fatalf("tool info query = %s, want workspace_id=ws-1", req.URL.RawQuery)
					}
					return newHTTPResponse(
						http.StatusOK,
						string(mustJSON(t, ToolResponseRecord{Tool: sampleToolsResponse().Tools[0]})),
					), nil
				case req.Method == http.MethodPost && req.URL.Path == "/api/tools/agh__tool_info/invoke":
					var request ToolInvokeRequest
					if err := json.NewDecoder(req.Body).Decode(&request); err != nil {
						t.Fatalf("decode invoke body: %v", err)
					}
					if string(request.Input) != `{"tool_id":"agh__skill_view"}` ||
						request.SessionID != "sess-1" ||
						len(request.SensitiveInputFields) != 1 ||
						request.SensitiveInputFields[0] != "token" {
						t.Fatalf("invoke request = %#v, want scoped tool input", request)
					}
					response := sampleInvokeResponse()
					response.Result.Preview = "token=agh_claim_secret"
					response.Result.Structured = json.RawMessage(
						`{"access_token":"super-secret","completion_tokens":9}`,
					)
					return newHTTPResponse(http.StatusOK, string(mustJSON(t, response))), nil
				case req.Method == http.MethodGet && req.URL.Path == "/api/toolsets":
					if req.URL.Query().Get("agent_name") != "coder" {
						t.Fatalf("toolsets query = %s, want agent_name=coder", req.URL.RawQuery)
					}
					return newHTTPResponse(http.StatusOK, string(mustJSON(t, sampleToolsetsResponse()))), nil
				case req.Method == http.MethodGet && req.URL.Path == "/api/toolsets/agh__catalog":
					if req.URL.Query().Get("session_id") != "sess-1" {
						t.Fatalf("toolset info query = %s, want session_id=sess-1", req.URL.RawQuery)
					}
					return newHTTPResponse(
						http.StatusOK,
						string(mustJSON(t, ToolsetResponseRecord{Toolset: sampleToolsetsResponse().Toolsets[0]})),
					), nil
				default:
					t.Fatalf("unexpected request = %s %s", req.Method, req.URL.Path)
					return nil, nil
				}
			}),
		},
	}

	t.Run("Should list tools", func(t *testing.T) {
		t.Parallel()

		response, err := client.ListTools(context.Background(), ToolQuery{
			WorkspaceID: "ws-1",
			SessionID:   "sess-1",
			AgentName:   "coder",
		})
		if err != nil {
			t.Fatalf("ListTools() error = %v", err)
		}
		if len(response.Tools) != 2 {
			t.Fatalf("ListTools() count = %d, want 2", len(response.Tools))
		}
	})

	t.Run("Should search tools", func(t *testing.T) {
		t.Parallel()

		response, err := client.SearchTools(context.Background(), ToolSearchRequest{
			Query:       " skill ",
			Limit:       2,
			WorkspaceID: " ws-1 ",
		})
		if err != nil {
			t.Fatalf("SearchTools() error = %v", err)
		}
		if len(response.Tools) != 2 {
			t.Fatalf("SearchTools() count = %d, want 2", len(response.Tools))
		}
	})

	t.Run("Should get tool info", func(t *testing.T) {
		t.Parallel()

		response, err := client.GetTool(context.Background(), toolspkg.ToolIDSkillView.String(), ToolQuery{
			WorkspaceID: "ws-1",
		})
		if err != nil {
			t.Fatalf("GetTool() error = %v", err)
		}
		if response.Tool.Descriptor.ToolID != toolspkg.ToolIDSkillView {
			t.Fatalf("GetTool() id = %q, want %q", response.Tool.Descriptor.ToolID, toolspkg.ToolIDSkillView)
		}
	})

	t.Run("Should invoke tool", func(t *testing.T) {
		t.Parallel()

		response, err := client.InvokeTool(context.Background(), toolspkg.ToolIDToolInfo.String(), ToolInvokeRequest{
			SessionID:            " sess-1 ",
			Input:                json.RawMessage(`{"tool_id":"agh__skill_view"}`),
			SensitiveInputFields: []string{" token "},
		})
		if err != nil {
			t.Fatalf("InvokeTool() error = %v", err)
		}
		if response.ToolID != toolspkg.ToolIDToolInfo {
			t.Fatalf("InvokeTool() id = %q, want %q", response.ToolID, toolspkg.ToolIDToolInfo)
		}
		encoded := string(mustJSON(t, response))
		if strings.Contains(encoded, "agh_claim_secret") || strings.Contains(encoded, "super-secret") {
			t.Fatalf("InvokeTool() leaked sensitive result fields: %s", encoded)
		}
		if !strings.Contains(encoded, "completion_tokens") {
			t.Fatalf("InvokeTool() response = %s, want benign token metrics preserved", encoded)
		}
	})

	t.Run("Should list toolsets", func(t *testing.T) {
		t.Parallel()

		response, err := client.ListToolsets(context.Background(), ToolQuery{AgentName: "coder"})
		if err != nil {
			t.Fatalf("ListToolsets() error = %v", err)
		}
		if len(response.Toolsets) != 2 {
			t.Fatalf("ListToolsets() count = %d, want 2", len(response.Toolsets))
		}
	})

	t.Run("Should get toolset", func(t *testing.T) {
		t.Parallel()

		response, err := client.GetToolset(context.Background(), toolspkg.ToolsetIDCatalog.String(), ToolQuery{
			SessionID: "sess-1",
		})
		if err != nil {
			t.Fatalf("GetToolset() error = %v", err)
		}
		if response.Toolset.ID != toolspkg.ToolsetIDCatalog {
			t.Fatalf("GetToolset() id = %q, want %q", response.Toolset.ID, toolspkg.ToolsetIDCatalog)
		}
	})
}

func TestUnixSocketClientToolMethodsReturnStructuredErrors(t *testing.T) {
	t.Parallel()

	client := &unixSocketClient{
		socketPath: "/tmp/agh.sock",
		httpClient: &http.Client{
			Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				switch {
				case strings.HasPrefix(req.URL.Path, "/api/tools"),
					strings.HasPrefix(req.URL.Path, "/api/toolsets"):
					return newHTTPResponse(
						http.StatusUnprocessableEntity,
						`{"error":{"code":"tool_unavailable","message":"tool token=super-secret unavailable","tool_id":"agh__skill_view","reason_codes":["backend_unhealthy"],"layer":"registry","details":{"secret":"super-secret"}}}`,
					), nil
				default:
					t.Fatalf("unexpected request = %s %s", req.Method, req.URL.Path)
					return nil, nil
				}
			}),
		},
	}

	testCases := []struct {
		name string
		run  func(context.Context) error
	}{
		{
			name: "Should return list tools error",
			run: func(ctx context.Context) error {
				_, err := client.ListTools(ctx, ToolQuery{})
				return err
			},
		},
		{
			name: "Should return search tools error",
			run: func(ctx context.Context) error {
				_, err := client.SearchTools(ctx, ToolSearchRequest{Query: "skill"})
				return err
			},
		},
		{
			name: "Should return get tool error",
			run: func(ctx context.Context) error {
				_, err := client.GetTool(ctx, toolspkg.ToolIDSkillView.String(), ToolQuery{})
				return err
			},
		},
		{
			name: "Should return invoke tool error",
			run: func(ctx context.Context) error {
				_, err := client.InvokeTool(ctx, toolspkg.ToolIDToolInfo.String(), ToolInvokeRequest{})
				return err
			},
		},
		{
			name: "Should return list toolsets error",
			run: func(ctx context.Context) error {
				_, err := client.ListToolsets(ctx, ToolQuery{})
				return err
			},
		},
		{
			name: "Should return get toolset error",
			run: func(ctx context.Context) error {
				_, err := client.GetToolset(ctx, toolspkg.ToolsetIDCatalog.String(), ToolQuery{})
				return err
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := tc.run(context.Background())
			var toolErr *toolAPIError
			if !errors.As(err, &toolErr) {
				t.Fatalf("tool client error = %T %[1]v, want toolAPIError", err)
			}
			if strings.Contains(toolErr.Error(), "super-secret") {
				t.Fatalf("tool client error leaked secret: %v", toolErr)
			}
			response := toolErr.Response()
			if response.Error.Code != toolspkg.ErrorCodeUnavailable ||
				len(response.Error.Details) > 0 ||
				response.Error.ReasonCodes[0] != toolspkg.ReasonBackendUnhealthy {
				t.Fatalf("tool client error response = %#v, want sanitized unavailable error", response.Error)
			}
		})
	}
}

func TestUnixSocketClientHostedMCPMethods(t *testing.T) {
	t.Parallel()

	t.Run("Should exercise hosted MCP client methods", func(t *testing.T) {
		t.Parallel()

		client := &unixSocketClient{
			socketPath: "/tmp/agh.sock",
			httpClient: &http.Client{
				Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
					switch {
					case req.Method == http.MethodPost && req.URL.Path == "/api/internal/hosted-mcp/bind":
						var request mcppkg.HostedBindRequest
						if err := json.NewDecoder(req.Body).Decode(&request); err != nil {
							t.Fatalf("decode bind body: %v", err)
						}
						if request.SessionID != "sess-1" || request.Nonce != "nonce-test" {
							t.Fatalf("bind request = %#v, want session and nonce", request)
						}
						return newHTTPResponse(
							http.StatusOK,
							`{"bind_id":"bind-1","scope":{"session_id":"sess-1","workspace_id":"ws-1","agent_name":"coder"},"tools":[],"digest":"digest-1"}`,
						), nil
					case req.Method == http.MethodGet && req.URL.Path == "/api/internal/hosted-mcp/projection":
						if req.URL.Query().Get("bind_id") != "bind-1" {
							t.Fatalf("projection query = %s, want bind_id", req.URL.RawQuery)
						}
						return newHTTPResponse(http.StatusOK, `{"tools":[],"digest":"digest-2"}`), nil
					case req.Method == http.MethodGet && req.URL.Path == "/api/internal/hosted-mcp/projection/stream":
						if req.URL.Query().Get("bind_id") != "bind-1" ||
							req.URL.Query().Get("last_digest") != "digest-1" {
							t.Fatalf("projection stream query = %s, want bind_id and last_digest", req.URL.RawQuery)
						}
						return newHTTPResponse(
							http.StatusOK,
							"event: ignored\n"+
								"data: {}\n\n"+
								"event: projection\n"+
								"data: {\"tools\":[],\"digest\":\"digest-3\"}\n\n",
						), nil
					case req.Method == http.MethodPost && req.URL.Path == "/api/internal/hosted-mcp/tools/call":
						var request mcppkg.HostedCallRequest
						if err := json.NewDecoder(req.Body).Decode(&request); err != nil {
							t.Fatalf("decode hosted call body: %v", err)
						}
						if request.BindID != "bind-1" ||
							request.ToolName != "skill_view" ||
							string(request.Input) != `{"ok":true}` {
							t.Fatalf("hosted call request = %#v, want bound tool call", request)
						}
						return newHTTPResponse(
							http.StatusOK,
							`{"result":{"structured":{"ok":true},"preview":"ok","bytes":2,"duration_ms":1}}`,
						), nil
					case req.Method == http.MethodPost && req.URL.Path == "/api/internal/hosted-mcp/release":
						var request mcppkg.HostedReleaseRequest
						if err := json.NewDecoder(req.Body).Decode(&request); err != nil {
							t.Fatalf("decode release body: %v", err)
						}
						if request.BindID != "bind-1" {
							t.Fatalf("release request = %#v, want bind-1", request)
						}
						return newHTTPResponse(http.StatusNoContent, ""), nil
					default:
						t.Fatalf("unexpected request = %s %s", req.Method, req.URL.Path)
						return nil, nil
					}
				}),
			},
		}

		bind, err := client.BindHostedMCP(context.Background(), mcppkg.HostedBindRequest{
			SessionID: "sess-1",
			Nonce:     "nonce-test",
		})
		if err != nil {
			t.Fatalf("BindHostedMCP() error = %v", err)
		}
		if bind.BindID != "bind-1" || bind.Scope.SessionID != "sess-1" || bind.Digest != "digest-1" {
			t.Fatalf("BindHostedMCP() = %#v, want bind response", bind)
		}

		projection, err := client.HostedMCPProjection(context.Background(), " bind-1 ")
		if err != nil {
			t.Fatalf("HostedMCPProjection() error = %v", err)
		}
		if projection.Digest != "digest-2" {
			t.Fatalf("HostedMCPProjection() digest = %q, want digest-2", projection.Digest)
		}

		var streamedDigest string
		err = client.StreamHostedMCPProjection(
			context.Background(),
			" bind-1 ",
			" digest-1 ",
			func(snapshot mcppkg.HostedProjectionResponse) error {
				streamedDigest = snapshot.Digest
				return nil
			},
		)
		if err != nil {
			t.Fatalf("StreamHostedMCPProjection() error = %v", err)
		}
		if streamedDigest != "digest-3" {
			t.Fatalf("streamed digest = %q, want digest-3", streamedDigest)
		}

		call, err := client.CallHostedMCP(context.Background(), mcppkg.HostedCallRequest{
			BindID:   "bind-1",
			ToolName: "skill_view",
			Input:    json.RawMessage(`{"ok":true}`),
		})
		if err != nil {
			t.Fatalf("CallHostedMCP() error = %v", err)
		}
		if compactJSON(call.Result.Structured) != `{"ok":true}` {
			t.Fatalf("CallHostedMCP() structured = %s, want ok", call.Result.Structured)
		}

		if err := client.ReleaseHostedMCP(
			context.Background(),
			mcppkg.HostedReleaseRequest{BindID: "bind-1"},
		); err != nil {
			t.Fatalf("ReleaseHostedMCP() error = %v", err)
		}
	})

	t.Run("Should redact hosted MCP stream errors", func(t *testing.T) {
		t.Parallel()

		client := &unixSocketClient{
			socketPath: "/tmp/agh.sock",
			httpClient: &http.Client{
				Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
					if req.Method != http.MethodGet || req.URL.Path != "/api/internal/hosted-mcp/projection/stream" {
						t.Fatalf("request = %s %s, want hosted MCP stream", req.Method, req.URL.Path)
					}
					return newHTTPResponse(
						http.StatusOK,
						"event: error\n"+
							"data: {\"error\":\"bind failed for agh_claim_secret\"}\n\n",
					), nil
				}),
			},
		}
		err := client.StreamHostedMCPProjection(context.Background(), "bind-1", "", nil)
		if err == nil {
			t.Fatal("StreamHostedMCPProjection(error event) error = nil, want redacted error")
		}
		if strings.Contains(err.Error(), "agh_claim_secret") || !strings.Contains(err.Error(), "[REDACTED]") {
			t.Fatalf("StreamHostedMCPProjection(error event) error = %q, want redacted claim token", err.Error())
		}
	})
}

func TestUnixSocketClientStreamsSessionEvents(t *testing.T) {
	t.Parallel()

	t.Run("Should stream session events", func(t *testing.T) {
		t.Parallel()

		client := &unixSocketClient{
			socketPath: "/tmp/agh.sock",
			httpClient: &http.Client{
				Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
					if req.Method == http.MethodGet && req.URL.Path == "/api/sessions/sess-1" {
						return newHTTPResponse(
							http.StatusOK,
							`{"session":{"id":"sess-1","agent_name":"coder","workspace_id":"ws-1","workspace_path":"/tmp","state":"active","created_at":"2026-04-03T12:00:00Z","updated_at":"2026-04-03T12:00:00Z"}}`,
						), nil
					}
					if req.Method != http.MethodGet || req.URL.Path != "/api/workspaces/ws-1/sessions/sess-1/stream" {
						t.Fatalf("request = %s %s, want GET session stream", req.Method, req.URL.Path)
					}
					if req.Header.Get("Last-Event-ID") != "evt-0" {
						t.Fatalf("Last-Event-ID = %q, want evt-0", req.Header.Get("Last-Event-ID"))
					}
					if req.URL.Query().Get("type") != "tool_call" ||
						req.URL.Query().Get("agent_name") != "coder" ||
						req.URL.Query().Get("limit") != "2" ||
						req.URL.Query().Get("frames") != contract.SessionStreamFrameRaw {
						t.Fatalf("stream query = %s, want filters", req.URL.RawQuery)
					}
					return newHTTPResponse(
						http.StatusOK,
						"id: evt-1\n"+
							"event: tool_call\n"+
							`data: {"id":"evt-1","session_id":"sess-1","sequence":1,"type":"tool_call","agent_name":"coder","timestamp":"2026-04-03T12:00:00Z"}`+
							"\n\n",
					), nil
				}),
			},
		}

		var events []SSEEvent
		err := client.StreamSessionEvents(
			context.Background(),
			" sess-1 ",
			SessionEventQuery{Type: "tool_call", AgentName: "coder", Last: 2},
			"evt-0",
			func(event SSEEvent) error {
				events = append(events, event)
				return nil
			},
		)
		if err != nil {
			t.Fatalf("StreamSessionEvents() error = %v", err)
		}
		if len(events) != 1 || events[0].ID != "evt-1" || events[0].Event != "tool_call" {
			t.Fatalf("events = %#v, want one tool_call event", events)
		}
	})
}

func TestUnixSocketClientTaskExecutionMethods(t *testing.T) {
	t.Parallel()

	t.Run("Should execute task mutation methods", func(t *testing.T) {
		t.Parallel()

		client := &unixSocketClient{
			socketPath: "/tmp/agh.sock",
			httpClient: &http.Client{
				Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
					if req.Method != http.MethodPost {
						t.Fatalf("request method = %s, want POST", req.Method)
					}
					switch req.URL.Path {
					case "/api/tasks/task-1/publish", "/api/tasks/task-1/start", "/api/tasks/task-1/approve":
						var request TaskExecutionRequest
						if err := json.NewDecoder(req.Body).Decode(&request); err != nil {
							t.Fatalf("decode task execution body: %v", err)
						}
						assertLiveNamedParticipationRequest(t, request.NetworkParticipation, "builders")
						if request.IdempotencyKey != "idem-1" {
							t.Fatalf("task execution request = %#v, want idempotency", request)
						}
						return newHTTPResponse(http.StatusOK, string(mustJSON(t, sampleTaskExecutionRecord()))), nil
					default:
						t.Fatalf("unexpected request path = %s", req.URL.Path)
						return nil, nil
					}
				}),
			},
		}
		request := TaskExecutionRequest{
			IdempotencyKey:       "idem-1",
			NetworkParticipation: testLiveNamedParticipationRequest("builders"),
			Metadata:             json.RawMessage(`{"source":"cli-test"}`),
		}

		published, err := client.PublishTask(context.Background(), " task-1 ", request)
		if err != nil {
			t.Fatalf("PublishTask() error = %v", err)
		}
		if published.Task.ID == "" || published.Run.ID == "" {
			t.Fatalf("PublishTask() = %#v, want task and run", published)
		}
		started, err := client.StartTask(context.Background(), " task-1 ", request)
		if err != nil {
			t.Fatalf("StartTask() error = %v", err)
		}
		if started.Task.ID != published.Task.ID {
			t.Fatalf("StartTask() task id = %q, want %q", started.Task.ID, published.Task.ID)
		}
		approved, err := client.ApproveTask(context.Background(), " task-1 ", request)
		if err != nil {
			t.Fatalf("ApproveTask() error = %v", err)
		}
		if approved.Run.ID != published.Run.ID {
			t.Fatalf("ApproveTask() run id = %q, want %q", approved.Run.ID, published.Run.ID)
		}
	})
}

func TestUnixSocketClientAgentContextAndSpawnMethods(t *testing.T) {
	t.Parallel()

	t.Run("Should get agent context and spawn child sessions", func(t *testing.T) {
		t.Parallel()

		credentials := agentidentity.Credentials{
			SessionID:   "sess-1",
			AgentName:   "coder",
			WorkspaceID: "ws-1",
		}
		client := &unixSocketClient{
			socketPath: "/tmp/agh.sock",
			httpClient: &http.Client{
				Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
					assertAgentRequestHeaders(t, req, credentials)
					switch {
					case req.Method == http.MethodGet && req.URL.Path == "/api/agent/context":
						return newHTTPResponse(
							http.StatusOK,
							`{"context":{"self":{"session_id":"sess-1","agent_name":"coder","provider":"test"},"workspace":{"id":"ws-1"},"session":{"id":"sess-1","agent_name":"coder","workspace_id":"ws-1","workspace_path":"/tmp","state":"active","created_at":"2026-04-03T12:00:00Z","updated_at":"2026-04-03T12:00:00Z"}}}`,
						), nil
					case req.Method == http.MethodPost && req.URL.Path == "/api/agent/spawn":
						var request AgentSpawnRequest
						if err := json.NewDecoder(req.Body).Decode(&request); err != nil {
							t.Fatalf("decode spawn body: %v", err)
						}
						if request.AgentName != "worker" || request.SpawnRole != "reviewer" {
							t.Fatalf("spawn request = %#v, want worker reviewer", request)
						}
						return newHTTPResponse(
							http.StatusCreated,
							`{"spawn":{"session":{"id":"sess-child","agent_name":"worker","workspace_id":"ws-1","workspace_path":"/tmp","state":"active","created_at":"2026-04-03T12:00:00Z","updated_at":"2026-04-03T12:00:00Z"},"lineage":{"parent_session_id":"sess-1","root_session_id":"sess-1","depth":1},"permissions":{}}}`,
						), nil
					default:
						t.Fatalf("unexpected request = %s %s", req.Method, req.URL.Path)
						return nil, nil
					}
				}),
			},
		}

		contextRecord, err := client.AgentContext(context.Background(), credentials)
		if err != nil {
			t.Fatalf("AgentContext() error = %v", err)
		}
		if contextRecord.Self.SessionID != "sess-1" || contextRecord.Workspace.ID != "ws-1" {
			t.Fatalf("AgentContext() = %#v, want caller context", contextRecord)
		}

		spawn, err := client.AgentSpawn(context.Background(), AgentSpawnRequest{
			AgentName: "worker",
			SpawnRole: "reviewer",
		}, credentials)
		if err != nil {
			t.Fatalf("AgentSpawn() error = %v", err)
		}
		if spawn.Session.ID != "sess-child" || spawn.Session.AgentName != "worker" {
			t.Fatalf("AgentSpawn() = %#v, want child session", spawn.Session)
		}
	})
}

func assertAgentRequestHeaders(t *testing.T, req *http.Request, credentials agentidentity.Credentials) {
	t.Helper()

	if got := req.Header.Get(agentidentity.HeaderSessionID); got != credentials.SessionID {
		t.Fatalf("%s = %q, want %q", agentidentity.HeaderSessionID, got, credentials.SessionID)
	}
	if got := req.Header.Get(agentidentity.HeaderAgent); got != credentials.AgentName {
		t.Fatalf("%s = %q, want %q", agentidentity.HeaderAgent, got, credentials.AgentName)
	}
	if got := req.Header.Get(agentidentity.HeaderWorkspaceID); got != credentials.WorkspaceID {
		t.Fatalf("%s = %q, want %q", agentidentity.HeaderWorkspaceID, got, credentials.WorkspaceID)
	}
}

func TestUnixSocketClientMethods(t *testing.T) {
	t.Parallel()

	client := &unixSocketClient{
		socketPath: "/tmp/agh.sock",
		httpClient: &http.Client{
			Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				switch {
				case req.Method == http.MethodGet && req.URL.Path == "/api/sessions/sess-1":
					return newHTTPResponse(
						http.StatusOK,
						`{"session":{"id":"sess-1","agent_name":"coder","workspace_id":"ws-1","workspace_path":"/tmp","state":"active","created_at":"2026-04-03T12:00:00Z","updated_at":"2026-04-03T12:00:00Z"}}`,
					), nil
				case req.Method == http.MethodGet && req.URL.Path == "/api/sessions":
					if got := req.URL.Query().Get("workspace"); got != "" && got != "ws-1" {
						t.Fatalf("session workspace query = %q, want empty or ws-1", got)
					}
					return newHTTPResponse(
						http.StatusOK,
						`{"sessions":[{"id":"sess-1","agent_name":"coder","workspace_id":"ws-1","workspace_path":"/tmp","state":"active","created_at":"2026-04-03T12:00:00Z","updated_at":"2026-04-03T12:00:00Z"}],"page":{"next_cursor":"cursor-next","has_more":true,"total":3,"limit":1}}`,
					), nil
				case req.Method == http.MethodPost && req.URL.Path == "/api/sessions":
					body, err := io.ReadAll(req.Body)
					if err != nil {
						t.Fatalf("io.ReadAll(session create body) error = %v", err)
					}
					if !strings.Contains(string(body), `"agent_name":"coder"`) {
						t.Fatalf("session create body = %s, want agent_name", body)
					}
					return newHTTPResponse(
						http.StatusCreated,
						`{"session":{"id":"sess-new","agent_name":"coder","workspace_id":"ws-1","workspace_path":"/tmp","state":"active","created_at":"2026-04-03T12:00:00Z","updated_at":"2026-04-03T12:00:00Z"}}`,
					), nil
				case req.Method == http.MethodGet && req.URL.Path == "/api/workspaces/ws-1/sessions/sess-1":
					return newHTTPResponse(
						http.StatusOK,
						`{"session":{"id":"sess-1","agent_name":"coder","workspace_id":"ws-1","workspace_path":"/tmp","state":"active","created_at":"2026-04-03T12:00:00Z","updated_at":"2026-04-03T12:00:00Z"}}`,
					), nil
				case req.Method == http.MethodPost && req.URL.Path == "/api/workspaces/ws-1/sessions/sess-1/stop":
					return newHTTPResponse(http.StatusNoContent, ``), nil
				case req.Method == http.MethodPost && req.URL.Path == "/api/workspaces/ws-1/sessions/sess-1/attach":
					return newHTTPResponse(
						http.StatusOK,
						`{"session":{"id":"sess-1","agent_name":"coder","workspace_id":"ws-1","workspace_path":"/tmp","state":"active","created_at":"2026-04-03T12:00:00Z","updated_at":"2026-04-03T12:00:00Z"}}`,
					), nil
				case req.Method == http.MethodPost && req.URL.Path == "/api/workspaces/ws-1/sessions/sess-1/repair":
					if got := req.URL.Query().Get("dry_run"); got != "true" {
						t.Fatalf("repair dry_run query = %q, want true", got)
					}
					if got := req.URL.Query().Get("force"); got != "true" {
						t.Fatalf("repair force query = %q, want true", got)
					}
					return newHTTPResponse(
						http.StatusOK,
						`{"repair":{"session_id":"sess-1","persisted":false,"issues":[],"actions":[{"code":"append_terminal_error","turn_id":"turn-1","persisted":false}]}}`,
					), nil
				case req.Method == http.MethodPost && req.URL.Path == "/api/workspaces/ws-1/sessions/sess-1/prompt":
					body, err := io.ReadAll(req.Body)
					if err != nil {
						t.Fatalf("io.ReadAll(prompt body) error = %v", err)
					}
					if got := req.URL.Query().Get("format"); got != "raw" {
						t.Fatalf("prompt format query = %q, want raw", got)
					}
					if !strings.Contains(string(body), `"message":"hello"`) {
						t.Fatalf("prompt body = %s, want message", body)
					}
					resp := newHTTPResponse(http.StatusOK, strings.Join([]string{
						"id: 1",
						"event: agent_message",
						`data: {"session_id":"sess-1","turn_id":"turn-1","type":"agent_message","timestamp":"2026-04-03T12:00:00Z","text":"hello back"}`,
						"",
					}, "\n"))
					resp.Header.Set("Content-Type", "text/event-stream")
					return resp, nil
				case req.Method == http.MethodGet && req.URL.Path == "/api/workspaces/ws-1/sessions/sess-1/events":
					if got := req.URL.Query().Get("type"); got != "tool_call" {
						t.Fatalf("session events type query = %q, want %q", got, "tool_call")
					}
					return newHTTPResponse(
						http.StatusOK,
						`{"events":[{"id":"evt-1","session_id":"sess-1","sequence":1,"turn_id":"turn-1","type":"tool_call","agent_name":"coder","timestamp":"2026-04-03T12:00:00Z"}]}`,
					), nil
				case req.Method == http.MethodGet && req.URL.Path == "/api/workspaces/ws-1/sessions/sess-1/history":
					if got := req.URL.Query().Get("limit"); got != "2" {
						t.Fatalf("history limit query = %q, want %q", got, "2")
					}
					return newHTTPResponse(
						http.StatusOK,
						`{"history":[{"turn_id":"turn-1","events":[{"id":"evt-1","session_id":"sess-1","sequence":1,"turn_id":"turn-1","type":"agent_message","agent_name":"coder","content":{"text":"hi"},"timestamp":"2026-04-03T12:00:00Z"}]}]}`,
					), nil
				case req.Method == http.MethodPost && req.URL.Path == "/api/workspaces":
					body, err := io.ReadAll(req.Body)
					if err != nil {
						t.Fatalf("io.ReadAll(workspace create body) error = %v", err)
					}
					if !strings.Contains(string(body), `"root_dir":"/workspace/project"`) {
						t.Fatalf("workspace create body = %s, want root_dir", body)
					}
					return newHTTPResponse(
						http.StatusCreated,
						`{"workspace":{"id":"ws-1","root_dir":"/workspace/project","name":"alpha","created_at":"2026-04-03T12:00:00Z","updated_at":"2026-04-03T12:00:00Z"}}`,
					), nil
				case req.Method == http.MethodGet && req.URL.Path == "/api/workspaces":
					return newHTTPResponse(
						http.StatusOK,
						`{"workspaces":[{"id":"ws-1","root_dir":"/workspace/project","name":"alpha","created_at":"2026-04-03T12:00:00Z","updated_at":"2026-04-03T12:00:00Z"}]}`,
					), nil
				case req.Method == http.MethodGet && req.URL.Path == "/api/workspaces/alpha":
					return newHTTPResponse(
						http.StatusOK,
						`{"workspace":{"id":"ws-1","root_dir":"/workspace/project","name":"alpha","created_at":"2026-04-03T12:00:00Z","updated_at":"2026-04-03T12:00:00Z"},"sessions":[{"id":"sess-1","agent_name":"coder","workspace_id":"ws-1","workspace_path":"/workspace/project","state":"active","created_at":"2026-04-03T12:00:00Z","updated_at":"2026-04-03T12:00:00Z"}],"agents":[{"name":"coder","provider":"fake","prompt":"hi"}],"skills":[{"name":"review","dir":"/workspace/project/.agh/skills/review","source":"workspace"}]}`,
					), nil
				case req.Method == http.MethodGet && req.URL.Path == "/api/workspaces/ws-1":
					return newHTTPResponse(
						http.StatusOK,
						`{"workspace":{"id":"ws-1","root_dir":"/workspace/project","name":"alpha","created_at":"2026-04-03T12:00:00Z","updated_at":"2026-04-03T12:00:00Z"},"sessions":[{"id":"sess-1","agent_name":"coder","workspace_id":"ws-1","workspace_path":"/workspace/project","state":"active","created_at":"2026-04-03T12:00:00Z","updated_at":"2026-04-03T12:00:00Z"}],"agents":[{"name":"coder","provider":"fake","prompt":"hi"}],"skills":[{"name":"review","dir":"/workspace/project/.agh/skills/review","source":"workspace"}]}`,
					), nil
				case req.Method == http.MethodPatch && req.URL.Path == "/api/workspaces/ws-1":
					body, err := io.ReadAll(req.Body)
					if err != nil {
						t.Fatalf("io.ReadAll(workspace update body) error = %v", err)
					}
					if !strings.Contains(string(body), `"name":"beta"`) {
						t.Fatalf("workspace update body = %s, want name", body)
					}
					return newHTTPResponse(
						http.StatusOK,
						`{"workspace":{"id":"ws-1","root_dir":"/workspace/project","name":"beta","created_at":"2026-04-03T12:00:00Z","updated_at":"2026-04-03T12:05:00Z"}}`,
					), nil
				case req.Method == http.MethodDelete && req.URL.Path == "/api/workspaces/ws-1":
					return newHTTPResponse(http.StatusNoContent, ``), nil
				case req.Method == http.MethodGet && req.URL.Path == "/api/agents":
					if got := req.URL.Query().Get("workspace"); got != "alpha" {
						t.Fatalf("agent list workspace query = %q, want %q", got, "alpha")
					}
					return newHTTPResponse(
						http.StatusOK,
						`{"agents":[{"name":"coder","provider":"fake","prompt":"You are coder."}]}`,
					), nil
				case req.Method == http.MethodGet && req.URL.Path == "/api/agents/coder":
					if got := req.URL.Query().Get("workspace"); got != "alpha" {
						t.Fatalf("agent info workspace query = %q, want %q", got, "alpha")
					}
					return newHTTPResponse(
						http.StatusOK,
						`{"agent":{"name":"coder","provider":"fake","prompt":"You are coder."}}`,
					), nil
				case req.Method == http.MethodGet && req.URL.Path == "/api/skills":
					if got := req.URL.Query().Get("workspace"); got != "alpha" {
						t.Fatalf("skill list workspace query = %q, want %q", got, "alpha")
					}
					return newHTTPResponse(
						http.StatusOK,
						`{"skills":[{"name":"review","description":"Review helper","source":"workspace","enabled":true,"dir":"/workspace/project/.agh/skills/review"}]}`,
					), nil
				case req.Method == http.MethodGet && req.URL.Path == "/api/skills/review":
					if got := req.URL.Query().Get("workspace"); got != "alpha" {
						t.Fatalf("skill info workspace query = %q, want %q", got, "alpha")
					}
					return newHTTPResponse(
						http.StatusOK,
						`{"skill":{"name":"review","description":"Review helper","version":"1.0.0","source":"workspace","enabled":true,"dir":"/workspace/project/.agh/skills/review","metadata":{"area":"qa"}}}`,
					), nil
				case req.Method == http.MethodGet && req.URL.Path == "/api/skills/review/content":
					if got := req.URL.Query().Get("workspace"); got != "alpha" {
						t.Fatalf("skill content workspace query = %q, want %q", got, "alpha")
					}
					return newHTTPResponse(http.StatusOK, `{"content":"# Review\n\nUse this skill."}`), nil
				case req.Method == http.MethodGet && req.URL.Path == "/api/skills/review/shadows":
					if got := req.URL.Query().Get("workspace"); got != "alpha" {
						t.Fatalf("skill shadows workspace query = %q, want %q", got, "alpha")
					}
					return newHTTPResponse(
						http.StatusOK,
						`{"name":"review","winner":{"path":"/workspace/project/.agh/skills/review/SKILL.md","tier":"workspace","resolved_to_winner":true,"detected_at":"2026-04-03T12:00:00Z"},"shadows":[{"path":"/workspace/project/.agh/skills/review/SKILL.md","tier":"workspace","resolved_to_winner":true,"detected_at":"2026-04-03T12:00:00Z"}]}`,
					), nil
				case req.Method == http.MethodGet && req.URL.Path == "/api/hooks/catalog":
					if got := req.URL.Query().Get("workspace"); got != "alpha" {
						t.Fatalf("hook catalog workspace query = %q, want %q", got, "alpha")
					}
					if got := req.URL.Query().Get("agent"); got != "coder" {
						t.Fatalf("hook catalog agent query = %q, want %q", got, "coder")
					}
					if got := req.URL.Query().Get("event"); got != "tool.pre_call" {
						t.Fatalf("hook catalog event query = %q, want %q", got, "tool.pre_call")
					}
					if got := req.URL.Query().Get("source"); got != "config" {
						t.Fatalf("hook catalog source query = %q, want %q", got, "config")
					}
					if got := req.URL.Query().Get("mode"); got != "sync" {
						t.Fatalf("hook catalog mode query = %q, want %q", got, "sync")
					}
					return newHTTPResponse(
						http.StatusOK,
						`{"hooks":[{"order":1,"name":"permission-guard","event":"tool.pre_call","source":"config","mode":"sync","priority":10,"executor_kind":"subprocess"}]}`,
					), nil
				case req.Method == http.MethodGet && req.URL.Path == "/api/workspaces/ws-1/hooks/runs":
					if got := req.URL.Query().Get("session"); got != "sess-1" {
						t.Fatalf("hook runs session query = %q, want %q", got, "sess-1")
					}
					if got := req.URL.Query().Get("event"); got != "permission.request" {
						t.Fatalf("hook runs event query = %q, want %q", got, "permission.request")
					}
					if got := req.URL.Query().Get("outcome"); got != "failed" {
						t.Fatalf("hook runs outcome query = %q, want %q", got, "failed")
					}
					if got := req.URL.Query().Get("since"); got != "2026-04-03T11:00:00Z" {
						t.Fatalf("hook runs since query = %q, want %q", got, "2026-04-03T11:00:00Z")
					}
					if got := req.URL.Query().Get("last"); got != "2" {
						t.Fatalf("hook runs last query = %q, want %q", got, "2")
					}
					return newHTTPResponse(
						http.StatusOK,
						`{"runs":[{"hook_name":"permission-guard","event":"permission.request","source":"config","mode":"sync","duration_ms":12,"outcome":"failed","error":"boom","recorded_at":"2026-04-03T12:00:00Z"}]}`,
					), nil
				case req.Method == http.MethodGet && req.URL.Path == "/api/hooks/events":
					if got := req.URL.Query().Get("family"); got != "tool" {
						t.Fatalf("hook events family query = %q, want %q", got, "tool")
					}
					if got := req.URL.Query().Get("sync_only"); got != "true" {
						t.Fatalf("hook events sync_only query = %q, want %q", got, "true")
					}
					return newHTTPResponse(
						http.StatusOK,
						`{"events":[{"event":"tool.pre_call","family":"tool","sync_eligible":true,"payload_schema":"ToolPreCallPayload","patch_schema":"ToolCallPatch"}]}`,
					), nil
				case req.Method == http.MethodGet && req.URL.Path == "/api/logs":
					if got := req.URL.Query().Get("workspace_id"); got != "ws-1" {
						t.Fatalf("logs workspace_id query = %q, want %q", got, "ws-1")
					}
					if got := req.URL.Query().Get("session_id"); got != "sess-1" {
						t.Fatalf("logs session_id query = %q, want %q", got, "sess-1")
					}
					return newHTTPResponse(
						http.StatusOK,
						`{"events":[{"id":"sum-1","session_id":"sess-1","type":"agent_message","agent_name":"coder","timestamp":"2026-04-03T12:00:00Z"}]}`,
					), nil
				case req.Method == http.MethodGet && req.URL.Path == "/api/logs/stream":
					if got := req.URL.Query().Get("workspace_id"); got != "ws-1" {
						t.Fatalf("logs stream workspace_id query = %q, want %q", got, "ws-1")
					}
					if got := req.Header.Get("Last-Event-ID"); got != "cursor-1" {
						t.Fatalf("Last-Event-ID = %q, want %q", got, "cursor-1")
					}
					return newHTTPResponse(http.StatusOK, strings.Join([]string{
						"id: 2026-04-03T12:00:00Z|sum-1",
						"event: agent_message",
						`data: {"id":"sum-1","session_id":"sess-1","type":"agent_message","agent_name":"coder","timestamp":"2026-04-03T12:00:00Z"}`,
						"",
					}, "\n")), nil
				case req.Method == http.MethodGet && req.URL.Path == "/api/status":
					return newHTTPResponse(
						http.StatusOK,
						`{"schema_version":"2026-05-20","generated_at":"2026-04-03T12:00:00Z","daemon":{"status":"running","pid":10,"started_at":"2026-04-03T12:00:00Z","socket":"/tmp/agh.sock","http_host":"localhost","http_port":2123,"active_sessions":1,"total_sessions":1,"version":"dev"},"health":{"status":"ok","uptime_seconds":10,"active_sessions":1,"active_agents":1,"global_db_size_bytes":100,"session_db_size_bytes":200,"version":"dev"}}`,
					), nil
				case req.Method == http.MethodPost && req.URL.Path == "/api/drain":
					return newHTTPResponse(http.StatusOK, `{"state":"draining"}`), nil
				case req.Method == http.MethodPost && req.URL.Path == "/api/undrain":
					return newHTTPResponse(http.StatusOK, `{"state":"active"}`), nil
				case req.Method == http.MethodGet && req.URL.Path == "/api/doctor":
					if got := req.URL.Query()["only"]; len(got) != 1 || got[0] != "provider" {
						t.Fatalf("doctor only query = %#v, want provider", got)
					}
					if got := req.URL.Query()["exclude"]; len(got) != 1 || got[0] != "network" {
						t.Fatalf("doctor exclude query = %#v, want network", got)
					}
					if got := req.URL.Query().Get("quiet"); got != "true" {
						t.Fatalf("doctor quiet query = %q, want true", got)
					}
					return newHTTPResponse(
						http.StatusOK,
						`{"schema_version":"2026-05-20","generated_at":"2026-04-03T12:00:00Z","duration_ms":1,"status":"ok","summary":{"total":0,"counts_by_severity":{}},"items":[]}`,
					), nil
				case req.Method == http.MethodGet && req.URL.Path == "/api/memory/health":
					if got := req.URL.Query().Get("workspace"); got != "/workspace/project" {
						t.Fatalf("memory health workspace = %q, want /workspace/project", got)
					}
					return newHTTPResponse(
						http.StatusOK,
						`{"status":"ok","enabled":true,"configured":true,"global_files":1,"workspace_files":1,"workspace_count":1,"indexed_files":2,"operation_count":3,"last_operation_at":"2026-04-03T12:00:00Z"}`,
					), nil
				case req.Method == http.MethodGet && req.URL.Path == "/api/memory/history":
					if got := req.URL.Query().Get("scope"); got != "workspace" {
						t.Fatalf("memory history scope = %q, want workspace", got)
					}
					if got := req.URL.Query().Get("operation"); got != "memory.write" {
						t.Fatalf("memory history operation = %q, want memory.write", got)
					}
					if got := req.URL.Query().Get("since"); got != "2026-04-03T11:00:00Z" {
						t.Fatalf("memory history since = %q, want 2026-04-03T11:00:00Z", got)
					}
					if got := req.URL.Query().Get("limit"); got != "4" {
						t.Fatalf("memory history limit = %q, want 4", got)
					}
					return newHTTPResponse(
						http.StatusOK,
						`{"operations":[{"id":"memevt_1","operation":"memory.write","scope":"workspace","workspace":"/workspace/project","filename":"memory.md","agent_name":"daemon","summary":"scope=workspace filename=memory.md","timestamp":"2026-04-03T12:00:00Z"}]}`,
					), nil
				case req.Method == http.MethodGet && req.URL.Path == "/api/memory":
					if got := req.URL.Query().Get("scope"); got != "global" {
						t.Fatalf("memory scope query = %q, want %q", got, "global")
					}
					return newHTTPResponse(
						http.StatusOK,
						`{"memories":[{"filename":"memory.md","mod_time":"2026-04-03T12:00:00Z","name":"Memory","description":"desc","type":"user","scope":"global","injection":true}]}`,
					), nil
				case req.Method == http.MethodPost && req.URL.Path == "/api/memory/search":
					var body contract.MemorySearchRequest
					if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
						t.Fatalf("decode memory search request error = %v", err)
					}
					if body.QueryText != "release plan" || body.Scope != memcontract.ScopeWorkspace ||
						body.WorkspaceID != "/workspace/project" || body.TopK != 5 {
						t.Fatalf("memory search body = %#v", body)
					}
					return newHTTPResponse(
						http.StatusOK,
						`{"results":[{"memory":{"filename":"release.md","scope":"workspace","workspace_id":"/workspace/project","type":"project","name":"Release Plan","description":"plan","mod_time":"2026-04-03T12:00:00Z","injection":true},"score":3.4,"snippet":"Ship phases incrementally"}],"recall":{"blocks":null}}`,
					), nil
				case req.Method == http.MethodGet && req.URL.Path == "/api/memory/memory.md":
					if got := req.URL.Query().Get("scope"); got != "global" {
						t.Fatalf("show memory scope query = %q, want %q", got, "global")
					}
					return newHTTPResponse(
						http.StatusOK,
						`{"memory":{"summary":{"filename":"memory.md","name":"Memory","type":"user","scope":"global","mod_time":"2026-04-03T12:00:00Z","injection":true},"content":"hello"}}`,
					), nil
				case req.Method == http.MethodPost && req.URL.Path == "/api/memory":
					var body contract.MemoryCreateRequest
					if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
						t.Fatalf("decode memory create request error = %v", err)
					}
					if body.Scope != memcontract.ScopeGlobal ||
						body.Type != memcontract.TypeUser ||
						body.Name != "Memory" ||
						body.Content != "payload" {
						t.Fatalf("memory create body = %#v", body)
					}
					return newHTTPResponse(
						http.StatusOK,
						`{"decision":{"id":"dec-write","candidate_hash":"sha256:test","op":"add","scope":"global","frontmatter":{"name":"Memory","type":"user"},"confidence":0.9,"source":"rule","decided_at":"2026-04-03T12:00:00Z"},"applied":true}`,
					), nil
				case req.Method == http.MethodDelete && req.URL.Path == "/api/memory/memory.md":
					if got := req.URL.Query().Get("scope"); got != "workspace" {
						t.Fatalf("delete memory scope query = %q, want %q", got, "workspace")
					}
					if got := req.URL.Query().Get("workspace_id"); got != "/workspace/project" {
						t.Fatalf("delete memory workspace_id query = %q, want %q", got, "/workspace/project")
					}
					return newHTTPResponse(
						http.StatusOK,
						`{"decision":{"id":"dec-delete","candidate_hash":"sha256:test","op":"delete","scope":"workspace","frontmatter":{"name":"Memory","type":"user"},"confidence":0.9,"source":"rule","decided_at":"2026-04-03T12:00:00Z"},"applied":true}`,
					), nil
				case req.Method == http.MethodPost && req.URL.Path == "/api/memory/dreams/trigger":
					var body contract.MemoryDreamTriggerRequest
					if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
						t.Fatalf("decode memory dream trigger request error = %v", err)
					}
					if body.Scope != memcontract.ScopeWorkspace || body.WorkspaceID != "/workspace/project" {
						t.Fatalf("memory dream trigger body = %#v", body)
					}
					return newHTTPResponse(
						http.StatusOK,
						`{"dream":{"id":"dream-1","status":"running","scope":"workspace","workspace_id":"/workspace/project","candidate_count":0,"promoted_count":0,"started_at":"2026-04-03T12:00:00Z"},"triggered":true}`,
					), nil
				case req.Method == http.MethodPost && req.URL.Path == "/api/memory/reindex":
					body, err := io.ReadAll(req.Body)
					if err != nil {
						t.Fatalf("io.ReadAll(memory reindex body) error = %v", err)
					}
					if !strings.Contains(string(body), `"scope":"workspace"`) ||
						!strings.Contains(string(body), `"workspace_id":"/workspace/project"`) {
						t.Fatalf("memory reindex body = %s, want scope/workspace", body)
					}
					return newHTTPResponse(
						http.StatusOK,
						`{"indexed_files":2,"scope":"workspace","workspace_id":"/workspace/project","completed_at":"2026-04-03T12:00:00Z"}`,
					), nil
				default:
					return newHTTPResponse(http.StatusNotFound, `{"error":"missing"}`), nil
				}
			}),
		},
	}

	ctx := context.Background()

	status, err := client.DaemonStatus(ctx)
	if err != nil || status.Status != "running" {
		t.Fatalf("DaemonStatus() = %#v, %v", status, err)
	}
	drainStatus, err := client.Drain(ctx)
	if err != nil || drainStatus.State != contract.DrainStateDraining {
		t.Fatalf("Drain() = %#v, %v", drainStatus, err)
	}
	activeStatus, err := client.Undrain(ctx)
	if err != nil || activeStatus.State != contract.DrainStateActive {
		t.Fatalf("Undrain() = %#v, %v", activeStatus, err)
	}

	sessions, err := client.ListSessions(ctx, SessionListQuery{Workspace: "ws-1"})
	if err != nil || len(sessions.Sessions) != 1 {
		t.Fatalf("ListSessions() = %#v, %v", sessions, err)
	}
	if sessions.Page.NextCursor != "cursor-next" || !sessions.Page.HasMore ||
		sessions.Page.Total != 3 || sessions.Page.Limit != 1 {
		t.Fatalf("ListSessions().Page = %#v, want decoded continuation metadata", sessions.Page)
	}

	createdSession, err := client.CreateSession(ctx, CreateSessionRequest{
		AgentName: "coder",
		Workspace: "ws-1",
	})
	if err != nil || createdSession.ID != "sess-new" {
		t.Fatalf("CreateSession() = %#v, %v", createdSession, err)
	}

	sessionInfo, err := client.GetSession(ctx, "sess-1")
	if err != nil || sessionInfo.ID != "sess-1" {
		t.Fatalf("GetSession() = %#v, %v", sessionInfo, err)
	}

	if err := client.StopSession(ctx, "sess-1"); err != nil {
		t.Fatalf("StopSession() error = %v", err)
	}

	resumed, err := client.ResumeSession(ctx, "sess-1")
	if err != nil || resumed.ID != "sess-1" {
		t.Fatalf("ResumeSession() = %#v, %v", resumed, err)
	}

	promptEvents, err := client.PromptSession(ctx, "sess-1", "hello")
	if err != nil || len(promptEvents) != 1 || promptEvents[0].Text != "hello back" {
		t.Fatalf("PromptSession() = %#v, %v", promptEvents, err)
	}

	sessionEvents, err := client.SessionEvents(ctx, "sess-1", SessionEventQuery{Type: "tool_call"})
	if err != nil || len(sessionEvents) != 1 || sessionEvents[0].Type != "tool_call" {
		t.Fatalf("SessionEvents() = %#v, %v", sessionEvents, err)
	}

	history, err := client.SessionHistory(ctx, "sess-1", SessionEventQuery{Last: 2})
	if err != nil || len(history) != 1 {
		t.Fatalf("SessionHistory() = %#v, %v", history, err)
	}

	agents, err := client.ListAgents(ctx, AgentQuery{Workspace: "alpha"})
	if err != nil || len(agents) != 1 {
		t.Fatalf("ListAgents() = %#v, %v", agents, err)
	}

	agent, err := client.GetAgent(ctx, "coder", AgentQuery{Workspace: "alpha"})
	if err != nil || agent.Name != "coder" {
		t.Fatalf("GetAgent() = %#v, %v", agent, err)
	}

	skills, err := client.ListSkills(ctx, SkillQuery{Workspace: "alpha"})
	if err != nil || len(skills) != 1 {
		t.Fatalf("ListSkills() = %#v, %v", skills, err)
	}

	skill, err := client.GetSkill(ctx, "review", SkillQuery{Workspace: "alpha"})
	if err != nil || skill.Name != "review" {
		t.Fatalf("GetSkill() = %#v, %v", skill, err)
	}

	content, err := client.GetSkillContent(ctx, "review", SkillQuery{Workspace: "alpha"})
	if err != nil || !strings.Contains(content, "Use this skill.") {
		t.Fatalf("GetSkillContent() = %q, %v", content, err)
	}

	shadows, err := client.GetSkillShadows(ctx, "review", SkillQuery{Workspace: "alpha"})
	if err != nil || shadows.Winner.Tier != "workspace" {
		t.Fatalf("GetSkillShadows() = %#v, %v", shadows, err)
	}

	hookCatalog, err := client.HookCatalog(ctx, HookCatalogQuery{
		Workspace: "alpha",
		Agent:     "coder",
		Event:     "tool.pre_call",
		Source:    "config",
		Mode:      "sync",
	})
	if err != nil || len(hookCatalog) != 1 || hookCatalog[0].ExecutorKind != "subprocess" {
		t.Fatalf("HookCatalog() = %#v, %v", hookCatalog, err)
	}

	hookRuns, err := client.HookRuns(ctx, "ws-1", HookRunsQuery{
		Session: "sess-1",
		Event:   "permission.request",
		Outcome: "failed",
		Since:   "2026-04-03T11:00:00Z",
		Last:    2,
	})
	if err != nil || len(hookRuns) != 1 || hookRuns[0].Outcome != "failed" {
		t.Fatalf("HookRuns() = %#v, %v", hookRuns, err)
	}

	hookEvents, err := client.HookEvents(ctx, HookEventsQuery{
		Family:   "tool",
		SyncOnly: true,
	})
	if err != nil || len(hookEvents) != 1 || hookEvents[0].Family != "tool" {
		t.Fatalf("HookEvents() = %#v, %v", hookEvents, err)
	}

	createdWorkspace, err := client.CreateWorkspace(ctx, WorkspaceCreateRequest{RootDir: "/workspace/project"})
	if err != nil || createdWorkspace.ID != "ws-1" {
		t.Fatalf("CreateWorkspace() = %#v, %v", createdWorkspace, err)
	}

	workspaces, err := client.ListWorkspaces(ctx)
	if err != nil || len(workspaces) != 1 {
		t.Fatalf("ListWorkspaces() = %#v, %v", workspaces, err)
	}

	workspaceDetail, err := client.GetWorkspace(ctx, "alpha")
	if err != nil || workspaceDetail.Workspace.ID != "ws-1" || len(workspaceDetail.Skills) != 1 {
		t.Fatalf("GetWorkspace() = %#v, %v", workspaceDetail, err)
	}

	workspaceDetailByPath, err := client.GetWorkspace(ctx, "/workspace/project")
	if err != nil || workspaceDetailByPath.Workspace.ID != "ws-1" || len(workspaceDetailByPath.Skills) != 1 {
		t.Fatalf("GetWorkspace(path) = %#v, %v", workspaceDetailByPath, err)
	}

	updatedWorkspace, err := client.UpdateWorkspace(ctx, "ws-1", WorkspaceUpdateRequest{Name: new("beta")})
	if err != nil || updatedWorkspace.Name != "beta" {
		t.Fatalf("UpdateWorkspace() = %#v, %v", updatedWorkspace, err)
	}

	if err := client.DeleteWorkspace(ctx, "ws-1"); err != nil {
		t.Fatalf("DeleteWorkspace() error = %v", err)
	}

	events, err := client.ListLogs(ctx, LogsListQuery{WorkspaceRef: "ws-1", SessionID: "sess-1"})
	if err != nil || len(events) != 1 {
		t.Fatalf("ListLogs() = %#v, %v", events, err)
	}

	var streamed []SSEEvent
	if err := client.StreamLogs(
		ctx,
		LogsListQuery{WorkspaceRef: "ws-1"},
		"cursor-1",
		func(event SSEEvent) error {
			streamed = append(streamed, event)
			return nil
		},
	); err != nil {
		t.Fatalf("StreamLogs() error = %v", err)
	}
	if len(streamed) != 1 || streamed[0].Event != "agent_message" {
		t.Fatalf("streamed = %#v, want one event", streamed)
	}

	runtimeStatus, err := client.Status(ctx)
	if err != nil || runtimeStatus.Health.Status != "ok" {
		t.Fatalf("Status() = %#v, %v", runtimeStatus, err)
	}

	doctor, err := client.Doctor(ctx, DoctorQuery{
		Only:    []string{"provider"},
		Exclude: []string{"network"},
		Quiet:   true,
	})
	if err != nil || doctor.Status != "ok" {
		t.Fatalf("Doctor() = %#v, %v", doctor, err)
	}

	memoryHealth, err := client.MemoryHealth(ctx, "/workspace/project")
	if err != nil || memoryHealth.Status != "ok" || memoryHealth.OperationCount != 3 {
		t.Fatalf("MemoryHealth() = %#v, %v", memoryHealth, err)
	}

	memoryHistoryRecords, err := client.MemoryHistory(ctx, MemoryHistoryQuery{
		Scope:       memcontract.ScopeWorkspace,
		WorkspaceID: "/workspace/project",
		Operation:   "memory.write",
		Since:       time.Date(2026, 4, 3, 11, 0, 0, 0, time.UTC),
		Limit:       4,
	})
	if err != nil || len(memoryHistoryRecords) != 1 || memoryHistoryRecords[0].Operation != "memory.write" {
		t.Fatalf("MemoryHistory() = %#v, %v", memoryHistoryRecords, err)
	}

	memories, err := client.ListMemory(ctx, MemoryListQuery{
		MemorySelectorQuery: MemorySelectorQuery{Scope: memcontract.ScopeGlobal},
	})
	if err != nil || len(memories.Memories) != 1 {
		t.Fatalf("ListMemory() = %#v, %v", memories, err)
	}

	searchResults, err := client.SearchMemory(ctx, MemorySearchRequest{
		QueryText:   "release plan",
		Scope:       memcontract.ScopeWorkspace,
		WorkspaceID: "/workspace/project",
		TopK:        5,
	})
	if err != nil || len(searchResults.Results) != 1 || searchResults.Results[0].Memory.Filename != "release.md" {
		t.Fatalf("SearchMemory() = %#v, %v", searchResults, err)
	}

	memoryRecord, err := client.ShowMemory(ctx, "memory.md", MemorySelectorQuery{Scope: memcontract.ScopeGlobal})
	if err != nil || !strings.Contains(memoryRecord.Memory.Content, "hello") {
		t.Fatalf("ShowMemory() = %#v, %v", memoryRecord, err)
	}

	written, err := client.CreateMemory(ctx, MemoryCreateRequest{
		Scope:   memcontract.ScopeGlobal,
		Type:    memcontract.TypeUser,
		Name:    "Memory",
		Content: "payload",
	})
	if err != nil || !written.Applied {
		t.Fatalf("CreateMemory() = %#v, %v", written, err)
	}

	deleted, err := client.DeleteMemory(ctx, "memory.md", MemorySelectorQuery{
		Scope:       memcontract.ScopeWorkspace,
		WorkspaceID: "/workspace/project",
	})
	if err != nil || !deleted.Applied {
		t.Fatalf("DeleteMemory() = %#v, %v", deleted, err)
	}

	reindexed, err := client.ReindexMemory(ctx, MemoryReindexRequest{
		Scope:       memcontract.ScopeWorkspace,
		WorkspaceID: "/workspace/project",
	})
	if err != nil || reindexed.IndexedFiles != 2 {
		t.Fatalf("ReindexMemory() = %#v, %v", reindexed, err)
	}

	dreamed, err := client.TriggerMemoryDream(ctx, MemoryDreamTriggerRequest{
		Scope:       memcontract.ScopeWorkspace,
		WorkspaceID: "/workspace/project",
	})
	if err != nil || !dreamed.Triggered {
		t.Fatalf("TriggerMemoryDream() = %#v, %v", dreamed, err)
	}
}

func TestSessionWorkspaceRefUsesDirectLookup(t *testing.T) {
	t.Parallel()

	t.Run("Should never scan the paged session catalog", func(t *testing.T) {
		t.Parallel()

		var paths []string
		client := &unixSocketClient{
			socketPath: "/tmp/agh.sock",
			httpClient: &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				paths = append(paths, req.URL.Path)
				if req.URL.Path != "/api/sessions/sess-target" {
					t.Fatalf("request path = %q, want direct session lookup", req.URL.Path)
				}
				return newHTTPResponse(
					http.StatusOK,
					`{"session":{"id":"sess-target","workspace_id":"ws-target","state":"active","created_at":"2026-04-03T12:00:00Z","updated_at":"2026-04-03T12:00:00Z"}}`,
				), nil
			})},
		}
		workspaceID, err := client.sessionWorkspaceRef(t.Context(), "sess-target")
		if err != nil {
			t.Fatalf("sessionWorkspaceRef() error = %v", err)
		}
		if workspaceID != "ws-target" {
			t.Fatalf("sessionWorkspaceRef() = %q, want ws-target", workspaceID)
		}
		if len(paths) != 1 || paths[0] == "/api/sessions" {
			t.Fatalf("sessionWorkspaceRef() paths = %#v, want one by-id request and no catalog scan", paths)
		}
	})
}

func TestUnixSocketClientRepairSession(t *testing.T) {
	t.Parallel()

	t.Run("Should repair session with dry run and force", func(t *testing.T) {
		t.Parallel()

		client := &unixSocketClient{
			socketPath: "/tmp/agh.sock",
			httpClient: &http.Client{
				Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
					if req.Method == http.MethodGet && req.URL.Path == "/api/sessions/sess-1" {
						return newHTTPResponse(
							http.StatusOK,
							`{"session":{"id":"sess-1","agent_name":"coder","workspace_id":"ws-1","workspace_path":"/tmp","state":"stopped","created_at":"2026-04-03T12:00:00Z","updated_at":"2026-04-03T12:00:00Z"}}`,
						), nil
					}
					if req.Method != http.MethodPost || req.URL.Path != "/api/workspaces/ws-1/sessions/sess-1/repair" {
						return newHTTPResponse(http.StatusNotFound, `{"error":"missing"}`), nil
					}
					if got := req.URL.Query().Get("dry_run"); got != "true" {
						t.Fatalf("repair dry_run query = %q, want true", got)
					}
					if got := req.URL.Query().Get("force"); got != "true" {
						t.Fatalf("repair force query = %q, want true", got)
					}
					return newHTTPResponse(
						http.StatusOK,
						`{"repair":{"session_id":"sess-1","persisted":false,"issues":[],"actions":[{"code":"append_terminal_error","turn_id":"turn-1","persisted":false}]}}`,
					), nil
				}),
			},
		}

		repaired, err := client.RepairSession(
			context.Background(),
			"sess-1",
			SessionRepairQuery{DryRun: true, Force: true},
		)
		if err != nil || repaired.SessionID != "sess-1" || len(repaired.Actions) != 1 {
			t.Fatalf("RepairSession() = %#v, %v", repaired, err)
		}
	})
}

func TestUnixSocketClientSkillMarketplaceLifecycleMethods(t *testing.T) {
	t.Parallel()

	t.Run("Should map skill marketplace endpoints", func(t *testing.T) {
		t.Parallel()

		client := &unixSocketClient{
			socketPath: "/tmp/agh.sock",
			httpClient: &http.Client{
				Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
					switch {
					case req.Method == http.MethodPost && req.URL.Path == "/api/skills/marketplace/install":
						var body SkillMarketplaceInstallRequest
						if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
							t.Fatalf("decode skill marketplace install request error = %v", err)
						}
						if body.Slug != "review" || body.Version != "1.2.0" {
							t.Fatalf("skill marketplace install body = %#v, want review 1.2.0", body)
						}
						return newHTTPResponse(
							http.StatusOK,
							`{"skill":{"name":"review","slug":"review","version":"1.2.0","registry":"clawhub","path":"/tmp/review","hash":"sha256:review","status":"installed"}}`,
						), nil
					case req.Method == http.MethodPost && req.URL.Path == "/api/skills/marketplace/update":
						var body SkillMarketplaceUpdateRequest
						if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
							t.Fatalf("decode skill marketplace update request error = %v", err)
						}
						if body.Name != "review" || !body.CheckOnly || body.All {
							t.Fatalf("skill marketplace update body = %#v, want review check-only", body)
						}
						return newHTTPResponse(
							http.StatusOK,
							`{"skills":[{"name":"review","slug":"review","current_version":"1.0.0","latest_version":"1.2.0","path":"/tmp/review","status":"update available"}]}`,
						), nil
					case req.Method == http.MethodDelete && req.URL.Path == "/api/skills/marketplace/review":
						return newHTTPResponse(
							http.StatusOK,
							`{"skill":{"name":"review","slug":"review","path":"/tmp/review","status":"removed"}}`,
						), nil
					default:
						return newHTTPResponse(http.StatusNotFound, `{"error":"missing"}`), nil
					}
				}),
			},
		}

		ctx := context.Background()
		installed, err := client.InstallSkillMarketplace(ctx, SkillMarketplaceInstallRequest{
			Slug:    "review",
			Version: "1.2.0",
		})
		if err != nil || installed.Status != "installed" {
			t.Fatalf("InstallSkillMarketplace() = %#v, %v", installed, err)
		}
		updates, err := client.UpdateSkillMarketplace(ctx, SkillMarketplaceUpdateRequest{
			Name:      "review",
			CheckOnly: true,
		})
		if err != nil || len(updates) != 1 || updates[0].Status != "update available" {
			t.Fatalf("UpdateSkillMarketplace() = %#v, %v", updates, err)
		}
		removed, err := client.RemoveSkillMarketplace(ctx, " review ")
		if err != nil || removed.Status != "removed" {
			t.Fatalf("RemoveSkillMarketplace() = %#v, %v", removed, err)
		}
	})
}

func TestUnixSocketClientExtensionMethods(t *testing.T) {
	t.Parallel()

	client := &unixSocketClient{
		socketPath: "/tmp/agh.sock",
		httpClient: &http.Client{
			Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				switch {
				case req.Method == http.MethodGet && req.URL.Path == "/api/extensions":
					return newHTTPResponse(
						http.StatusOK,
						`{"extensions":[{"name":"ext-a","version":"0.1.0","type":"resource","source":"user","enabled":true,"state":"active","daemon_running":true}]}`,
					), nil
				case req.Method == http.MethodPost && req.URL.Path == "/api/extensions":
					body, err := io.ReadAll(req.Body)
					if err != nil {
						t.Fatalf("io.ReadAll(extension install body) error = %v", err)
					}
					if !strings.Contains(string(body), `"path":"/tmp/ext-a"`) ||
						!strings.Contains(string(body), `"checksum":"abc123"`) {
						t.Fatalf("extension install body = %s, want path and checksum", body)
					}
					return newHTTPResponse(
						http.StatusCreated,
						`{"extension":{"name":"ext-a","version":"0.1.0","type":"resource","source":"user","enabled":true,"state":"active","daemon_running":true}}`,
					), nil
				case req.Method == http.MethodPost && req.URL.Path == "/api/extensions/ext-a/enable":
					return newHTTPResponse(
						http.StatusOK,
						`{"extension":{"name":"ext-a","version":"0.1.0","type":"resource","source":"user","enabled":true,"state":"active","daemon_running":true}}`,
					), nil
				case req.Method == http.MethodPost && req.URL.Path == "/api/extensions/ext-a/disable":
					return newHTTPResponse(
						http.StatusOK,
						`{"extension":{"name":"ext-a","version":"0.1.0","type":"resource","source":"user","enabled":false,"state":"disabled","daemon_running":true}}`,
					), nil
				case req.Method == http.MethodGet && req.URL.Path == "/api/extensions/ext-a":
					return newHTTPResponse(
						http.StatusOK,
						`{"extension":{"name":"ext-a","version":"0.1.0","type":"resource","source":"user","enabled":true,"state":"active","daemon_running":true}}`,
					), nil
				default:
					return newHTTPResponse(http.StatusNotFound, `{"error":"missing"}`), nil
				}
			}),
		},
	}

	ctx := context.Background()

	listed, err := client.ListExtensions(ctx)
	if err != nil || len(listed) != 1 || listed[0].Name != "ext-a" {
		t.Fatalf("ListExtensions() = %#v, %v", listed, err)
	}

	installed, err := client.InstallExtension(ctx, InstallExtensionRequest{
		Path:     "/tmp/ext-a",
		Checksum: "abc123",
	})
	if err != nil || installed.Name != "ext-a" {
		t.Fatalf("InstallExtension() = %#v, %v", installed, err)
	}

	enabled, err := client.EnableExtension(ctx, " ext-a ")
	if err != nil || !enabled.Enabled {
		t.Fatalf("EnableExtension() = %#v, %v", enabled, err)
	}

	disabled, err := client.DisableExtension(ctx, " ext-a ")
	if err != nil || disabled.Enabled {
		t.Fatalf("DisableExtension() = %#v, %v", disabled, err)
	}

	status, err := client.ExtensionStatus(ctx, "ext-a")
	if err != nil || status.State != "active" {
		t.Fatalf("ExtensionStatus() = %#v, %v", status, err)
	}
}

func TestUnixSocketClientAutomationMethods(t *testing.T) {
	t.Parallel()

	startedAt := time.Date(2026, 4, 11, 12, 0, 0, 0, time.UTC)
	endedAt := startedAt.Add(2 * time.Minute)

	client := &unixSocketClient{
		socketPath: "/tmp/agh.sock",
		httpClient: &http.Client{
			Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				switch {
				case req.Method == http.MethodGet &&
					req.URL.Path == "/api/workspaces/ws-alpha/automation/suggestions":
					if got := req.URL.Query().Get("status"); got != "pending" {
						t.Fatalf("suggestion status query = %q, want pending", got)
					}
					body := mustJSON(t, contract.AutomationSuggestionsResponse{
						Suggestions: []contract.AutomationSuggestionPayload{sampleAutomationSuggestionRecord()},
					})
					return newHTTPResponse(http.StatusOK, string(body)), nil
				case req.Method == http.MethodPost &&
					req.URL.Path == "/api/workspaces/ws-alpha/automation/suggestions/suggestion-1/accept":
					accepted := sampleAutomationSuggestionRecord()
					accepted.Status = automationpkg.SuggestionStatusAccepted
					body := mustJSON(t, contract.AutomationSuggestionAcceptanceResponse{
						Suggestion: accepted,
						Job:        accepted.Payload,
					})
					return newHTTPResponse(http.StatusOK, string(body)), nil
				case req.Method == http.MethodPost &&
					req.URL.Path == "/api/workspaces/ws-alpha/automation/suggestions/suggestion-1/dismiss":
					dismissed := sampleAutomationSuggestionRecord()
					dismissed.Status = automationpkg.SuggestionStatusDismissed
					body := mustJSON(t, contract.AutomationSuggestionResponse{Suggestion: dismissed})
					return newHTTPResponse(http.StatusOK, string(body)), nil
				case req.Method == http.MethodGet && req.URL.Path == "/api/automation/jobs":
					if got := req.URL.Query().Get("scope"); got != "workspace" {
						t.Fatalf("job scope query = %q, want %q", got, "workspace")
					}
					if got := req.URL.Query().Get("workspace_id"); got != "ws-alpha" {
						t.Fatalf("job workspace_id query = %q, want %q", got, "ws-alpha")
					}
					if got := req.URL.Query().Get("source"); got != "package" {
						t.Fatalf("job source query = %q, want %q", got, "package")
					}
					if got := req.URL.Query().Get("enabled"); got != "false" {
						t.Fatalf("job enabled query = %q, want %q", got, "false")
					}
					if got := req.URL.Query().Get("loop"); got != "triage" {
						t.Fatalf("job loop query = %q, want %q", got, "triage")
					}
					if got := req.URL.Query().Get("q"); got != "digest" {
						t.Fatalf("job q query = %q, want %q", got, "digest")
					}
					if got := req.URL.Query().Get("cursor"); got != "job-cursor" {
						t.Fatalf("job cursor query = %q, want %q", got, "job-cursor")
					}
					if got := req.URL.Query().Get("limit"); got != "3" {
						t.Fatalf("job limit query = %q, want %q", got, "3")
					}
					return newHTTPResponse(
						http.StatusOK,
						`{"jobs":[{"id":"job-1","scope":"workspace","workspace_id":"ws-alpha","name":"nightly","agent_name":"coder","prompt":"review repo","schedule":{"mode":"every","interval":"1h"},"enabled":true,"retry":{"strategy":"none"},"fire_limit":{"max":12,"window":"1h"},"source":"dynamic","created_at":"2026-04-11T12:00:00Z","updated_at":"2026-04-11T12:00:00Z"}],"page":{"next_cursor":"job-next","has_more":true,"total":4,"limit":3}}`,
					), nil
				case req.Method == http.MethodPost && req.URL.Path == "/api/automation/jobs":
					body, err := io.ReadAll(req.Body)
					if err != nil {
						t.Fatalf("io.ReadAll(job create body) error = %v", err)
					}
					if !strings.Contains(string(body), `"workspace_id":"ws-alpha"`) ||
						!strings.Contains(string(body), `"mode":"every"`) {
						t.Fatalf("job create body = %s, want workspace_id and schedule", body)
					}
					return newHTTPResponse(
						http.StatusCreated,
						`{"job":{"id":"job-created","scope":"workspace","workspace_id":"ws-alpha","name":"nightly","agent_name":"coder","prompt":"review repo","schedule":{"mode":"every","interval":"1h"},"enabled":true,"retry":{"strategy":"none"},"fire_limit":{"max":12,"window":"1h"},"source":"dynamic","created_at":"2026-04-11T12:00:00Z","updated_at":"2026-04-11T12:00:00Z"}}`,
					), nil
				case req.Method == http.MethodGet && req.URL.Path == "/api/automation/jobs/job-created":
					return newHTTPResponse(
						http.StatusOK,
						`{"job":{"id":"job-created","scope":"workspace","workspace_id":"ws-alpha","name":"nightly","agent_name":"coder","prompt":"review repo","schedule":{"mode":"every","interval":"1h"},"enabled":true,"retry":{"strategy":"none"},"fire_limit":{"max":12,"window":"1h"},"source":"dynamic","created_at":"2026-04-11T12:00:00Z","updated_at":"2026-04-11T12:00:00Z","next_run":"2026-04-11T13:00:00Z"}}`,
					), nil
				case req.Method == http.MethodPatch && req.URL.Path == "/api/automation/jobs/job-created":
					body, err := io.ReadAll(req.Body)
					if err != nil {
						t.Fatalf("io.ReadAll(job update body) error = %v", err)
					}
					if !strings.Contains(string(body), `"enabled":false`) ||
						!strings.Contains(string(body), `"prompt":"review now"`) {
						t.Fatalf("job update body = %s, want enabled and prompt", body)
					}
					return newHTTPResponse(
						http.StatusOK,
						`{"job":{"id":"job-created","scope":"workspace","workspace_id":"ws-alpha","name":"nightly","agent_name":"coder","prompt":"review now","schedule":{"mode":"every","interval":"1h"},"enabled":false,"retry":{"strategy":"none"},"fire_limit":{"max":12,"window":"1h"},"source":"dynamic","created_at":"2026-04-11T12:00:00Z","updated_at":"2026-04-11T12:05:00Z"}}`,
					), nil
				case req.Method == http.MethodDelete && req.URL.Path == "/api/automation/jobs/job-created":
					return newHTTPResponse(http.StatusNoContent, ``), nil
				case req.Method == http.MethodPost && req.URL.Path == "/api/automation/jobs/job-created/trigger":
					return newHTTPResponse(
						http.StatusOK,
						`{"run":{"id":"run-job","job_id":"job-created","session_id":"sess-job","status":"completed","attempt":1,"started_at":"2026-04-11T12:00:00Z","ended_at":"2026-04-11T12:02:00Z"}}`,
					), nil
				case req.Method == http.MethodGet && req.URL.Path == "/api/automation/jobs/job-created/runs":
					if got := req.URL.Query().Get("status"); got != "completed" {
						t.Fatalf("job runs status query = %q, want %q", got, "completed")
					}
					if got := req.URL.Query().Get("since"); got != "2026-04-11T11:00:00Z" {
						t.Fatalf("job runs since query = %q, want %q", got, "2026-04-11T11:00:00Z")
					}
					if got := req.URL.Query().Get("until"); got != "2026-04-11T13:00:00Z" {
						t.Fatalf("job runs until query = %q, want %q", got, "2026-04-11T13:00:00Z")
					}
					if got := req.URL.Query().Get("limit"); got != "2" {
						t.Fatalf("job runs limit query = %q, want %q", got, "2")
					}
					return newHTTPResponse(
						http.StatusOK,
						`{"runs":[{"id":"run-job","job_id":"job-created","session_id":"sess-job","status":"completed","attempt":1,"started_at":"2026-04-11T12:00:00Z","ended_at":"2026-04-11T12:02:00Z"}]}`,
					), nil
				case req.Method == http.MethodGet && req.URL.Path == "/api/automation/triggers":
					if got := req.URL.Query().Get("scope"); got != "workspace" {
						t.Fatalf("trigger scope query = %q, want %q", got, "workspace")
					}
					if got := req.URL.Query().Get("workspace_id"); got != "ws-alpha" {
						t.Fatalf("trigger workspace_id query = %q, want %q", got, "ws-alpha")
					}
					if got := req.URL.Query().Get("event"); got != "webhook" {
						t.Fatalf("trigger event query = %q, want %q", got, "webhook")
					}
					if got := req.URL.Query().Get("source"); got != "package" {
						t.Fatalf("trigger source query = %q, want %q", got, "package")
					}
					if got := req.URL.Query().Get("enabled"); got != "true" {
						t.Fatalf("trigger enabled query = %q, want %q", got, "true")
					}
					if got := req.URL.Query().Get("loop"); got != "triage" {
						t.Fatalf("trigger loop query = %q, want %q", got, "triage")
					}
					if got := req.URL.Query().Get("q"); got != "review" {
						t.Fatalf("trigger q query = %q, want %q", got, "review")
					}
					if got := req.URL.Query().Get("cursor"); got != "trigger-cursor" {
						t.Fatalf("trigger cursor query = %q, want %q", got, "trigger-cursor")
					}
					if got := req.URL.Query().Get("limit"); got != "2" {
						t.Fatalf("trigger limit query = %q, want %q", got, "2")
					}
					return newHTTPResponse(
						http.StatusOK,
						`{"triggers":[{"id":"trg-1","scope":"workspace","workspace_id":"ws-alpha","name":"deploy-review","agent_name":"coder","prompt":"review {{ index .Data \"payload\" }}","event":"webhook","filter":{"data.branch":"main"},"enabled":true,"retry":{"strategy":"none"},"fire_limit":{"max":12,"window":"1h"},"source":"dynamic","webhook_id":"wbh_123","endpoint_slug":"deploy-review","created_at":"2026-04-11T12:00:00Z","updated_at":"2026-04-11T12:00:00Z"}],"page":{"next_cursor":"trigger-next","has_more":true,"total":3,"limit":2}}`,
					), nil
				case req.Method == http.MethodPost && req.URL.Path == "/api/automation/triggers":
					body, err := io.ReadAll(req.Body)
					if err != nil {
						t.Fatalf("io.ReadAll(trigger create body) error = %v", err)
					}
					if !strings.Contains(string(body), `"event":"webhook"`) ||
						!strings.Contains(string(body), `"data.branch":"main"`) {
						t.Fatalf("trigger create body = %s, want event and filter", body)
					}
					return newHTTPResponse(
						http.StatusCreated,
						`{"trigger":{"id":"trg-created","scope":"workspace","workspace_id":"ws-alpha","name":"deploy-review","agent_name":"coder","prompt":"review {{ index .Data \"payload\" }}","event":"webhook","filter":{"data.branch":"main"},"enabled":true,"retry":{"strategy":"none"},"fire_limit":{"max":12,"window":"1h"},"source":"dynamic","webhook_id":"wbh_123","endpoint_slug":"deploy-review","created_at":"2026-04-11T12:00:00Z","updated_at":"2026-04-11T12:00:00Z"}}`,
					), nil
				case req.Method == http.MethodGet && req.URL.Path == "/api/automation/triggers/trg-created":
					return newHTTPResponse(
						http.StatusOK,
						`{"trigger":{"id":"trg-created","scope":"workspace","workspace_id":"ws-alpha","name":"deploy-review","agent_name":"coder","prompt":"review {{ index .Data \"payload\" }}","event":"webhook","filter":{"data.branch":"main"},"enabled":true,"retry":{"strategy":"none"},"fire_limit":{"max":12,"window":"1h"},"source":"dynamic","webhook_id":"wbh_123","endpoint_slug":"deploy-review","created_at":"2026-04-11T12:00:00Z","updated_at":"2026-04-11T12:00:00Z"}}`,
					), nil
				case req.Method == http.MethodPatch && req.URL.Path == "/api/automation/triggers/trg-created":
					body, err := io.ReadAll(req.Body)
					if err != nil {
						t.Fatalf("io.ReadAll(trigger update body) error = %v", err)
					}
					if !strings.Contains(string(body), `"prompt":"inspect {{ index .Data \"payload\" }}"`) ||
						!strings.Contains(string(body), `"enabled":false`) {
						t.Fatalf("trigger update body = %s, want prompt and enabled", body)
					}
					return newHTTPResponse(
						http.StatusOK,
						`{"trigger":{"id":"trg-created","scope":"workspace","workspace_id":"ws-alpha","name":"deploy-review","agent_name":"coder","prompt":"inspect {{ index .Data \"payload\" }}","event":"webhook","filter":{"data.branch":"main"},"enabled":false,"retry":{"strategy":"none"},"fire_limit":{"max":12,"window":"1h"},"source":"dynamic","webhook_id":"wbh_123","endpoint_slug":"deploy-review","created_at":"2026-04-11T12:00:00Z","updated_at":"2026-04-11T12:05:00Z"}}`,
					), nil
				case req.Method == http.MethodDelete && req.URL.Path == "/api/automation/triggers/trg-created":
					return newHTTPResponse(http.StatusNoContent, ``), nil
				case req.Method == http.MethodGet && req.URL.Path == "/api/automation/triggers/trg-created/runs":
					if got := req.URL.Query().Get("status"); got != "completed" {
						t.Fatalf("trigger runs status query = %q, want %q", got, "completed")
					}
					if got := req.URL.Query().Get("limit"); got != "1" {
						t.Fatalf("trigger runs limit query = %q, want %q", got, "1")
					}
					return newHTTPResponse(
						http.StatusOK,
						`{"runs":[{"id":"run-trigger","trigger_id":"trg-created","session_id":"sess-trigger","status":"completed","attempt":1,"started_at":"2026-04-11T12:00:00Z","ended_at":"2026-04-11T12:02:00Z"}]}`,
					), nil
				case req.Method == http.MethodGet && req.URL.Path == "/api/automation/runs":
					if got := req.URL.Query().Get("job_id"); got != "job-created" {
						t.Fatalf("runs job_id query = %q, want %q", got, "job-created")
					}
					if got := req.URL.Query().Get("trigger_id"); got != "trg-created" {
						t.Fatalf("runs trigger_id query = %q, want %q", got, "trg-created")
					}
					if got := req.URL.Query().Get("status"); got != "completed" {
						t.Fatalf("runs status query = %q, want %q", got, "completed")
					}
					if got := req.URL.Query().Get("since"); got != "2026-04-11T12:00:00Z" {
						t.Fatalf("runs since query = %q, want %q", got, "2026-04-11T12:00:00Z")
					}
					if got := req.URL.Query().Get("until"); got != "2026-04-11T13:00:00Z" {
						t.Fatalf("runs until query = %q, want %q", got, "2026-04-11T13:00:00Z")
					}
					if got := req.URL.Query().Get("limit"); got != "5" {
						t.Fatalf("runs limit query = %q, want %q", got, "5")
					}
					return newHTTPResponse(
						http.StatusOK,
						`{"runs":[{"id":"run-shared","job_id":"job-created","trigger_id":"trg-created","session_id":"sess-shared","status":"completed","attempt":1,"started_at":"2026-04-11T12:00:00Z","ended_at":"2026-04-11T12:02:00Z"}]}`,
					), nil
				case req.Method == http.MethodGet && req.URL.Path == "/api/automation/runs/run-shared":
					return newHTTPResponse(
						http.StatusOK,
						`{"run":{"id":"run-shared","job_id":"job-created","trigger_id":"trg-created","session_id":"sess-shared","status":"completed","attempt":1,"started_at":"2026-04-11T12:00:00Z","ended_at":"2026-04-11T12:02:00Z"}}`,
					), nil
				default:
					return newHTTPResponse(http.StatusNotFound, `{"error":"missing"}`), nil
				}
			}),
		},
	}

	ctx := context.Background()

	t.Run("Should preserve workspace suggestion routes", func(t *testing.T) {
		suggestions, err := client.ListAutomationSuggestions(
			ctx,
			"ws-alpha",
			automationpkg.SuggestionStatusPending,
		)
		if err != nil || len(suggestions.Suggestions) != 1 || suggestions.Suggestions[0].ID != "suggestion-1" {
			t.Fatalf("ListAutomationSuggestions() = %#v, %v", suggestions, err)
		}

		accepted, err := client.AcceptAutomationSuggestion(ctx, "ws-alpha", "suggestion-1")
		if err != nil || accepted.Suggestion.Status != automationpkg.SuggestionStatusAccepted ||
			accepted.Job.ID != "job-1" {
			t.Fatalf("AcceptAutomationSuggestion() = %#v, %v", accepted, err)
		}

		dismissed, err := client.DismissAutomationSuggestion(ctx, "ws-alpha", "suggestion-1")
		if err != nil || dismissed.Suggestion.Status != automationpkg.SuggestionStatusDismissed {
			t.Fatalf("DismissAutomationSuggestion() = %#v, %v", dismissed, err)
		}
	})

	t.Run("Should list automation jobs", func(t *testing.T) {
		jobs, err := client.ListAutomationJobs(ctx, AutomationJobQuery{
			Scope:       automationpkg.AutomationScopeWorkspace,
			WorkspaceID: "ws-alpha",
			Source:      automationpkg.JobSourcePackage,
			Enabled:     new(false),
			LoopName:    "triage",
			Search:      "digest",
			Cursor:      "job-cursor",
			Limit:       3,
		})
		if err != nil || len(jobs.Jobs) != 1 || jobs.Jobs[0].ID != "job-1" || jobs.Page.Total != 4 {
			t.Fatalf("ListAutomationJobs() = %#v, %v", jobs, err)
		}
	})

	t.Run("Should create automation jobs", func(t *testing.T) {
		createdJob, err := client.CreateAutomationJob(ctx, AutomationJobCreateRequest{
			Scope:       automationpkg.AutomationScopeWorkspace,
			WorkspaceID: "ws-alpha",
			Name:        "nightly",
			AgentName:   "coder",
			Prompt:      "review repo",
			Schedule: automationpkg.ScheduleSpec{
				Mode:     automationpkg.ScheduleModeEvery,
				Interval: "1h",
			},
		})
		if err != nil || createdJob.ID != "job-created" {
			t.Fatalf("CreateAutomationJob() = %#v, %v", createdJob, err)
		}
	})

	t.Run("Should get automation jobs", func(t *testing.T) {
		job, err := client.GetAutomationJob(ctx, "job-created")
		if err != nil || job.NextRun == nil {
			t.Fatalf("GetAutomationJob() = %#v, %v", job, err)
		}
	})

	t.Run("Should update automation jobs", func(t *testing.T) {
		updatedJob, err := client.UpdateAutomationJob(ctx, "job-created", AutomationJobUpdateRequest{
			Prompt:  new("review now"),
			Enabled: new(false),
		})
		if err != nil || updatedJob.Enabled {
			t.Fatalf("UpdateAutomationJob() = %#v, %v", updatedJob, err)
		}
	})

	t.Run("Should trigger automation jobs", func(t *testing.T) {
		triggeredRun, err := client.TriggerAutomationJob(ctx, "job-created")
		if err != nil || triggeredRun.ID != "run-job" {
			t.Fatalf("TriggerAutomationJob() = %#v, %v", triggeredRun, err)
		}
	})

	t.Run("Should list automation job runs", func(t *testing.T) {
		jobRuns, err := client.AutomationJobRuns(ctx, "job-created", AutomationRunQuery{
			Status: automationpkg.RunCompleted,
			Since:  startedAt.Add(-time.Hour),
			Until:  startedAt.Add(time.Hour),
			Limit:  2,
		})
		if err != nil || len(jobRuns) != 1 || jobRuns[0].JobID != "job-created" {
			t.Fatalf("AutomationJobRuns() = %#v, %v", jobRuns, err)
		}
	})

	t.Run("Should delete automation jobs", func(t *testing.T) {
		if err := client.DeleteAutomationJob(ctx, "job-created"); err != nil {
			t.Fatalf("DeleteAutomationJob() error = %v", err)
		}
	})

	t.Run("Should list automation triggers", func(t *testing.T) {
		triggers, err := client.ListAutomationTriggers(ctx, AutomationTriggerQuery{
			Scope:       automationpkg.AutomationScopeWorkspace,
			WorkspaceID: "ws-alpha",
			Event:       "webhook",
			Source:      automationpkg.JobSourcePackage,
			Enabled:     new(true),
			LoopName:    "triage",
			Search:      "review",
			Cursor:      "trigger-cursor",
			Limit:       2,
		})
		if err != nil || len(triggers.Triggers) != 1 || triggers.Triggers[0].ID != "trg-1" || triggers.Page.Total != 3 {
			t.Fatalf("ListAutomationTriggers() = %#v, %v", triggers, err)
		}
	})

	t.Run("Should create automation triggers", func(t *testing.T) {
		createdTrigger, err := client.CreateAutomationTrigger(ctx, AutomationTriggerCreateRequest{
			Scope:              automationpkg.AutomationScopeWorkspace,
			WorkspaceID:        "ws-alpha",
			Name:               "deploy-review",
			AgentName:          "coder",
			Prompt:             `review {{ index .Data "payload" }}`,
			Event:              "webhook",
			Filter:             map[string]string{"data.branch": "main"},
			EndpointSlug:       "deploy-review",
			WebhookSecretValue: "shared-secret",
		})
		if err != nil || createdTrigger.ID != "trg-created" {
			t.Fatalf("CreateAutomationTrigger() = %#v, %v", createdTrigger, err)
		}
	})

	t.Run("Should get automation triggers", func(t *testing.T) {
		trigger, err := client.GetAutomationTrigger(ctx, "trg-created")
		if err != nil || trigger.WebhookID != "wbh_123" {
			t.Fatalf("GetAutomationTrigger() = %#v, %v", trigger, err)
		}
	})

	t.Run("Should update automation triggers", func(t *testing.T) {
		updatedTrigger, err := client.UpdateAutomationTrigger(ctx, "trg-created", AutomationTriggerUpdateRequest{
			Prompt:  new(`inspect {{ index .Data "payload" }}`),
			Enabled: new(false),
		})
		if err != nil || updatedTrigger.Enabled {
			t.Fatalf("UpdateAutomationTrigger() = %#v, %v", updatedTrigger, err)
		}
	})

	t.Run("Should list automation trigger runs", func(t *testing.T) {
		triggerRuns, err := client.AutomationTriggerRuns(ctx, "trg-created", AutomationRunQuery{
			Status: automationpkg.RunCompleted,
			Limit:  1,
		})
		if err != nil || len(triggerRuns) != 1 || triggerRuns[0].TriggerID != "trg-created" {
			t.Fatalf("AutomationTriggerRuns() = %#v, %v", triggerRuns, err)
		}
	})

	t.Run("Should delete automation triggers", func(t *testing.T) {
		if err := client.DeleteAutomationTrigger(ctx, "trg-created"); err != nil {
			t.Fatalf("DeleteAutomationTrigger() error = %v", err)
		}
	})

	t.Run("Should list automation runs", func(t *testing.T) {
		runs, err := client.ListAutomationRuns(ctx, AutomationRunQuery{
			JobID:     "job-created",
			TriggerID: "trg-created",
			Status:    automationpkg.RunCompleted,
			Since:     startedAt,
			Until:     startedAt.Add(time.Hour),
			Limit:     5,
		})
		if err != nil || len(runs) != 1 || runs[0].ID != "run-shared" {
			t.Fatalf("ListAutomationRuns() = %#v, %v", runs, err)
		}
	})

	t.Run("Should get automation runs", func(t *testing.T) {
		run, err := client.GetAutomationRun(ctx, "run-shared")
		if err != nil || run.EndedAt == nil || !run.EndedAt.Equal(endedAt) {
			t.Fatalf("GetAutomationRun() = %#v, %v", run, err)
		}
	})
}

func TestUnixSocketClientTaskMethods(t *testing.T) {
	t.Parallel()

	client := &unixSocketClient{
		socketPath: "/tmp/agh.sock",
		httpClient: &http.Client{
			Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				switch {
				case req.Method == http.MethodGet && req.URL.Path == "/api/tasks":
					if got := req.URL.Query().Get("scope"); got != "workspace" {
						t.Fatalf("task scope query = %q, want %q", got, "workspace")
					}
					if got := req.URL.Query().Get("workspace"); got != "alpha" {
						t.Fatalf("task workspace query = %q, want %q", got, "alpha")
					}
					if got := req.URL.Query().Get("status"); got != "ready" {
						t.Fatalf("task status query = %q, want %q", got, "ready")
					}
					if got := req.URL.Query().Get("owner_kind"); got != "pool" {
						t.Fatalf("task owner_kind query = %q, want %q", got, "pool")
					}
					if got := req.URL.Query().Get("owner_ref"); got != "triage" {
						t.Fatalf("task owner_ref query = %q, want %q", got, "triage")
					}
					if got := req.URL.Query().Get("parent_task_id"); got != "task-root" {
						t.Fatalf("task parent_task_id query = %q, want %q", got, "task-root")
					}
					if got := req.URL.Query().Get("participation_channel"); got != "builders" {
						t.Fatalf("task participation_channel query = %q, want %q", got, "builders")
					}
					if got := req.URL.Query().Get("limit"); got != "3" {
						t.Fatalf("task limit query = %q, want %q", got, "3")
					}
					body := mustJSON(
						t,
						contract.TasksResponse{Tasks: []contract.TaskCatalogItemPayload{sampleTaskCatalogItemRecord()}},
					)
					return newHTTPResponse(http.StatusOK, string(body)), nil
				case req.Method == http.MethodPost && req.URL.Path == "/api/tasks":
					var payload contract.CreateTaskRequest
					if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
						t.Fatalf("json.Decode(create task body) error = %v", err)
					}
					assertLiveNamedParticipationRequest(t, payload.NetworkParticipation, "builders")
					if payload.Scope != taskpkg.ScopeWorkspace ||
						payload.Workspace != "alpha" ||
						payload.Title != "Investigate flaky task runs" ||
						payload.Owner == nil ||
						payload.Owner.Kind != taskpkg.OwnerKindPool ||
						payload.Owner.Ref != "triage" {
						t.Fatalf("create task payload = %#v", payload)
					}
					body := mustJSON(t, contract.TaskResponse{Task: sampleTaskRecord()})
					return newHTTPResponse(http.StatusCreated, string(body)), nil
				case req.Method == http.MethodGet && req.URL.Path == "/api/tasks/task-1":
					body := mustJSON(t, contract.TaskDetailResponse{Task: sampleTaskDetailRecord()})
					return newHTTPResponse(http.StatusOK, string(body)), nil
				case req.Method == http.MethodPatch && req.URL.Path == "/api/tasks/task-1":
					var payload contract.UpdateTaskRequest
					if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
						t.Fatalf("json.Decode(update task body) error = %v", err)
					}
					assertLiveNamedParticipationRequest(t, payload.NetworkParticipation, "ops")
					if payload.Title == nil || *payload.Title != "Investigate resolved" {
						t.Fatalf("update task payload = %#v", payload)
					}
					updated := sampleTaskRecord()
					updated.Title = "Investigate resolved"
					updated.ResolvedNetworkParticipation = testLiveResolvedParticipation("ops")
					body := mustJSON(t, contract.TaskResponse{Task: updated})
					return newHTTPResponse(http.StatusOK, string(body)), nil
				case req.Method == http.MethodPost && req.URL.Path == "/api/tasks/task-1/cancel":
					var payload contract.CancelTaskRequest
					if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
						t.Fatalf("json.Decode(cancel task body) error = %v", err)
					}
					if payload.Reason != "operator-request" {
						t.Fatalf("cancel task payload = %#v, want reason", payload)
					}
					canceled := sampleTaskRecord()
					canceled.Status = taskpkg.TaskStatusCanceled
					body := mustJSON(t, contract.TaskResponse{Task: canceled})
					return newHTTPResponse(http.StatusOK, string(body)), nil
				case req.Method == http.MethodPost && req.URL.Path == "/api/tasks/task-1/children":
					var payload contract.CreateTaskChildRequest
					if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
						t.Fatalf("json.Decode(create child body) error = %v", err)
					}
					if payload.Scope != taskpkg.ScopeWorkspace || payload.Workspace != "alpha" ||
						payload.Title != "Check runtime logs" {
						t.Fatalf("create child payload = %#v", payload)
					}
					child := sampleTaskRecord()
					child.ID = "task-child"
					child.Title = "Check runtime logs"
					body := mustJSON(t, contract.TaskResponse{Task: child})
					return newHTTPResponse(http.StatusCreated, string(body)), nil
				case req.Method == http.MethodPost && req.URL.Path == "/api/tasks/task-1/dependencies":
					var payload contract.AddTaskDependencyRequest
					if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
						t.Fatalf("json.Decode(add dependency body) error = %v", err)
					}
					if payload.DependsOnTaskID != "task-blocker" || payload.Kind != taskpkg.DependencyKindBlocks {
						t.Fatalf("add dependency payload = %#v", payload)
					}
					body := mustJSON(t, contract.TaskDetailResponse{Task: sampleTaskDetailRecord()})
					return newHTTPResponse(http.StatusOK, string(body)), nil
				case req.Method == http.MethodDelete && req.URL.Path == "/api/tasks/task-1/dependencies/task-blocker":
					body := mustJSON(t, contract.TaskDetailResponse{Task: sampleTaskDetailRecord()})
					return newHTTPResponse(http.StatusOK, string(body)), nil
				case req.Method == http.MethodPost && req.URL.Path == "/api/tasks/task-1/runs":
					var payload contract.EnqueueTaskRunRequest
					if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
						t.Fatalf("json.Decode(enqueue run body) error = %v", err)
					}
					assertLiveNamedParticipationRequest(t, payload.NetworkParticipation, "builders")
					if payload.IdempotencyKey != "idem-1" {
						t.Fatalf("enqueue run payload = %#v", payload)
					}
					if got, want := string(payload.Metadata), `{"schema":"agh.harness.detached.v1"}`; got != want {
						t.Fatalf("enqueue run metadata = %q, want %q", got, want)
					}
					body := mustJSON(t, contract.TaskRunResponse{Run: sampleTaskRunRecord(taskpkg.TaskRunStatusQueued)})
					return newHTTPResponse(http.StatusCreated, string(body)), nil
				case req.Method == http.MethodGet && req.URL.Path == "/api/tasks/task-1/runs":
					if got := req.URL.Query().Get("status"); got != "running" {
						t.Fatalf("task runs status query = %q, want %q", got, "running")
					}
					if got := req.URL.Query().Get("session_id"); got != "sess-1" {
						t.Fatalf("task runs session_id query = %q, want %q", got, "sess-1")
					}
					if got := req.URL.Query().Get("participation_channel"); got != "builders" {
						t.Fatalf("task runs participation_channel query = %q, want %q", got, "builders")
					}
					if got := req.URL.Query().Get("limit"); got != "2" {
						t.Fatalf("task runs limit query = %q, want %q", got, "2")
					}
					body := mustJSON(
						t,
						contract.TaskRunsResponse{
							Runs: []contract.TaskRunPayload{sampleTaskRunRecord(taskpkg.TaskRunStatusRunning)},
						},
					)
					return newHTTPResponse(http.StatusOK, string(body)), nil
				case req.Method == http.MethodGet && req.URL.Path == "/api/task-runs/run-1":
					body := mustJSON(t, contract.TaskRunDetailResponse{
						Run: sampleTaskRunDetailRecord(taskpkg.TaskRunStatusQueued),
					})
					return newHTTPResponse(http.StatusOK, string(body)), nil
				case req.Method == http.MethodPost && req.URL.Path == "/api/task-runs/run-1/start":
					var payload contract.StartTaskRunRequest
					if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
						t.Fatalf("json.Decode(start run body) error = %v", err)
					}
					if payload.IdempotencyKey != "idem-start" {
						t.Fatalf("start run payload = %#v", payload)
					}
					body := mustJSON(
						t,
						contract.TaskRunResponse{Run: sampleTaskRunRecord(taskpkg.TaskRunStatusRunning)},
					)
					return newHTTPResponse(http.StatusOK, string(body)), nil
				case req.Method == http.MethodPost && req.URL.Path == "/api/task-runs/run-1/attach-session":
					var payload contract.AttachTaskRunSessionRequest
					if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
						t.Fatalf("json.Decode(attach session body) error = %v", err)
					}
					if payload.SessionID != "sess-attach" {
						t.Fatalf("attach session payload = %#v", payload)
					}
					body := mustJSON(
						t,
						contract.TaskRunResponse{Run: sampleTaskRunRecord(taskpkg.TaskRunStatusStarting)},
					)
					return newHTTPResponse(http.StatusOK, string(body)), nil
				case req.Method == http.MethodPost && req.URL.Path == "/api/task-runs/run-1/complete":
					var payload contract.CompleteTaskRunRequest
					if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
						t.Fatalf("json.Decode(complete run body) error = %v", err)
					}
					if string(payload.Result) != `{"ok":true}` {
						t.Fatalf("complete run payload = %#v", payload)
					}
					body := mustJSON(
						t,
						contract.TaskRunResponse{Run: sampleTaskRunRecord(taskpkg.TaskRunStatusCompleted)},
					)
					return newHTTPResponse(http.StatusOK, string(body)), nil
				case req.Method == http.MethodPost && req.URL.Path == "/api/task-runs/run-1/fail":
					var payload contract.FailTaskRunRequest
					if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
						t.Fatalf("json.Decode(fail run body) error = %v", err)
					}
					if payload.Error != "boom" || string(payload.Metadata) != `{"code":"E_TASK"}` {
						t.Fatalf("fail run payload = %#v", payload)
					}
					body := mustJSON(t, contract.TaskRunResponse{Run: sampleTaskRunRecord(taskpkg.TaskRunStatusFailed)})
					return newHTTPResponse(http.StatusOK, string(body)), nil
				case req.Method == http.MethodPost && req.URL.Path == "/api/task-runs/run-1/cancel":
					var payload contract.CancelTaskRunRequest
					if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
						t.Fatalf("json.Decode(cancel run body) error = %v", err)
					}
					if payload.Reason != "operator-request" || string(payload.Metadata) != `{"source":"cli"}` {
						t.Fatalf("cancel run payload = %#v", payload)
					}
					body := mustJSON(
						t,
						contract.TaskRunResponse{Run: sampleTaskRunRecord(taskpkg.TaskRunStatusCanceled)},
					)
					return newHTTPResponse(http.StatusOK, string(body)), nil
				default:
					return newHTTPResponse(http.StatusNotFound, `{"error":"missing"}`), nil
				}
			}),
		},
	}

	ctx := context.Background()

	t.Run("Should list tasks", func(t *testing.T) {
		tasks, err := client.ListTasks(ctx, TaskListQuery{
			Scope:                taskpkg.CatalogScopeWorkspace,
			Workspace:            "alpha",
			Status:               taskpkg.TaskStatusReady,
			OwnerKind:            taskpkg.OwnerKindPool,
			OwnerRef:             "triage",
			ParentTaskID:         "task-root",
			ParticipationChannel: "builders",
			Limit:                3,
		})
		if err != nil || len(tasks.Tasks) != 1 || tasks.Tasks[0].ID != "task-1" {
			t.Fatalf("ListTasks() = %#v, %v", tasks, err)
		}
	})

	t.Run("Should create get update and cancel tasks", func(t *testing.T) {
		created, err := client.CreateTask(ctx, CreateTaskRequest{
			Scope:                taskpkg.ScopeWorkspace,
			Workspace:            "alpha",
			NetworkParticipation: testLiveNamedParticipationRequest("builders"),
			Title:                "Investigate flaky task runs",
			Owner:                &taskpkg.Ownership{Kind: taskpkg.OwnerKindPool, Ref: "triage"},
		})
		if err != nil || created.ID != "task-1" {
			t.Fatalf("CreateTask() = %#v, %v", created, err)
		}

		detail, err := client.GetTask(ctx, "task-1")
		if err != nil || detail.Task.ID != "task-1" || len(detail.Dependencies) != 1 {
			t.Fatalf("GetTask() = %#v, %v", detail, err)
		}

		updated, err := client.UpdateTask(ctx, "task-1", UpdateTaskRequest{
			Title:                new("Investigate resolved"),
			NetworkParticipation: testLiveNamedParticipationRequest("ops"),
		})
		if err != nil || updated.Title != "Investigate resolved" {
			t.Fatalf("UpdateTask() = %#v, %v", updated, err)
		}
		assertResolvedParticipation(t, updated.ResolvedNetworkParticipation, *testLiveResolvedParticipation("ops"))

		canceled, err := client.CancelTask(ctx, "task-1", CancelTaskRequest{Reason: "operator-request"})
		if err != nil || canceled.Status != taskpkg.TaskStatusCanceled {
			t.Fatalf("CancelTask() = %#v, %v", canceled, err)
		}
	})

	t.Run("Should manage child tasks dependencies and runs", func(t *testing.T) {
		child, err := client.CreateChildTask(ctx, "task-1", CreateTaskChildRequest{
			Scope:     taskpkg.ScopeWorkspace,
			Workspace: "alpha",
			Title:     "Check runtime logs",
		})
		if err != nil || child.ID != "task-child" {
			t.Fatalf("CreateChildTask() = %#v, %v", child, err)
		}

		detail, err := client.AddTaskDependency(ctx, "task-1", AddTaskDependencyRequest{
			DependsOnTaskID: "task-blocker",
			Kind:            taskpkg.DependencyKindBlocks,
		})
		if err != nil || len(detail.Dependencies) != 1 {
			t.Fatalf("AddTaskDependency() = %#v, %v", detail, err)
		}

		detail, err = client.RemoveTaskDependency(ctx, "task-1", "task-blocker")
		if err != nil || len(detail.Runs) != 1 {
			t.Fatalf("RemoveTaskDependency() = %#v, %v", detail, err)
		}

		enqueued, err := client.EnqueueTaskRun(ctx, "task-1", EnqueueTaskRunRequest{
			IdempotencyKey:       "idem-1",
			NetworkParticipation: testLiveNamedParticipationRequest("builders"),
			Metadata:             json.RawMessage(`{"schema":"agh.harness.detached.v1"}`),
		})
		if err != nil || enqueued.Status != taskpkg.TaskRunStatusQueued {
			t.Fatalf("EnqueueTaskRun() = %#v, %v", enqueued, err)
		}

		runs, err := client.ListTaskRuns(ctx, "task-1", TaskRunListQuery{
			Status:               taskpkg.TaskRunStatusRunning,
			SessionID:            "sess-1",
			ParticipationChannel: "builders",
			Limit:                2,
		})
		if err != nil || len(runs) != 1 || runs[0].Status != taskpkg.TaskRunStatusRunning {
			t.Fatalf("ListTaskRuns() = %#v, %v", runs, err)
		}

		shown, err := client.GetTaskRun(ctx, "run-1")
		if err != nil ||
			shown.Run.ID != "run-1" ||
			shown.Run.Status != taskpkg.TaskRunStatusQueued ||
			shown.Task.ID != "task-1" {
			t.Fatalf("GetTaskRun() = %#v, %v", shown, err)
		}

		started, err := client.StartTaskRun(ctx, "run-1", StartTaskRunRequest{IdempotencyKey: "idem-start"})
		if err != nil || started.Status != taskpkg.TaskRunStatusRunning {
			t.Fatalf("StartTaskRun() = %#v, %v", started, err)
		}

		attached, err := client.AttachTaskRunSession(
			ctx,
			"run-1",
			AttachTaskRunSessionRequest{SessionID: "sess-attach"},
		)
		if err != nil || attached.Status != taskpkg.TaskRunStatusStarting {
			t.Fatalf("AttachTaskRunSession() = %#v, %v", attached, err)
		}

		completed, err := client.CompleteTaskRun(
			ctx,
			"run-1",
			CompleteTaskRunRequest{Result: mustJSON(t, map[string]bool{"ok": true})},
		)
		if err != nil || completed.Status != taskpkg.TaskRunStatusCompleted {
			t.Fatalf("CompleteTaskRun() = %#v, %v", completed, err)
		}

		failed, err := client.FailTaskRun(
			ctx,
			"run-1",
			FailTaskRunRequest{Error: "boom", Metadata: mustJSON(t, map[string]string{"code": "E_TASK"})},
		)
		if err != nil || failed.Status != taskpkg.TaskRunStatusFailed {
			t.Fatalf("FailTaskRun() = %#v, %v", failed, err)
		}

		canceled, err := client.CancelTaskRun(
			ctx,
			"run-1",
			CancelTaskRunRequest{Reason: "operator-request", Metadata: mustJSON(t, map[string]string{"source": "cli"})},
		)
		if err != nil || canceled.Status != taskpkg.TaskRunStatusCanceled {
			t.Fatalf("CancelTaskRun() = %#v, %v", canceled, err)
		}
	})
}

func TestUnixSocketClientBridgeListCatalogQuery(t *testing.T) {
	t.Parallel()

	t.Run("Should serialize every bounded bridge catalog query parameter", func(t *testing.T) {
		t.Parallel()

		client := &unixSocketClient{
			socketPath: "/tmp/agh.sock",
			httpClient: &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				if req.Method != http.MethodGet || req.URL.Path != "/api/bridges" {
					t.Fatalf("request = %s %s, want GET /api/bridges", req.Method, req.URL.Path)
				}
				want := map[string]string{
					"scope":        "all",
					"workspace_id": "ws-alpha",
					"workspace":    "alpha",
					"q":            "needle",
					"platform":     "slack",
					"status":       "error",
					"sort":         "name",
					"cursor":       "cursor-1",
					"limit":        "25",
				}
				for key, value := range want {
					if got := req.URL.Query().Get(key); got != value {
						t.Fatalf("query[%s] = %q, want %q; raw=%s", key, got, value, req.URL.RawQuery)
					}
				}
				return newHTTPResponse(
					http.StatusOK,
					`{"bridges":[],"bridge_health":{},"facets":{"platforms":{},"statuses":{"disabled":0,"starting":0,"ready":0,"degraded":0,"auth_required":0,"error":0}},"page":{"has_more":false,"total":0,"limit":25}}`,
				), nil
			})},
		}
		result, err := client.ListBridges(context.Background(), BridgeListQuery{
			Scope:       "all",
			WorkspaceID: "ws-alpha",
			Workspace:   "alpha",
			Search:      "needle",
			Platform:    "slack",
			Status:      "error",
			Sort:        "name",
			Cursor:      "cursor-1",
			Limit:       25,
		})
		if err != nil {
			t.Fatalf("ListBridges() error = %v", err)
		}
		if result.Page.Total != 0 || result.Page.Limit != 25 || result.Bridges == nil {
			t.Fatalf("ListBridges() = %#v, want explicit empty counted page", result)
		}
	})
}

func TestUnixSocketClientBridgeMethods(t *testing.T) {
	t.Parallel()

	client := &unixSocketClient{
		socketPath: "/tmp/agh.sock",
		httpClient: &http.Client{
			Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				switch {
				case req.Method == http.MethodGet && req.URL.Path == "/api/bridges":
					return newHTTPResponse(
						http.StatusOK,
						`{"bridges":[{"id":"brg-a","scope":"global","platform":"telegram","extension_name":"ext-telegram","display_name":"Support","enabled":true,"status":"ready","routing_policy":{"include_peer":true},"created_at":"2026-04-11T12:00:00Z","updated_at":"2026-04-11T12:00:00Z"}],"bridge_health":{"brg-a":{"bridge_instance_id":"brg-a","status":"ready","route_count":0,"delivery_backlog":0,"delivery_dropped_total":0,"delivery_failures_total":0,"auth_failures_total":0}},"facets":{"platforms":{"telegram":1},"statuses":{"disabled":0,"starting":0,"ready":1,"degraded":0,"auth_required":0,"error":0}},"page":{"has_more":false,"total":1,"limit":50}}`,
					), nil
				case req.Method == http.MethodPost && req.URL.Path == "/api/bridges":
					var payload contract.CreateBridgeRequest
					if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
						t.Fatalf("json.Decode(create bridge body) error = %v", err)
					}
					if payload.Scope != bridgepkg.ScopeGlobal ||
						payload.WorkspaceID != "" ||
						payload.Platform != "telegram" ||
						payload.ExtensionName != "ext-telegram" ||
						payload.DisplayName != "Support" ||
						!payload.Enabled ||
						!reflect.DeepEqual(payload.RoutingPolicy, bridgepkg.RoutingPolicy{IncludePeer: true}) ||
						len(payload.DeliveryDefaults) != 0 {
						t.Fatalf("create bridge payload = %#v", payload)
					}
					return newHTTPResponse(
						http.StatusCreated,
						`{"bridge":{"id":"brg-a","scope":"global","platform":"telegram","extension_name":"ext-telegram","display_name":"Support","enabled":true,"status":"starting","routing_policy":{"include_peer":true},"created_at":"2026-04-11T12:00:00Z","updated_at":"2026-04-11T12:00:00Z"}}`,
					), nil
				case req.Method == http.MethodGet && req.URL.Path == "/api/bridges/brg-a":
					return newHTTPResponse(
						http.StatusOK,
						`{"bridge":{"id":"brg-a","scope":"global","platform":"telegram","extension_name":"ext-telegram","display_name":"Support","enabled":true,"status":"ready","routing_policy":{"include_peer":true},"created_at":"2026-04-11T12:00:00Z","updated_at":"2026-04-11T12:00:00Z"}}`,
					), nil
				case req.Method == http.MethodPatch && req.URL.Path == "/api/bridges/brg-a":
					var payload contract.UpdateBridgeRequest
					if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
						t.Fatalf("json.Decode(update bridge body) error = %v", err)
					}
					if payload.DisplayName == nil ||
						*payload.DisplayName != "Support Ops" ||
						payload.RoutingPolicy == nil ||
						!reflect.DeepEqual(
							*payload.RoutingPolicy,
							bridgepkg.RoutingPolicy{IncludePeer: true, IncludeThread: true},
						) ||
						payload.DeliveryDefaults != nil {
						t.Fatalf("update bridge payload = %#v, want updated display name", payload)
					}
					return newHTTPResponse(
						http.StatusOK,
						`{"bridge":{"id":"brg-a","scope":"global","platform":"telegram","extension_name":"ext-telegram","display_name":"Support Ops","enabled":true,"status":"ready","routing_policy":{"include_peer":true,"include_thread":true},"created_at":"2026-04-11T12:00:00Z","updated_at":"2026-04-11T12:05:00Z"}}`,
					), nil
				case req.Method == http.MethodPost && req.URL.Path == "/api/bridges/brg-a/enable":
					return newHTTPResponse(
						http.StatusOK,
						`{"bridge":{"id":"brg-a","scope":"global","platform":"telegram","extension_name":"ext-telegram","display_name":"Support","enabled":true,"status":"starting","routing_policy":{"include_peer":true},"created_at":"2026-04-11T12:00:00Z","updated_at":"2026-04-11T12:06:00Z"}}`,
					), nil
				case req.Method == http.MethodPost && req.URL.Path == "/api/bridges/brg-a/disable":
					return newHTTPResponse(
						http.StatusOK,
						`{"bridge":{"id":"brg-a","scope":"global","platform":"telegram","extension_name":"ext-telegram","display_name":"Support","enabled":false,"status":"disabled","routing_policy":{"include_peer":true},"created_at":"2026-04-11T12:00:00Z","updated_at":"2026-04-11T12:07:00Z"}}`,
					), nil
				case req.Method == http.MethodPost && req.URL.Path == "/api/bridges/brg-a/restart":
					return newHTTPResponse(
						http.StatusOK,
						`{"bridge":{"id":"brg-a","scope":"global","platform":"telegram","extension_name":"ext-telegram","display_name":"Support","enabled":true,"status":"starting","routing_policy":{"include_peer":true},"created_at":"2026-04-11T12:00:00Z","updated_at":"2026-04-11T12:08:00Z"}}`,
					), nil
				case req.Method == http.MethodGet && req.URL.Path == "/api/bridges/brg-a/routes":
					return newHTTPResponse(
						http.StatusOK,
						`{"routes":[{"routing_key_hash":"hash-a","scope":"global","bridge_instance_id":"brg-a","peer_id":"peer-1","thread_id":"thread-1","session_id":"sess-1","agent_name":"coder","last_activity_at":"2026-04-11T12:09:00Z","created_at":"2026-04-11T12:00:00Z","updated_at":"2026-04-11T12:09:00Z"}]}`,
					), nil
				case req.Method == http.MethodGet && req.URL.Path == "/api/bridges/brg-a/targets":
					if req.URL.Query().Get("q") != "support" || req.URL.Query().Get("limit") != "25" {
						t.Fatalf("bridge targets query = %s, want q=support&limit=25", req.URL.RawQuery)
					}
					return newHTTPResponse(
						http.StatusOK,
						`{"bridge_id":"brg-a","targets":[{"bridge_id":"brg-a","canonical_route":"telegram:channel:support","display_name":"Support room","normalized":"support room","target_type":"channel","qualifier":"telegram","capabilities":["reply"],"updated_at":"2026-04-11T12:09:00Z","last_seen_at":"2026-04-11T12:09:00Z"}],"total":1,"cache_stale":false,"generated_at":"2026-04-11T12:09:30Z"}`,
					), nil
				case req.Method == http.MethodPost && req.URL.Path == "/api/bridges/brg-a/resolve":
					var payload contract.BridgeResolveTargetRequest
					if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
						t.Fatalf("json.Decode(resolve target body) error = %v", err)
					}
					if payload.Name != "Support room" {
						t.Fatalf("resolve target payload = %#v, want Support room", payload)
					}
					return newHTTPResponse(
						http.StatusOK,
						`{"result":{"step":2,"ambiguous":false,"match":{"bridge_id":"brg-a","canonical_route":"telegram:channel:support","display_name":"Support room","normalized":"support room","target_type":"channel","qualifier":"telegram","capabilities":["reply"],"updated_at":"2026-04-11T12:09:00Z","last_seen_at":"2026-04-11T12:09:00Z"}}}`,
					), nil
				case req.Method == http.MethodPost && req.URL.Path == "/api/bridges/brg-a/test-delivery":
					var payload contract.BridgeTestDeliveryRequest
					if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
						t.Fatalf("json.Decode(test delivery body) error = %v", err)
					}
					if payload.Message != "hello" || payload.Target.PeerID != "peer-1" ||
						payload.Target.ThreadID != "thread-1" ||
						payload.Target.Mode != bridgepkg.DeliveryModeReply {
						t.Fatalf("test delivery payload = %#v", payload)
					}
					return newHTTPResponse(
						http.StatusOK,
						`{"status":"resolved","message":"hello","delivery_target":{"bridge_instance_id":"brg-a","peer_id":"peer-1","thread_id":"thread-1","mode":"reply"}}`,
					), nil
				default:
					return newHTTPResponse(http.StatusNotFound, `{"error":"missing"}`), nil
				}
			}),
		},
	}

	ctx := context.Background()

	listed, err := client.ListBridges(ctx, BridgeListQuery{})
	if err != nil || len(listed.Bridges) != 1 || listed.Bridges[0].ID != "brg-a" || listed.Page.Total != 1 {
		t.Fatalf("ListBridges() = %#v, %v", listed, err)
	}

	created, err := client.CreateBridge(ctx, CreateBridgeRequest{
		Scope:         bridgepkg.ScopeGlobal,
		Platform:      "telegram",
		ExtensionName: "ext-telegram",
		DisplayName:   "Support",
		Enabled:       true,
		RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true},
	})
	if err != nil || created.ID != "brg-a" {
		t.Fatalf("CreateBridge() = %#v, %v", created, err)
	}

	status, err := client.GetBridge(ctx, "brg-a")
	if err != nil || status.Status != bridgepkg.BridgeStatusReady {
		t.Fatalf("GetBridge() = %#v, %v", status, err)
	}

	updated, err := client.UpdateBridge(ctx, "brg-a", UpdateBridgeRequest{
		DisplayName: new("Support Ops"),
		RoutingPolicy: &bridgepkg.RoutingPolicy{
			IncludePeer:   true,
			IncludeThread: true,
		},
	})
	if err != nil || updated.DisplayName != "Support Ops" || !updated.RoutingPolicy.IncludeThread {
		t.Fatalf("UpdateBridge() = %#v, %v", updated, err)
	}

	enabled, err := client.EnableBridge(ctx, " brg-a ")
	if err != nil || enabled.Status != bridgepkg.BridgeStatusStarting || !enabled.Enabled {
		t.Fatalf("EnableBridge() = %#v, %v", enabled, err)
	}

	disabled, err := client.DisableBridge(ctx, " brg-a ")
	if err != nil || disabled.Status != bridgepkg.BridgeStatusDisabled || disabled.Enabled {
		t.Fatalf("DisableBridge() = %#v, %v", disabled, err)
	}

	restarted, err := client.RestartBridge(ctx, " brg-a ")
	if err != nil || restarted.Status != bridgepkg.BridgeStatusStarting || !restarted.Enabled {
		t.Fatalf("RestartBridge() = %#v, %v", restarted, err)
	}

	routes, err := client.BridgeRoutes(ctx, "brg-a")
	if err != nil || len(routes) != 1 || routes[0].ThreadID != "thread-1" {
		t.Fatalf("BridgeRoutes() = %#v, %v", routes, err)
	}

	targets, err := client.BridgeTargets(ctx, "brg-a", "support", 25)
	if err != nil || len(targets.Targets) != 1 || targets.Targets[0].CanonicalRoute != "telegram:channel:support" {
		t.Fatalf("BridgeTargets() = %#v, %v", targets, err)
	}

	resolved, err := client.ResolveBridgeTarget(ctx, "brg-a", "Support room")
	if err != nil || resolved.Result.Match == nil ||
		resolved.Result.Match.CanonicalRoute != "telegram:channel:support" {
		t.Fatalf("ResolveBridgeTarget() = %#v, %v", resolved, err)
	}

	delivery, err := client.TestBridgeDelivery(ctx, "brg-a", BridgeTestDeliveryRequest{
		Message: "hello",
		Target: BridgeDeliveryTargetInput{
			PeerID:   "peer-1",
			ThreadID: "thread-1",
			Mode:     bridgepkg.DeliveryModeReply,
		},
	})
	if err != nil || delivery.DeliveryTarget.Mode != bridgepkg.DeliveryModeReply ||
		delivery.DeliveryTarget.ThreadID != "thread-1" {
		t.Fatalf("TestBridgeDelivery() = %#v, %v", delivery, err)
	}
}

func TestReadAPIErrorAndHelpers(t *testing.T) {
	t.Parallel()

	resp := newHTTPResponse(http.StatusBadRequest, `{"error":"boom"}`)
	defer func() {
		_ = resp.Body.Close()
	}()
	err := readAPIError(resp)
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("readAPIError() = %v, want boom", err)
	}

	toolResp := newHTTPResponse(
		http.StatusAccepted,
		`{"error":{"code":"tool_approval_required","message":"tool approval token=super-secret is required","tool_id":"agh__skill_view","reason_codes":["approval_token_missing"],"layer":"approval","details":{"approval_token":"approval-token-secret"}}}`,
	)
	defer func() {
		_ = toolResp.Body.Close()
	}()
	err = readAPIError(toolResp)
	var toolErr *toolAPIError
	if !errors.As(err, &toolErr) {
		t.Fatalf("readAPIError(tool) = %T %[1]v, want toolAPIError", err)
	}
	toolPayload := toolErr.Response()
	if toolPayload.Error.Code != toolspkg.ErrorCodeApprovalRequired ||
		len(toolPayload.Error.ReasonCodes) != 1 ||
		toolPayload.Error.ReasonCodes[0] != toolspkg.ReasonApprovalTokenMissing {
		t.Fatalf("tool error payload = %#v, want approval required", toolPayload.Error)
	}
	if strings.Contains(toolErr.Error(), "super-secret") ||
		strings.Contains(string(mustJSON(t, toolPayload)), "approval-token-secret") {
		t.Fatalf("tool error leaked sensitive material: error=%q payload=%s", toolErr.Error(), mustJSON(t, toolPayload))
	}

	if got := sessionEventValues(SessionEventQuery{
		Type:          "agent_message",
		AgentName:     "coder",
		TurnID:        "turn-1",
		Since:         fixedTestNow,
		Last:          3,
		AfterSequence: 9,
	}); got.Get("after_sequence") != "9" || got.Get("limit") != "3" {
		t.Fatalf("sessionEventValues() = %v, want limit/after_sequence", got)
	}

	if got := sessionListValues(SessionListQuery{
		Workspace:     "ws-1",
		State:         "active",
		Type:          "user",
		Agent:         "coder",
		Query:         "needle",
		Resumable:     true,
		IncludeHealth: true,
		Sort:          "last_activity",
		Cursor:        "cursor-1",
		Limit:         2,
	}); got.Get("workspace") != "ws-1" || got.Get("state") != "active" || got.Get("type") != "user" ||
		got.Get("agent") != "coder" ||
		got.Get("q") != "needle" || got.Get("resumable") != "true" || got.Get("sort") != "last_activity" ||
		got.Get("include_health") != "true" || got.Get("cursor") != "cursor-1" || got.Get("limit") != "2" {
		t.Fatalf("sessionListValues() = %v, want all page filters", got)
	}

	if got := logsListValues(LogsListQuery{
		SessionID: "sess-1",
		AgentName: "coder",
		Type:      "done",
		Since:     fixedTestNow,
		Last:      2,
	}); got.Get("session_id") != "sess-1" || got.Get("limit") != "2" {
		t.Fatalf("logsListValues() = %v, want session_id/limit", got)
	}

	if got := hookCatalogValues(HookCatalogQuery{
		Workspace: "alpha",
		Agent:     "coder",
		Event:     "tool.pre_call",
		Source:    "config",
		Mode:      "sync",
	}); got.Get("workspace") != "alpha" || got.Get("source") != "config" || got.Get("mode") != "sync" {
		t.Fatalf("hookCatalogValues() = %v, want all hook catalog filters", got)
	}

	if got := hookRunsValues(HookRunsQuery{
		Session: "sess-1",
		Event:   "permission.request",
		Outcome: "failed",
		Since:   "2026-04-03T11:00:00Z",
		Last:    2,
	}); got.Get("outcome") != "failed" || got.Get("last") != "2" || got.Get("since") != "2026-04-03T11:00:00Z" {
		t.Fatalf("hookRunsValues() = %v, want all hook runs filters", got)
	}

	if got := hookEventsValues(
		HookEventsQuery{Family: "tool", SyncOnly: true},
	); got.Get("family") != "tool" ||
		got.Get("sync_only") != "true" {
		t.Fatalf("hookEventsValues() = %v, want family/sync_only", got)
	}

	if got := memorySelectorValues(MemorySelectorQuery{
		Scope:         memcontract.ScopeWorkspace,
		WorkspaceID:   "/workspace/project",
		AgentName:     "reviewer",
		AgentTier:     memcontract.AgentTierWorkspace,
		IncludeSystem: true,
	}); got.Get("scope") != "workspace" ||
		got.Get("workspace_id") != "/workspace/project" ||
		got.Get("agent_name") != "reviewer" ||
		got.Get("agent_tier") != "workspace" ||
		got.Get("include_system") != "true" {
		t.Fatalf("memorySelectorValues() = %v, want selector filters", got)
	}

	if got := memoryListValues(MemoryListQuery{
		MemorySelectorQuery: MemorySelectorQuery{Scope: memcontract.ScopeWorkspace},
		Type:                memcontract.TypeProject,
		Sort:                "name",
		Cursor:              "next-page",
		Limit:               25,
	}); got.Get("scope") != "workspace" ||
		got.Get("type") != "project" ||
		got.Get("sort") != "name" ||
		got.Get("cursor") != "next-page" ||
		got.Get("limit") != "25" {
		t.Fatalf("memoryListValues() = %v, want list filters", got)
	}

	if got := memoryHistoryValues(MemoryHistoryQuery{
		Scope:       memcontract.ScopeWorkspace,
		WorkspaceID: "/workspace/project",
		Operation:   "memory.delete",
		Since:       time.Date(2026, 4, 3, 11, 0, 0, 0, time.UTC),
		Limit:       6,
	}); got.Get("scope") != "workspace" ||
		got.Get("workspace_id") != "/workspace/project" ||
		got.Get("operation") != "memory.delete" ||
		got.Get("since") != "2026-04-03T11:00:00Z" ||
		got.Get("limit") != "6" {
		t.Fatalf("memoryHistoryValues() = %v, want all history filters", got)
	}

	if got := memoryDecisionValues(MemoryDecisionListQuery{
		Scope:          memcontract.ScopeWorkspace,
		WorkspaceID:    "/workspace/project",
		Operation:      "update",
		TargetFilename: "project.md",
		Since:          time.Date(2026, 4, 3, 11, 0, 0, 0, time.UTC),
		Reason:         "accepted",
		Limit:          1,
	}); got.Get("scope") != "workspace" ||
		got.Get("workspace_id") != "/workspace/project" ||
		got.Get("op") != "update" ||
		got.Get("filename") != "project.md" ||
		got.Get("since") != "2026-04-03T11:00:00Z" ||
		got.Get("reason") != "accepted" ||
		got.Get("limit") != "1" {
		t.Fatalf("memoryDecisionValues() = %v, want all decision filters", got)
	}

	if got := automationJobValues(AutomationJobQuery{
		Scope:       automationpkg.AutomationScopeWorkspace,
		WorkspaceID: "ws-alpha",
		Source:      automationpkg.JobSourceDynamic,
		Search:      "digest",
		Cursor:      "job-cursor",
		Limit:       3,
	}); got.Get("scope") != "workspace" || got.Get("workspace_id") != "ws-alpha" || got.Get("source") != "dynamic" || got.Get("q") != "digest" || got.Get("cursor") != "job-cursor" || got.Get("limit") != "3" {
		t.Fatalf("automationJobValues() = %v, want all job filters", got)
	}

	if got := automationTriggerValues(AutomationTriggerQuery{
		Scope:       automationpkg.AutomationScopeWorkspace,
		WorkspaceID: "ws-alpha",
		Event:       "webhook",
		Source:      automationpkg.JobSourceDynamic,
		Search:      "review",
		Cursor:      "trigger-cursor",
		Limit:       2,
	}); got.Get("scope") != "workspace" || got.Get("workspace_id") != "ws-alpha" || got.Get("event") != "webhook" || got.Get("source") != "dynamic" || got.Get("q") != "review" || got.Get("cursor") != "trigger-cursor" || got.Get("limit") != "2" {
		t.Fatalf("automationTriggerValues() = %v, want all trigger filters", got)
	}

	if got := automationRunValues(AutomationRunQuery{
		JobID:     "job-1",
		TriggerID: "trg-1",
		Status:    automationpkg.RunCompleted,
		Since:     fixedTestNow,
		Until:     fixedTestNow.Add(time.Hour),
		Limit:     4,
	}); got.Get("job_id") != "job-1" || got.Get("trigger_id") != "trg-1" || got.Get("status") != "completed" || got.Get("limit") != "4" {
		t.Fatalf("automationRunValues() = %v, want all run filters", got)
	}

	if got := taskValues(TaskListQuery{
		Scope:                taskpkg.CatalogScopeWorkspace,
		Workspace:            "alpha",
		Status:               taskpkg.TaskStatusReady,
		OwnerKind:            taskpkg.OwnerKindPool,
		OwnerRef:             "triage",
		ParentTaskID:         "task-root",
		ParticipationChannel: "builders",
		Limit:                3,
	}); got.Get("scope") != "workspace" || got.Get("workspace") != "alpha" || got.Get("status") != "ready" || got.Get("owner_kind") != "pool" || got.Get("owner_ref") != "triage" || got.Get("parent_task_id") != "task-root" || got.Get("participation_channel") != "builders" || got.Get("limit") != "3" {
		t.Fatalf("taskValues() = %v, want all task filters", got)
	}

	if got := taskRunValues(TaskRunListQuery{
		Status:               taskpkg.TaskRunStatusRunning,
		SessionID:            "sess-1",
		ParticipationChannel: "builders",
		Limit:                2,
	}); got.Get("status") != "running" || got.Get("session_id") != "sess-1" ||
		got.Get("participation_channel") != "builders" || got.Get("limit") != "2" {
		t.Fatalf("taskRunValues() = %v, want all task run filters", got)
	}

	plain := newHTTPResponse(http.StatusInternalServerError, "plain failure")
	defer func() {
		_ = plain.Body.Close()
	}()
	if err := readAPIError(plain); err == nil || !strings.Contains(err.Error(), "plain failure") {
		t.Fatalf("readAPIError(plain) = %v, want plain failure", err)
	}

	large := newHTTPResponse(http.StatusInternalServerError, strings.Repeat("x", 2<<20))
	defer func() {
		_ = large.Body.Close()
	}()
	if err := readAPIError(large); err == nil {
		t.Fatal("readAPIError(large) error = nil, want non-nil")
	} else if len(err.Error()) > (1<<20)+128 {
		t.Fatalf("readAPIError(large) len = %d, want capped body size", len(err.Error()))
	}
}

func TestDecodeSSEStopsEarly(t *testing.T) {
	t.Parallel()

	body := strings.Join([]string{
		"id: 1",
		"event: done",
		`data: {"ok":true}`,
		"",
		"id: 2",
		"event: later",
		`data: {"ok":false}`,
		"",
	}, "\n")

	count := 0
	err := decodeSSE(context.Background(), io.NopCloser(strings.NewReader(body)), func(event SSEEvent) error {
		count++
		if event.Event == "done" {
			return errStopSSE
		}
		return nil
	})
	if err != nil {
		t.Fatalf("decodeSSE() error = %v", err)
	}
	if count != 1 {
		t.Fatalf("decodeSSE() count = %d, want 1", count)
	}
}

func TestDecodeSSERejectsNilArguments(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name    string
		ctx     context.Context
		body    io.ReadCloser
		handler SSEHandler
		wantErr string
	}{
		{
			name:    "nil context",
			ctx:     nil,
			body:    io.NopCloser(strings.NewReader("event: ping\n\n")),
			handler: func(SSEEvent) error { return nil },
			wantErr: "sse: context is required",
		},
		{
			name:    "nil body",
			ctx:     context.Background(),
			body:    nil,
			handler: func(SSEEvent) error { return nil },
			wantErr: "sse: body is required",
		},
		{
			name:    "nil handler",
			ctx:     context.Background(),
			body:    io.NopCloser(strings.NewReader("event: ping\n\n")),
			handler: nil,
			wantErr: "sse: handler is required",
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := decodeSSE(tt.ctx, tt.body, tt.handler)
			if err == nil || err.Error() != tt.wantErr {
				t.Fatalf("decodeSSE() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestDecodeSSEPropagatesHandlerError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("boom")
	body := strings.Join([]string{
		"id: 1",
		"event: done",
		`data: {"ok":true}`,
		"",
	}, "\n")

	err := decodeSSE(context.Background(), io.NopCloser(strings.NewReader(body)), func(SSEEvent) error {
		return wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("decodeSSE() error = %v, want %v", err, wantErr)
	}
}

func TestDecodeSSEPreservesMultiLineData(t *testing.T) {
	t.Parallel()

	body := strings.Join([]string{
		"id: 1",
		"event: message",
		`data: {"first":true}`,
		`data: {"second":true}`,
		"",
	}, "\n")

	var seen SSEEvent
	err := decodeSSE(context.Background(), io.NopCloser(strings.NewReader(body)), func(event SSEEvent) error {
		seen = event
		return nil
	})
	if err != nil {
		t.Fatalf("decodeSSE() error = %v", err)
	}
	if got, want := string(seen.Data), "{\"first\":true}\n{\"second\":true}"; got != want {
		t.Fatalf("decodeSSE() data = %q, want %q", got, want)
	}
}

func newHTTPResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    &http.Request{Method: http.MethodGet},
	}
}

func nilContext() context.Context {
	return nil
}

func TestNewClientRequiresSocket(t *testing.T) {
	t.Parallel()

	if _, err := NewClient(""); err == nil {
		t.Fatal("NewClient(\"\") error = nil, want non-nil")
	}
}

func TestNewClientConfiguresTimeouts(t *testing.T) {
	t.Parallel()

	client, err := NewClient("/tmp/agh.sock")
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	udsClient, ok := client.(*unixSocketClient)
	if !ok {
		t.Fatalf("NewClient() = %T, want *unixSocketClient", client)
	}
	if udsClient.httpClient == nil {
		t.Fatal("httpClient = nil, want timeout-bound client")
	}
	if udsClient.httpClient.Timeout != defaultUnixSocketClientTimeout {
		t.Fatalf("httpClient timeout = %v, want %v", udsClient.httpClient.Timeout, defaultUnixSocketClientTimeout)
	}
	if udsClient.streamClient == nil {
		t.Fatal("streamClient = nil, want dedicated streaming client")
	}
	if udsClient.streamClient.Timeout != 0 {
		t.Fatalf("streamClient timeout = %v, want no client-level timeout", udsClient.streamClient.Timeout)
	}
}

func TestDoRequestSetsHeaders(t *testing.T) {
	t.Parallel()

	client := &unixSocketClient{
		socketPath: "/tmp/agh.sock",
		httpClient: &http.Client{
			Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				if got := req.Header.Get("User-Agent"); got != defaultUserAgentName {
					t.Fatalf("User-Agent = %q, want %q", got, defaultUserAgentName)
				}
				if got := req.Header.Get("Last-Event-ID"); got != "cursor-9" {
					t.Fatalf("Last-Event-ID = %q, want %q", got, "cursor-9")
				}
				if got := req.URL.Query().Get("since"); got == "" {
					t.Fatal("expected encoded query string")
				}
				return newHTTPResponse(http.StatusOK, `{"events":[]}`), nil
			}),
		},
	}

	err := client.doSSE(
		context.Background(),
		"/api/logs/stream",
		logsListValues(LogsListQuery{Since: time.Now().UTC()}),
		"cursor-9",
		func(SSEEvent) error {
			return nil
		},
	)
	if err != nil {
		t.Fatalf("doSSE() error = %v", err)
	}
}

func TestDoRequestRejectsNilContext(t *testing.T) {
	t.Parallel()

	client := &unixSocketClient{
		socketPath: "/tmp/agh.sock",
		httpClient: &http.Client{},
	}

	response, err := client.doRequest(nilContext(), http.MethodGet, "/api/status", nil, nil)
	if response != nil {
		defer func() {
			_ = response.Body.Close()
		}()
	}
	if err == nil {
		t.Fatal("doRequest(nil) error = nil, want non-nil")
	}
}

func TestCLIUsesSharedContractAliases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		cliType any
		want    any
	}{
		{
			name:    "Should alias CreateSessionRequest to the shared contract",
			cliType: CreateSessionRequest{},
			want:    contract.CreateSessionRequest{},
		},
		{
			name:    "Should alias SessionRecord to the shared contract",
			cliType: SessionRecord{},
			want:    contract.SessionPayload{},
		},
		{
			name:    "Should alias SessionEventRecord to the shared contract",
			cliType: SessionEventRecord{},
			want:    contract.SessionEventPayload{},
		},
		{
			name:    "Should alias TurnHistoryRecord to the shared contract",
			cliType: TurnHistoryRecord{},
			want:    contract.TurnHistoryPayload{},
		},
		{
			name:    "Should alias SessionRepairRecord to the shared contract",
			cliType: SessionRepairRecord{},
			want:    contract.SessionRepairPayload{},
		},
		{
			name:    "Should alias SessionRepairIssueRecord to the shared contract",
			cliType: SessionRepairIssueRecord{},
			want:    contract.SessionRepairIssuePayload{},
		},
		{
			name:    "Should alias SessionRepairActionRecord to the shared contract",
			cliType: SessionRepairActionRecord{},
			want:    contract.SessionRepairActionPayload{},
		},
		{
			name:    "Should alias AgentRecord to the shared contract",
			cliType: AgentRecord{},
			want:    contract.AgentPayload{},
		},
		{
			name:    "Should alias AgentEventRecord to the shared contract",
			cliType: AgentEventRecord{},
			want:    contract.AgentEventPayload{},
		},
		{
			name:    "Should alias HookCatalogQuery to the shared contract",
			cliType: HookCatalogQuery{},
			want:    contract.HookCatalogQuery{},
		},
		{
			name:    "Should alias HookCatalogRecord to the shared contract",
			cliType: HookCatalogRecord{},
			want:    contract.HookCatalogPayload{},
		},
		{
			name:    "Should alias HookRunsQuery to the shared contract",
			cliType: HookRunsQuery{},
			want:    contract.HookRunsQuery{},
		},
		{
			name:    "Should alias HookRunRecord to the shared contract",
			cliType: HookRunRecord{},
			want:    contract.HookRunPayload{},
		},
		{
			name:    "Should alias HookEventsQuery to the shared contract",
			cliType: HookEventsQuery{},
			want:    contract.HookEventsQuery{},
		},
		{
			name:    "Should alias HookEventRecord to the shared contract",
			cliType: HookEventRecord{},
			want:    contract.HookEventPayload{},
		},
		{
			name:    "Should alias LogEventRecord to the shared contract",
			cliType: LogEventRecord{},
			want:    contract.LogEventPayload{},
		},
		{
			name:    "Should alias WorkspaceCreateRequest to the shared contract",
			cliType: WorkspaceCreateRequest{},
			want:    contract.CreateWorkspaceRequest{},
		},
		{
			name:    "Should alias WorkspaceUpdateRequest to the shared contract",
			cliType: WorkspaceUpdateRequest{},
			want:    contract.UpdateWorkspaceRequest{},
		},
		{
			name:    "Should alias WorkspaceRecord to the shared contract",
			cliType: WorkspaceRecord{},
			want:    contract.WorkspacePayload{},
		},
		{
			name:    "Should alias WorkspaceSkillRecord to the shared contract",
			cliType: WorkspaceSkillRecord{},
			want:    contract.WorkspaceSkillPayload{},
		},
		{
			name:    "Should alias MemoryEntryRecord to the shared contract",
			cliType: MemoryEntryRecord{},
			want:    contract.MemoryEntryResponse{},
		},
		{
			name:    "Should alias MemoryCreateRequest to the shared contract",
			cliType: MemoryCreateRequest{},
			want:    contract.MemoryCreateRequest{},
		},
		{
			name:    "Should alias MemoryHealthRecord to the shared contract",
			cliType: MemoryHealthRecord{},
			want:    contract.MemoryHealthPayload{},
		},
		{
			name:    "Should alias MemoryHistoryRecord to the shared contract",
			cliType: MemoryHistoryRecord{},
			want:    contract.MemoryOperationHistoryPayload{},
		},
		{
			name:    "Should alias MemoryMutationRecord to the shared contract",
			cliType: MemoryMutationRecord{},
			want:    contract.MemoryMutationDecisionResponse{},
		},
		{
			name:    "Should alias MemoryDreamTriggerRecord to the shared contract",
			cliType: MemoryDreamTriggerRecord{},
			want:    contract.MemoryDreamTriggerResponse{},
		},
		{
			name:    "Should alias AutomationJobCreateRequest to the shared contract",
			cliType: AutomationJobCreateRequest{},
			want:    contract.CreateJobRequest{},
		},
		{
			name:    "Should alias AutomationJobUpdateRequest to the shared contract",
			cliType: AutomationJobUpdateRequest{},
			want:    contract.UpdateJobRequest{},
		},
		{
			name:    "Should alias AutomationTriggerCreateRequest to the shared contract",
			cliType: AutomationTriggerCreateRequest{},
			want:    contract.CreateTriggerRequest{},
		},
		{
			name:    "Should alias AutomationTriggerUpdateRequest to the shared contract",
			cliType: AutomationTriggerUpdateRequest{},
			want:    contract.UpdateTriggerRequest{},
		},
		{name: "Should alias JobRecord to the shared contract", cliType: JobRecord{}, want: contract.JobPayload{}},
		{
			name:    "Should alias TriggerRecord to the shared contract",
			cliType: TriggerRecord{},
			want:    contract.TriggerPayload{},
		},
		{name: "Should alias RunRecord to the shared contract", cliType: RunRecord{}, want: contract.RunPayload{}},
		{
			name:    "Should alias DaemonStatus to the shared contract",
			cliType: DaemonStatus{},
			want:    contract.DaemonStatusPayload{},
		},
		{
			name:    "Should alias CreateBridgeRequest to the shared contract",
			cliType: CreateBridgeRequest{},
			want:    contract.CreateBridgeRequest{},
		},
		{
			name:    "Should alias UpdateBridgeRequest to the shared contract",
			cliType: UpdateBridgeRequest{},
			want:    contract.UpdateBridgeRequest{},
		},
		{
			name:    "Should alias BridgeTestDeliveryRequest to the shared contract",
			cliType: BridgeTestDeliveryRequest{},
			want:    contract.BridgeTestDeliveryRequest{},
		},
		{
			name:    "Should alias BridgeDeliveryTargetInput to the shared contract",
			cliType: BridgeDeliveryTargetInput{},
			want:    contract.BridgeDeliveryTargetInput{},
		},
		{
			name:    "Should alias BridgeRecord to the bridge domain type",
			cliType: BridgeRecord{},
			want:    bridgepkg.BridgeInstance{},
		},
		{
			name:    "Should alias BridgeRouteRecord to the bridge domain type",
			cliType: BridgeRouteRecord{},
			want:    bridgepkg.BridgeRoute{},
		},
		{
			name:    "Should alias DeliveryTargetRecord to the bridge domain type",
			cliType: DeliveryTargetRecord{},
			want:    bridgepkg.DeliveryTarget{},
		},
		{
			name:    "Should alias BridgeTestDeliveryRecord to the shared contract",
			cliType: BridgeTestDeliveryRecord{},
			want:    contract.BridgeTestDeliveryResponse{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			gotType := reflect.TypeOf(tt.cliType)
			wantType := reflect.TypeOf(tt.want)
			if gotType != wantType {
				t.Fatalf("reflect.TypeOf(%s) = %v, want %v", tt.name, gotType, wantType)
			}
		})
	}
}

func TestSharedContractJSONParity(t *testing.T) {
	t.Parallel()

	sessionResponse := `{"sessions":[{"id":"sess-1","name":"demo","agent_name":"coder","workspace_id":"ws-1","workspace_path":"/workspace/project","state":"active","acp_caps":{"supports_load_session":true,"supported_modes":["chat"]},"created_at":"2026-04-03T12:00:00Z","updated_at":"2026-04-03T12:00:00Z"}]}`
	var cliSessions struct {
		Sessions []SessionRecord `json:"sessions"`
	}
	if err := json.Unmarshal([]byte(sessionResponse), &cliSessions); err != nil {
		t.Fatalf("json.Unmarshal(cli session response) error = %v", err)
	}
	if len(cliSessions.Sessions) != 1 || cliSessions.Sessions[0].ACPCaps == nil ||
		!cliSessions.Sessions[0].ACPCaps.SupportsLoadSession {
		t.Fatalf("cli session decode = %#v, want decoded shared contract payload", cliSessions)
	}

	memoryRequest := MemoryCreateRequest{
		Scope:       memcontract.ScopeWorkspace,
		WorkspaceID: "/workspace/project",
		Type:        memcontract.TypeProject,
		Name:        "Memory",
		Content:     "payload",
	}
	cliMemoryJSON, err := json.Marshal(memoryRequest)
	if err != nil {
		t.Fatalf("json.Marshal(cli memory request) error = %v", err)
	}
	var sharedMemoryRequest = memoryRequest
	sharedMemoryJSON, err := json.Marshal(sharedMemoryRequest)
	if err != nil {
		t.Fatalf("json.Marshal(shared memory request) error = %v", err)
	}
	if !bytes.Equal(cliMemoryJSON, sharedMemoryJSON) {
		t.Fatalf("memory request json = %s, want %s", cliMemoryJSON, sharedMemoryJSON)
	}

	bridgeRequest := CreateBridgeRequest{
		Scope:         bridgepkg.ScopeGlobal,
		Platform:      "telegram",
		ExtensionName: "ext-telegram",
		DisplayName:   "Support",
		Enabled:       true,
		RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true},
	}
	cliBridgeJSON, err := json.Marshal(bridgeRequest)
	if err != nil {
		t.Fatalf("json.Marshal(cli bridge request) error = %v", err)
	}
	var sharedBridgeRequest = bridgeRequest
	sharedBridgeJSON, err := json.Marshal(sharedBridgeRequest)
	if err != nil {
		t.Fatalf("json.Marshal(shared bridge request) error = %v", err)
	}
	if !bytes.Equal(cliBridgeJSON, sharedBridgeJSON) {
		t.Fatalf("bridge request json = %s, want %s", cliBridgeJSON, sharedBridgeJSON)
	}

	readResponse := `{"memory":{"summary":{"filename":"memory.md","name":"Memory","type":"project","scope":"workspace","mod_time":"2026-04-03T12:00:00Z","injection":true},"content":"stored memory body"}}`
	var cliRead MemoryEntryRecord
	if err := json.Unmarshal([]byte(readResponse), &cliRead); err != nil {
		t.Fatalf("json.Unmarshal(cli memory show) error = %v", err)
	}
	var sharedRead contract.MemoryEntryResponse
	if err := json.Unmarshal([]byte(readResponse), &sharedRead); err != nil {
		t.Fatalf("json.Unmarshal(shared memory show) error = %v", err)
	}
	if !reflect.DeepEqual(cliRead, sharedRead) {
		t.Fatalf("memory show decode = %#v, want %#v", cliRead, sharedRead)
	}

	observeResponse := `{"events":[{"id":"sum-1","session_id":"sess-1","type":"done","agent_name":"coder","summary":"complete","timestamp":"2026-04-03T12:00:00Z"}]}`
	var cliObserve struct {
		Events []LogEventRecord `json:"events"`
	}
	if err := json.Unmarshal([]byte(observeResponse), &cliObserve); err != nil {
		t.Fatalf("json.Unmarshal(cli observe response) error = %v", err)
	}
	var sharedObserve struct {
		Events []contract.LogEventPayload `json:"events"`
	}
	if err := json.Unmarshal([]byte(observeResponse), &sharedObserve); err != nil {
		t.Fatalf("json.Unmarshal(shared observe response) error = %v", err)
	}
	if !reflect.DeepEqual(cliObserve, sharedObserve) {
		t.Fatalf("observe decode = %#v, want %#v", cliObserve, sharedObserve)
	}

	hookCatalogResponse := `{"hooks":[{"order":1,"name":"permission-guard","event":"tool.pre_call","source":"config","mode":"sync","priority":10,"executor_kind":"subprocess","matcher":{"tool_id":"agh__shell"},"metadata":{"origin":"config"}}]}`
	var cliHookCatalog struct {
		Hooks []HookCatalogRecord `json:"hooks"`
	}
	if err := json.Unmarshal([]byte(hookCatalogResponse), &cliHookCatalog); err != nil {
		t.Fatalf("json.Unmarshal(cli hook catalog response) error = %v", err)
	}
	var sharedHookCatalog struct {
		Hooks []contract.HookCatalogPayload `json:"hooks"`
	}
	if err := json.Unmarshal([]byte(hookCatalogResponse), &sharedHookCatalog); err != nil {
		t.Fatalf("json.Unmarshal(shared hook catalog response) error = %v", err)
	}
	if !reflect.DeepEqual(cliHookCatalog, sharedHookCatalog) {
		t.Fatalf("hook catalog decode = %#v, want %#v", cliHookCatalog, sharedHookCatalog)
	}

	hookRunsResponse := `{"runs":[{"hook_name":"permission-guard","event":"permission.request","source":"config","mode":"sync","duration_ms":12,"outcome":"failed","error":"boom","recorded_at":"2026-04-03T12:00:00Z"}]}`
	var cliHookRuns struct {
		Runs []HookRunRecord `json:"runs"`
	}
	if err := json.Unmarshal([]byte(hookRunsResponse), &cliHookRuns); err != nil {
		t.Fatalf("json.Unmarshal(cli hook runs response) error = %v", err)
	}
	var sharedHookRuns struct {
		Runs []contract.HookRunPayload `json:"runs"`
	}
	if err := json.Unmarshal([]byte(hookRunsResponse), &sharedHookRuns); err != nil {
		t.Fatalf("json.Unmarshal(shared hook runs response) error = %v", err)
	}
	if !reflect.DeepEqual(cliHookRuns, sharedHookRuns) {
		t.Fatalf("hook runs decode = %#v, want %#v", cliHookRuns, sharedHookRuns)
	}

	hookEventsResponse := `{"events":[{"event":"tool.pre_call","family":"tool","sync_eligible":true,"payload_schema":"ToolPreCallPayload","patch_schema":"ToolCallPatch"}]}`
	var cliHookEvents struct {
		Events []HookEventRecord `json:"events"`
	}
	if err := json.Unmarshal([]byte(hookEventsResponse), &cliHookEvents); err != nil {
		t.Fatalf("json.Unmarshal(cli hook events response) error = %v", err)
	}
	var sharedHookEvents struct {
		Events []contract.HookEventPayload `json:"events"`
	}
	if err := json.Unmarshal([]byte(hookEventsResponse), &sharedHookEvents); err != nil {
		t.Fatalf("json.Unmarshal(shared hook events response) error = %v", err)
	}
	if !reflect.DeepEqual(cliHookEvents, sharedHookEvents) {
		t.Fatalf("hook events decode = %#v, want %#v", cliHookEvents, sharedHookEvents)
	}

	daemonResponse := `{"daemon":{"status":"running","pid":10,"started_at":"2026-04-03T12:00:00Z","socket":"/tmp/agh.sock","http_host":"localhost","http_port":2123,"active_sessions":1,"total_sessions":2,"version":"dev"}}`
	var cliDaemon struct {
		Daemon DaemonStatus `json:"daemon"`
	}
	if err := json.Unmarshal([]byte(daemonResponse), &cliDaemon); err != nil {
		t.Fatalf("json.Unmarshal(cli daemon response) error = %v", err)
	}
	var sharedDaemon struct {
		Daemon contract.DaemonStatusPayload `json:"daemon"`
	}
	if err := json.Unmarshal([]byte(daemonResponse), &sharedDaemon); err != nil {
		t.Fatalf("json.Unmarshal(shared daemon response) error = %v", err)
	}
	if !reflect.DeepEqual(cliDaemon, sharedDaemon) {
		t.Fatalf("daemon decode = %#v, want %#v", cliDaemon, sharedDaemon)
	}

	bridgeResponse := `{"bridge":{"id":"brg-1","scope":"workspace","workspace_id":"ws-alpha","platform":"telegram","extension_name":"ext-telegram","display_name":"Support","enabled":true,"status":"ready","routing_policy":{"include_peer":true,"include_thread":true},"delivery_defaults":{"mode":"reply"},"created_at":"2026-04-11T12:00:00Z","updated_at":"2026-04-11T12:00:00Z"}}`
	var cliBridge struct {
		Bridge BridgeRecord `json:"bridge"`
	}
	if err := json.Unmarshal([]byte(bridgeResponse), &cliBridge); err != nil {
		t.Fatalf("json.Unmarshal(cli bridge response) error = %v", err)
	}
	var sharedBridge struct {
		Bridge bridgepkg.BridgeInstance `json:"bridge"`
	}
	if err := json.Unmarshal([]byte(bridgeResponse), &sharedBridge); err != nil {
		t.Fatalf("json.Unmarshal(shared bridge response) error = %v", err)
	}
	if !reflect.DeepEqual(cliBridge, sharedBridge) {
		t.Fatalf("bridge decode = %#v, want %#v", cliBridge, sharedBridge)
	}
}

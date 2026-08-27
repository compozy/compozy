package cli

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
)

func TestCallCreateCommand(t *testing.T) {
	t.Parallel()

	t.Run("Should parse the complete call contract and render every output format", func(t *testing.T) {
		t.Parallel()

		var captured contract.CreateCallRequest
		client := &stubClient{createCallFn: func(
			_ context.Context,
			workspaceID string,
			request contract.CreateCallRequest,
		) (contract.CallCreatePayload, error) {
			if workspaceID != "" {
				t.Fatalf("CreateCall workspace = %q, want global", workspaceID)
			}
			captured = request
			return contract.CallCreatePayload{
				CallID: "call-1", ChildSessionID: "ses_child", State: "queued", Replayed: false,
			}, nil
		}}
		deps := newDefaultProfileTestDeps(t, client)
		args := []string{
			"call", "reviewer", "Review this", "--expect", `{"type":"object"}`,
			"--strict", "--result-budget", "512KiB", "--result-overflow", "reject",
			"--idle-ttl", "1500ms", "--deadline", "2500ms", "--idempotency-key", "retry-1",
			"--runtime", "anthropic/opus/high/fast", "--tools", "read,write", "--skills", "review",
			"--mcp-servers", "github", "--sandbox-profiles", "restricted",
			"--workspace-paths", "/repo", "--network-channels", "engineering",
		}
		for _, format := range []string{"human", "json", "jsonl", "toon"} {
			t.Run("Should render call creation as "+format, func(t *testing.T) {
				stdout, stderr, err := executeRootCommand(t, deps, append(args, "-o", format)...)
				if err != nil || stderr != "" {
					t.Fatalf("call create -o %s error/stderr = %v/%q", format, err, stderr)
				}
				if !strings.Contains(stdout, "call-1") {
					t.Fatalf("call create -o %s output = %q", format, stdout)
				}
			})
		}
		item := captured.CreateCallItemRequest
		if item.Target.Agent != "reviewer" || item.Target.SessionID != "" || item.Prompt != "Review this" ||
			!item.Strict || item.ResultBudget != "512KiB" || item.ResultOverflow != "reject" ||
			item.IdempotencyKey != "retry-1" || item.IdleTTLSeconds == nil || *item.IdleTTLSeconds != 2 ||
			item.DeadlineSeconds == nil || *item.DeadlineSeconds != 3 ||
			item.Runtime == nil || item.Runtime.Provider != "anthropic" || item.Runtime.Model != "opus" ||
			item.Runtime.ReasoningEffort != "high" || item.Runtime.Speed != "fast" {
			t.Fatalf("CreateCall request = %#v", captured)
		}
		if string(item.Expect) != `{"type":"object"}` || len(item.Narrow.Tools) != 2 ||
			len(item.Narrow.Skills) != 1 || len(item.Narrow.WorkspacePaths) != 1 ||
			len(item.Narrow.NetworkChannels) != 1 || len(item.Narrow.MCPServers) != 1 ||
			len(item.Narrow.SandboxProfiles) != 1 {
			t.Fatalf("CreateCall contract/narrowing = %#v", item)
		}
	})

	t.Run("Should reject malformed expect locally with exit code two", func(t *testing.T) {
		t.Parallel()

		called := false
		client := &stubClient{createCallFn: func(
			context.Context,
			string,
			contract.CreateCallRequest,
		) (contract.CallCreatePayload, error) {
			called = true
			return contract.CallCreatePayload{}, nil
		}}
		code, stdout, stderr := executeRootCommandWithExit(
			t, newDefaultProfileTestDeps(t, client), "call", "reviewer", "Review", "--expect", `{`, "-o", "json",
		)
		if code != 2 || stdout != "" || !strings.Contains(stderr, "--expect must contain valid JSON") || called {
			t.Fatalf("malformed expect = code %d stdout %q stderr %q called %t", code, stdout, stderr, called)
		}
	})
}

func TestCallBatchCommand(t *testing.T) {
	t.Parallel()

	t.Run("Should send one exclusive tasks payload and render mixed outcomes", func(t *testing.T) {
		t.Parallel()

		client := &stubClient{createCallBatchFn: func(
			_ context.Context,
			workspaceID string,
			request contract.CreateCallRequest,
		) ([]contract.CallBatchItemPayload, error) {
			if workspaceID != "" || !request.TasksPresent || len(request.Tasks) != 2 ||
				request.Prompt != "" || request.Target != (contract.CallTargetRequest{}) {
				t.Fatalf("CreateCallBatch request = workspace %q %#v", workspaceID, request)
			}
			return []contract.CallBatchItemPayload{
				{CallID: "call-1", State: "queued"},
				{Error: &contract.CallErrorResponse{Code: "call_agent_unknown", Error: "missing"}},
			}, nil
		}}
		payload := `[{"target":{"agent":"reviewer"},"prompt":"Review"},{"target":{"session_id":"sess-child"},"prompt":"Continue"}]`
		stdout, stderr, err := executeRootCommand(
			t,
			newDefaultProfileTestDeps(t, client),
			"call",
			"batch",
			payload,
			"-o",
			"json",
		)
		if err != nil || stderr != "" || !strings.Contains(stdout, `"call_id": "call-1"`) ||
			!strings.Contains(stdout, `"code": "call_agent_unknown"`) {
			t.Fatalf("call batch output/stderr/error = %q/%q/%v", stdout, stderr, err)
		}
	})

	t.Run("Should reject an empty batch before transport", func(t *testing.T) {
		t.Parallel()

		called := false
		client := &stubClient{createCallBatchFn: func(
			context.Context,
			string,
			contract.CreateCallRequest,
		) ([]contract.CallBatchItemPayload, error) {
			called = true
			return nil, nil
		}}
		code, stdout, stderr := executeRootCommandWithExit(
			t,
			newDefaultProfileTestDeps(t, client),
			"call",
			"batch",
			`[]`,
			"-o",
			"json",
		)
		if code != 2 || stdout != "" || !strings.Contains(stderr, "at least one task") || called {
			t.Fatalf("empty call batch = code %d stdout %q stderr %q called %t", code, stdout, stderr, called)
		}
	})
}

func TestCallReadCommands(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	record := contract.CallPayload{
		CallID: "call-1", Agent: "reviewer", ChildSessionID: "ses_child", State: "completed",
		Verdict: "returned", ResultBytes: 12, CreatedAt: now, UpdatedAt: now,
	}
	client := &stubClient{
		listCallsFn: func(_ context.Context, query callListQuery) (contract.CallsResponse, error) {
			if query.Limit != 7 || query.Cursor != "after-0" || query.Caller != "ses_parent" ||
				len(query.States) != 2 || query.Attention == nil || !*query.Attention ||
				query.ChildSessionID != "ses_child" || query.RootSessionID != "ses_root" ||
				query.Agent != "reviewer" {
				t.Fatalf("ListCalls query = %#v", query)
			}
			return contract.CallsResponse{Items: []contract.CallPayload{record}, NextCursor: "after-1"}, nil
		},
		getCallFn: func(_ context.Context, _ string, callID string) (contract.CallPayload, error) {
			if callID != "call-1" {
				t.Fatalf("GetCall id = %q", callID)
			}
			return record, nil
		},
		getCallResultFn: func(_ context.Context, _ string, callID string) (contract.CallResultResponse, error) {
			return contract.CallResultResponse{CallID: callID, Result: json.RawMessage(`{"score":9}`)}, nil
		},
		getCallPromptFn: func(_ context.Context, _ string, callID string) (contract.CallPromptResponse, error) {
			return contract.CallPromptResponse{CallID: callID, Prompt: "Review this"}, nil
		},
		getCallSupersededFn: func(
			_ context.Context,
			_ string,
			callID string,
		) (contract.CallSupersededResponse, error) {
			return contract.CallSupersededResponse{CallID: callID, Result: json.RawMessage(`{"score":8}`)}, nil
		},
		cancelCallFn: func(
			_ context.Context,
			_ string,
			callID string,
			request contract.CancelCallRequest,
		) (contract.CancelCallResponse, error) {
			if callID != "call-1" || request.Reason != "done" {
				t.Fatalf("CancelCall = %q %#v", callID, request)
			}
			return contract.CancelCallResponse{State: "canceled"}, nil
		},
		publishCallFn: func(
			_ context.Context,
			_ string,
			callID string,
			request contract.PublishCallRequest,
		) (contract.PublishCallResponse, error) {
			if callID != "call-1" || request.Channel != "reviews" || request.ThreadID != "thread-1" {
				t.Fatalf("PublishCall = %q %#v", callID, request)
			}
			return contract.PublishCallResponse{NetworkMessageID: "network-1", Published: true}, nil
		},
	}
	deps := newDefaultProfileTestDeps(t, client)

	t.Run("Should render list show result cancel and publish shapes", func(t *testing.T) {
		stdout, _, err := executeRootCommand(t, deps,
			"call", "list", "--state", "running,completed", "--caller", "ses_parent",
			"--attention", "--child-session", "ses_child", "--root-session", "ses_root", "--agent", "reviewer",
			"--cursor", "after-0", "--limit", "7", "-o", "json")
		if err != nil || !strings.Contains(stdout, `"next_cursor": "after-1"`) {
			t.Fatalf("call list output/error = %q/%v", stdout, err)
		}
		for _, test := range []struct {
			name string
			args []string
			want string
		}{
			{name: "Should show a call", args: []string{"call", "show", "call-1", "-o", "human"}, want: "reviewer"},
			{name: "Should print a call result", args: []string{"call", "result", "call-1", "-o", "json"}, want: `"score": 9`},
			{name: "Should print a call prompt", args: []string{"call", "prompt", "call-1", "-o", "human"}, want: "Review this"},
			{name: "Should print superseded evidence", args: []string{"call", "superseded", "call-1", "-o", "json"}, want: `"score": 8`},
			{name: "Should cancel a call", args: []string{"call", "cancel", "call-1", "--reason", "done", "-o", "jsonl"}, want: `"state":"canceled"`},
			{name: "Should publish a call", args: []string{"call", "publish", "call-1", "--channel", "reviews", "--thread", "thread-1", "-o", "toon"}, want: "network-1"},
		} {
			t.Run(test.name, func(t *testing.T) {
				stdout, stderr, err := executeRootCommand(t, deps, test.args...)
				if err != nil || stderr != "" || !strings.Contains(stdout, test.want) {
					t.Fatalf("%v output/stderr/error = %q/%q/%v", test.args, stdout, stderr, err)
				}
			})
		}
	})

	t.Run("Should print the complete stored result without resolution metadata", func(t *testing.T) {
		// not parallel: this suite intentionally shares one mutable call client stub.

		stdout, stderr, err := executeRootCommand(t, deps, "call", "result", "call-1", "-o", "json")
		if err != nil || stderr != "" {
			t.Fatalf("call result output/stderr/error = %q/%q/%v", stdout, stderr, err)
		}
		var payload map[string]json.RawMessage
		if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
			t.Fatalf("json.Unmarshal(call result) error = %v; output=%s", err, stdout)
		}
		if _, found := payload["resolution_source"]; found {
			t.Fatalf("call result = %s, want stored payload without resolution metadata", stdout)
		}
		if string(payload["score"]) != "9" {
			t.Fatalf("call result score = %s, want 9", payload["score"])
		}
	})

	t.Run("Should emit await checkpoint data and exit three on timeout", func(t *testing.T) {
		client.awaitCallFn = func(
			_ context.Context,
			_ string,
			callID string,
			request contract.AwaitCallsRequest,
		) (contract.AwaitCallsResponse, error) {
			if callID != "call-1" || request.TimeoutMS != 1000 || request.Resume != "resume-0" {
				t.Fatalf("AwaitCall = %q %#v", callID, request)
			}
			return contract.AwaitCallsResponse{
				Pending: []string{"call-1"}, Outcome: "timeout", Resume: "resume-1", ClampedTimeoutMS: 1000,
			}, nil
		}
		code, stdout, stderr := executeRootCommandWithExit(
			t, deps, "call", "await", "call-1", "--timeout", "1s", "--resume", "resume-0", "-o", "json",
		)
		if code != 3 || !strings.Contains(stdout, `"resume": "resume-1"`) ||
			!strings.Contains(stderr, "timeout checkpoint") {
			t.Fatalf("call await = code %d stdout %q stderr %q", code, stdout, stderr)
		}
	})

	t.Run("Should preserve typed call errors and exit two", func(t *testing.T) {
		// not parallel: this suite intentionally shares one mutable call client stub.

		client.publishCallFn = func(
			context.Context,
			string,
			string,
			contract.PublishCallRequest,
		) (contract.PublishCallResponse, error) {
			return contract.PublishCallResponse{}, &daemonAPIError{
				statusCode: 409,
				status:     "409 Conflict",
				payload: contract.ErrorPayload{
					Error: "call_publish_not_settled: call is running",
					Code:  "call_publish_not_settled",
				},
			}
		}
		code, stdout, stderr := executeRootCommandWithExit(
			t,
			deps,
			"call",
			"publish",
			"call-1",
			"--channel",
			"reviews",
			"-o",
			"json",
		)
		if code != 2 || stdout != "" || !strings.Contains(stderr, `"code":"call_publish_not_settled"`) {
			t.Fatalf("call publish error = code %d stdout %q stderr %q", code, stdout, stderr)
		}
	})
}

func TestMessageAndSubtreeCommands(t *testing.T) {
	t.Parallel()

	client := &stubClient{
		sendCallMessageFn: func(
			_ context.Context,
			_ string,
			request contract.SendCallMessageRequest,
		) (contract.SendCallMessageResponse, error) {
			if request.To.SessionID != "ses_child" || request.Text != "Check this" || request.CallID != "call-1" {
				t.Fatalf("SendCallMessage request = %#v", request)
			}
			return contract.SendCallMessageResponse{MessageID: "message-1", Delivery: "queued"}, nil
		},
		listCallMessagesFn: func(_ context.Context, query callListQuery) (contract.CallMessagesResponse, error) {
			if query.SessionID != "ses_child" || query.Cursor != "after-0" || query.Limit != 5 {
				t.Fatalf("ListCallMessages query = %#v", query)
			}
			return contract.CallMessagesResponse{Items: []contract.CallMessagePayload{{
				MessageID: "message-1", From: contract.CallOwnerPayload{Kind: "operator", ID: "operator"},
				ToSessionID: "ses_child", Delivery: "queued", Text: "Check this",
			}}}, nil
		},
		stopSessionSubtreeFn: func(_ context.Context, id, reason string) (contract.StopSessionSubtreeResponse, error) {
			if id != "ses_root" || reason != "done" {
				t.Fatalf("StopSessionSubtree = %q %q", id, reason)
			}
			return contract.StopSessionSubtreeResponse{
				StoppedChildren: 2, ClosedCalls: 3, PreservedResults: 1,
			}, nil
		},
	}
	deps := newDefaultProfileTestDeps(t, client)

	t.Run("Should send and list inert messages", func(t *testing.T) {
		stdout, _, err := executeRootCommand(
			t, deps, "message", "send", "ses_child", "Check this", "--call", "call-1", "-o", "json",
		)
		if err != nil || !strings.Contains(stdout, `"message_id": "message-1"`) {
			t.Fatalf("message send output/error = %q/%v", stdout, err)
		}
		stdout, _, err = executeRootCommand(
			t, deps, "message", "list", "--session", "ses_child", "--cursor", "after-0", "--limit", "5", "-o", "human",
		)
		if err != nil || !strings.Contains(stdout, "message-1") || !strings.Contains(stdout, "Check this") {
			t.Fatalf("message list output/error = %q/%v", stdout, err)
		}
	})

	t.Run("Should render subtree drain counts in every output format", func(t *testing.T) {
		for _, format := range []string{"human", "json", "jsonl", "toon"} {
			stdout, stderr, err := executeRootCommand(
				t, deps, "session", "stop", "ses_root", "--subtree", "--reason", "done", "-o", format,
			)
			if err != nil || stderr != "" || !strings.Contains(stdout, "ses_root") ||
				(!strings.Contains(stdout, "Preserved Results") && !strings.Contains(stdout, "preserved_results")) {
				t.Fatalf("session stop -o %s output/stderr/error = %q/%q/%v", format, stdout, stderr, err)
			}
		}
	})
}

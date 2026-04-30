package events

import (
	"bytes"
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/compozy/compozy/pkg/compozy/events/kinds"
)

func TestEventJSONRoundTripPreservesEnvelope(t *testing.T) {
	t.Parallel()

	ts := time.Date(2026, time.April, 5, 12, 34, 56, 0, time.UTC)
	payload := json.RawMessage(`{"index":7,"status":"running"}`)
	event := Event{
		SchemaVersion: SchemaVersion,
		RunID:         "run-123",
		Seq:           42,
		Timestamp:     ts,
		Kind:          EventKindSessionUpdate,
		Payload:       payload,
	}

	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("marshal event: %v", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal raw event: %v", err)
	}

	for _, key := range []string{"schema_version", "run_id", "seq", "ts", "kind", "payload"} {
		if _, ok := raw[key]; !ok {
			t.Fatalf("expected key %q in %s", key, string(data))
		}
	}

	var decoded Event
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal event: %v", err)
	}

	if decoded.SchemaVersion != event.SchemaVersion {
		t.Fatalf("unexpected schema version: %q", decoded.SchemaVersion)
	}
	if decoded.RunID != event.RunID {
		t.Fatalf("unexpected run id: %q", decoded.RunID)
	}
	if decoded.Seq != event.Seq {
		t.Fatalf("unexpected seq: %d", decoded.Seq)
	}
	if !decoded.Timestamp.Equal(event.Timestamp) {
		t.Fatalf("unexpected timestamp: %v", decoded.Timestamp)
	}
	if decoded.Kind != event.Kind {
		t.Fatalf("unexpected kind: %q", decoded.Kind)
	}
	if !bytes.Equal(decoded.Payload, event.Payload) {
		t.Fatalf("unexpected payload: %s", string(decoded.Payload))
	}
}

func TestPayloadStructsRoundTripJSON(t *testing.T) {
	t.Parallel()

	uri := "https://example.com/image.png"
	oldText := "old"
	block, err := kinds.NewContentBlock(kinds.ToolUseBlock{
		ID:       "tool-1",
		Name:     "shell",
		Title:    "Shell",
		ToolName: "exec",
		Input:    json.RawMessage(`{"cmd":"echo test"}`),
		RawInput: json.RawMessage(`{"cmd":"echo test","cwd":"/tmp"}`),
	})
	if err != nil {
		t.Fatalf("create content block: %v", err)
	}

	now := time.Date(2026, time.April, 5, 10, 9, 8, 0, time.UTC)
	cases := []struct {
		name    string
		payload any
	}{
		{
			name: "run queued",
			payload: kinds.RunQueuedPayload{
				Mode:            "prd",
				Name:            "engine-kernel",
				WorkspaceRoot:   "/repo",
				IDE:             "codex",
				Model:           "gpt-5.5",
				ReasoningEffort: "high",
				AccessMode:      "full",
			},
		},
		{
			name: "run started",
			payload: kinds.RunStartedPayload{
				Mode:            "prd",
				Name:            "engine-kernel",
				WorkspaceRoot:   "/repo",
				IDE:             "codex",
				Model:           "gpt-5.5",
				ReasoningEffort: "high",
				AccessMode:      "full",
				ArtifactsDir:    "/repo/.compozy/runs/run-1",
				JobsTotal:       4,
			},
		},
		{
			name: "run crashed",
			payload: kinds.RunCrashedPayload{
				ArtifactsDir: "/repo/.compozy/runs/run-1",
				DurationMs:   75,
				Error:        "daemon restarted before terminal event flush",
				ResultPath:   "/repo/.compozy/runs/run-1/result.json",
			},
		},
		{
			name: "run completed",
			payload: kinds.RunCompletedPayload{
				ArtifactsDir:   "/repo/.compozy/runs/run-1",
				JobsTotal:      4,
				JobsSucceeded:  4,
				DurationMs:     1500,
				ResultPath:     "/repo/.compozy/runs/run-1/result.json",
				SummaryMessage: "completed",
			},
		},
		{
			name: "run failed",
			payload: kinds.RunFailedPayload{
				ArtifactsDir: "/repo/.compozy/runs/run-1",
				DurationMs:   50,
				Error:        "boom",
				ResultPath:   "/repo/.compozy/runs/run-1/result.json",
			},
		},
		{
			name:    "run canceled",
			payload: kinds.RunCancelledPayload{Reason: "sigint", RequestedBy: "signal", DurationMs: 200},
		},
		{
			name: "Should round-trip job queued payload with runtime fields",
			payload: kinds.JobQueuedPayload{
				Index:           1,
				CodeFile:        "task_01.md",
				CodeFiles:       []string{"task_01.md"},
				Issues:          1,
				TaskTitle:       "Events package",
				TaskType:        "refactor",
				SafeName:        "task_01",
				IDE:             "codex",
				Model:           "gpt-5.5",
				ReasoningEffort: "high",
				AccessMode:      "full",
				OutLog:          "/tmp/out.log",
				ErrLog:          "/tmp/err.log",
			},
		},
		{
			name: "Should round-trip job started payload with runtime fields",
			payload: kinds.JobStartedPayload{
				JobAttemptInfo:  kinds.JobAttemptInfo{Index: 1, Attempt: 1, MaxAttempts: 3},
				IDE:             "codex",
				Model:           "gpt-5.5",
				ReasoningEffort: "high",
				AccessMode:      "full",
			},
		},
		{
			name: "job attempt started",
			payload: kinds.JobAttemptStartedPayload{
				JobAttemptInfo: kinds.JobAttemptInfo{Index: 1, Attempt: 2, MaxAttempts: 3},
			},
		},
		{
			name: "job attempt finished",
			payload: kinds.JobAttemptFinishedPayload{
				JobAttemptInfo: kinds.JobAttemptInfo{Index: 1, Attempt: 2, MaxAttempts: 3},
				Status:         "failure",
				ExitCode:       1,
				Retryable:      true,
				Error:          "transient",
			},
		},
		{
			name: "job retry scheduled",
			payload: kinds.JobRetryScheduledPayload{
				JobAttemptInfo: kinds.JobAttemptInfo{Index: 1, Attempt: 2, MaxAttempts: 3},
				Reason:         "retryable",
			},
		},
		{
			name: "job completed",
			payload: kinds.JobCompletedPayload{
				JobAttemptInfo: kinds.JobAttemptInfo{Index: 1, Attempt: 1, MaxAttempts: 3},
				ExitCode:       0,
				DurationMs:     900,
			},
		},
		{
			name: "job failed",
			payload: kinds.JobFailedPayload{
				JobAttemptInfo: kinds.JobAttemptInfo{Index: 1, Attempt: 3, MaxAttempts: 3},
				CodeFile:       "task_01.md",
				ExitCode:       1,
				OutLog:         "/tmp/out.log",
				ErrLog:         "/tmp/err.log",
				Error:          "failed",
			},
		},
		{
			name: "job canceled",
			payload: kinds.JobCancelledPayload{
				JobAttemptInfo: kinds.JobAttemptInfo{Index: 1, Attempt: 1, MaxAttempts: 3},
				Reason:         "shutdown",
			},
		},
		{
			name: "session started",
			payload: kinds.SessionStartedPayload{
				Index:          1,
				ACPSessionID:   "acp-1",
				AgentSessionID: "agent-1",
				Resumed:        true,
			},
		},
		{
			name: "session update",
			payload: kinds.SessionUpdatePayload{
				Index: 1,
				Update: kinds.SessionUpdate{
					Kind:          kinds.UpdateKindToolCallUpdated,
					ToolCallID:    "tool-1",
					ToolCallState: kinds.ToolCallStateInProgress,
					Blocks:        []kinds.ContentBlock{block},
					ThoughtBlocks: []kinds.ContentBlock{block},
					PlanEntries: []kinds.SessionPlanEntry{
						{Content: "Ship task", Priority: "high", Status: "in_progress"},
					},
					AvailableCommands: []kinds.SessionAvailableCommand{
						{Name: "/help", Description: "Show help", ArgumentHint: "[topic]"},
					},
					CurrentModeID: "default",
					Usage: kinds.Usage{
						InputTokens:  10,
						OutputTokens: 5,
						TotalTokens:  15,
						CacheReads:   1,
						CacheWrites:  2,
					},
					Status: kinds.StatusRunning,
				},
			},
		},
		{
			name: "session completed",
			payload: kinds.SessionCompletedPayload{
				Index: 1,
				Usage: kinds.Usage{InputTokens: 10, OutputTokens: 5, TotalTokens: 15},
			},
		},
		{
			name: "session failed",
			payload: kinds.SessionFailedPayload{
				Index: 1,
				Error: "acp failed",
				Usage: kinds.Usage{InputTokens: 10, OutputTokens: 5, TotalTokens: 15},
			},
		},
		{
			name: "reusable agent lifecycle",
			payload: kinds.ReusableAgentLifecyclePayload{
				Stage:             kinds.ReusableAgentLifecycleStageNestedBlocked,
				AgentName:         "child",
				AgentSource:       "workspace",
				ParentAgentName:   "parent",
				AvailableAgents:   2,
				SystemPromptBytes: 512,
				MCPServers:        []string{"compozy", "filesystem"},
				Resumed:           true,
				ToolCallID:        "tool-1",
				NestedDepth:       2,
				MaxNestedDepth:    3,
				OutputRunID:       "run-child",
				Blocked:           true,
				BlockedReason:     kinds.ReusableAgentBlockedReasonCycleDetected,
				Error:             "nested execution blocked: cycle detected",
			},
		},
		{
			name: "tool call started",
			payload: kinds.ToolCallStartedPayload{
				Index:      1,
				ToolCallID: "tool-1",
				Name:       "shell",
				Title:      "Shell",
				ToolName:   "exec",
				Input:      json.RawMessage(`{"cmd":"echo hi"}`),
				RawInput:   json.RawMessage(`{"cmd":"echo hi","cwd":"/repo"}`),
			},
		},
		{
			name: "tool call updated",
			payload: kinds.ToolCallUpdatedPayload{
				Index:      1,
				ToolCallID: "tool-1",
				State:      kinds.ToolCallStateInProgress,
				Input:      json.RawMessage(`{"cmd":"echo hi"}`),
				RawInput:   json.RawMessage(`{"cmd":"echo hi","cwd":"/repo"}`),
			},
		},
		{
			name: "tool call failed",
			payload: kinds.ToolCallFailedPayload{
				Index:      1,
				ToolCallID: "tool-1",
				State:      kinds.ToolCallStateFailed,
				Error:      "exit 1",
			},
		},
		{
			name: "usage updated",
			payload: kinds.UsageUpdatedPayload{
				Index: 1,
				Usage: kinds.Usage{InputTokens: 10, OutputTokens: 5, TotalTokens: 15, CacheReads: 1, CacheWrites: 2},
			},
		},
		{
			name: "usage aggregated",
			payload: kinds.UsageAggregatedPayload{
				Usage: kinds.Usage{InputTokens: 30, OutputTokens: 10, TotalTokens: 40, CacheReads: 2, CacheWrites: 1},
			},
		},
		{
			name: "task file updated",
			payload: kinds.TaskFileUpdatedPayload{
				TasksDir:  "/repo/.compozy/tasks/engine-kernel",
				TaskName:  "task_01.md",
				FilePath:  "/repo/.compozy/tasks/engine-kernel/task_01.md",
				OldStatus: "pending",
				NewStatus: "completed",
			},
		},
		{
			name: "task metadata refreshed",
			payload: kinds.TaskMetadataRefreshedPayload{
				TasksDir:  "/repo/.compozy/tasks/engine-kernel",
				CreatedAt: now,
				UpdatedAt: now,
				Total:     8,
				Completed: 1,
				Pending:   7,
			},
		},
		{
			name: "review status finalized",
			payload: kinds.ReviewStatusFinalizedPayload{
				ReviewsDir: "/repo/.compozy/reviews/round-001",
				IssueIDs:   []string{"001", "002"},
			},
		},
		{
			name: "review round refreshed",
			payload: kinds.ReviewRoundRefreshedPayload{
				ReviewsDir: "/repo/.compozy/reviews/round-001",
				Provider:   "github",
				PR:         "123",
				Round:      4,
				CreatedAt:  now,
				Total:      3,
				Resolved:   2,
				Unresolved: 1,
			},
		},
		{
			name: "review issue resolved",
			payload: kinds.ReviewIssueResolvedPayload{
				ReviewsDir:     "/repo/.compozy/reviews/round-001",
				IssueID:        "001",
				FilePath:       "/repo/.compozy/reviews/round-001/001.md",
				Provider:       "github",
				PR:             "123",
				ProviderRef:    "thread-1",
				ProviderPosted: true,
				PostedAt:       now,
			},
		},
		{
			name: "review watch lifecycle",
			payload: kinds.ReviewWatchPayload{
				Provider:        "coderabbit",
				PR:              "123",
				Workflow:        "engine-kernel",
				Round:           2,
				RunID:           "watch-run",
				ChildRunID:      "fix-run",
				HeadSHA:         "abc123",
				ReviewID:        "review-1",
				ReviewState:     "current_reviewed",
				Status:          "completed",
				Remote:          "origin",
				Branch:          "feature",
				Total:           3,
				Resolved:        3,
				Unresolved:      0,
				Dirty:           true,
				UnpushedCommits: 1,
				Error:           "push failed",
			},
		},
		{
			name: "provider call started",
			payload: kinds.ProviderCallStartedPayload{
				CallID:     "call-1",
				Provider:   "github",
				Endpoint:   "/threads/resolve",
				Method:     "POST",
				PR:         "123",
				IssueCount: 2,
			},
		},
		{
			name: "provider call completed",
			payload: kinds.ProviderCallCompletedPayload{
				CallID:       "call-1",
				Provider:     "github",
				Endpoint:     "/threads/resolve",
				Method:       "POST",
				StatusCode:   204,
				DurationMs:   88,
				PayloadBytes: 256,
			},
		},
		{
			name: "provider call failed",
			payload: kinds.ProviderCallFailedPayload{
				CallID:       "call-1",
				Provider:     "github",
				Endpoint:     "/threads/resolve",
				Method:       "POST",
				StatusCode:   503,
				DurationMs:   88,
				PayloadBytes: 256,
				Error:        "service unavailable",
			},
		},
		{
			name: "shutdown requested",
			payload: kinds.ShutdownRequestedPayload{
				ShutdownBase: kinds.ShutdownBase{
					Source:      "signal",
					RequestedAt: now,
					DeadlineAt:  now.Add(3 * time.Second),
				},
			},
		},
		{
			name: "shutdown draining",
			payload: kinds.ShutdownDrainingPayload{
				ShutdownBase: kinds.ShutdownBase{
					Source:      "signal",
					RequestedAt: now,
					DeadlineAt:  now.Add(3 * time.Second),
				},
			},
		},
		{
			name: "shutdown terminated",
			payload: kinds.ShutdownTerminatedPayload{
				ShutdownBase: kinds.ShutdownBase{
					Source:      "signal",
					RequestedAt: now,
					DeadlineAt:  now.Add(3 * time.Second),
				},
				Forced: true,
			},
		},
		{name: "standalone text block", payload: kinds.TextBlock{Text: "hello"}},
		{
			name:    "standalone tool result block",
			payload: kinds.ToolResultBlock{ToolUseID: "tool-1", Content: "ok", IsError: true},
		},
		{
			name: "standalone diff block",
			payload: kinds.DiffBlock{
				FilePath: "pkg/compozy/events/bus.go",
				Diff:     "@@ -1 +1 @@",
				OldText:  &oldText,
				NewText:  "new",
			},
		},
		{
			name:    "standalone terminal output block",
			payload: kinds.TerminalOutputBlock{Command: "make verify", Output: "ok", ExitCode: 0, TerminalID: "term-1"},
		},
		{
			name:    "standalone image block",
			payload: kinds.ImageBlock{Data: "data:image/png;base64,AA==", MimeType: "image/png", URI: &uri},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assertJSONRoundTrip(t, tc.payload)
		})
	}
}

func assertJSONRoundTrip(t *testing.T, payload any) {
	t.Helper()

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	target := reflect.New(reflect.TypeOf(payload))
	if err := json.Unmarshal(data, target.Interface()); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}

	roundTrip, err := json.Marshal(target.Elem().Interface())
	if err != nil {
		t.Fatalf("marshal round-tripped payload: %v", err)
	}

	if !bytes.Equal(data, roundTrip) {
		t.Fatalf("payload changed after round trip:\noriginal: %s\nroundtrip: %s", string(data), string(roundTrip))
	}
}

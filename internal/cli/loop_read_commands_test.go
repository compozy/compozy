package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	looppkg "github.com/compozy/compozy/internal/loop"
)

func TestLoopRunReadCommands(t *testing.T) {
	now := time.Date(2026, 8, 20, 18, 52, 0, 0, time.UTC)
	client := loopReadCommandClient(t, now)
	deps := newTestDeps(t, client)

	t.Run("Should render the documented why transcript and executable unblocker", func(t *testing.T) {
		stdout, _, err := executeRootCommand(t, deps, "loop", "why", "run-a", "--workspace", "alpha")
		if err != nil {
			t.Fatalf("loop why error = %v", err)
		}
		want := strings.Join([]string{
			`NEEDS YOU · round 1 — Approval "release" waiting 3m`,
			"The gate is asking whether to continue. 4 of 6 steps done in round 1.",
			"Unblock: compozy loop approve run-a --gate release",
			"Watch: compozy loop events run-a --follow",
		}, "\n")
		if strings.TrimSpace(stdout) != want {
			t.Fatalf("loop why output = %q, want %q", strings.TrimSpace(stdout), want)
		}
	})

	t.Run("Should drain every roster page for run all and preserve recovered attempts", func(t *testing.T) {
		stdout, _, err := executeRootCommand(
			t,
			deps,
			"loop",
			"nodes",
			"--run",
			"run-a",
			"--all",
			"--state",
			"all",
			"--workspace",
			"alpha",
			"-o",
			"json",
		)
		if err != nil {
			t.Fatalf("loop nodes error = %v", err)
		}
		var page contract.LoopRunNodesResponse
		if err := json.Unmarshal([]byte(stdout), &page); err != nil {
			t.Fatalf("decode roster = %v", err)
		}
		if len(page.Nodes) != 2 || page.Nodes[0].Attempt != 2 ||
			len(page.Nodes[0].Attempts) != 2 || page.Nodes[1].NodeID != "ship" {
			t.Fatalf("roster = %#v", page)
		}
	})

	t.Run("Should render the documented roster transcript", func(t *testing.T) {
		stdout, _, err := executeRootCommand(
			t,
			deps,
			"loop",
			"nodes",
			"--run",
			"run-a",
			"--all",
			"--state",
			"all",
			"--workspace",
			"alpha",
		)
		if err != nil {
			t.Fatalf("loop nodes human error = %v", err)
		}
		want := strings.Join([]string{
			"ROUND  STEP   STATE      ATTEMPT  DURATION  SESSION",
			"g1     build  succeeded  2        2m31s     session-build",
			"g1     ship   queued     —        —         —",
		}, "\n")
		if strings.TrimSpace(stdout) != want {
			t.Fatalf("loop nodes output = %q, want %q", strings.TrimSpace(stdout), want)
		}
	})

	t.Run("Should render the documented timeline transcript", func(t *testing.T) {
		stdout, _, err := executeRootCommand(
			t,
			deps,
			"loop",
			"events",
			"run-a",
			"--limit",
			"3",
			"--workspace",
			"alpha",
		)
		if err != nil {
			t.Fatalf("loop events human error = %v", err)
		}
		want := strings.Join([]string{
			"SEQ  ROUND  EVENT",
			"84   g1     step build succeeded",
			`85   g1     approval "release" opened`,
			"86   —      run status: running",
		}, "\n")
		if strings.TrimSpace(stdout) != want {
			t.Fatalf("loop events output = %q, want %q", strings.TrimSpace(stdout), want)
		}
	})

	t.Run("Should render the documented run list envelope and table", func(t *testing.T) {
		stdout, _, err := executeRootCommand(t, deps, "loop", "runs", "--workspace", "alpha", "-o", "json")
		if err != nil {
			t.Fatalf("loop runs JSON error = %v", err)
		}
		var envelope struct {
			Items []map[string]any `json:"items"`
		}
		if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
			t.Fatalf("decode loop runs JSON error = %v", err)
		}
		if len(envelope.Items) != 3 || envelope.Items[0]["run_id"] != "run-needs-you" ||
			envelope.Items[0]["id"] != nil || envelope.Items[0]["attention"] == nil ||
			envelope.Items[0]["progress"] == nil {
			t.Fatalf("loop runs JSON = %#v", envelope)
		}

		stdout, _, err = executeRootCommand(t, deps, "loop", "runs", "--workspace", "alpha")
		if err != nil {
			t.Fatalf("loop runs human error = %v", err)
		}
		want := strings.Join([]string{
			"STATUS     LOOP      PROGRESS       STARTED  DURATION",
			"NEEDS YOU  review    step 4/6 · r1  18:32    22m   approval (1)",
			"running    assisted  step 2/9 · r1  18:41    13m",
			"done       review    —              17:40    18m",
		}, "\n")
		if strings.TrimSpace(stdout) != want {
			t.Fatalf("loop runs output = %q, want %q", strings.TrimSpace(stdout), want)
		}
	})

	t.Run("Should drain every missed timeline page and preserve live structured fields", func(t *testing.T) {
		stdout, _, err := executeRootCommand(
			t,
			deps,
			"loop",
			"events",
			"run-a",
			"--after",
			"1",
			"--limit",
			"2",
			"--view",
			"all",
			"--follow",
			"--workspace",
			"alpha",
			"-o",
			"jsonl",
		)
		if err != nil {
			t.Fatalf("loop events error = %v", err)
		}
		lines := strings.Split(strings.TrimSpace(stdout), "\n")
		if len(lines) != 5 {
			t.Fatalf("timeline lines = %d: %q", len(lines), stdout)
		}
		for index, line := range lines {
			var entry looppkg.TimelineEntry
			if err := json.Unmarshal([]byte(line), &entry); err != nil {
				t.Fatalf("decode timeline line %d error = %v", index, err)
			}
			wantSequence := int64(index + 2)
			if entry.Seq != wantSequence {
				t.Fatalf("timeline line %d sequence = %d, want %d", index, entry.Seq, wantSequence)
			}
			if entry.Seq == 5 && (entry.Generation != 1 || entry.NodeID != "ship" ||
				entry.Attempt != 2 || entry.Title != "step ship running") {
				t.Fatalf("live entry = %#v", entry)
			}
		}
	})

	t.Run("Should return the documented beyond-head and roster validation exit codes", func(t *testing.T) {
		exitCode, _, stderr := executeRootCommandWithExit(
			t,
			deps,
			"loop",
			"events",
			"run-a",
			"--after",
			"7",
			"--workspace",
			"alpha",
			"-o",
			"jsonl",
		)
		if exitCode != 1 || stderr != "error: position 7 is beyond this run's history (head: 6)\n" {
			t.Fatalf("loop events exact error = %q", stderr)
		}

		exitCode, _, stderr = executeRootCommandWithExit(
			t,
			deps,
			"loop",
			"nodes",
			"--state",
			"running",
			"--workspace",
			"alpha",
		)
		wantError := "error: --state running requires --run " +
			"(workspace inventory tracks exception states only)\n"
		if exitCode != 2 || stderr != wantError {
			t.Fatalf("loop nodes exact guard = %q", stderr)
		}

		exitCode, _, stderr = executeRootCommandWithExit(
			t,
			deps,
			"loop",
			"nodes",
			"--run",
			"run-a",
			"--state",
			"pending",
			"--workspace",
			"alpha",
		)
		if exitCode != 2 || !strings.Contains(
			stderr,
			"all|running|queued|waiting|retrying|paused|quarantined|succeeded|failed|canceled|not_taken",
		) {
			t.Fatalf("loop nodes allowlist = exit %d stderr %q", exitCode, stderr)
		}
		wantAllowlist := `error: invalid --state "pending"; allowed: all|running|queued|waiting|retrying|paused|quarantined|succeeded|failed|canceled|not_taken`
		if strings.TrimSpace(stderr) != wantAllowlist {
			t.Fatalf("loop nodes exact allowlist = %q, want %q", stderr, wantAllowlist)
		}

		for _, testCase := range []struct {
			args []string
			want string
		}{
			{args: []string{"loop", "nodes", "--run", "run-a", "--limit", "0", "--workspace", "alpha"}, want: "error: --limit must be between 1 and 500"},
			{args: []string{"loop", "nodes", "--run", "run-a", "--limit", "501", "--workspace", "alpha"}, want: "error: --limit must be between 1 and 500"},
			{args: []string{"loop", "events", "run-a", "--limit", "0", "--workspace", "alpha"}, want: "error: --limit must be between 1 and 500"},
			{args: []string{"loop", "events", "run-a", "--limit", "501", "--workspace", "alpha"}, want: "error: --limit must be between 1 and 500"},
			{args: []string{"loop", "runs", "--limit", "0", "--workspace", "alpha"}, want: "error: --limit must be between 1 and 500"},
			{args: []string{"loop", "runs", "--limit", "501", "--workspace", "alpha"}, want: "error: --limit must be between 1 and 500"},
		} {
			exitCode, _, stderr = executeRootCommandWithExit(t, deps, testCase.args...)
			if exitCode != 2 || strings.TrimSpace(stderr) != testCase.want {
				t.Fatalf("%v = exit %d stderr %q, want exit 2 %q", testCase.args, exitCode, stderr, testCase.want)
			}
		}

		exitCode, _, stderr = executeRootCommandWithExit(
			t,
			deps,
			"loop",
			"why",
			"run-missing",
			"--workspace",
			"alpha",
		)
		if exitCode != 1 || stderr != "error: loop run \"run-missing\" not found\n" {
			t.Fatalf("unknown loop run = exit %d stderr %q", exitCode, stderr)
		}
	})
}

func loopReadCommandClient(t *testing.T, now time.Time) *stubClient {
	t.Helper()
	return &stubClient{
		getWorkspaceFn: resolveTestLoopWorkspace(t),
		getLoopBriefingFn: func(_ context.Context, _ string, runID string) (contract.LoopBriefingResponse, error) {
			if runID == "run-missing" {
				return contract.LoopBriefingResponse{}, fmt.Errorf("loop run %q not found", runID)
			}
			return looppkg.Briefing{
				RunID:    "run-a",
				Status:   looppkg.StatusRunning,
				Tone:     looppkg.BriefingToneNeedsYou,
				Headline: `Approval "release" waiting 3m`,
				Detail:   "The gate is asking whether to continue. 4 of 6 steps done in round 1.",
				Blockers: []looppkg.Blocker{{
					Kind:      "approval",
					Unblocker: "compozy loop approve run-a --gate release",
				}},
				Progress: looppkg.StepProgress{Round: 1, StepsDone: 4, StepsTotal: 6},
			}, nil
		},
		listLoopRunsFn:     loopReadRunsFixture(now),
		getLoopRunNodesFn:  loopReadRosterFixture(t, now),
		getLoopTimelineFn:  loopReadTimelineFixture(now),
		streamLoopEventsFn: loopReadStreamFixture(t, now),
	}
}

func loopReadRunsFixture(now time.Time) func(
	context.Context,
	string,
	LoopRunListQuery,
) (contract.LoopRunsResponse, error) {
	return func(context.Context, string, LoopRunListQuery) (contract.LoopRunsResponse, error) {
		return contract.LoopRunsResponse{Runs: []contract.LoopRunPayload{
			{
				ID:             "run-needs-you",
				LoopName:       "review",
				Status:         contract.LoopRunStatusRunning,
				StartedAt:      time.Date(2026, 8, 20, 18, 32, 0, 0, time.Local),
				LastProgressAt: time.Date(2026, 8, 20, 18, 54, 0, 0, time.Local),
				Attention:      &contract.LoopRunAttention{Kind: "approval", Count: 1, Since: now},
				Progress:       contract.LoopRunProgress{Round: 1, StepsDone: 4, StepsTotal: 6},
			},
			{
				ID:             "run-active",
				LoopName:       "assisted",
				Status:         contract.LoopRunStatusRunning,
				StartedAt:      time.Date(2026, 8, 20, 18, 41, 0, 0, time.Local),
				LastProgressAt: time.Date(2026, 8, 20, 18, 54, 0, 0, time.Local),
				Progress:       contract.LoopRunProgress{Round: 1, StepsDone: 2, StepsTotal: 9},
			},
			{
				ID:             "run-done",
				LoopName:       "review",
				Status:         contract.LoopRunStatusDone,
				StartedAt:      time.Date(2026, 8, 20, 17, 40, 0, 0, time.Local),
				LastProgressAt: time.Date(2026, 8, 20, 17, 58, 0, 0, time.Local),
				Progress:       contract.LoopRunProgress{Round: 2, StepsDone: 6, StepsTotal: 6},
			},
		}}, nil
	}
}

func loopReadRosterFixture(t *testing.T, now time.Time) func(
	context.Context,
	string,
	string,
	LoopRunNodesQuery,
) (contract.LoopRunNodesResponse, error) {
	t.Helper()
	return func(
		_ context.Context,
		_ string,
		_ string,
		query LoopRunNodesQuery,
	) (contract.LoopRunNodesResponse, error) {
		if query.State != "all" || query.Limit != 500 {
			t.Fatalf("roster query = %#v", query)
		}
		if query.Cursor == "page-2" {
			return looppkg.RosterPage{
				RunID: "run-a",
				Nodes: []looppkg.RosterNode{{
					Generation: 1,
					NodeID:     "ship",
					State:      looppkg.NodeStateQueued,
				}},
			}, nil
		}
		if query.Cursor != "" {
			return contract.LoopRunNodesResponse{}, errors.New("unexpected roster cursor")
		}
		startedAt := now.Add(-151 * time.Second)
		endedAt := now
		return looppkg.RosterPage{
			RunID: "run-a",
			Nodes: []looppkg.RosterNode{{
				Generation: 1,
				NodeID:     "build",
				State:      looppkg.NodeStateSucceeded,
				Attempt:    2,
				StartedAt:  &startedAt,
				EndedAt:    &endedAt,
				SessionID:  "session-build",
				Attempts: []looppkg.NodeAttemptView{
					{Attempt: 1, State: looppkg.NodeStateFailed},
					{Attempt: 2, State: looppkg.NodeStateSucceeded},
				},
			}},
			NextCursor: "page-2",
		}, nil
	}
}

func loopReadTimelineFixture(now time.Time) func(
	context.Context,
	string,
	string,
	LoopTimelineQuery,
) (contract.LoopTimelineResponse, error) {
	return func(
		_ context.Context,
		_ string,
		_ string,
		query LoopTimelineQuery,
	) (contract.LoopTimelineResponse, error) {
		if query.After > 6 {
			return contract.LoopTimelineResponse{}, &looppkg.TimelinePositionError{
				Position: query.After,
				Head:     6,
			}
		}
		if query.After == 1 {
			if query.Limit != 2 {
				return contract.LoopTimelineResponse{}, errors.New("timeline limit was not preserved")
			}
			if query.Cursor == "older" {
				return looppkg.TimelinePage{
					RunID:   "run-a",
					HeadSeq: 4,
					Entries: []looppkg.TimelineEntry{{
						Seq:   2,
						Kind:  looppkg.RunEventGenerationStarted,
						Title: "round 1 started",
						At:    now.Add(2 * time.Second),
					}},
				}, nil
			}
			return looppkg.TimelinePage{
				RunID:   "run-a",
				HeadSeq: 4,
				Entries: []looppkg.TimelineEntry{
					{
						Seq:        4,
						Kind:       looppkg.RunEventNodeSucceeded,
						Generation: 1,
						NodeID:     "build",
						Attempt:    1,
						Title:      "step build succeeded",
						At:         now.Add(4 * time.Second),
					},
					{
						Seq:        3,
						Kind:       looppkg.RunEventNodeRunning,
						Generation: 1,
						NodeID:     "build",
						Attempt:    1,
						Title:      "step build running",
						At:         now.Add(3 * time.Second),
					},
				},
				NextCursor: "older",
			}, nil
		}
		return looppkg.TimelinePage{
			RunID:   "run-a",
			HeadSeq: 86,
			Entries: []looppkg.TimelineEntry{
				{Seq: 86, Kind: looppkg.RunEventStatusChanged, Title: "run status: running", At: now},
				{
					Seq: 85, Kind: looppkg.RunEventNeedsApproval, Generation: 1,
					Title: `approval "release" opened`, At: now,
				},
				{
					Seq: 84, Kind: looppkg.RunEventNodeSucceeded, Generation: 1,
					NodeID: "build", Title: "step build succeeded", At: now,
				},
			},
		}, nil
	}
}

func loopReadStreamFixture(t *testing.T, now time.Time) func(
	context.Context,
	string,
	string,
	int64,
	SSEHandler,
) error {
	t.Helper()
	return func(
		_ context.Context,
		_ string,
		_ string,
		after int64,
		handler SSEHandler,
	) error {
		if after != 4 {
			t.Fatalf("SSE after = %d, want durable head 4", after)
		}
		payloads := []contract.LoopRunEventPayload{
			{
				ID:          "event-5",
				LoopRunID:   "run-a",
				WorkspaceID: "ws-test",
				Seq:         5,
				Kind:        contract.LoopRunEventNodeRunning,
				Payload:     json.RawMessage(`{"generation":1,"node_id":"ship","attempt":2}`),
				At:          now.Add(5 * time.Second),
			},
			{
				ID:          "event-6",
				LoopRunID:   "run-a",
				WorkspaceID: "ws-test",
				Seq:         6,
				Kind:        contract.LoopRunEventStatusChanged,
				Payload:     json.RawMessage(`{"status":"done"}`),
				At:          now.Add(6 * time.Second),
			},
		}
		for _, payload := range payloads {
			raw, err := json.Marshal(payload)
			if err != nil {
				t.Fatalf("json.Marshal(SSE payload) error = %v", err)
			}
			if err := handler(SSEEvent{Data: raw}); err != nil {
				return err
			}
		}
		return errors.New("stream ended before terminal event")
	}
}

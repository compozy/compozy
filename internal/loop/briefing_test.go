package loop

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/loop/dsl"
	"github.com/compozy/compozy/internal/task"
	"github.com/kballard/go-shellquote"
)

func TestBriefingContract(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)
	t.Run("Should satisfy UT-001 with healthy running progress and usage", func(t *testing.T) {
		t.Parallel()
		source := healthyBriefing(now)
		got := ProjectBriefing(&source)
		if got.Tone != BriefingToneOK || got.Progress.StepsDone != 1 || got.Progress.StepsTotal != 2 ||
			got.Usage.Tokens != 25 || got.Usage.Duration != "1h0m0s" || got.Detail == "" ||
			!strings.Contains(got.Headline, "live") {
			t.Fatalf("briefing = %#v", got)
		}
	})
	t.Run("Should use the newest durable roster round for public progress", func(t *testing.T) {
		t.Parallel()
		source := healthyBriefing(now)
		source.Roster.Nodes = append(source.Roster.Nodes, RosterNode{
			Generation: 3, NodeID: "carried", State: NodeStateQuarantined, Action: true,
		})
		got := ProjectBriefing(&source)
		if got.Progress.Round != 3 {
			t.Fatalf("briefing progress round = %d, want newest durable round 3", got.Progress.Round)
		}
	})
	t.Run("Should satisfy UT-002 with the exact approval unblocker", func(t *testing.T) {
		t.Parallel()
		source := healthyBriefing(now)
		source.Run.Status = StatusNeedsApproval
		source.Run.ActiveGateID = "release"
		source.Run.LastProgressAt = now.Add(-time.Minute)
		source.ApprovalWaitingSince = now.Add(-5 * time.Minute)
		got := ProjectBriefing(&source)
		if got.Tone != BriefingToneNeedsYou || got.Detail == "" ||
			!got.Blockers[0].WaitingSince.Equal(source.ApprovalWaitingSince) {
			t.Fatalf("briefing = %#v", got)
		}
		assertBlockerCommand(t, got.Blockers, 0, []string{
			"compozy", "loop", "approve", "run-a",
			"--workspace", "ws-a",
			"--gate", "release",
		})
	})
	t.Run("Should satisfy UT-003 with approval quarantine request ordering", func(t *testing.T) {
		t.Parallel()
		source := healthyBriefing(now)
		source.Run.ActiveGateID = "gate"
		source.Roster.Nodes = append(
			source.Roster.Nodes,
			RosterNode{Generation: 2, NodeID: "q", State: NodeStateQuarantined},
		)
		source.Requests = []Request{
			{
				LoopRunID: "run-a",
				NodeID:    "ask",
				State:     "pending",
				OpenedAt:  now,
			},
		}
		got := ProjectBriefing(&source)
		if len(got.Blockers) != 3 || got.Blockers[0].Kind != gateResultApprovalKey ||
			got.Blockers[1].Kind != string(NodeControlMutationQuarantine) ||
			got.Blockers[2].Kind != NodeWaitKindRequest {
			t.Fatalf("blockers = %#v", got.Blockers)
		}
		wantArguments := []string{
			"compozy", "loop", "node", "requeue",
			"--workspace", "ws-a",
			"--run-id", "run-a",
			"--node", "q",
		}
		assertBlockerCommand(t, got.Blockers, 1, wantArguments)
	})
	t.Run("Should satisfy UT-004 with expired request truth and no retry field", func(t *testing.T) {
		t.Parallel()
		expired := now.Add(-time.Minute)
		source := healthyBriefing(now)
		source.Run.WorkspaceID = "workspace with spaces"
		source.Requests = []Request{
			{
				LoopRunID:  "run-a",
				Generation: 7,
				NodeID:     "ask;echo",
				ItemIndex:  3,
				Kind:       RequestKindAsk,
				State:      "pending",
				Expect: json.RawMessage(
					`{"type":"object","required":["environment"],"properties":{"environment":{"type":"string"}}}`,
				),
				OpenedAt:  now.Add(-time.Hour),
				ExpiresAt: &expired,
			},
		}
		got := ProjectBriefing(&source)
		if !got.Blockers[0].Expired || got.Blockers[0].ExpiresAt == nil {
			t.Fatalf("blocker = %#v", got.Blockers[0])
		}
		wantArguments := []string{
			"compozy", "loop", "respond",
			"--workspace", "workspace with spaces",
			"--run-id", "run-a",
			"--generation", "7",
			"--node", "ask;echo",
			"--item", "3",
			"--decision", "respond",
			"--payload-stdin",
		}
		assertBlockerCommand(t, got.Blockers, 0, wantArguments)
		if strings.Contains(got.Blockers[0].Unblocker, "--payload") &&
			!strings.Contains(got.Blockers[0].Unblocker, "--payload-stdin") {
			t.Fatalf(
				"request unblocker = %q, want explicit operator input without a fabricated payload",
				got.Blockers[0].Unblocker,
			)
		}
	})
	t.Run("Should preserve an explicit decision placeholder for multi-decision reviews", func(t *testing.T) {
		t.Parallel()
		source := healthyBriefing(now)
		source.Requests = []Request{
			{
				LoopRunID:  "run-a",
				Generation: 2,
				NodeID:     "review",
				Kind:       RequestKindReview,
				State:      "pending",
				Decisions:  []string{RequestDecisionApprove, RequestDecisionRespond},
				OpenedAt:   now,
			},
		}
		got := ProjectBriefing(&source)
		arguments := assertBlockerCommand(t, got.Blockers, 0, nil)
		if !slices.Contains(arguments, "<decision>") || !slices.Contains(arguments, "--payload-stdin") {
			t.Fatalf(
				"review unblocker arguments = %#v, want explicit decision and operator-provided payload",
				arguments,
			)
		}
	})
	t.Run("Should satisfy UT-005 with neutral canceled actor outcome", func(t *testing.T) {
		t.Parallel()
		source := healthyBriefing(now)
		source.Run.Status = StatusCanceled
		source.Run.LastProgressAt = now
		source.Run.ControlActor = task.ActorIdentity{Kind: task.ActorKindHuman, Ref: "pedro"}
		source.Outcome = &RunOutcome{Status: StatusCanceled, Cause: "operator_cancel", At: now}
		got := ProjectBriefing(&source)
		if got.Tone != BriefingToneOK || got.Outcome == nil || got.Outcome.ActorKind != "human" ||
			got.Outcome.Cause != "operator_cancel" || !got.Outcome.At.Equal(now) ||
			!strings.Contains(got.Headline, "pedro") {
			t.Fatalf("briefing = %#v", got)
		}
	})
	t.Run("Should satisfy UT-006 with truthful no-op and no artifacts", func(t *testing.T) {
		t.Parallel()
		source := healthyBriefing(now)
		source.Run.Status = StatusNoOp
		source.Artifacts = []RunArtifact{{Name: "invented"}}
		got := ProjectBriefing(&source)
		if len(got.Artifacts) != 0 || !strings.Contains(got.Headline, "without producing outputs") {
			t.Fatalf("briefing = %#v", got)
		}
	})
	t.Run("Should satisfy UT-007 and UT-010 with a non-empty queued boundary verdict", func(t *testing.T) {
		t.Parallel()
		source := BriefingSource{
			Run:    Run{ID: "r", Status: StatusQueued},
			Roster: RosterPage{},
			Now:    now,
		}
		got := ProjectBriefing(&source)
		if got.Tone != BriefingToneOK || !strings.Contains(strings.ToLower(got.Headline), "waiting to start") {
			t.Fatalf("briefing = %#v", got)
		}
		encoded, err := json.Marshal(got)
		if err != nil {
			t.Fatalf("json.Marshal(briefing) error = %v", err)
		}
		if strings.Contains(string(encoded), `"blockers":null`) ||
			strings.Contains(string(encoded), `"artifacts":null`) {
			t.Fatalf("briefing collections = %s, want stable empty arrays", encoded)
		}
	})
	t.Run("Should satisfy UT-008 with calm dormant watch truth", func(t *testing.T) {
		t.Parallel()
		source := healthyBriefing(now)
		source.Run.Status = StatusWatching
		got := ProjectBriefing(&source)
		if got.Tone != BriefingToneOK || !strings.Contains(got.Headline, "watch source") {
			t.Fatalf("briefing = %#v", got)
		}
	})
	t.Run("Should satisfy UT-009 with the typed unknown-run error", func(t *testing.T) {
		t.Parallel()
		service := NewRunReadService(missingRunReadStore{}, func() time.Time { return now })
		_, err := service.Briefing(context.Background(), "ws", "missing")
		if !errors.Is(err, ErrRunNotFound) {
			t.Fatalf("Briefing() error = %v", err)
		}
	})
	t.Run("Should load the exact approval opening from the durable run summary", func(t *testing.T) {
		t.Parallel()
		openedAt := now.Add(-7 * time.Minute)
		store := &approvalSummaryStore{summaries: map[RunID]RunListSummary{
			"run-a": {
				RunID: "run-a",
				Attention: &RunListAttention{
					Kind: gateResultApprovalKey, Count: 1, Since: openedAt,
				},
			},
		}}
		service := &computedRunReadService{store: store}
		got, err := service.loadApprovalWaitingSince(
			t.Context(),
			"ws-a",
			Run{ID: "run-a", Status: StatusNeedsApproval, LastProgressAt: now},
		)
		if err != nil {
			t.Fatalf("loadApprovalWaitingSince() error = %v", err)
		}
		if !got.Equal(openedAt) {
			t.Fatalf("approval opening = %s, want %s", got, openedAt)
		}
	})
	t.Run("Should reject approval state without durable opening evidence", func(t *testing.T) {
		t.Parallel()
		service := &computedRunReadService{store: &approvalSummaryStore{
			summaries: map[RunID]RunListSummary{"run-a": {RunID: "run-a"}},
		}}
		_, err := service.loadApprovalWaitingSince(
			t.Context(),
			"ws-a",
			Run{ID: "run-a", Status: StatusNeedsApproval, LastProgressAt: now},
		)
		if err == nil || !strings.Contains(err.Error(), "durable approval attention is unavailable") {
			t.Fatalf("loadApprovalWaitingSince() error = %v", err)
		}
	})
	t.Run("Should satisfy UT-049 because an absorbed failure cannot replace current activity", func(t *testing.T) {
		t.Parallel()
		source := healthyBriefing(now)
		failedEnd := now.Add(-time.Minute)
		source.Roster.Nodes[1].Attempts = []NodeAttemptView{
			{Attempt: 1, State: NodeStateFailed, EndedAt: &failedEnd},
			{Attempt: 2, State: NodeStateRunning},
		}
		got := ProjectBriefing(&source)
		if got.Tone != BriefingToneOK || !strings.Contains(got.Headline, "live") {
			t.Fatalf("briefing = %#v", got)
		}
	})
	t.Run("Should ignore historical failures and controls after a new round starts", func(t *testing.T) {
		t.Parallel()
		source := healthyBriefing(now)
		source.Roster.Nodes = append(source.Roster.Nodes,
			RosterNode{Generation: 1, NodeID: "old-failure", State: NodeStateFailed},
			RosterNode{Generation: 1, NodeID: "old-quarantine", State: NodeStateQuarantined},
			RosterNode{Generation: 1, NodeID: "old-retry", State: NodeStateRetrying},
			RosterNode{Generation: 3, NodeID: "current-worker", State: NodeStateRunning, Action: true},
		)
		got := ProjectBriefing(&source)
		if len(got.Blockers) != 0 || got.Tone != BriefingToneOK || got.Progress.Round != 3 ||
			!strings.Contains(got.Headline, "current-worker") {
			t.Fatalf("historical state replaced current work: %#v", got)
		}
		source.Run.Status = StatusDone
		source.Run.Generation = 3
		source.Roster.Nodes[len(source.Roster.Nodes)-1].State = NodeStateSucceeded
		got = ProjectBriefing(&source)
		if len(got.Blockers) != 0 || got.Tone != BriefingToneOK {
			t.Fatalf("completed recovery retains historical blockers: %#v", got)
		}
	})
	t.Run(
		"Should offer a terminal rerun that preserves the failed lane and its pending dependents",
		func(t *testing.T) {
			t.Parallel()
			source := healthyBriefing(now)
			source.Run.Status = StatusFailed
			source.Run.WorkspaceID = "workspace with spaces"
			source.Graph = dsl.Graph{
				Nodes: []dsl.Node{
					{ID: "failed", Class: dsl.NodeClassAction},
					{ID: "after", Class: dsl.NodeClassAction},
				},
				Edges: []dsl.Edge{{From: "failed", To: "after"}},
			}
			source.Roster.Nodes = []RosterNode{{Generation: 2, NodeID: "failed", State: NodeStateFailed}}
			source.Outputs = []GenerationOutput{
				{Generation: 2, NodeID: "failed", Status: "failed"},
				{Generation: 2, NodeID: "after", Status: "pending"},
			}
			got := ProjectBriefing(&source)
			assertBlockerCommand(t, got.Blockers, 0, []string{
				"compozy", "loop", "rerun", "--workspace", "workspace with spaces",
				"--run-id", "run-a", "--from-node", "failed", "--item", "0",
			})
			source.Run.Generation = 1
			if latest := ProjectBriefing(&source); latest.Blockers[0].Unblocker != got.Blockers[0].Unblocker {
				t.Fatalf("latest persisted round lost recovery guidance: %#v", latest.Blockers)
			}
			// An unrelated in-flight cell makes this same suggestion invalid at the owner.
			source.Outputs = append(source.Outputs, GenerationOutput{
				Generation: 2, NodeID: "unrelated", Status: "enqueued",
			})
			got = ProjectBriefing(&source)
			if len(got.Blockers) != 1 || got.Blockers[0].Unblocker != "" {
				t.Fatalf("advertised a rerun the planner rejects: %#v", got.Blockers)
			}
		},
	)
	t.Run("Should not recommend terminal-only reruns during live failure settlement", func(t *testing.T) {
		t.Parallel()
		source := healthyBriefing(now)
		source.Roster.Nodes[1].State = NodeStateFailed
		got := ProjectBriefing(&source)
		if len(got.Blockers) != 1 || got.Blockers[0].Unblocker != "" {
			t.Fatalf("live failure command = %#v", got.Blockers)
		}
	})
	t.Run("Should not recommend requeue after terminal quarantine settlement", func(t *testing.T) {
		t.Parallel()
		source := healthyBriefing(now)
		source.Run.Status = StatusFailed
		source.Roster.Nodes[1].State = NodeStateQuarantined
		got := ProjectBriefing(&source)
		if len(got.Blockers) != 1 || got.Blockers[0].Unblocker != "" {
			t.Fatalf("terminal quarantine command = %#v", got.Blockers)
		}
	})
	t.Run("Should satisfy UT-051 with typed terminal outcome and artifact availability", func(t *testing.T) {
		t.Parallel()
		source := healthyBriefing(now)
		source.Run.Status = StatusDone
		source.Run.LastProgressAt = now
		source.Outcome = &RunOutcome{Status: StatusDone, Cause: "verified", At: now}
		source.Artifacts = []RunArtifact{
			{
				Name:         "report",
				Output:       "task-1",
				Ref:          "sha256:x",
				Availability: ArtifactAvailable,
			},
			{Name: "summary", Availability: ArtifactPartial},
			{Name: "archive", Availability: ArtifactPruned},
		}
		got := ProjectBriefing(&source)
		if got.Outcome == nil || len(got.Artifacts) != 3 ||
			got.Outcome.Cause != "verified" || got.Detail == "" ||
			got.Artifacts[2].Availability != ArtifactPruned ||
			got.Headline != "Run finished: done." {
			t.Fatalf("briefing = %#v", got)
		}
	})
	t.Run("Should label mixed named and unnamed terminal outputs", func(t *testing.T) {
		t.Parallel()
		source := healthyBriefing(now)
		source.Run.Status = StatusDone
		source.Artifacts = []RunArtifact{
			{Name: " report ", Output: "report-output", Ref: "sha256:report"},
			{Name: " ", Output: "summary-output", Ref: "sha256:summary"},
			{Ref: "sha256:archive"},
			{},
		}
		got := ProjectBriefing(&source)
		wantNames := []string{"report", "summary-output", "output 3", "output 4"}
		gotNames := make([]string, 0, len(got.Artifacts))
		for _, artifact := range got.Artifacts {
			gotNames = append(gotNames, artifact.Name)
		}
		if !slices.Equal(gotNames, wantNames) ||
			got.Headline != "Run finished: done." {
			t.Fatalf("briefing = %#v, want artifact names %#v", got, wantNames)
		}
	})
	t.Run("Should number all unnamed terminal outputs without empty headline entries", func(t *testing.T) {
		t.Parallel()
		source := healthyBriefing(now)
		source.Run.Status = StatusDone
		source.Artifacts = []RunArtifact{{}, {}}
		got := ProjectBriefing(&source)
		if len(got.Artifacts) != 2 ||
			got.Headline != "Run finished: done." ||
			got.Artifacts[0].Name != "output 1" || got.Artifacts[1].Name != "output 2" {
			t.Fatalf("briefing = %#v", got)
		}
	})
	t.Run("Should keep terminal failure outcome when failed nodes remain in the roster", func(t *testing.T) {
		t.Parallel()
		source := healthyBriefing(now)
		source.Run.Status = StatusFailed
		source.Roster.Nodes[1].State = NodeStateFailed
		source.Outcome = &RunOutcome{Status: StatusFailed, Cause: "coordinator_failure", At: now}
		got := ProjectBriefing(&source)
		if got.Outcome == nil || got.Outcome.Cause != "coordinator_failure" || len(got.Blockers) != 1 ||
			got.Tone != BriefingToneFailed {
			t.Fatalf("briefing = %#v", got)
		}
	})
	t.Run("Should preserve durable logical artifact identity in UT-051", func(t *testing.T) {
		t.Parallel()
		artifacts := artifactsFromOutputs(dsl.Graph{}, []GenerationOutput{{
			NodeID:       "writer-node",
			ItemIndex:    3,
			TaskRunID:    "task-run-internal",
			OutputID:     "saida",
			ArtifactName: "post-final.md",
			OutputRef:    "sha256:retained",
		}}, map[string]bool{"sha256:retained": true})
		if len(artifacts) != 1 || artifacts[0].Name != "post-final.md" ||
			artifacts[0].Output != "saida" || artifacts[0].Ref != "sha256:retained" {
			t.Fatalf("artifacts = %#v", artifacts)
		}
	})
	t.Run("Should keep large result payloads out of the terminal headline", func(t *testing.T) {
		t.Parallel()
		source := healthyBriefing(now)
		source.Run.Status = StatusCanceled
		source.Outcome = &RunOutcome{Status: StatusCanceled, ActorKind: "human", ActorRef: "pedro", At: now}
		payload := `{"summary":"` + strings.Repeat("result ", 1000) + `"}`
		for range 100 {
			source.Artifacts = append(source.Artifacts, RunArtifact{Ref: payload, Availability: ArtifactAvailable})
		}
		got := ProjectBriefing(&source)
		if got.Headline != "Run canceled by human pedro." || len(got.Artifacts) != 100 ||
			got.Artifacts[0].Name != "output 1" || got.Artifacts[0].Ref != payload ||
			got.Outcome.ActorRef != source.Outcome.ActorRef {
			t.Fatalf("terminal briefing lost detail or promoted it into the headline: %#v", got)
		}
	})
	t.Run("Should expose retained action results without control markers or false pruning", func(t *testing.T) {
		t.Parallel()
		graph := dsl.Graph{Nodes: []dsl.Node{
			{ID: "review", Class: dsl.NodeClassAction},
			{ID: "branch", Class: dsl.NodeClassControl},
			{ID: "collect", Class: dsl.NodeClassControl},
			{ID: "input", Class: dsl.NodeClassSource},
		}}
		retained := OutputRefForPayload(json.RawMessage(`{"report":"retained"}`))
		pruned := OutputRefForPayload(json.RawMessage(`{"report":"pruned"}`))
		outputs := []GenerationOutput{
			{NodeID: "input", OutputRef: `"slug"`},
			{NodeID: "branch", OutputRef: "branch:true"},
			{NodeID: "collect", OutputRef: `{"count":2}`},
			{NodeID: "review", OutputRef: branchSkippedOutputRef},
			{NodeID: "review", Generation: 1, Status: "succeeded", OutputRef: `{"findings":[]}`},
			{NodeID: "review", Generation: 2, Status: "partial", OutputRef: `{"findings":["incomplete"]}`},
			{NodeID: "review", Generation: 2, ItemIndex: 1, Status: "succeeded", OutputRef: retained},
			{NodeID: "review", Generation: 2, ItemIndex: 2, Status: "succeeded", OutputRef: pruned},
		}
		got := artifactsFromOutputs(graph, outputs, map[string]bool{retained: true})
		if len(got) != 4 {
			t.Fatalf("artifacts = %#v, want four action results", got)
		}
		want := []ArtifactAvailability{ArtifactPartial, ArtifactAvailable, ArtifactPruned, ArtifactAvailable}
		wantRefs := []string{outputs[5].OutputRef, retained, pruned, outputs[4].OutputRef}
		for index, item := range got {
			if item.Availability != want[index] || item.Ref != wantRefs[index] ||
				strings.Contains(item.Name, "{") || !strings.Contains(item.Name, "review") {
				t.Fatalf("artifact %d = %#v", index, item)
			}
		}
		if got[0].Name == got[1].Name || got[1].Name == got[2].Name {
			t.Fatalf("round and item identities are ambiguous: %#v", got)
		}
	})
	t.Run("Should derive the terminal outcome from durable status evidence", func(t *testing.T) {
		t.Parallel()
		source := RosterSource{Run: Run{ID: "run-a", Status: StatusDone}}
		eventAt := now.Add(2 * time.Minute)
		err := applyRunReadEvidence(&source, []RunEvent{{
			LoopRunID: "run-a",
			Kind:      string(RunEventStatusChanged),
			Payload:   []byte(`{"status":"done","cause":"contract"}`),
			At:        eventAt,
		}})
		if err != nil {
			t.Fatalf("applyRunReadEvidence() error = %v", err)
		}
		if source.Outcome == nil || source.Outcome.Status != StatusDone ||
			source.Outcome.Cause != "verified" || !source.Outcome.At.Equal(eventAt) {
			t.Fatalf("terminal outcome = %#v", source.Outcome)
		}
	})
	t.Run("Should project a complete briefing roster beyond the maximum page size", func(t *testing.T) {
		t.Parallel()
		source := RosterSource{
			Run:   Run{ID: "run-large", Status: StatusRunning, Generation: 1},
			Graph: dsl.Graph{Nodes: []dsl.Node{{ID: "fan", Class: dsl.NodeClassAction}}},
		}
		for itemIndex := range 600 {
			source.Outputs = append(source.Outputs, GenerationOutput{
				Generation: 1,
				NodeID:     "fan",
				ItemIndex:  itemIndex,
				Status:     "succeeded",
			})
		}
		roster, err := projectCompleteRoster(&source, NodeStateFilterAll, 0)
		if err != nil {
			t.Fatalf("projectCompleteRoster() error = %v", err)
		}
		briefing := ProjectBriefing(&BriefingSource{Run: source.Run, Roster: roster, Now: now})
		if len(roster.Nodes) != 600 || briefing.Progress.StepsDone != 600 ||
			briefing.Progress.StepsTotal != 600 {
			t.Fatalf("roster/progress = %d/%#v, want 600 settled steps", len(roster.Nodes), briefing.Progress)
		}
	})
}

func assertBlockerCommand(
	t testing.TB,
	blockers []Blocker,
	index int,
	want []string,
) []string {
	t.Helper()
	if index < 0 || index >= len(blockers) {
		t.Fatalf("blocker index %d is outside %d blockers", index, len(blockers))
	}
	arguments, err := shellquote.Split(blockers[index].Unblocker)
	if err != nil {
		t.Fatalf("shellquote.Split(blocker %d) error = %v", index, err)
	}
	if want != nil && !slices.Equal(arguments, want) {
		t.Fatalf("blocker %d arguments = %#v, want %#v", index, arguments, want)
	}
	return arguments
}

type missingRunReadStore struct{ RunReadStore }

func (missingRunReadStore) GetLoopRun(context.Context, WorkspaceID, RunID) (Run, error) {
	return Run{}, ErrRunNotFound
}

type approvalSummaryStore struct {
	RunReadStore
	summaries map[RunID]RunListSummary
}

func (s *approvalSummaryStore) ListLoopRunSummaries(
	context.Context,
	WorkspaceID,
	[]RunID,
) (map[RunID]RunListSummary, error) {
	return s.summaries, nil
}

func healthyBriefing(now time.Time) BriefingSource {
	return BriefingSource{
		Run: Run{
			ID:             "run-a",
			WorkspaceID:    "ws-a",
			Status:         StatusRunning,
			Generation:     2,
			StartedAt:      now.Add(-time.Hour),
			LastProgressAt: now.Add(-time.Minute),
			TokensUsed:     25,
			BudgetTokens:   100,
		},
		Roster: RosterPage{
			Nodes: []RosterNode{
				{Generation: 2, NodeID: "done", State: NodeStateSucceeded, Action: true},
				{Generation: 2, NodeID: "live", State: NodeStateRunning, Action: true},
			},
		},
		Now: now,
	}
}

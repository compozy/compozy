package loop

import (
	"context"
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
			got.Usage.Tokens != 25 || !strings.Contains(got.Headline, "live") {
			t.Fatalf("briefing = %#v", got)
		}
	})
	t.Run("Should satisfy UT-002 with the exact approval unblocker", func(t *testing.T) {
		t.Parallel()
		source := healthyBriefing(now)
		source.Run.Status = StatusNeedsApproval
		source.Run.ActiveGateID = "release"
		got := ProjectBriefing(&source)
		if got.Tone != BriefingToneNeedsYou ||
			got.Blockers[0].Unblocker != "compozy loop approve run-a --gate release --workspace ws-a" {
			t.Fatalf("briefing = %#v", got)
		}
	})
	t.Run("Should satisfy UT-003 with approval quarantine request ordering", func(t *testing.T) {
		t.Parallel()
		source := healthyBriefing(now)
		source.Run.ActiveGateID = "gate"
		source.Roster.Nodes = append(source.Roster.Nodes, RosterNode{NodeID: "q", State: NodeStateQuarantined})
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
		arguments, err := shellquote.Split(got.Blockers[1].Unblocker)
		if err != nil {
			t.Fatalf("shellquote.Split(quarantine unblocker) error = %v", err)
		}
		wantArguments := []string{
			"compozy", "loop", "node", "requeue",
			"--workspace", "ws-a",
			"--run-id", "run-a",
			"--node", "q",
		}
		if !slices.Equal(arguments, wantArguments) {
			t.Fatalf("quarantine unblocker arguments = %#v, want %#v", arguments, wantArguments)
		}
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
				OpenedAt:   now.Add(-time.Hour),
				ExpiresAt:  &expired,
			},
		}
		got := ProjectBriefing(&source)
		if !got.Blockers[0].Expired || got.Blockers[0].ExpiresAt == nil {
			t.Fatalf("blocker = %#v", got.Blockers[0])
		}
		arguments, err := shellquote.Split(got.Blockers[0].Unblocker)
		if err != nil {
			t.Fatalf("shellquote.Split(unblocker) error = %v", err)
		}
		wantArguments := []string{
			"compozy", "loop", "respond",
			"--workspace", "workspace with spaces",
			"--run-id", "run-a",
			"--generation", "7",
			"--node", "ask;echo",
			"--item", "3",
			"--decision", "respond",
			"--payload", "<json>",
		}
		if !slices.Equal(arguments, wantArguments) {
			t.Fatalf("unblocker arguments = %#v, want %#v", arguments, wantArguments)
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
		arguments, err := shellquote.Split(got.Blockers[0].Unblocker)
		if err != nil {
			t.Fatalf("shellquote.Split(review unblocker) error = %v", err)
		}
		if !slices.Contains(arguments, "<decision>") || !slices.Contains(arguments, "<json>") {
			t.Fatalf("review unblocker arguments = %#v, want explicit decision and payload placeholders", arguments)
		}
	})
	t.Run("Should satisfy UT-005 with neutral canceled actor outcome", func(t *testing.T) {
		t.Parallel()
		source := healthyBriefing(now)
		source.Run.Status = StatusCanceled
		source.Run.LastProgressAt = now
		source.Run.ControlActor = task.ActorIdentity{Kind: task.ActorKindHuman, Ref: "pedro"}
		got := ProjectBriefing(&source)
		if got.Tone != BriefingToneOK || got.Outcome == nil || got.Outcome.ActorKind != "human" ||
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
	t.Run("Should satisfy UT-051 with typed terminal outcome and artifact availability", func(t *testing.T) {
		t.Parallel()
		source := healthyBriefing(now)
		source.Run.Status = StatusDone
		source.Run.LastProgressAt = now
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
			got.Artifacts[2].Availability != ArtifactPruned ||
			!strings.Contains(got.Headline, "Produced: report, summary, archive") {
			t.Fatalf("briefing = %#v", got)
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
		roster, err := projectCompleteRoster(&source, RosterQuery{State: NodeStateFilterAll})
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

type missingRunReadStore struct{ RunReadStore }

func (missingRunReadStore) GetLoopRun(context.Context, WorkspaceID, RunID) (Run, error) {
	return Run{}, ErrRunNotFound
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

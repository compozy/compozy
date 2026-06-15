package ui

import (
	"encoding/json"
	"testing"
	"time"

	apicore "github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/pkg/compozy/events"
	"github.com/compozy/compozy/pkg/compozy/events/kinds"
)

func TestReviewWatchModelAddsChildTabFromFixStartedEvent(t *testing.T) {
	t.Parallel()

	mdl := newRemoteReviewWatchModel(RemoteReviewWatchAttachOptions{
		Snapshot: apicore.RunSnapshot{
			Run: apicore.Run{
				RunID:        "review-watch-1",
				Mode:         "review_watch",
				WorkflowSlug: "pr-51",
				Status:       remoteRunStatusRunning,
			},
		},
	})

	mdl.handleParentEvent(reviewWatchEvent(t, events.EventKindReviewWatchFixStarted, kinds.ReviewWatchPayload{
		Provider:   "coderabbit",
		PR:         "51",
		Workflow:   "pr-51",
		Round:      2,
		HeadSHA:    "1234567890abcdef",
		ChildRunID: "review-fix-2",
	}))

	if mdl.overview.phase != reviewWatchStatusFixing {
		t.Fatalf("overview phase = %q, want fixing", mdl.overview.phase)
	}
	if mdl.overview.workflow != "pr-51" || mdl.overview.pr != "51" {
		t.Fatalf("overview = %#v, want workflow/pr metadata", mdl.overview)
	}
	if len(mdl.children) != 1 {
		t.Fatalf("child tabs = %d, want 1", len(mdl.children))
	}
	if child := mdl.children[0]; child.runID != "review-fix-2" || child.slug != "round 002" {
		t.Fatalf("child tab = %#v, want review-fix-2 round 002", child)
	}
	if mdl.activeTab != 1 {
		t.Fatalf("active tab = %d, want child tab 1", mdl.activeTab)
	}
	if len(mdl.overview.lines) != 1 || mdl.overview.lines[0] == "" {
		t.Fatalf("overview timeline = %#v, want fix-started line", mdl.overview.lines)
	}
}

func TestReviewWatchModelAppliesWorkspaceRootToConfig(t *testing.T) {
	t.Parallel()

	t.Run("Should apply workspace root to config", func(t *testing.T) {
		t.Parallel()

		mdl := newRemoteReviewWatchModel(RemoteReviewWatchAttachOptions{
			Snapshot: apicore.RunSnapshot{
				Run: apicore.Run{RunID: "review-watch-1", Status: remoteRunStatusRunning},
			},
			WorkspaceRoot: "  /tmp/compozy-review  ",
		})
		if mdl.cfg.WorkspaceRoot != "/tmp/compozy-review" {
			t.Fatalf("workspace root = %q, want trimmed workspace root", mdl.cfg.WorkspaceRoot)
		}
	})
}

func TestReviewWatchModelUpdatesChildStatusFromFixCompletedEvent(t *testing.T) {
	t.Parallel()

	mdl := newRemoteReviewWatchModel(RemoteReviewWatchAttachOptions{})
	mdl.handleParentEvent(reviewWatchEvent(t, events.EventKindReviewWatchFixStarted, kinds.ReviewWatchPayload{
		Round:      1,
		ChildRunID: "review-fix-1",
	}))
	mdl.handleParentEvent(reviewWatchEvent(t, events.EventKindReviewWatchFixCompleted, kinds.ReviewWatchPayload{
		Round:      1,
		ChildRunID: "review-fix-1",
		Status:     remoteRunStatusCompleted,
	}))

	if len(mdl.children) != 1 {
		t.Fatalf("child tabs = %d, want 1", len(mdl.children))
	}
	if mdl.children[0].status != taskMultiStatusCompleted || !mdl.children[0].terminal {
		t.Fatalf("child tab = %#v, want completed terminal", mdl.children[0])
	}
	if childRunID := childRunIDFromReviewWatchEvent(reviewWatchEvent(
		t,
		events.EventKindReviewWatchFixCompleted,
		kinds.ReviewWatchPayload{ChildRunID: "review-fix-1"},
	)); childRunID != "review-fix-1" {
		t.Fatalf("childRunIDFromReviewWatchEvent() = %q, want review-fix-1", childRunID)
	}
}

func TestReviewWatchModelSuppressesDuplicateChildTabs(t *testing.T) {
	t.Parallel()

	mdl := newRemoteReviewWatchModel(RemoteReviewWatchAttachOptions{})
	started := reviewWatchEvent(t, events.EventKindReviewWatchFixStarted, kinds.ReviewWatchPayload{
		Round:      1,
		ChildRunID: "review-fix-1",
	})
	mdl.handleParentEvent(started)
	mdl.handleParentEvent(started)

	if len(mdl.children) != 1 {
		t.Fatalf("child tabs = %d, want 1", len(mdl.children))
	}
	if mdl.children[0].runID != "review-fix-1" {
		t.Fatalf("child run id = %q, want review-fix-1", mdl.children[0].runID)
	}
}

func TestReviewWatchModelStopQuitRequestsDrainAndCancelsChildren(t *testing.T) {
	t.Parallel()

	mdl := newRemoteReviewWatchModel(RemoteReviewWatchAttachOptions{
		OwnerSession: true,
		Snapshot: apicore.RunSnapshot{
			Run: apicore.Run{
				RunID:  "review-watch-1",
				Mode:   "review_watch",
				Status: remoteRunStatusRunning,
			},
		},
	})
	mdl.handleParentEvent(reviewWatchEvent(t, events.EventKindReviewWatchFixStarted, kinds.ReviewWatchPayload{
		Round:      1,
		ChildRunID: "review-fix-1",
	}))
	var requests []uiQuitRequest
	mdl.onQuit = func(req uiQuitRequest) {
		requests = append(requests, req)
	}

	if cmd := mdl.handleQuitKey(); cmd != nil {
		t.Fatalf("handleQuitKey() returned cmd before dialog confirmation")
	}
	if !mdl.quitDialog.Active {
		t.Fatal("quit dialog is closed, want open")
	}
	mdl.quitDialog.Selected = quitDialogActionStop
	cmd := mdl.confirmQuitDialog()
	if cmd == nil {
		t.Fatal("confirmQuitDialog(stop) returned nil cmd")
	}
	_ = cmd()

	if len(requests) != 1 || requests[0] != uiQuitRequestDrain {
		t.Fatalf("quit requests = %#v, want one drain request", requests)
	}
	if mdl.shutdown.Phase != shutdownPhaseDraining {
		t.Fatalf("shutdown phase = %q, want draining", mdl.shutdown.Phase)
	}
	if len(mdl.children) != 1 || mdl.children[0].status != taskMultiStatusCanceled || !mdl.children[0].terminal {
		t.Fatalf("child tabs after stop = %#v, want canceled terminal child", mdl.children)
	}
	if mdl.children[0].errorText != "stop requested" {
		t.Fatalf("child error text = %q, want stop requested", mdl.children[0].errorText)
	}
}

func TestReviewWatchModelParentTerminalEventsCloseQuitDialog(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		kind       events.EventKind
		wantStatus string
		wantPhase  string
	}{
		{
			name:       "completed",
			kind:       events.EventKindRunCompleted,
			wantStatus: remoteRunStatusCompleted,
			wantPhase:  reviewWatchStatusDone,
		},
		{
			name:       "failed",
			kind:       events.EventKindRunFailed,
			wantStatus: remoteRunStatusFailed,
			wantPhase:  reviewWatchStatusFailed,
		},
		{
			name:       "canceled",
			kind:       events.EventKindRunCancelled,
			wantStatus: remoteRunStatusCanceled,
			wantPhase:  reviewWatchStatusCanceled,
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			mdl := newRemoteReviewWatchModel(RemoteReviewWatchAttachOptions{})
			mdl.quitDialog.Open()
			mdl.handleParentEvent(reviewWatchEvent(t, tc.kind, kinds.ReviewWatchPayload{}))

			if mdl.parentRun.Status != tc.wantStatus {
				t.Fatalf("parent status = %q, want %q", mdl.parentRun.Status, tc.wantStatus)
			}
			if mdl.overview.phase != tc.wantPhase {
				t.Fatalf("overview phase = %q, want %q", mdl.overview.phase, tc.wantPhase)
			}
			if mdl.quitDialog.Active {
				t.Fatal("quit dialog is open, want closed")
			}
		})
	}
}

func reviewWatchEvent(t *testing.T, kind events.EventKind, payload kinds.ReviewWatchPayload) events.Event {
	t.Helper()

	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	return events.Event{
		SchemaVersion: events.SchemaVersion,
		RunID:         "review-watch-1",
		Seq:           1,
		Kind:          kind,
		Timestamp:     time.Date(2026, 6, 11, 12, 0, 0, 0, time.UTC),
		Payload:       raw,
	}
}

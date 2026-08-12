package worktree

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

// Canonical suite: daemon-owned exit ladder, refusal copy, scope, and cleanup evidence.
func TestExitPlan(t *testing.T) {
	t.Parallel()

	forge := &ForgeCapabilities{Provider: "github", OpenActionLabel: "Open PR", ViewActionLabel: "View PR"}
	openState, prNumber := "open", 42
	cases := []struct {
		name       string
		status     Status
		forge      *ForgeCapabilities
		forgeState *ForgeStatus
		baseAhead  int
		browserURL string
		pause      string
		primary    ExitAction
		blocked    map[ExitAction]string
		viewLabel  string
	}{
		{
			name: "dirty worktree starts at commit", status: exitStatus(2, true, 0, 0), forge: forge,
			baseAhead: 1, primary: ExitActionCommit,
			blocked: map[ExitAction]string{ExitActionPush: reasonCommitFirst, ExitActionOpenPR: reasonCommitFirst},
		},
		{
			name: "unpublished clean branch advances to push", status: exitStatus(0, false, 0, 0), forge: forge,
			baseAhead: 1, primary: ExitActionPush,
			blocked: map[ExitAction]string{ExitActionOpenPR: reasonPushFirst},
		},
		{
			name: "pushed branch advances to open PR", status: exitStatus(0, true, 0, 0), forge: forge,
			baseAhead: 1, primary: ExitActionOpenPR,
		},
		{
			name: "cached PR advances to view PR", status: exitStatus(0, true, 0, 0), forge: forge,
			forgeState: &ForgeStatus{PRState: &openState, PRNumber: &prNumber, PRURL: "https://github.com/acme/repo/pull/42"},
			baseAhead:  1, primary: ExitActionViewPR, viewLabel: "View PR",
		},
		{
			name: "cached PR remains visible without an available provider", status: exitStatus(0, true, 0, 0),
			forgeState: &ForgeStatus{PRState: &openState, PRNumber: &prNumber, PRURL: "https://github.com/acme/repo/pull/42"},
			baseAhead:  1, primary: ExitActionViewPR, viewLabel: "View in browser",
		},
		{
			name: "diverged branch refuses publish actions", status: exitStatus(0, true, 2, 1), forge: forge,
			baseAhead: 2,
			blocked:   map[ExitAction]string{ExitActionPush: reasonDiverged, ExitActionOpenPR: reasonDiverged},
		},
		{
			name: "behind branch refuses publish actions", status: exitStatus(0, true, 0, 1), forge: forge,
			baseAhead: 1,
			blocked:   map[ExitAction]string{ExitActionPush: reasonBehind, ExitActionOpenPR: reasonBehind},
		},
		{
			name: "zero base delta refuses PR", status: exitStatus(0, true, 0, 0), forge: forge,
			blocked: map[ExitAction]string{ExitActionOpenPR: reasonNoPRChanges},
		},
		{
			name: "browser tier remains actionable without forge fields", status: exitStatus(0, true, 0, 0),
			baseAhead: 1, browserURL: "https://github.com/acme/repo/compare/main...feature", primary: ExitActionOpenPR,
		},
		{
			name: "global pause disables every action", status: exitStatus(1, true, 0, 0), forge: forge,
			baseAhead: 1, pause: reasonSessionRunning, primary: ExitActionCommit,
			blocked: map[ExitAction]string{
				ExitActionCommit: reasonSessionRunning, ExitActionCommitPush: reasonSessionRunning,
				ExitActionPush: reasonSessionRunning, ExitActionOpenPR: reasonSessionRunning,
			},
		},
	}
	for _, testCase := range cases {
		t.Run("Should "+testCase.name, func(t *testing.T) {
			t.Parallel()
			rows := buildExitActions(
				&testCase.status, testCase.forge, testCase.forgeState, testCase.baseAhead,
				true, testCase.browserURL, testCase.pause,
			)
			if got := selectExitPrimary(rows, &testCase.status); got != testCase.primary {
				t.Fatalf("primary = %q, want %q; rows=%#v", got, testCase.primary, rows)
			}
			for action, reason := range testCase.blocked {
				row := exitActionRow(t, rows, action)
				if row.Enabled || row.BlockedReason != reason {
					t.Fatalf("%s row = %#v, want disabled %q", action, row, reason)
				}
			}
			if testCase.browserURL != "" {
				row := exitActionRow(t, rows, ExitActionOpenPR)
				if row.URL != testCase.browserURL || row.Label != "Open in browser" {
					t.Fatalf("browser row = %#v, want neutral browser action", row)
				}
			}
			if testCase.viewLabel != "" {
				row := exitActionRow(t, rows, ExitActionViewPR)
				if row.Label != testCase.viewLabel || row.URL != testCase.forgeState.PRURL {
					t.Fatalf("cached PR row = %#v, want label %q", row, testCase.viewLabel)
				}
			}
		})
	}

	t.Run("Should bound named untracked scope without counting ignored entries", func(t *testing.T) {
		t.Parallel()
		var fixture strings.Builder
		for index := range exitUntrackedFileLimit + 5 {
			fmt.Fprintf(&fixture, "? file-%03d.txt\x00", index)
		}
		fixture.WriteString("! ignored.txt\x00")
		service := NewService(newMemoryWorktreeStore(), &recordingGitRunner{responses: []gitResponse{{
			stdout: []byte(fixture.String()),
		}}})
		scope, err := service.readExitCommitScope(context.Background(), Worktree{Path: "/repo"}, &Status{})
		if err != nil {
			t.Fatalf("readExitCommitScope() error = %v", err)
		}
		if len(scope.UntrackedFiles) != exitUntrackedFileLimit || scope.UntrackedTotal != exitUntrackedFileLimit+5 ||
			!scope.UntrackedTruncated || strings.Contains(strings.Join(scope.UntrackedFiles, "\n"), "ignored.txt") {
			t.Fatalf("scope = %#v, want bounded non-ignored names", scope)
		}
	})

	t.Run("Should prefer local and remote cleanup evidence without claiming unique commits are safe", func(t *testing.T) {
		t.Parallel()
		for _, testCase := range []struct {
			name      string
			responses []gitResponse
			want      ExitCleanupEvidence
		}{
			{name: "no unique commits", responses: []gitResponse{{stdout: []byte("0\n")}},
				want: ExitCleanupEvidence{Safe: true, Source: "local", Summary: "No commits unique to this branch."}},
			{name: "remote branch downgrade", responses: []gitResponse{{stdout: []byte("2\n")}, {stdout: []byte("origin/feature\n")}},
				want: ExitCleanupEvidence{Safe: true, Source: "local", Downgraded: true, Summary: "Unique commits are already on a remote branch."}},
			{name: "unique commits block", responses: []gitResponse{{stdout: []byte("2\n")}, {}},
				want: ExitCleanupEvidence{Source: "local", Blocker: "2 commits exist nowhere else."}},
		} {
			t.Run("Should report "+testCase.name, func(t *testing.T) {
				t.Parallel()
				service := NewService(newMemoryWorktreeStore(), &recordingGitRunner{responses: testCase.responses})
				got := service.exitCleanupEvidence(context.Background(), Worktree{Path: "/repo", Branch: "feature"}, nil)
				if got != testCase.want {
					t.Fatalf("cleanup = %#v, want %#v", got, testCase.want)
				}
			})
		}
	})

	t.Run("Should apply merged evidence freshness before local cleanup evidence", func(t *testing.T) {
		t.Parallel()
		fetchedAt := time.Date(2026, time.August, 12, 12, 0, 0, 0, time.UTC)
		merged, closed := "merged", "closed"
		for _, testCase := range []struct {
			name      string
			forge     *ForgeStatus
			responses []gitResponse
			want      ExitCleanupEvidence
		}{
			{
				name:      "fresh merged forge verdict",
				forge:     &ForgeStatus{Provider: "github", PRState: &merged, FetchedAt: &fetchedAt},
				responses: []gitResponse{{stdout: []byte("2026-08-12T11:00:00Z\n")}},
				want: ExitCleanupEvidence{
					ForgeState: "merged", Safe: true, Source: "forge", Summary: "Merged on github",
				},
			},
			{
				name:  "local commit newer than merged verdict",
				forge: &ForgeStatus{Provider: "github", PRState: &merged, FetchedAt: &fetchedAt},
				responses: []gitResponse{
					{stdout: []byte("2026-08-12T13:00:00Z\n")}, {stdout: []byte("1\n")}, {},
				},
				want: ExitCleanupEvidence{
					ForgeState: "merged", Source: "local", Blocker: "1 commits exist nowhere else.",
				},
			},
			{
				name:      "closed but not merged verdict",
				forge:     &ForgeStatus{Provider: "github", PRState: &closed, FetchedAt: &fetchedAt},
				responses: []gitResponse{{stdout: []byte("1\n")}, {}},
				want: ExitCleanupEvidence{
					ForgeState: "closed", Source: "local", Blocker: "1 commits exist nowhere else.",
				},
			},
			{
				name:      "merged verdict without a timestamp",
				forge:     &ForgeStatus{Provider: "github", PRState: &merged},
				responses: []gitResponse{{stdout: []byte("1\n")}, {}},
				want: ExitCleanupEvidence{
					ForgeState: "merged", Source: "local", Blocker: "1 commits exist nowhere else.",
				},
			},
			{
				name:      "merged verdict with unreadable local commit time",
				forge:     &ForgeStatus{Provider: "github", PRState: &merged, FetchedAt: &fetchedAt},
				responses: []gitResponse{{err: context.Canceled}, {stdout: []byte("1\n")}, {}},
				want: ExitCleanupEvidence{
					ForgeState: "merged", Source: "local", Blocker: "1 commits exist nowhere else.",
				},
			},
			{
				name:      "merged verdict with invalid local commit time",
				forge:     &ForgeStatus{Provider: "github", PRState: &merged, FetchedAt: &fetchedAt},
				responses: []gitResponse{{stdout: []byte("not-a-time\n")}, {stdout: []byte("1\n")}, {}},
				want: ExitCleanupEvidence{
					ForgeState: "merged", Source: "local", Blocker: "1 commits exist nowhere else.",
				},
			},
		} {
			t.Run("Should prefer "+testCase.name, func(t *testing.T) {
				t.Parallel()
				service := NewService(newMemoryWorktreeStore(), &recordingGitRunner{responses: testCase.responses})
				got := service.exitCleanupEvidence(
					context.Background(), Worktree{Path: "/repo", Branch: "feature"}, testCase.forge,
				)
				if got != testCase.want {
					t.Fatalf("cleanup = %#v, want %#v", got, testCase.want)
				}
			})
		}
	})

	t.Run("Should resolve PR base from merge base then upstream then provider default", func(t *testing.T) {
		t.Parallel()
		statusWithUpstream := exitStatus(0, true, 0, 0)
		for _, testCase := range []struct {
			name      string
			status    *Status
			forge     *ForgeCapabilities
			responses []gitResponse
			want      string
		}{
			{
				name: "recorded gh merge base", status: &statusWithUpstream,
				responses: []gitResponse{{stdout: []byte("release\n")}}, want: "release",
			},
			{
				name: "different upstream branch", status: &statusWithUpstream,
				responses: []gitResponse{{err: context.Canceled}, {stdout: []byte("origin/trunk\n")}}, want: "trunk",
			},
			{
				name: "provider default", status: &Status{}, forge: &ForgeCapabilities{DefaultBranch: "develop"},
				responses: []gitResponse{{err: context.Canceled}}, want: "develop",
			},
		} {
			t.Run("Should use "+testCase.name, func(t *testing.T) {
				t.Parallel()
				service := NewService(newMemoryWorktreeStore(), &recordingGitRunner{responses: testCase.responses})
				got := service.resolveExitBase(
					context.Background(), Worktree{Path: "/repo", Branch: "feature", BaseRef: "main"},
					testCase.status, testCase.forge,
				)
				if got != testCase.want {
					t.Fatalf("resolveExitBase() = %q, want %q", got, testCase.want)
				}
			})
		}
	})
}

func exitStatus(dirty int, upstream bool, ahead, behind int) Status {
	detached := false
	return Status{
		DirtyFiles: &dirty, Detached: &detached, HasUpstream: &upstream, Ahead: &ahead, Behind: &behind,
	}
}

func exitActionRow(t *testing.T, rows []ExitActionPlan, action ExitAction) ExitActionPlan {
	t.Helper()
	for _, row := range rows {
		if row.Action == action {
			return row
		}
	}
	t.Fatalf("action %q missing from %#v", action, rows)
	return ExitActionPlan{}
}

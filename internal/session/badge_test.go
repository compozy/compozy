package session

import (
	"testing"

	"github.com/compozy/compozy/internal/heartbeat"
	"github.com/compozy/compozy/internal/store"
)

func TestCanonicalBadgeAttentionPrecedence(t *testing.T) {
	t.Parallel()

	failure := &store.SessionFailure{Kind: store.FailurePrompt}
	tests := []struct {
		name  string
		input BadgeInputs
		want  Badge
	}{
		{
			name: "Should request attention for unverified stopping despite stale stopped health",
			input: BadgeInputs{
				State:                  StateStopping,
				StopVerificationFailed: true,
				HealthState:            heartbeat.SessionHealthStateStopped,
			},
			want: BadgeNeedsAttention,
		},
		{
			name:  "Should preserve stopping when the health projection is stale",
			input: BadgeInputs{State: StateStopping, HealthState: heartbeat.SessionHealthStateStopped},
			want:  BadgeRunning,
		},
		{
			name:  "Should surface a pending clarification",
			input: BadgeInputs{State: StateActive, PendingClarify: true},
			want:  BadgeWaitingForInput,
		},
		{
			name:  "Should prefer authorization over clarification",
			input: BadgeInputs{State: StateActive, PendingAuth: true, PendingClarify: true},
			want:  BadgeWaitingForAuth,
		},
		{
			name:  "Should mark unseen settled work done",
			input: BadgeInputs{State: StateActive, Unseen: true},
			want:  BadgeDone,
		},
		{
			name:  "Should prefer a live prompt over unseen settled work",
			input: BadgeInputs{State: StateActive, ActivePrompt: true, Unseen: true},
			want:  BadgeRunning,
		},
		{
			name:  "Should prefer stopped lifecycle over unseen work",
			input: BadgeInputs{State: StateStopped, Unseen: true},
			want:  BadgeStopped,
		},
		{
			name:  "Should prefer terminal failure over stopped",
			input: BadgeInputs{State: StateStopped, Failure: failure, PendingAuth: true},
			want:  BadgeFailed,
		},
		{
			name:  "Should preserve pending authorization across a clean stop",
			input: BadgeInputs{State: StateStopped, PendingAuth: true},
			want:  BadgeWaitingForAuth,
		},
		{
			name:  "Should preserve pending clarification across a clean stop",
			input: BadgeInputs{State: StateStopped, PendingClarify: true},
			want:  BadgeWaitingForInput,
		},
		{
			name:  "Should prefer clarification over hung",
			input: BadgeInputs{State: StateActive, PendingClarify: true, Stalled: true},
			want:  BadgeWaitingForInput,
		},
		{
			name:  "Should prefer hung over unhealthy",
			input: BadgeInputs{State: StateActive, Stalled: true, Health: heartbeat.SessionHealthDegraded},
			want:  BadgeHung,
		},
		{
			name: "Should prefer unhealthy over running",
			input: BadgeInputs{
				State: StateActive, Health: heartbeat.SessionHealthDegraded, ActivePrompt: true,
			},
			want: BadgeUnhealthy,
		},
		{
			name:  "Should request attention for a turn that ended early instead of completing it",
			input: BadgeInputs{State: StateActive, EndedEarly: true, Unseen: true},
			want:  BadgeNeedsAttention,
		},
		{
			name:  "Should keep an early stop after the operator has seen the turn",
			input: BadgeInputs{State: StateActive, EndedEarly: true},
			want:  BadgeNeedsAttention,
		},
		{
			name:  "Should prefer a live prompt over an early stop",
			input: BadgeInputs{State: StateActive, EndedEarly: true, ActivePrompt: true},
			want:  BadgeRunning,
		},
		{
			name:  "Should prefer open work over an early stop",
			input: BadgeInputs{State: StateActive, EndedEarly: true, Waiting: true},
			want:  BadgeWaiting,
		},
		{
			name:  "Should prefer stopped lifecycle over an early stop",
			input: BadgeInputs{State: StateStopped, EndedEarly: true},
			want:  BadgeStopped,
		},
		{
			name:  "Should derive idle for an active settled session",
			input: BadgeInputs{State: StateActive},
			want:  BadgeIdle,
		},
		{name: "Should keep zero-value input unknown", input: BadgeInputs{}, want: BadgeUnknown},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := CanonicalBadge(test.input); got != test.want {
				t.Fatalf("CanonicalBadge() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestClassForBadgeUsesCanonicalAttentionClasses(t *testing.T) {
	t.Parallel()

	tests := []struct {
		badge Badge
		want  AttentionClass
	}{
		{badge: BadgeWaitingForInput, want: AttentionNeedsYou},
		{badge: BadgeNeedsAttention, want: AttentionNeedsYou},
		{badge: BadgeWaitingForAuth, want: AttentionNeedsYou},
		{badge: BadgeFailed, want: AttentionNeedsYou},
		{badge: BadgeDone, want: AttentionFinished},
		{badge: BadgeRunning, want: AttentionNone},
		{badge: BadgeWaiting, want: AttentionNone},
		{badge: BadgeIdle, want: AttentionNone},
		{badge: BadgeStopped, want: AttentionNone},
		{badge: BadgeHung, want: AttentionNone},
		{badge: BadgeUnhealthy, want: AttentionNone},
		{badge: BadgeUnknown, want: AttentionNone},
		{badge: Badge("future-badge"), want: AttentionNone},
	}
	for _, test := range tests {
		t.Run("Should classify "+string(test.badge), func(t *testing.T) {
			t.Parallel()
			if got := ClassForBadge(test.badge); got != test.want {
				t.Fatalf("ClassForBadge(%q) = %q, want %q", test.badge, got, test.want)
			}
		})
	}
}

func TestParseBadgeFiltersUsesThePublicVocabulary(t *testing.T) {
	t.Parallel()

	t.Run("Should deduplicate and canonically order comma-separated badges", func(t *testing.T) {
		t.Parallel()
		got, err := ParseBadgeFilters([]string{"done,waiting-for-input", "done"})
		if err != nil {
			t.Fatalf("ParseBadgeFilters() error = %v", err)
		}
		want := []Badge{BadgeWaitingForInput, BadgeDone}
		if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
			t.Fatalf("ParseBadgeFilters() = %#v, want %#v", got, want)
		}
	})

	t.Run("Should reject unknown and internal fallback badges", func(t *testing.T) {
		t.Parallel()
		for _, value := range []string{"sleeping", string(BadgeUnknown)} {
			if _, err := ParseBadgeFilters([]string{value}); err == nil {
				t.Fatalf("ParseBadgeFilters(%q) error = nil", value)
			}
		}
	})
}

func TestBoardColumnFor(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		badge Badge
		state PullRequestState
		want  BoardColumn
	}{
		{"Should prioritize input over an open PR", BadgeWaitingForInput, PullRequestOpen, BoardNeedsYou},
		{"Should prioritize running over an open PR", BadgeRunning, PullRequestOpen, BoardWorking},
		{"Should place waiting sessions without a PR in waiting", BadgeWaiting, "", BoardWaiting},
		{"Should keep waiting sessions with an open PR in waiting", BadgeWaiting, PullRequestOpen, BoardWaiting},
		{"Should place idle sessions with an open PR in review", BadgeIdle, PullRequestOpen, BoardInReview},
		{"Should place merged sessions in done", BadgeDone, PullRequestMerged, BoardDone},
		{"Should place stopped sessions without a PR in done", BadgeStopped, "", BoardDone},
		{"Should place unknown sessions with an open PR in review", BadgeUnknown, PullRequestOpen, BoardInReview},
		{"Should prioritize failure over an open PR", BadgeFailed, PullRequestOpen, BoardNeedsYou},
		{"Should prioritize failure without a PR", BadgeFailed, "", BoardNeedsYou},
		{"Should keep hung sessions working", BadgeHung, PullRequestOpen, BoardWorking},
		{"Should keep unhealthy sessions working", BadgeUnhealthy, "", BoardWorking},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := BoardColumnFor(tt.badge, tt.state); got != tt.want {
				t.Fatalf("BoardColumnFor(%q, %q) = %q, want %q", tt.badge, tt.state, got, tt.want)
			}
		})
	}
}

func TestWaitingBadgePrecedenceAndReasons(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		input BadgeInputs
		want  Badge
	}{
		{"Should wait instead of completing unseen work", BadgeInputs{State: StateActive, Waiting: true, Unseen: true}, BadgeWaiting},
		{"Should preserve authorization precedence", BadgeInputs{State: StateActive, Waiting: true, PendingAuth: true}, BadgeWaitingForAuth},
		{"Should preserve running precedence", BadgeInputs{State: StateActive, Waiting: true, ActivePrompt: true}, BadgeRunning},
		{"Should preserve stopped precedence", BadgeInputs{State: StateStopped, Waiting: true}, BadgeStopped},
		{"Should preserve degraded precedence", BadgeInputs{State: StateActive, Waiting: true, Stalled: true}, BadgeHung},
		{"Should keep an ineligible session unknown", BadgeInputs{Waiting: true}, BadgeUnknown},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := CanonicalBadge(tc.input); got != tc.want {
				t.Fatalf("badge = %s, want %s", got, tc.want)
			}
		})
	}
	for _, kind := range WorkSignalKindValues() {
		t.Run("Should derive the waiting reason for "+kind, func(t *testing.T) {
			t.Parallel()
			signals := []WorkSignal{{Kind: WorkSignalKind(kind)}}
			want := WaitingOnSessions
			switch WorkSignalKind(kind) {
			case WorkSignalAgentProgress, WorkSignalToolRunning:
				want = ""
			case WorkSignalScheduledWait, WorkSignalActiveMonitor:
				want = WaitingOnMonitor
			}
			if got := WaitingOnFor(signals); got != want {
				t.Fatalf("reason = %q, want %q", got, want)
			}
			info := &Info{
				State:               StateActive,
				LastSettledRevision: 1,
				Supervision:         &SupervisionState{WorkSignals: signals},
			}
			badge := BadgeDone
			if want != "" {
				badge = BadgeWaiting
			}
			if got := BadgeForInfo(info); got != badge {
				t.Fatalf("info badge = %s, want %s", got, badge)
			}
			if got := BadgeForHealth(
				info,
				heartbeat.SessionHealth{State: heartbeat.SessionHealthStateIdle},
			); got != badge {
				t.Fatalf("health badge = %s, want %s", got, badge)
			}
		})
	}
	t.Run("Should prefer sessions over monitors regardless of order", func(t *testing.T) {
		t.Parallel()
		for _, signals := range [][]WorkSignal{
			{{Kind: WorkSignalActiveMonitor}, {Kind: WorkSignalAwaitingReply}},
			{{Kind: WorkSignalAwaitingReply}, {Kind: WorkSignalScheduledWait}},
		} {
			if got := WaitingOnFor(signals); got != WaitingOnSessions {
				t.Fatalf("reason = %s", got)
			}
		}
		if got := WaitingOnFor(nil); got != "" {
			t.Fatalf("empty reason = %s", got)
		}
	})
}

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
			name:  "Should reveal clarification immediately after authorization clears",
			input: BadgeInputs{State: StateActive, PendingClarify: true},
			want:  BadgeWaitingForInput,
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
			name:  "Should prefer authorization over clarification at the adjacent boundary",
			input: BadgeInputs{State: StateActive, PendingAuth: true, PendingClarify: true},
			want:  BadgeWaitingForAuth,
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
			name:  "Should prefer running over done",
			input: BadgeInputs{State: StateActive, ActivePrompt: true, Unseen: true},
			want:  BadgeRunning,
		},
		{
			name:  "Should prefer done over idle",
			input: BadgeInputs{State: StateActive, Unseen: true},
			want:  BadgeDone,
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
		{badge: BadgeWaitingForAuth, want: AttentionNeedsYou},
		{badge: BadgeFailed, want: AttentionNeedsYou},
		{badge: BadgeDone, want: AttentionFinished},
		{badge: BadgeRunning, want: AttentionNone},
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

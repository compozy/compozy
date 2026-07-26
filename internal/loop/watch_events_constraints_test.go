package loop

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	watchpkg "github.com/compozy/agh/internal/loop/watch"
)

func TestWatchEventsSessionConstraintsShouldResolveStreamKeys(t *testing.T) {
	t.Parallel()

	sessionContract := WatchEventsContract{Stream: watchEventsSessionStream}
	tests := []struct {
		name       string
		filter     string
		inputs     map[string]any
		want       []string
		wantErr    string
		wantHasIDs bool
	}{
		{
			name:       "Should resolve a literal session equality",
			filter:     `"sess-literal" == event.session_id && event.payload.record_type == "agent_message"`,
			want:       []string{"session_events:sess-literal"},
			wantHasIDs: true,
		},
		{
			name:       "Should resolve a dotted inputs path",
			filter:     `event.session_id == inputs.session_id`,
			inputs:     map[string]any{"session_id": " sess-input "},
			want:       []string{"session_events:sess-input"},
			wantHasIDs: true,
		},
		{
			name:       "Should resolve a bracketed nested inputs path",
			filter:     `event.session_id == inputs["session"]["id"]`,
			inputs:     map[string]any{"session": map[string]any{"id": "sess-nested"}},
			want:       []string{"session_events:sess-nested"},
			wantHasIDs: true,
		},
		{
			name:    "Should reject a missing session input",
			filter:  `event.session_id == inputs.missing`,
			wantErr: `watch-events session_id input "missing" is required`,
		},
		{
			name:    "Should reject a non-equality session predicate",
			filter:  `event.session_id != "sess-wide"`,
			wantErr: `watch-events kind "event.post_record" requires a session_id filter`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			streams, err := watchEventsStreamsForSubscription(
				watchpkg.EventSubscriptionRef{Kind: "event.post_record", Filter: tt.filter},
				sessionContract,
				tt.inputs,
			)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("watchEventsStreamsForSubscription() error = nil, want non-nil")
				}
				if !errors.Is(err, ErrValidation) {
					t.Fatalf("watchEventsStreamsForSubscription() error = %v, want ErrValidation", err)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("watchEventsStreamsForSubscription() error = %q, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("watchEventsStreamsForSubscription() error = %v", err)
			}
			if !reflect.DeepEqual(streams, tt.want) {
				t.Fatalf("watchEventsStreamsForSubscription() = %#v, want %#v", streams, tt.want)
			}
			if got := watchEventsFilterHasSessionConstraint(tt.filter); got != tt.wantHasIDs {
				t.Fatalf("watchEventsFilterHasSessionConstraint() = %v, want %v", got, tt.wantHasIDs)
			}
		})
	}
}

func TestWatchEventsSessionStreamsShouldNormalizeDynamicKeys(t *testing.T) {
	t.Parallel()

	t.Run("Should normalize dynamic session stream keys and preserve static streams", func(t *testing.T) {
		t.Parallel()

		stream := WatchEventsSessionStreamForSession(" sess-1 ")
		if got, want := stream, "session_events:sess-1"; got != want {
			t.Fatalf("WatchEventsSessionStreamForSession() = %q, want %q", got, want)
		}
		sessionID, ok := WatchEventsSessionIDFromStream(stream)
		if !ok || sessionID != "sess-1" {
			t.Fatalf("WatchEventsSessionIDFromStream() = (%q, %v), want (%q, true)", sessionID, ok, "sess-1")
		}
		if got, want := WatchEventsBaseStream(stream), watchEventsSessionStream; got != want {
			t.Fatalf("WatchEventsBaseStream(dynamic) = %q, want %q", got, want)
		}
		if got, want := WatchEventsBaseStream(watchEventsObserveStream), watchEventsObserveStream; got != want {
			t.Fatalf("WatchEventsBaseStream(static) = %q, want %q", got, want)
		}
		if empty := WatchEventsSessionStreamForSession(" "); empty != "" {
			t.Fatalf("WatchEventsSessionStreamForSession(empty) = %q, want empty", empty)
		}
		if sessionID, ok := WatchEventsSessionIDFromStream("session_events: "); ok || sessionID != "" {
			t.Fatalf("WatchEventsSessionIDFromStream(empty) = (%q, %v), want empty false", sessionID, ok)
		}
	})
}

package kinds

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestJobParkedPayloadJSONRoundTrip(t *testing.T) {
	t.Parallel()

	payload := JobParkedPayload{
		JobAttemptInfo: JobAttemptInfo{
			Index:       2,
			Attempt:     2,
			MaxAttempts: 2,
		},
		Reason:          "no output for 3m0s",
		LastToolCall:    "shell.run",
		LastProgressSeq: 4096,
		WorktreePath:    "/home/user/.compozy/state/worktrees/ws/parent/02-beta",
		LogPath:         "/home/user/.compozy/runs/run-2/journal.log",
	}

	t.Run("Should serialize every field to a stable JSON map", func(t *testing.T) {
		t.Parallel()
		got := mustMarshalMap(t, payload)
		want := map[string]any{
			"index":             float64(2),
			"attempt":           float64(2),
			"max_attempts":      float64(2),
			"reason":            "no output for 3m0s",
			"last_tool_call":    "shell.run",
			"last_progress_seq": float64(4096),
			"worktree_path":     "/home/user/.compozy/state/worktrees/ws/parent/02-beta",
			"log_path":          "/home/user/.compozy/runs/run-2/journal.log",
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("job parked payload JSON mismatch: got %#v want %#v", got, want)
		}
	})

	t.Run("Should deserialize back into an identical payload", func(t *testing.T) {
		t.Parallel()
		raw, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("marshal payload: %v", err)
		}
		var decoded JobParkedPayload
		if err := json.Unmarshal(raw, &decoded); err != nil {
			t.Fatalf("unmarshal payload: %v", err)
		}
		if !reflect.DeepEqual(decoded, payload) {
			t.Fatalf("decoded payload = %#v, want %#v", decoded, payload)
		}
	})

	t.Run("Should omit empty optional fields", func(t *testing.T) {
		t.Parallel()
		got := mustMarshalMap(t, JobParkedPayload{JobAttemptInfo: JobAttemptInfo{Index: 1}})
		want := map[string]any{"index": float64(1)}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("empty job parked payload JSON = %#v, want %#v", got, want)
		}
	})
}

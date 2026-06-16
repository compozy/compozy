package kinds

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"
)

func TestShutdownPayloadJSONCompatibility(t *testing.T) {
	t.Parallel()

	now := time.Unix(10, 0).UTC()
	payload := ShutdownTerminatedPayload{
		ShutdownBase: ShutdownBase{
			Source:      "signal",
			RequestedAt: now,
			DeadlineAt:  now.Add(3 * time.Second),
		},
		Forced: true,
	}

	got := mustMarshalMap(t, payload)
	want := map[string]any{
		"source":       "signal",
		"requested_at": now.Format(time.RFC3339),
		"deadline_at":  now.Add(3 * time.Second).Format(time.RFC3339),
		"forced":       true,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("shutdown payload JSON mismatch: got %#v want %#v", got, want)
	}
}

func TestJobAttemptPayloadJSONCompatibility(t *testing.T) {
	t.Parallel()

	payload := JobAttemptFinishedPayload{
		JobAttemptInfo: JobAttemptInfo{
			Index:       1,
			Attempt:     2,
			MaxAttempts: 3,
		},
		Status:    "failure",
		ExitCode:  17,
		Retryable: true,
		Error:     "transient",
	}

	got := mustMarshalMap(t, payload)
	want := map[string]any{
		"index":        float64(1),
		"attempt":      float64(2),
		"max_attempts": float64(3),
		"status":       "failure",
		"exit_code":    float64(17),
		"retryable":    true,
		"error":        "transient",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("job attempt payload JSON mismatch: got %#v want %#v", got, want)
	}
}

func TestExtensionReadyPayloadJSONCompatibility(t *testing.T) {
	t.Parallel()

	payload := ExtensionReadyPayload{
		Extension:            "mock-ext",
		Source:               "workspace",
		Version:              "1.0.0",
		ProtocolVersion:      "1",
		AcceptedCapabilities: []string{"events.read", "tasks.read"},
		SupportedHookEvents:  []string{"prompt.post_build"},
	}

	got := mustMarshalMap(t, payload)
	want := map[string]any{
		"extension":             "mock-ext",
		"source":                "workspace",
		"version":               "1.0.0",
		"protocol_version":      "1",
		"accepted_capabilities": []any{"events.read", "tasks.read"},
		"supported_hook_events": []any{"prompt.post_build"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("extension ready payload JSON mismatch: got %#v want %#v", got, want)
	}
}

func TestReviewWatchPayloadJSONCompatibility(t *testing.T) {
	t.Parallel()

	t.Run("Should serialize ReviewWatchPayload to a stable JSON map", func(t *testing.T) {
		t.Parallel()

		payload := ReviewWatchPayload{
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
			Resolved:        2,
			Unresolved:      1,
			Dirty:           true,
			UnpushedCommits: 4,
			Error:           "push failed",
		}

		got := mustMarshalMap(t, payload)
		want := map[string]any{
			"provider":         "coderabbit",
			"pr":               "123",
			"workflow":         "engine-kernel",
			"round":            float64(2),
			"run_id":           "watch-run",
			"child_run_id":     "fix-run",
			"head_sha":         "abc123",
			"review_id":        "review-1",
			"review_state":     "current_reviewed",
			"status":           "completed",
			"remote":           "origin",
			"branch":           "feature",
			"total":            float64(3),
			"resolved":         float64(2),
			"unresolved":       float64(1),
			"dirty":            true,
			"unpushed_commits": float64(4),
			"error":            "push failed",
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("review watch payload JSON mismatch: got %#v want %#v", got, want)
		}
	})
}

func TestTaskRunMultiplePayloadJSONCompatibility(t *testing.T) {
	payload := TaskRunMultiplePayload{
		RunID:      "multi-run-1",
		Mode:       "enqueued",
		Slug:       "alpha",
		Slugs:      []string{"alpha", "beta"},
		Index:      1,
		Total:      2,
		Status:     "completed",
		ChildRunID: "task-run-alpha",
		Error:      "boom",
	}

	t.Run("Should produce compatible JSON", func(t *testing.T) {
		t.Parallel()

		got := mustMarshalMap(t, payload)
		want := map[string]any{
			"run_id":       "multi-run-1",
			"mode":         "enqueued",
			"slug":         "alpha",
			"slugs":        []any{"alpha", "beta"},
			"index":        float64(1),
			"total":        float64(2),
			"status":       "completed",
			"child_run_id": "task-run-alpha",
			"error":        "boom",
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("task run multiple payload JSON mismatch: got %#v want %#v", got, want)
		}
	})

	t.Run("Should include worktree metadata and parallel limit when set", func(t *testing.T) {
		t.Parallel()

		withWorktree := TaskRunMultiplePayload{
			RunID:          "multi-run-1",
			Mode:           "parallel",
			Slug:           "alpha",
			Slugs:          []string{"alpha", "beta"},
			Index:          1,
			Total:          2,
			ParallelLimit:  3,
			Status:         "running",
			ChildRunID:     "task-run-alpha",
			Error:          "boom",
			WorktreePath:   "/home/user/.compozy/state/worktrees/ws/parent/01-alpha",
			BaseBranch:     "main",
			BaseCommit:     "abc123def456",
			WorktreeStatus: "preserved",
		}

		got := mustMarshalMap(t, withWorktree)
		want := map[string]any{
			"run_id":          "multi-run-1",
			"mode":            "parallel",
			"slug":            "alpha",
			"slugs":           []any{"alpha", "beta"},
			"index":           float64(1),
			"total":           float64(2),
			"parallel_limit":  float64(3),
			"status":          "running",
			"child_run_id":    "task-run-alpha",
			"error":           "boom",
			"worktree_path":   "/home/user/.compozy/state/worktrees/ws/parent/01-alpha",
			"base_branch":     "main",
			"base_commit":     "abc123def456",
			"worktree_status": "preserved",
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("task run multiple worktree payload JSON mismatch: got %#v want %#v", got, want)
		}
	})

	t.Run("Should decode old payload JSON without worktree metadata", func(t *testing.T) {
		t.Parallel()

		const legacyJSON = `{"run_id":"multi-run-1","mode":"enqueued","slug":"alpha",` +
			`"slugs":["alpha","beta"],"index":1,"total":2,"status":"completed",` +
			`"child_run_id":"task-run-alpha","error":"boom"}`

		var decoded TaskRunMultiplePayload
		if err := json.Unmarshal([]byte(legacyJSON), &decoded); err != nil {
			t.Fatalf("unmarshal legacy payload: %v", err)
		}
		if decoded.Slug != "alpha" || decoded.Status != "completed" || decoded.ChildRunID != "task-run-alpha" {
			t.Fatalf("legacy payload core fields = %#v, want slug/status/child_run_id preserved", decoded)
		}
		if decoded.ParallelLimit != 0 ||
			decoded.WorktreePath != "" ||
			decoded.BaseBranch != "" ||
			decoded.BaseCommit != "" ||
			decoded.WorktreeStatus != "" {
			t.Fatalf("legacy payload worktree fields = %#v, want zero values", decoded)
		}
	})

	t.Run("Should decode new payload JSON with worktree metadata", func(t *testing.T) {
		t.Parallel()

		const newJSON = `{"run_id":"multi-run-1","mode":"parallel","slug":"alpha","index":0,"total":2,` +
			`"parallel_limit":2,"status":"running","child_run_id":"task-run-alpha",` +
			`"worktree_path":"/wt/01-alpha","base_branch":"main","base_commit":"abc123","worktree_status":"preserved"}`

		var decoded TaskRunMultiplePayload
		if err := json.Unmarshal([]byte(newJSON), &decoded); err != nil {
			t.Fatalf("unmarshal new payload: %v", err)
		}
		want := TaskRunMultiplePayload{
			RunID:          "multi-run-1",
			Mode:           "parallel",
			Slug:           "alpha",
			Total:          2,
			ParallelLimit:  2,
			Status:         "running",
			ChildRunID:     "task-run-alpha",
			WorktreePath:   "/wt/01-alpha",
			BaseBranch:     "main",
			BaseCommit:     "abc123",
			WorktreeStatus: "preserved",
		}
		if !reflect.DeepEqual(decoded, want) {
			t.Fatalf("decoded new payload = %#v, want %#v", decoded, want)
		}
	})
}

func mustMarshalMap(t *testing.T, payload any) map[string]any {
	t.Helper()

	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	return decoded
}

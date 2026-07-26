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

func TestSpeedLifecyclePayloadJSONCompatibility(t *testing.T) {
	t.Parallel()

	applied := SpeedResolution{
		Requested: SpeedFast,
		Status:    SpeedResolutionStatusApplied,
	}
	unsupported := SpeedResolution{
		Requested: SpeedNormal,
		Status:    SpeedResolutionStatusUnsupported,
		Reason:    SpeedResolutionReasonCapabilityAbsent,
	}
	rejected := SpeedResolution{
		Requested: SpeedFast,
		Status:    SpeedResolutionStatusRejected,
		Reason:    SpeedResolutionReasonProviderRejected,
	}
	tests := []struct {
		name    string
		payload any
		want    string
	}{
		{
			name:    "run queued intent",
			payload: RunQueuedPayload{Mode: "prd", Speed: SpeedFast},
			want:    `{"mode":"prd","speed":"fast"}`,
		},
		{
			name:    "run started intent",
			payload: RunStartedPayload{Mode: "prd", Speed: SpeedFast},
			want:    `{"mode":"prd","speed":"fast"}`,
		},
		{
			name:    "job queued intent",
			payload: JobQueuedPayload{Index: 1, Speed: SpeedFast},
			want:    `{"index":1,"speed":"fast"}`,
		},
		{
			name: "job started intent",
			payload: JobStartedPayload{
				JobAttemptInfo: JobAttemptInfo{Index: 1},
				Speed:          SpeedFast,
			},
			want: `{"index":1,"speed":"fast"}`,
		},
		{
			name: "session started applied resolution",
			payload: SessionStartedPayload{
				Index:           1,
				ACPSessionID:    "acp-1",
				SpeedResolution: &applied,
			},
			want: `{"index":1,"acp_session_id":"acp-1","speed_resolution":{"requested":"fast","status":"applied"}}`,
		},
		{
			name: "session started unsupported resolution",
			payload: SessionStartedPayload{
				Index:           1,
				ACPSessionID:    "acp-1",
				SpeedResolution: &unsupported,
			},
			want: `{"index":1,"acp_session_id":"acp-1","speed_resolution":{"requested":"normal","status":"unsupported","reason":"capability_absent"}}`,
		},
		{
			name: "job failed rejected resolution",
			payload: JobFailedPayload{
				JobAttemptInfo:  JobAttemptInfo{Index: 1},
				Error:           "speed rejected",
				SpeedResolution: &rejected,
			},
			want: `{"index":1,"error":"speed rejected","speed_resolution":{"requested":"fast","status":"rejected","reason":"provider_rejected"}}`,
		},
		{
			name: "absent session resolution remains omitted",
			payload: SessionStartedPayload{
				Index:        1,
				ACPSessionID: "acp-1",
			},
			want: `{"index":1,"acp_session_id":"acp-1"}`,
		},
		{
			name:    "absent failed resolution remains omitted",
			payload: JobFailedPayload{JobAttemptInfo: JobAttemptInfo{Index: 1}},
			want:    `{"index":1}`,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			data, err := json.Marshal(test.payload)
			if err != nil {
				t.Fatalf("json.Marshal() error = %v", err)
			}
			if got := string(data); got != test.want {
				t.Fatalf("payload JSON = %s, want %s", got, test.want)
			}
		})
	}
}

func TestHistoricalSpeedLifecyclePayloadJSONCompatibility(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		data   string
		decode func(t *testing.T, data []byte)
	}{
		{
			name: "run queued",
			data: `{"mode":"prd"}`,
			decode: func(t *testing.T, data []byte) {
				t.Helper()
				var payload RunQueuedPayload
				if err := json.Unmarshal(data, &payload); err != nil {
					t.Fatalf("json.Unmarshal() error = %v", err)
				}
				if payload.Speed != "" {
					t.Fatalf("historical run queued speed = %q, want empty", payload.Speed)
				}
			},
		},
		{
			name: "run started",
			data: `{"mode":"prd"}`,
			decode: func(t *testing.T, data []byte) {
				t.Helper()
				var payload RunStartedPayload
				if err := json.Unmarshal(data, &payload); err != nil {
					t.Fatalf("json.Unmarshal() error = %v", err)
				}
				if payload.Speed != "" {
					t.Fatalf("historical run started speed = %q, want empty", payload.Speed)
				}
			},
		},
		{
			name: "job queued",
			data: `{"index":1}`,
			decode: func(t *testing.T, data []byte) {
				t.Helper()
				var payload JobQueuedPayload
				if err := json.Unmarshal(data, &payload); err != nil {
					t.Fatalf("json.Unmarshal() error = %v", err)
				}
				if payload.Speed != "" {
					t.Fatalf("historical job queued speed = %q, want empty", payload.Speed)
				}
			},
		},
		{
			name: "job started",
			data: `{"index":1}`,
			decode: func(t *testing.T, data []byte) {
				t.Helper()
				var payload JobStartedPayload
				if err := json.Unmarshal(data, &payload); err != nil {
					t.Fatalf("json.Unmarshal() error = %v", err)
				}
				if payload.Speed != "" {
					t.Fatalf("historical job started speed = %q, want empty", payload.Speed)
				}
			},
		},
		{
			name: "session started",
			data: `{"index":1,"acp_session_id":"acp-1"}`,
			decode: func(t *testing.T, data []byte) {
				t.Helper()
				var payload SessionStartedPayload
				if err := json.Unmarshal(data, &payload); err != nil {
					t.Fatalf("json.Unmarshal() error = %v", err)
				}
				if payload.SpeedResolution != nil {
					t.Fatalf("historical session resolution = %#v, want nil", payload.SpeedResolution)
				}
			},
		},
		{
			name: "job failed",
			data: `{"index":1,"error":"failed"}`,
			decode: func(t *testing.T, data []byte) {
				t.Helper()
				var payload JobFailedPayload
				if err := json.Unmarshal(data, &payload); err != nil {
					t.Fatalf("json.Unmarshal() error = %v", err)
				}
				if payload.SpeedResolution != nil {
					t.Fatalf("historical job resolution = %#v, want nil", payload.SpeedResolution)
				}
			},
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			test.decode(t, []byte(test.data))
		})
	}
}

func TestRunRecoveryPayloadJSONCompatibility(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		payload any
		want    string
		decode  func(t *testing.T, data []byte)
	}{
		{
			name: "started",
			payload: RunRecoveryStartedPayload{
				Attempt:       1,
				Strategy:      "agentic",
				RecoveryRunID: "recovery-run",
			},
			want: `{"attempt":1,"strategy":"agentic","recovery_run_id":"recovery-run"}`,
			decode: func(t *testing.T, data []byte) {
				t.Helper()
				var got RunRecoveryStartedPayload
				if err := json.Unmarshal(data, &got); err != nil {
					t.Fatalf("unmarshal started payload: %v", err)
				}
				if got.Attempt != 1 || got.Strategy != "agentic" || got.RecoveryRunID != "recovery-run" {
					t.Fatalf("unexpected started payload: %#v", got)
				}
			},
		},
		{
			name: "restarting",
			payload: RunRecoveryRestartingPayload{
				FailedJobIDs: []string{"task_02", "task_03"},
			},
			want: `{"failed_job_ids":["task_02","task_03"]}`,
			decode: func(t *testing.T, data []byte) {
				t.Helper()
				var got RunRecoveryRestartingPayload
				if err := json.Unmarshal(data, &got); err != nil {
					t.Fatalf("unmarshal restarting payload: %v", err)
				}
				if !reflect.DeepEqual(got.FailedJobIDs, []string{"task_02", "task_03"}) {
					t.Fatalf("unexpected restarting payload: %#v", got)
				}
			},
		},
		{
			name:    "recovered",
			payload: RunRecoveredPayload{Attempts: 1},
			want:    `{"attempts":1}`,
			decode: func(t *testing.T, data []byte) {
				t.Helper()
				var got RunRecoveredPayload
				if err := json.Unmarshal(data, &got); err != nil {
					t.Fatalf("unmarshal recovered payload: %v", err)
				}
				if got.Attempts != 1 {
					t.Fatalf("unexpected recovered payload: %#v", got)
				}
			},
		},
		{
			name: "exhausted",
			payload: RunRecoveryExhaustedPayload{
				Error:      "rejected",
				ResultPath: "/repo/.compozy/runs/run-1/result.json",
			},
			want: `{"error":"rejected","result_path":"/repo/.compozy/runs/run-1/result.json"}`,
			decode: func(t *testing.T, data []byte) {
				t.Helper()
				var got RunRecoveryExhaustedPayload
				if err := json.Unmarshal(data, &got); err != nil {
					t.Fatalf("unmarshal exhausted payload: %v", err)
				}
				if got.Error != "rejected" || got.ResultPath != "/repo/.compozy/runs/run-1/result.json" {
					t.Fatalf("unexpected exhausted payload: %#v", got)
				}
			},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			payload, err := json.Marshal(tc.payload)
			if err != nil {
				t.Fatalf("marshal payload: %v", err)
			}
			if string(payload) != tc.want {
				t.Fatalf("payload JSON mismatch: got %s want %s", string(payload), tc.want)
			}
			tc.decode(t, payload)
		})
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

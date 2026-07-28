package model

import (
	"strings"
	"testing"
	"time"
)

func TestValidateTriggerFilter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		filter  map[string]string
		wantErr string
	}{
		{
			name: "Should accept built in envelope fields",
			filter: map[string]string{
				"kind":         "session.stopped",
				"scope":        "workspace",
				"workspace_id": "ws_alpha",
				"source":       "observer",
			},
		},
		{
			name: "Should accept nested data paths with trimmed segments",
			filter: map[string]string{
				" data.metadata. step ": "complete",
			},
		},
		{
			name: "Should reject empty data path segment",
			filter: map[string]string{
				"data.metadata..step": "complete",
			},
			wantErr: "data.metadata..step",
		},
		{
			name: "Should reject trailing empty data path segment",
			filter: map[string]string{
				"data.metadata. ": "complete",
			},
			wantErr: "data.metadata.",
		},
		{
			name: "Should reject unsupported top level path",
			filter: map[string]string{
				"payload.agent": "researcher",
			},
			wantErr: "payload.agent",
		},
		{
			name: "Should reject empty filter value",
			filter: map[string]string{
				"kind": " ",
			},
			wantErr: "filter[\"kind\"]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateTriggerFilter(tt.filter, "filter")
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateTriggerFilter() error = %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("ValidateTriggerFilter() error = nil, want non-nil")
			}
			if got := err.Error(); !strings.Contains(got, tt.wantErr) {
				t.Fatalf("ValidateTriggerFilter() error = %q, want substring %q", got, tt.wantErr)
			}
		})
	}
}

func TestSchedulerStateValidate(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)
	valid := SchedulerState{
		JobID:         "job-daily",
		CatchUpPolicy: SchedulerCatchUpPolicySkipMissed,
		UpdatedAt:     now,
	}
	tests := []struct {
		name    string
		state   SchedulerState
		wantErr string
	}{
		{
			name:  "Should accept valid scheduler state",
			state: valid,
		},
		{
			name: "Should accept coalesce catch up policy",
			state: SchedulerState{
				JobID:         "job-coalesce",
				CatchUpPolicy: SchedulerCatchUpPolicyCoalesce,
				UpdatedAt:     now,
			},
		},
		{
			name: "Should accept replay catch up policy",
			state: SchedulerState{
				JobID:         "job-replay",
				CatchUpPolicy: SchedulerCatchUpPolicyReplay,
				UpdatedAt:     now,
			},
		},
		{
			name: "Should accept run once catch up policy",
			state: SchedulerState{
				JobID:         "job-run-once",
				CatchUpPolicy: SchedulerCatchUpPolicyRunOnce,
				UpdatedAt:     now,
			},
		},
		{
			name: "Should reject missing job id",
			state: SchedulerState{
				CatchUpPolicy: SchedulerCatchUpPolicySkipMissed,
				UpdatedAt:     now,
			},
			wantErr: "job_id",
		},
		{
			name: "Should reject unsupported catch up policy",
			state: SchedulerState{
				JobID:         "job-daily",
				CatchUpPolicy: SchedulerCatchUpPolicy("replay_all"),
				UpdatedAt:     now,
			},
			wantErr: "catch_up_policy",
		},
		{
			name: "Should reject negative misfire grace",
			state: SchedulerState{
				JobID:               "job-daily",
				CatchUpPolicy:       SchedulerCatchUpPolicySkipMissed,
				MisfireGraceSeconds: -1,
				UpdatedAt:           now,
			},
			wantErr: "misfire_grace_seconds",
		},
		{
			name: "Should reject negative resume failures",
			state: SchedulerState{
				JobID:                     "job-daily",
				CatchUpPolicy:             SchedulerCatchUpPolicySkipMissed,
				ConsecutiveResumeFailures: -1,
				UpdatedAt:                 now,
			},
			wantErr: "consecutive_resume_failures",
		},
		{
			name: "Should reject negative misfire count",
			state: SchedulerState{
				JobID:         "job-daily",
				CatchUpPolicy: SchedulerCatchUpPolicySkipMissed,
				MisfireCount:  -1,
				UpdatedAt:     now,
			},
			wantErr: "misfire_count",
		},
		{
			name: "Should reject missing updated time",
			state: SchedulerState{
				JobID:         "job-daily",
				CatchUpPolicy: SchedulerCatchUpPolicySkipMissed,
			},
			wantErr: "updated_at",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.state.Validate("scheduler_state")
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("SchedulerState.Validate() error = %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("SchedulerState.Validate() error = nil, want non-nil")
			}
			if got := err.Error(); !strings.Contains(got, tt.wantErr) {
				t.Fatalf("SchedulerState.Validate() error = %q, want substring %q", got, tt.wantErr)
			}
		})
	}
}

func TestSchedulerClaimValidate(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)
	valid := SchedulerClaim{
		JobID:       "job-daily",
		RunID:       "run-1",
		FireID:      "fire-1",
		ScheduledAt: now,
		ClaimedAt:   now,
	}
	tests := []struct {
		name    string
		claim   SchedulerClaim
		wantErr string
	}{
		{
			name:  "Should accept valid scheduler claim",
			claim: valid,
		},
		{
			name: "Should reject missing job id",
			claim: SchedulerClaim{
				RunID:       "run-1",
				FireID:      "fire-1",
				ScheduledAt: now,
				ClaimedAt:   now,
			},
			wantErr: "job_id",
		},
		{
			name: "Should reject missing run id",
			claim: SchedulerClaim{
				JobID:       "job-daily",
				FireID:      "fire-1",
				ScheduledAt: now,
				ClaimedAt:   now,
			},
			wantErr: "run_id",
		},
		{
			name: "Should reject missing fire id",
			claim: SchedulerClaim{
				JobID:       "job-daily",
				RunID:       "run-1",
				ScheduledAt: now,
				ClaimedAt:   now,
			},
			wantErr: "fire_id",
		},
		{
			name: "Should reject missing scheduled time",
			claim: SchedulerClaim{
				JobID:     "job-daily",
				RunID:     "run-1",
				FireID:    "fire-1",
				ClaimedAt: now,
			},
			wantErr: "scheduled_at",
		},
		{
			name: "Should reject missing claimed time",
			claim: SchedulerClaim{
				JobID:       "job-daily",
				RunID:       "run-1",
				FireID:      "fire-1",
				ScheduledAt: now,
			},
			wantErr: "claimed_at",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.claim.Validate("scheduler_claim")
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("SchedulerClaim.Validate() error = %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("SchedulerClaim.Validate() error = nil, want non-nil")
			}
			if got := err.Error(); !strings.Contains(got, tt.wantErr) {
				t.Fatalf("SchedulerClaim.Validate() error = %q, want substring %q", got, tt.wantErr)
			}
		})
	}
}

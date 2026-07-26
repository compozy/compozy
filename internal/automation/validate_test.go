package automation

import (
	"strings"
	"testing"
	"time"

	"github.com/compozy/agh/internal/network/participation"
)

func TestScheduleSpecValidate(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name    string
		spec    ScheduleSpec
		wantErr string
	}{
		{
			name: "cron valid",
			spec: ScheduleSpec{
				Mode: ScheduleModeCron,
				Expr: "0 9 * * *",
			},
		},
		{
			name: "every valid",
			spec: ScheduleSpec{
				Mode:                ScheduleModeEvery,
				Interval:            "30m",
				CatchUpPolicy:       SchedulerCatchUpPolicyRunOnce,
				MisfireGraceSeconds: 300,
			},
		},
		{
			name: "at valid",
			spec: ScheduleSpec{
				Mode: ScheduleModeAt,
				Time: "2026-04-15T15:00:00Z",
			},
		},
		{
			name: "mode invalid",
			spec: ScheduleSpec{
				Mode: ScheduleMode("sometimes"),
			},
			wantErr: "schedule.mode",
		},
		{
			name: "cron malformed",
			spec: ScheduleSpec{
				Mode: ScheduleModeCron,
				Expr: "* * *",
			},
			wantErr: "schedule.expr",
		},
		{
			name: "cron conflicting interval",
			spec: ScheduleSpec{
				Mode:     ScheduleModeCron,
				Expr:     "0 9 * * *",
				Interval: "5m",
			},
			wantErr: "schedule.interval",
		},
		{
			name: "every conflicting expr",
			spec: ScheduleSpec{
				Mode:     ScheduleModeEvery,
				Interval: "30m",
				Expr:     "0 9 * * *",
			},
			wantErr: "schedule.expr",
		},
		{
			name: "at conflicting interval",
			spec: ScheduleSpec{
				Mode:     ScheduleModeAt,
				Time:     "2026-04-15T15:00:00Z",
				Interval: "5m",
			},
			wantErr: "schedule.interval",
		},
		{
			name: "every malformed interval",
			spec: ScheduleSpec{
				Mode:     ScheduleModeEvery,
				Interval: "later",
			},
			wantErr: "schedule.interval",
		},
		{
			name: "at malformed time",
			spec: ScheduleSpec{
				Mode: ScheduleModeAt,
				Time: "tomorrow",
			},
			wantErr: "schedule.time",
		},
		{
			name: "Should reject catch up policy for one-shot schedule",
			spec: ScheduleSpec{
				Mode:          ScheduleModeAt,
				Time:          "2026-04-15T15:00:00Z",
				CatchUpPolicy: SchedulerCatchUpPolicyRunOnce,
			},
			wantErr: "schedule.catch_up_policy",
		},
		{
			name: "Should reject misfire grace for one-shot schedule",
			spec: ScheduleSpec{
				Mode:                ScheduleModeAt,
				Time:                "2026-04-15T15:00:00Z",
				MisfireGraceSeconds: 60,
			},
			wantErr: "schedule.misfire_grace_seconds",
		},
		{
			name: "catch up policy invalid",
			spec: ScheduleSpec{
				Mode:          ScheduleModeEvery,
				Interval:      "30m",
				CatchUpPolicy: SchedulerCatchUpPolicy("burst"),
			},
			wantErr: "schedule.catch_up_policy",
		},
		{
			name: "misfire grace negative",
			spec: ScheduleSpec{
				Mode:                ScheduleModeEvery,
				Interval:            "30m",
				MisfireGraceSeconds: -1,
			},
			wantErr: "schedule.misfire_grace_seconds",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := tc.spec.Validate("schedule")
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("ScheduleSpec.Validate() error = %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("ScheduleSpec.Validate() error = nil, want non-nil")
			}
			if got := err.Error(); !strings.Contains(got, tc.wantErr) {
				t.Fatalf("ScheduleSpec.Validate() error = %q, want substring %q", got, tc.wantErr)
			}
		})
	}
}

func TestRetryConfigValidate(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name    string
		cfg     RetryConfig
		wantErr string
	}{
		{
			name: "none valid",
			cfg:  DefaultRetryConfig(),
		},
		{
			name: "backoff valid",
			cfg: RetryConfig{
				Strategy:   RetryStrategyBackoff,
				MaxRetries: 3,
				BaseDelay:  "2s",
			},
		},
		{
			name: "none with max retries invalid",
			cfg: RetryConfig{
				Strategy:   RetryStrategyNone,
				MaxRetries: 1,
			},
			wantErr: "max_retries",
		},
		{
			name: "backoff missing delay invalid",
			cfg: RetryConfig{
				Strategy:   RetryStrategyBackoff,
				MaxRetries: 3,
			},
			wantErr: "base_delay",
		},
		{
			name: "backoff zero retries invalid",
			cfg: RetryConfig{
				Strategy:  RetryStrategyBackoff,
				BaseDelay: "2s",
			},
			wantErr: "max_retries",
		},
		{
			name: "none with base delay invalid",
			cfg: RetryConfig{
				Strategy:  RetryStrategyNone,
				BaseDelay: "1s",
			},
			wantErr: "base_delay",
		},
		{
			name: "backoff malformed delay invalid",
			cfg: RetryConfig{
				Strategy:   RetryStrategyBackoff,
				MaxRetries: 2,
				BaseDelay:  "soon",
			},
			wantErr: "base_delay",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := tc.cfg.Validate("retry")
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("RetryConfig.Validate() error = %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("RetryConfig.Validate() error = nil, want non-nil")
			}
			if got := err.Error(); !strings.Contains(got, tc.wantErr) {
				t.Fatalf("RetryConfig.Validate() error = %q, want substring %q", got, tc.wantErr)
			}
		})
	}
}

func TestFireLimitConfigValidate(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name    string
		cfg     FireLimitConfig
		wantErr string
	}{
		{
			name: "valid",
			cfg: FireLimitConfig{
				Max:    12,
				Window: "1h",
			},
		},
		{
			name: "max invalid",
			cfg: FireLimitConfig{
				Max:    0,
				Window: "1h",
			},
			wantErr: "max",
		},
		{
			name: "window invalid",
			cfg: FireLimitConfig{
				Max:    12,
				Window: "later",
			},
			wantErr: "window",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := tc.cfg.Validate("fire_limit")
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("FireLimitConfig.Validate() error = %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("FireLimitConfig.Validate() error = nil, want non-nil")
			}
			if got := err.Error(); !strings.Contains(got, tc.wantErr) {
				t.Fatalf("FireLimitConfig.Validate() error = %q, want substring %q", got, tc.wantErr)
			}
		})
	}
}

func TestValidateScopeBinding(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name      string
		scope     Scope
		workspace string
		wantErr   string
	}{
		{
			name:  "global valid",
			scope: AutomationScopeGlobal,
		},
		{
			name:      "global with workspace invalid",
			scope:     AutomationScopeGlobal,
			workspace: "/workspace",
			wantErr:   "workspace",
		},
		{
			name:    "workspace missing binding invalid",
			scope:   AutomationScopeWorkspace,
			wantErr: "workspace",
		},
		{
			name:      "workspace valid",
			scope:     AutomationScopeWorkspace,
			workspace: "ws_alpha",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateScopeBinding(tc.scope, tc.workspace, "trigger", "workspace")
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateScopeBinding() error = %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("ValidateScopeBinding() error = nil, want non-nil")
			}
			if got := err.Error(); !strings.Contains(got, tc.wantErr) {
				t.Fatalf("ValidateScopeBinding() error = %q, want substring %q", got, tc.wantErr)
			}
		})
	}
}

func TestDefaultsAndEnumValidation(t *testing.T) {
	t.Parallel()

	backoff := DefaultBackoffRetryConfig()
	if backoff.Strategy != RetryStrategyBackoff || backoff.MaxRetries != 3 || backoff.BaseDelay != "2s" {
		t.Fatalf("DefaultBackoffRetryConfig() = %#v", backoff)
	}

	testCases := []struct {
		name    string
		err     error
		wantErr string
	}{
		{
			name:    "scope invalid",
			err:     Scope("team").Validate("scope"),
			wantErr: "scope",
		},
		{
			name:    "job source invalid",
			err:     JobSource("manual").Validate("source"),
			wantErr: "source",
		},
		{
			name:    "run status invalid",
			err:     RunStatus("stuck").Validate("status"),
			wantErr: "status",
		},
		{
			name:    "activation source invalid",
			err:     ActivationSource("socket").Validate("source"),
			wantErr: "source",
		},
		{
			name:    "retry strategy invalid",
			err:     RetryStrategy("linear").Validate("retry.strategy"),
			wantErr: "retry.strategy",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if tc.err == nil {
				t.Fatal("Validate() error = nil, want non-nil")
			}
			if got := tc.err.Error(); !strings.Contains(got, tc.wantErr) {
				t.Fatalf("Validate() error = %q, want substring %q", got, tc.wantErr)
			}
		})
	}
}

func TestValidateTriggerFilter(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name    string
		filter  map[string]string
		wantErr string
	}{
		{
			name: "valid built in fields",
			filter: map[string]string{
				"kind":       "session.stopped",
				"data.agent": "researcher",
			},
		},
		{
			name: "invalid path",
			filter: map[string]string{
				"payload.agent": "researcher",
			},
			wantErr: "payload.agent",
		},
		{
			name: "empty value",
			filter: map[string]string{
				"kind": " ",
			},
			wantErr: "kind",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateTriggerFilter(tc.filter, "trigger.filter")
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateTriggerFilter() error = %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("ValidateTriggerFilter() error = nil, want non-nil")
			}
			if got := err.Error(); !strings.Contains(got, tc.wantErr) {
				t.Fatalf("ValidateTriggerFilter() error = %q, want substring %q", got, tc.wantErr)
			}
		})
	}
}

func TestJobValidate(t *testing.T) {
	t.Parallel()

	job := Job{
		Scope:       AutomationScopeWorkspace,
		Name:        "daily-report",
		AgentName:   "researcher",
		WorkspaceID: "ws_alpha",
		Prompt:      "Generate daily report",
		Schedule: &ScheduleSpec{
			Mode:     ScheduleModeEvery,
			Interval: "15m",
		},
		Retry:     DefaultRetryConfig(),
		FireLimit: DefaultFireLimitConfig(),
		Source:    JobSourceConfig,
	}
	if err := job.Validate("job"); err != nil {
		t.Fatalf("Job.Validate() error = %v", err)
	}

	job.Source = JobSource("manual")
	if err := job.Validate("job"); err == nil {
		t.Fatal("Job.Validate() error = nil, want non-nil")
	}

	job.Source = JobSourceConfig
	job.Schedule = nil
	if err := job.Validate("job"); err == nil {
		t.Fatal("Job.Validate() error = nil, want non-nil")
	}

	job.Schedule = &ScheduleSpec{Mode: ScheduleModeEvery, Interval: "15m"}
	job.Scope = AutomationScopeGlobal
	if err := job.Validate("job"); err == nil {
		t.Fatal("Job.Validate() error = nil, want non-nil")
	}
}

func TestJobValidateRejectsMissingRequiredFields(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name    string
		job     Job
		wantErr string
	}{
		{
			name:    "missing name",
			wantErr: "job.name is required",
			job: Job{
				Scope:     AutomationScopeGlobal,
				AgentName: "researcher",
				Prompt:    "Generate daily report",
				Schedule:  &ScheduleSpec{Mode: ScheduleModeEvery, Interval: "15m"},
				Retry:     DefaultRetryConfig(),
				FireLimit: DefaultFireLimitConfig(),
				Source:    JobSourceConfig,
			},
		},
		{
			name:    "missing agent",
			wantErr: "job.agent_name is required",
			job: Job{
				Scope:     AutomationScopeGlobal,
				Name:      "daily-report",
				Prompt:    "Generate daily report",
				Schedule:  &ScheduleSpec{Mode: ScheduleModeEvery, Interval: "15m"},
				Retry:     DefaultRetryConfig(),
				FireLimit: DefaultFireLimitConfig(),
				Source:    JobSourceConfig,
			},
		},
		{
			name:    "missing prompt",
			wantErr: "job.prompt is required",
			job: Job{
				Scope:     AutomationScopeGlobal,
				Name:      "daily-report",
				AgentName: "researcher",
				Schedule:  &ScheduleSpec{Mode: ScheduleModeEvery, Interval: "15m"},
				Retry:     DefaultRetryConfig(),
				FireLimit: DefaultFireLimitConfig(),
				Source:    JobSourceConfig,
			},
		},
		{
			name:    "global direct task with live participation",
			wantErr: "global direct task has no workspace binding",
			job: Job{
				Scope:    AutomationScopeGlobal,
				Name:     "live-task",
				Schedule: &ScheduleSpec{Mode: ScheduleModeEvery, Interval: "15m"},
				Task: &JobTaskConfig{
					Title:                "Live task",
					NetworkParticipation: testNamedParticipation("builders"),
				},
				Retry:     DefaultRetryConfig(),
				FireLimit: DefaultFireLimitConfig(),
				Source:    JobSourceConfig,
			},
		},
		{
			name:    "agent target with participation-only loop data",
			wantErr: `job.loop_target must be empty when target_kind is "agent"`,
			job: Job{
				Scope:      AutomationScopeGlobal,
				Name:       "daily-report",
				TargetKind: TargetKindAgent,
				AgentName:  "researcher",
				Prompt:     "Generate daily report",
				Schedule:   &ScheduleSpec{Mode: ScheduleModeEvery, Interval: "15m"},
				LoopTarget: &LoopTarget{NetworkParticipation: &participation.Request{}},
				Retry:      DefaultRetryConfig(),
				FireLimit:  DefaultFireLimitConfig(),
				Source:     JobSourceConfig,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := tc.job.Validate("job")
			if err == nil {
				t.Fatal("Job.Validate() error = nil, want non-nil")
			}
			if got := err.Error(); !strings.Contains(got, tc.wantErr) {
				t.Fatalf("Job.Validate() error = %q, want substring %q", got, tc.wantErr)
			}
		})
	}
}

func TestTriggerValidate(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name    string
		trigger Trigger
		wantErr string
	}{
		{
			name: "webhook valid",
			trigger: Trigger{
				Scope:            AutomationScopeGlobal,
				Name:             "deploy",
				AgentName:        "reviewer",
				Prompt:           `{{ index .Data "payload" }}`,
				Event:            "webhook",
				Retry:            DefaultRetryConfig(),
				FireLimit:        DefaultFireLimitConfig(),
				Source:           JobSourceConfig,
				EndpointSlug:     "deploy-review",
				WebhookSecretRef: "env:AGH_TEST_WEBHOOK_SECRET",
			},
		},
		{
			name: "non webhook with endpoint invalid",
			trigger: Trigger{
				Scope:        AutomationScopeGlobal,
				Name:         "deploy",
				AgentName:    "reviewer",
				Prompt:       `{{ .Kind }}`,
				Event:        "session.stopped",
				Retry:        DefaultRetryConfig(),
				FireLimit:    DefaultFireLimitConfig(),
				Source:       JobSourceConfig,
				EndpointSlug: "deploy-review",
			},
			wantErr: "endpoint_slug",
		},
		{
			name: "webhook missing fields invalid",
			trigger: Trigger{
				Scope:     AutomationScopeGlobal,
				Name:      "deploy",
				AgentName: "reviewer",
				Prompt:    `{{ .Kind }}`,
				Event:     "webhook",
				Retry:     DefaultRetryConfig(),
				FireLimit: DefaultFireLimitConfig(),
				Source:    JobSourceConfig,
			},
			wantErr: "endpoint_slug",
		},
		{
			name: "webhook invalid webhook id prefix",
			trigger: Trigger{
				Scope:            AutomationScopeGlobal,
				Name:             "deploy",
				AgentName:        "reviewer",
				Prompt:           `{{ .Kind }}`,
				Event:            "webhook",
				Retry:            DefaultRetryConfig(),
				FireLimit:        DefaultFireLimitConfig(),
				Source:           JobSourceConfig,
				EndpointSlug:     "deploy-review",
				WebhookID:        "qa-webhook-id",
				WebhookSecretRef: "env:AGH_TEST_WEBHOOK_SECRET",
			},
			wantErr: "webhook_id",
		},
		{
			name: "invalid prompt invalid",
			trigger: Trigger{
				Scope:     AutomationScopeGlobal,
				Name:      "deploy",
				AgentName: "reviewer",
				Prompt:    `{{ .EnvelopeID }}`,
				Event:     "session.stopped",
				Retry:     DefaultRetryConfig(),
				FireLimit: DefaultFireLimitConfig(),
				Source:    JobSourceConfig,
			},
			wantErr: "prompt",
		},
		{
			name: "variable rooted field invalid",
			trigger: Trigger{
				Scope:     AutomationScopeGlobal,
				Name:      "deploy",
				AgentName: "reviewer",
				Prompt:    `{{ $root := . }}{{ $root.Kind }}`,
				Event:     "session.stopped",
				Retry:     DefaultRetryConfig(),
				FireLimit: DefaultFireLimitConfig(),
				Source:    JobSourceConfig,
			},
			wantErr: "variable-rooted lookups are not supported",
		},
		{
			name: "variable rooted index invalid",
			trigger: Trigger{
				Scope:     AutomationScopeGlobal,
				Name:      "deploy",
				AgentName: "reviewer",
				Prompt:    `{{ $root := . }}{{ index $root "Kind" }}`,
				Event:     "session.stopped",
				Retry:     DefaultRetryConfig(),
				FireLimit: DefaultFireLimitConfig(),
				Source:    JobSourceConfig,
			},
			wantErr: "variable-rooted lookups are not supported",
		},
		{
			name: "non webhook with webhook id invalid",
			trigger: Trigger{
				Scope:     AutomationScopeGlobal,
				Name:      "deploy",
				AgentName: "reviewer",
				Prompt:    `{{ .Kind }}`,
				Event:     "session.stopped",
				Retry:     DefaultRetryConfig(),
				FireLimit: DefaultFireLimitConfig(),
				Source:    JobSourceConfig,
				WebhookID: "wh_123",
			},
			wantErr: "webhook_id",
		},
		{
			name: "agent target with participation-only loop data",
			trigger: Trigger{
				Scope:      AutomationScopeGlobal,
				Name:       "deploy",
				TargetKind: TargetKindAgent,
				AgentName:  "reviewer",
				Prompt:     `{{ .Kind }}`,
				Event:      "session.stopped",
				LoopTarget: &LoopTarget{NetworkParticipation: &participation.Request{}},
				Retry:      DefaultRetryConfig(),
				FireLimit:  DefaultFireLimitConfig(),
				Source:     JobSourceConfig,
			},
			wantErr: `trigger.loop_target must be empty when target_kind is "agent"`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := tc.trigger.Validate("trigger")
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("Trigger.Validate() error = %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("Trigger.Validate() error = nil, want non-nil")
			}
			if got := err.Error(); !strings.Contains(got, tc.wantErr) {
				t.Fatalf("Trigger.Validate() error = %q, want substring %q", got, tc.wantErr)
			}
		})
	}
}

func TestTriggerValidateRejectsMissingRequiredFieldsAndPolicies(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name    string
		trigger Trigger
		wantErr string
	}{
		{
			name:    "missing name",
			wantErr: "trigger.name is required",
			trigger: Trigger{
				Scope:     AutomationScopeGlobal,
				AgentName: "reviewer",
				Prompt:    `{{ .Kind }}`,
				Event:     "session.stopped",
				Retry:     DefaultRetryConfig(),
				FireLimit: DefaultFireLimitConfig(),
				Source:    JobSourceConfig,
			},
		},
		{
			name:    "missing agent",
			wantErr: "trigger.agent_name is required",
			trigger: Trigger{
				Scope:     AutomationScopeGlobal,
				Name:      "deploy",
				Prompt:    `{{ .Kind }}`,
				Event:     "session.stopped",
				Retry:     DefaultRetryConfig(),
				FireLimit: DefaultFireLimitConfig(),
				Source:    JobSourceConfig,
			},
		},
		{
			name:    "missing prompt",
			wantErr: "trigger.prompt is required",
			trigger: Trigger{
				Scope:     AutomationScopeGlobal,
				Name:      "deploy",
				AgentName: "reviewer",
				Event:     "session.stopped",
				Retry:     DefaultRetryConfig(),
				FireLimit: DefaultFireLimitConfig(),
				Source:    JobSourceConfig,
			},
		},
		{
			name:    "missing event",
			wantErr: "trigger.event is required",
			trigger: Trigger{
				Scope:     AutomationScopeGlobal,
				Name:      "deploy",
				AgentName: "reviewer",
				Prompt:    `{{ .Kind }}`,
				Retry:     DefaultRetryConfig(),
				FireLimit: DefaultFireLimitConfig(),
				Source:    JobSourceConfig,
			},
		},
		{
			name:    "invalid filter",
			wantErr: `unsupported filter path "payload.kind"`,
			trigger: Trigger{
				Scope:     AutomationScopeGlobal,
				Name:      "deploy",
				AgentName: "reviewer",
				Prompt:    `{{ .Kind }}`,
				Event:     "session.stopped",
				Filter: map[string]string{
					"payload.kind": "session.stopped",
				},
				Retry:     DefaultRetryConfig(),
				FireLimit: DefaultFireLimitConfig(),
				Source:    JobSourceConfig,
			},
		},
		{
			name:    "invalid retry",
			wantErr: "trigger.retry.max_retries must be positive",
			trigger: Trigger{
				Scope:     AutomationScopeGlobal,
				Name:      "deploy",
				AgentName: "reviewer",
				Prompt:    `{{ .Kind }}`,
				Event:     "session.stopped",
				Retry: RetryConfig{
					Strategy: RetryStrategyBackoff,
				},
				FireLimit: DefaultFireLimitConfig(),
				Source:    JobSourceConfig,
			},
		},
		{
			name:    "invalid fire limit",
			wantErr: "trigger.fire_limit.max must be positive",
			trigger: Trigger{
				Scope:     AutomationScopeGlobal,
				Name:      "deploy",
				AgentName: "reviewer",
				Prompt:    `{{ .Kind }}`,
				Event:     "session.stopped",
				Retry:     DefaultRetryConfig(),
				FireLimit: FireLimitConfig{},
				Source:    JobSourceConfig,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := tc.trigger.Validate("trigger")
			if err == nil {
				t.Fatal("Trigger.Validate() error = nil, want non-nil")
			}
			if got := err.Error(); !strings.Contains(got, tc.wantErr) {
				t.Fatalf("Trigger.Validate() error = %q, want substring %q", got, tc.wantErr)
			}
		})
	}
}

func TestRunAndEnvelopeValidate(t *testing.T) {
	t.Parallel()

	startedAt := time.Now().UTC()
	endedAt := startedAt.Add(-time.Minute)

	run := Run{
		Status:    RunRunning,
		Attempt:   1,
		StartedAt: &startedAt,
		EndedAt:   &endedAt,
	}
	if err := run.Validate("run"); err == nil {
		t.Fatal("Run.Validate() error = nil, want non-nil")
	} else if got := err.Error(); !strings.Contains(got, "run.ended_at must not be before run.started_at") {
		t.Fatalf("Run.Validate() error = %q, want ended_at ordering failure", got)
	}

	run.Attempt = -1
	run.EndedAt = nil
	if err := run.Validate("run"); err == nil {
		t.Fatal("Run.Validate() error = nil, want non-nil")
	} else if got := err.Error(); !strings.Contains(got, "run.attempt must be zero or positive") {
		t.Fatalf("Run.Validate() error = %q, want attempt failure", got)
	}

	validRun := Run{
		Status:  RunCompleted,
		Attempt: 1,
	}
	if err := validRun.Validate("run"); err != nil {
		t.Fatalf("Run.Validate(valid) error = %v", err)
	}

	delegatedMissingTaskID := Run{
		Status:    RunDelegated,
		Attempt:   1,
		TaskRunID: "task-run-1",
	}
	if err := delegatedMissingTaskID.Validate("run"); err == nil {
		t.Fatal("Run.Validate(delegated missing task id) error = nil, want non-nil")
	} else if got := err.Error(); !strings.Contains(got, "run.task_id is required when run.task_run_id is set") {
		t.Fatalf("Run.Validate(delegated missing task id) error = %q, want delegated task_id failure", got)
	}

	delegatedMissingTaskRunID := Run{
		Status:  RunDelegated,
		Attempt: 1,
		TaskID:  "task-1",
	}
	if err := delegatedMissingTaskRunID.Validate("run"); err == nil {
		t.Fatal("Run.Validate(delegated missing task run id) error = nil, want non-nil")
	} else if got := err.Error(); !strings.Contains(got, "run.status requires exactly one of") {
		t.Fatalf("Run.Validate(delegated missing task run id) error = %q, want delegated task_run_id failure", got)
	}

	delegatedLoopRun := Run{
		Status:    RunDelegated,
		Attempt:   1,
		LoopRunID: "looprun-1",
	}
	if err := delegatedLoopRun.Validate("run"); err != nil {
		t.Fatalf("Run.Validate(loop delegated) error = %v", err)
	}

	delegatedBothTargets := Run{
		Status:    RunDelegated,
		Attempt:   1,
		TaskID:    "task-1",
		TaskRunID: "task-run-1",
		LoopRunID: "looprun-1",
	}
	if err := delegatedBothTargets.Validate("run"); err == nil {
		t.Fatal("Run.Validate(delegated both targets) error = nil, want non-nil")
	} else if got := err.Error(); !strings.Contains(got, "run.status requires exactly one of") {
		t.Fatalf("Run.Validate(delegated both targets) error = %q, want XOR failure", got)
	}

	if err := (JobTaskConfig{
		NetworkParticipation: testNamedParticipation("bad channel"),
	}).Validate("job.task"); err == nil {
		t.Fatal("JobTaskConfig.Validate(invalid channel) error = nil, want non-nil")
	} else if got := err.Error(); !strings.Contains(got, "job.task.network_participation is invalid") ||
		!strings.Contains(got, participation.ErrStrategyInvalid.Error()) {
		t.Fatalf("JobTaskConfig.Validate(invalid channel) error = %q, want participation validation", got)
	}
	loopRunStrategy := participation.StrategyLoopRun
	liveMode := participation.ModeLive
	if err := (JobTaskConfig{
		NetworkParticipation: &participation.Request{
			Mode:            &liveMode,
			ChannelStrategy: &loopRunStrategy,
		},
	}).Validate("job.task"); err == nil {
		t.Fatal("JobTaskConfig.Validate(loop run strategy) error = nil, want non-nil")
	} else if got := err.Error(); !strings.Contains(got, participation.ErrStrategyInvalid.Error()) ||
		!strings.Contains(got, "direct Automation tasks") {
		t.Fatalf("JobTaskConfig.Validate(loop run strategy) error = %q, want direct-task strategy failure", got)
	}

	envelope := ActivationEnvelope{
		Kind:   "session.stopped",
		Scope:  AutomationScopeWorkspace,
		Source: ActivationSourceHook,
	}
	if err := envelope.Validate("envelope"); err == nil {
		t.Fatal("ActivationEnvelope.Validate() error = nil, want non-nil")
	} else if got := err.Error(); !strings.Contains(got, "envelope.workspace_id") {
		t.Fatalf("ActivationEnvelope.Validate() error = %q, want workspace binding failure", got)
	}

	validEnvelope := ActivationEnvelope{
		Kind:        "session.stopped",
		Scope:       AutomationScopeWorkspace,
		WorkspaceID: "ws_alpha",
		Source:      ActivationSourceHook,
	}
	if err := validEnvelope.Validate("envelope"); err != nil {
		t.Fatalf("ActivationEnvelope.Validate(valid) error = %v", err)
	}

	validEnvelope.Source = ActivationSource("socket")
	if err := validEnvelope.Validate("envelope"); err == nil {
		t.Fatal("ActivationEnvelope.Validate() error = nil, want non-nil")
	} else if got := err.Error(); !strings.Contains(got, "envelope.source") {
		t.Fatalf("ActivationEnvelope.Validate() error = %q, want source failure", got)
	}
}

func containsAll(got string, want ...string) bool {
	for _, part := range want {
		if !strings.Contains(got, part) {
			return false
		}
	}
	return true
}

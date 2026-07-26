package automation

import (
	"context"
	"testing"
	"time"

	"github.com/compozy/agh/internal/acp"
	hookspkg "github.com/compozy/agh/internal/hooks"
	"github.com/compozy/agh/internal/testutil"
)

func TestDispatchAutomationJobPreFireHookCanMutatePrompt(t *testing.T) {
	t.Parallel()

	store := newMemoryRunStore()
	creator := newRecordingSessionCreator()
	hooks := &recordingAutomationHooks{
		onJobPreFire: func(_ context.Context, payload hookspkg.AutomationJobPreFirePayload) (hookspkg.AutomationJobPreFirePayload, error) {
			if payload.JobID == "" {
				t.Fatal("job pre-fire payload job_id = empty, want non-empty")
			}
			if payload.Prompt != "Summarize the latest state." {
				t.Fatalf("job pre-fire payload prompt = %q, want original job prompt", payload.Prompt)
			}
			if payload.Attempt != 1 {
				t.Fatalf("job pre-fire payload attempt = %d, want 1", payload.Attempt)
			}
			if payload.Schedule == nil || payload.Schedule.Interval != "30m" {
				t.Fatalf("job pre-fire payload schedule = %#v, want 30m interval", payload.Schedule)
			}

			payload.Prompt = "Summarize the latest state with hook context."
			return payload, nil
		},
	}
	dispatcher := newTestDispatcher(t, creator, store, WithDispatcherHooks(hooks))

	job := testJob(AutomationScopeWorkspace, "job-hook-mutate", "ws-alpha")
	run, err := dispatcher.Dispatch(testutil.Context(t), DispatchRequest{
		Kind: DispatchKindSchedule,
		Job:  &job,
	})
	if err != nil {
		t.Fatalf("Dispatch() error = %v", err)
	}
	if run.Status != RunCompleted {
		t.Fatalf("run.Status = %q, want %q", run.Status, RunCompleted)
	}

	prompts := creator.promptCalls()
	if len(prompts) != 1 {
		t.Fatalf("len(prompt calls) = %d, want 1", len(prompts))
	}
	if got, want := prompts[0].message, "Summarize the latest state with hook context."; got != want {
		t.Fatalf("Prompt().message = %q, want %q", got, want)
	}
}

func TestDispatchAutomationTriggerPreFireHookCanCancelBeforeDispatch(t *testing.T) {
	t.Parallel()

	store := newMemoryRunStore()
	creator := newRecordingSessionCreator()
	hooks := &recordingAutomationHooks{
		onTriggerPreFire: func(_ context.Context, payload hookspkg.AutomationTriggerPreFirePayload) (hookspkg.AutomationTriggerPreFirePayload, error) {
			if payload.TriggerID == "" {
				t.Fatal("trigger pre-fire payload trigger_id = empty, want non-empty")
			}
			if payload.Event != "ext.github.push" {
				t.Fatalf("trigger pre-fire payload event = %q, want ext.github.push", payload.Event)
			}
			if payload.Prompt != "Review repo acme/api" {
				t.Fatalf("trigger pre-fire payload prompt = %q, want rendered trigger prompt", payload.Prompt)
			}
			if got := payload.Payload["repo"]; got != "acme/api" {
				t.Fatalf("trigger pre-fire payload repo = %#v, want acme/api", got)
			}
			if payload.Attempt != 1 {
				t.Fatalf("trigger pre-fire payload attempt = %d, want 1", payload.Attempt)
			}

			return payload, hookspkg.ErrAutomationFireCancelled
		},
	}
	dispatcher := newTestDispatcher(t, creator, store, WithDispatcherHooks(hooks))

	trigger := testTrigger(AutomationScopeWorkspace, "trigger-hook-cancel", "ws-alpha")
	trigger.Event = "ext.github.push"
	trigger.Prompt = `Review repo {{ index .Data "repo" }}`
	trigger.WebhookID = ""
	trigger.EndpointSlug = ""
	trigger.WebhookSecretRef = ""

	run, err := dispatcher.Dispatch(testutil.Context(t), DispatchRequest{
		Kind:    DispatchKindExtension,
		Trigger: &trigger,
		Envelope: &ActivationEnvelope{
			Kind:        "ext.github.push",
			Scope:       AutomationScopeWorkspace,
			WorkspaceID: "ws-alpha",
			Source:      ActivationSourceExtension,
			Data: map[string]any{
				"repo": "acme/api",
			},
		},
	})
	if err != nil {
		t.Fatalf("Dispatch() error = %v, want nil when hook cancels fire", err)
	}
	if run.Status != RunCancelled {
		t.Fatalf("run.Status = %q, want %q", run.Status, RunCancelled)
	}
	if got := len(creator.promptCalls()); got != 0 {
		t.Fatalf("len(prompt calls) = %d, want 0 after cancellation", got)
	}
}

func TestDispatchAutomationRunFailedHookIncludesRetryMetadata(t *testing.T) {
	t.Parallel()

	store := newMemoryRunStore()
	creator := newRecordingSessionCreator(
		sessionAttemptPlan{
			events: []acp.AgentEvent{{Error: "first failure"}},
		},
		sessionAttemptPlan{},
	)
	hooks := &recordingAutomationHooks{}
	dispatcher := newTestDispatcher(
		t,
		creator,
		store,
		WithDispatcherHooks(hooks),
		WithDispatcherSleep(func(context.Context, time.Duration) error { return nil }),
	)

	job := testJob(AutomationScopeGlobal, "job-run-failed", "")
	job.Retry = RetryConfig{
		Strategy:   RetryStrategyBackoff,
		MaxRetries: 1,
		BaseDelay:  "1s",
	}

	run, err := dispatcher.Dispatch(testutil.Context(t), DispatchRequest{
		Kind: DispatchKindSchedule,
		Job:  &job,
	})
	if err != nil {
		t.Fatalf("Dispatch() error = %v", err)
	}
	if run.Attempt != 2 {
		t.Fatalf("run.Attempt = %d, want 2 after retry success", run.Attempt)
	}
	if len(hooks.runFailed) != 1 {
		t.Fatalf("len(run failed hooks) = %d, want 1", len(hooks.runFailed))
	}
	failed := hooks.runFailed[0]
	if failed.JobID != job.ID {
		t.Fatalf("run failed payload job_id = %q, want %q", failed.JobID, job.ID)
	}
	if failed.Attempt != 1 {
		t.Fatalf("run failed payload attempt = %d, want 1", failed.Attempt)
	}
	if !failed.WillRetry {
		t.Fatal("run failed payload will_retry = false, want true")
	}
	if failed.Error == "" {
		t.Fatal("run failed payload error = empty, want populated error")
	}
}

type recordingAutomationHooks struct {
	onJobPreFire     func(context.Context, hookspkg.AutomationJobPreFirePayload) (hookspkg.AutomationJobPreFirePayload, error)
	onTriggerPreFire func(context.Context, hookspkg.AutomationTriggerPreFirePayload) (hookspkg.AutomationTriggerPreFirePayload, error)
	runFailed        []hookspkg.AutomationRunFailedPayload
}

func (r *recordingAutomationHooks) DispatchAutomationJobPreFire(
	ctx context.Context,
	payload hookspkg.AutomationJobPreFirePayload,
) (hookspkg.AutomationJobPreFirePayload, error) {
	if r != nil && r.onJobPreFire != nil {
		return r.onJobPreFire(ctx, payload)
	}
	return payload, nil
}

func (r *recordingAutomationHooks) DispatchAutomationJobPostFire(
	_ context.Context,
	payload hookspkg.AutomationJobPostFirePayload,
) (hookspkg.AutomationJobPostFirePayload, error) {
	return payload, nil
}

func (r *recordingAutomationHooks) DispatchAutomationTriggerPreFire(
	ctx context.Context,
	payload hookspkg.AutomationTriggerPreFirePayload,
) (hookspkg.AutomationTriggerPreFirePayload, error) {
	if r != nil && r.onTriggerPreFire != nil {
		return r.onTriggerPreFire(ctx, payload)
	}
	return payload, nil
}

func (r *recordingAutomationHooks) DispatchAutomationTriggerPostFire(
	_ context.Context,
	payload hookspkg.AutomationTriggerPostFirePayload,
) (hookspkg.AutomationTriggerPostFirePayload, error) {
	return payload, nil
}

func (r *recordingAutomationHooks) DispatchAutomationRunCompleted(
	_ context.Context,
	payload hookspkg.AutomationRunCompletedPayload,
) (hookspkg.AutomationRunCompletedPayload, error) {
	return payload, nil
}

func (r *recordingAutomationHooks) DispatchAutomationRunFailed(
	_ context.Context,
	payload hookspkg.AutomationRunFailedPayload,
) (hookspkg.AutomationRunFailedPayload, error) {
	if r != nil {
		r.runFailed = append(r.runFailed, payload)
	}
	return payload, nil
}

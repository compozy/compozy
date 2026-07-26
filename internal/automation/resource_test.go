package automation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/network/participation"
	"github.com/compozy/agh/internal/resources"
	taskpkg "github.com/compozy/agh/internal/task"
	"github.com/compozy/agh/internal/testutil"
	"github.com/compozy/agh/internal/vault"
)

func TestAutomationResourceCodecsRejectInvalidSpecs(t *testing.T) {
	t.Parallel()

	jobCodec, err := NewJobResourceCodec()
	if err != nil {
		t.Fatalf("NewJobResourceCodec() error = %v", err)
	}
	triggerCodec, err := NewTriggerResourceCodec()
	if err != nil {
		t.Fatalf("NewTriggerResourceCodec() error = %v", err)
	}

	ctx := testutil.Context(t)
	workspaceScope := resources.ResourceScope{Kind: resources.ResourceScopeKindWorkspace, ID: "ws-resource"}

	validJob := testJob(AutomationScopeWorkspace, "resource-job", "ws-resource")
	jobWithScopeMismatch := validJob
	jobWithScopeMismatch.WorkspaceID = "ws-other"
	if _, err := jobCodec.DecodeAndValidate(
		ctx,
		workspaceScope,
		mustAutomationJSON(t, jobWithScopeMismatch),
	); !errors.Is(
		err,
		resources.ErrInvalidScopeBinding,
	) {
		t.Fatalf("job scope mismatch error = %v, want ErrInvalidScopeBinding", err)
	} else if !strings.Contains(err.Error(), "automation: bind job resource scope") {
		t.Fatalf("job scope mismatch error = %v, want bind job resource scope context", err)
	}

	malformedJob := validJob
	malformedJob.Schedule = &ScheduleSpec{Mode: ScheduleModeEvery, Interval: "0s"}
	if _, err := jobCodec.DecodeAndValidate(ctx, workspaceScope, mustAutomationJSON(t, malformedJob)); err == nil {
		t.Fatal("job codec accepted malformed schedule")
	} else if !strings.Contains(err.Error(), "automation: validate job resource spec") {
		t.Fatalf("malformed job error = %v, want validate job resource spec context", err)
	}

	validTrigger := Trigger{
		Scope:       AutomationScopeWorkspace,
		Name:        "resource-trigger",
		AgentName:   "reviewer",
		WorkspaceID: "ws-resource",
		Prompt:      `Review {{ index .Data "session_id" }}`,
		Event:       "session.stopped",
		Enabled:     true,
		Retry:       DefaultRetryConfig(),
		FireLimit:   DefaultFireLimitConfig(),
		Source:      JobSourceDynamic,
	}
	triggerWithBadFilter := validTrigger
	triggerWithBadFilter.Filter = map[string]string{"unsupported": "value"}
	if _, err := triggerCodec.DecodeAndValidate(
		ctx,
		workspaceScope,
		mustAutomationJSON(t, triggerWithBadFilter),
	); err == nil {
		t.Fatal("trigger codec accepted malformed filter")
	} else if !strings.Contains(err.Error(), "automation: validate trigger resource spec") {
		t.Fatalf("trigger filter error = %v, want validate trigger resource spec context", err)
	}

	webhookWithoutEndpoint := validTrigger
	webhookWithoutEndpoint.Event = "webhook"
	if _, err := triggerCodec.DecodeAndValidate(
		ctx,
		workspaceScope,
		mustAutomationJSON(t, webhookWithoutEndpoint),
	); err == nil {
		t.Fatal("trigger codec accepted webhook without endpoint_slug or webhook_id")
	} else if !strings.Contains(err.Error(), "automation: validate trigger resource spec") {
		t.Fatalf("webhook trigger error = %v, want validate trigger resource spec context", err)
	}

	t.Run("Should canonicalize authored participation for every resource target", func(t *testing.T) {
		t.Parallel()

		channelID := "  resource-channel  "
		mode := participation.Mode(" live ")
		strategy := participation.ChannelStrategy(" named ")
		request := &participation.Request{
			Mode:            &mode,
			ChannelStrategy: &strategy,
			ChannelID:       &channelID,
		}

		taskJob := validJob
		taskJob.AgentName = ""
		taskJob.Prompt = ""
		taskJob.Task = &JobTaskConfig{
			Title:                "Run task-backed automation",
			NetworkParticipation: request,
		}
		canonicalTaskJob, err := jobCodec.DecodeAndValidate(
			ctx,
			workspaceScope,
			mustAutomationJSON(t, taskJob),
		)
		if err != nil {
			t.Fatalf("jobCodec.DecodeAndValidate(task participation) error = %v", err)
		}
		assertNamedParticipation(t, canonicalTaskJob.Task.NetworkParticipation, "resource-channel")

		loopJob := validJob
		loopJob.TargetKind = TargetKindLoop
		loopJob.AgentName = ""
		loopJob.Prompt = ""
		loopJob.LoopTarget = &LoopTarget{
			WorkspaceID:          "ws-resource",
			LoopName:             "release-loop",
			NetworkParticipation: request,
		}
		canonicalLoopJob, err := jobCodec.DecodeAndValidate(
			ctx,
			workspaceScope,
			mustAutomationJSON(t, loopJob),
		)
		if err != nil {
			t.Fatalf("jobCodec.DecodeAndValidate(loop participation) error = %v", err)
		}
		assertNamedParticipation(t, canonicalLoopJob.LoopTarget.NetworkParticipation, "resource-channel")

		loopTrigger := validTrigger
		loopTrigger.TargetKind = TargetKindLoop
		loopTrigger.AgentName = ""
		loopTrigger.Prompt = ""
		loopTrigger.LoopTarget = &LoopTarget{
			WorkspaceID:          "ws-resource",
			LoopName:             "release-loop",
			NetworkParticipation: request,
		}
		canonicalLoopTrigger, err := triggerCodec.DecodeAndValidate(
			ctx,
			workspaceScope,
			mustAutomationJSON(t, loopTrigger),
		)
		if err != nil {
			t.Fatalf("triggerCodec.DecodeAndValidate(loop participation) error = %v", err)
		}
		assertNamedParticipation(t, canonicalLoopTrigger.LoopTarget.NetworkParticipation, "resource-channel")
	})
}

func TestManagerStartRegistersResourceDefinitionsAtStartup(t *testing.T) {
	t.Parallel()

	h := newManagerResourceHarness(t)
	jobRecord := h.putJobResource(t, "job-startup", "startup-job")
	triggerRecord := h.putTriggerResource(t, "trigger-startup", "startup-trigger")
	manager := h.newResourceManager(t)
	if err := manager.Start(h.ctx); err != nil {
		t.Fatalf("manager.Start() error = %v", err)
	}
	t.Cleanup(func() {
		if err := manager.Shutdown(testutil.Context(t)); err != nil {
			t.Fatalf("manager.Shutdown() error = %v", err)
		}
	})

	jobs, err := manager.Jobs(h.ctx)
	if err != nil {
		t.Fatalf("manager.Jobs() error = %v", err)
	}
	if got := findJobByID(jobs, jobRecord.ID); got == nil {
		t.Fatalf("jobs missing resource-backed job %q after Start", jobRecord.ID)
	}
	if got, want := len(manager.scheduler.States()), 1; got != want {
		t.Fatalf("len(manager.scheduler.States()) = %d, want %d", got, want)
	}

	triggers, err := manager.Triggers(h.ctx)
	if err != nil {
		t.Fatalf("manager.Triggers() error = %v", err)
	}
	if got := findTriggerByID(triggers, triggerRecord.ID); got == nil {
		t.Fatalf("triggers missing resource-backed trigger %q after Start", triggerRecord.ID)
	}
	manager.triggers.mu.RLock()
	registered := len(manager.triggers.registrations)
	manager.triggers.mu.RUnlock()
	if got, want := registered, 1; got != want {
		t.Fatalf("len(manager.triggers.registrations) = %d, want %d", got, want)
	}

	t.Run("Should preserve live runtimes when boot reconciliation repeats the loaded snapshot", func(t *testing.T) {
		liveScheduler := manager.scheduler
		liveTriggerEngine := manager.triggers

		jobPlan, err := manager.BuildJobResourceState(h.ctx, []resources.Record[Job]{jobRecord})
		if err != nil {
			t.Fatalf("BuildJobResourceState(unchanged) error = %v", err)
		}
		if got := jobPlan.OperationCount(); got != 0 {
			t.Fatalf("unchanged job plan operations = %d, want 0", got)
		}
		if err := manager.ApplyJobResourceState(h.ctx, jobPlan); err != nil {
			t.Fatalf("ApplyJobResourceState(unchanged) error = %v", err)
		}
		if manager.scheduler != liveScheduler {
			t.Fatal("unchanged job projection replaced the live scheduler")
		}

		triggerPlan, err := manager.BuildTriggerResourceState(h.ctx, []resources.Record[Trigger]{triggerRecord})
		if err != nil {
			t.Fatalf("BuildTriggerResourceState(unchanged) error = %v", err)
		}
		if got := triggerPlan.OperationCount(); got != 0 {
			t.Fatalf("unchanged trigger plan operations = %d, want 0", got)
		}
		if err := manager.ApplyTriggerResourceState(h.ctx, triggerPlan); err != nil {
			t.Fatalf("ApplyTriggerResourceState(unchanged) error = %v", err)
		}
		if manager.triggers != liveTriggerEngine {
			t.Fatal("unchanged trigger projection replaced the live trigger engine")
		}
	})
}

func TestManagerResourceListsSearchSortAndPage(t *testing.T) {
	t.Parallel()

	t.Run("Should filter the effective source union before the page cut", func(t *testing.T) {
		t.Parallel()

		h := newManagerResourceHarness(t)
		configJob := h.putJobResourceWithSource(t, "job-union-config", "needle-config", JobSourceConfig)
		packageJob := h.putJobResourceWithSource(t, "job-union-package", "needle-package", JobSourcePackage)
		h.putJobResourceWithSource(t, "job-union-dynamic", "needle-dynamic", JobSourceDynamic)
		configTrigger := h.putTriggerResourceWithSource(
			t,
			"trigger-union-config",
			"needle-config",
			JobSourceConfig,
		)
		packageTrigger := h.putTriggerResourceWithSource(
			t,
			"trigger-union-package",
			"needle-package",
			JobSourcePackage,
		)
		h.putTriggerResourceWithSource(
			t,
			"trigger-union-dynamic",
			"needle-dynamic",
			JobSourceDynamic,
		)
		if _, err := h.db.SetJobEnabledOverlay(h.ctx, JobEnabledOverlay{
			JobID:           configJob.ID,
			EnabledOverride: false,
		}); err != nil {
			t.Fatalf("SetJobEnabledOverlay(config) error = %v", err)
		}
		if _, err := h.db.SetJobEnabledOverlay(h.ctx, JobEnabledOverlay{
			JobID:           packageJob.ID,
			EnabledOverride: false,
		}); err != nil {
			t.Fatalf("SetJobEnabledOverlay(package) error = %v", err)
		}
		if _, err := h.db.SetTriggerEnabledOverlay(h.ctx, TriggerEnabledOverlay{
			TriggerID:       configTrigger.ID,
			EnabledOverride: false,
		}); err != nil {
			t.Fatalf("SetTriggerEnabledOverlay(config) error = %v", err)
		}
		if _, err := h.db.SetTriggerEnabledOverlay(h.ctx, TriggerEnabledOverlay{
			TriggerID:       packageTrigger.ID,
			EnabledOverride: false,
		}); err != nil {
			t.Fatalf("SetTriggerEnabledOverlay(package) error = %v", err)
		}

		manager := h.newResourceManager(t)
		if err := manager.Start(h.ctx); err != nil {
			t.Fatalf("manager.Start() error = %v", err)
		}
		t.Cleanup(func() {
			if err := manager.Shutdown(testutil.Context(t)); err != nil {
				t.Errorf("manager.Shutdown() error = %v", err)
			}
		})
		disabled := false
		jobQuery := JobListQuery{Search: "needle", Enabled: &disabled, Limit: 1}
		jobPage, err := manager.ListJobs(h.ctx, jobQuery)
		if err != nil {
			t.Fatalf("manager.ListJobs(first) error = %v", err)
		}
		if jobPage.Total != 2 || len(jobPage.Jobs) != 1 || jobPage.Jobs[0].ID != configJob.ID ||
			!jobPage.HasMore || jobPage.NextCursor == "" {
			t.Fatalf("manager.ListJobs(first) = %#v, want config-first effective union page", jobPage)
		}
		jobQuery.Cursor = jobPage.NextCursor
		jobPage, err = manager.ListJobs(h.ctx, jobQuery)
		if err != nil {
			t.Fatalf("manager.ListJobs(second) error = %v", err)
		}
		if jobPage.Total != 2 || len(jobPage.Jobs) != 1 || jobPage.Jobs[0].ID != packageJob.ID || jobPage.HasMore {
			t.Fatalf("manager.ListJobs(second) = %#v, want package terminal page", jobPage)
		}

		triggerQuery := TriggerListQuery{Search: "needle", Enabled: &disabled, Limit: 1}
		triggerPage, err := manager.ListTriggers(h.ctx, triggerQuery)
		if err != nil {
			t.Fatalf("manager.ListTriggers(first) error = %v", err)
		}
		if triggerPage.Total != 2 || len(triggerPage.Triggers) != 1 ||
			triggerPage.Triggers[0].ID != configTrigger.ID || !triggerPage.HasMore ||
			triggerPage.NextCursor == "" {
			t.Fatalf("manager.ListTriggers(first) = %#v, want config-first effective union page", triggerPage)
		}
		triggerQuery.Cursor = triggerPage.NextCursor
		triggerPage, err = manager.ListTriggers(h.ctx, triggerQuery)
		if err != nil {
			t.Fatalf("manager.ListTriggers(second) error = %v", err)
		}
		if triggerPage.Total != 2 || len(triggerPage.Triggers) != 1 ||
			triggerPage.Triggers[0].ID != packageTrigger.ID || triggerPage.HasMore {
			t.Fatalf("manager.ListTriggers(second) = %#v, want package terminal page", triggerPage)
		}
	})

	t.Run("Should walk matching jobs and triggers with one canonical order", func(t *testing.T) {
		t.Parallel()

		h := newManagerResourceHarness(t)
		sources := []JobSource{JobSourceDynamic, JobSourceConfig, JobSourcePackage}
		expectedJobs := make([]Job, 0, 13)
		expectedTriggers := make([]Trigger, 0, 13)
		for index := range 13 {
			jobRecord := h.putJobResourceWithSource(
				t,
				fmt.Sprintf("job-list-%02d", index),
				fmt.Sprintf("needle-job-%02d", 12-index),
				sources[index%len(sources)],
			)
			expectedJobs = append(expectedJobs, Job{
				ID:     jobRecord.ID,
				Name:   jobRecord.Spec.Name,
				Source: jobRecord.Spec.Source,
			})
			triggerRecord := h.putTriggerResourceWithSource(
				t,
				fmt.Sprintf("trigger-list-%02d", index),
				fmt.Sprintf("needle-trigger-%02d", 12-index),
				sources[index%len(sources)],
			)
			expectedTriggers = append(expectedTriggers, Trigger{
				ID:     triggerRecord.ID,
				Name:   triggerRecord.Spec.Name,
				Source: triggerRecord.Spec.Source,
			})
		}
		h.putJobResource(t, "job-list-decoy", "unrelated-job")
		h.putTriggerResource(t, "trigger-list-decoy", "unrelated-trigger")

		manager := h.newResourceManager(t)
		if err := manager.Start(h.ctx); err != nil {
			t.Fatalf("manager.Start() error = %v", err)
		}
		t.Cleanup(func() {
			if err := manager.Shutdown(testutil.Context(t)); err != nil {
				t.Fatalf("manager.Shutdown() error = %v", err)
			}
		})

		SortJobsForList(expectedJobs)
		jobQuery := JobListQuery{Search: " NeEdLe ", Limit: 4}
		walkedJobIDs := make([]string, 0, len(expectedJobs))
		for {
			page, err := manager.ListJobs(h.ctx, jobQuery)
			if err != nil {
				t.Fatalf("manager.ListJobs(cursor=%q) error = %v", jobQuery.Cursor, err)
			}
			if got, want := page.Total, len(expectedJobs); got != want {
				t.Fatalf("manager.ListJobs().Total = %d, want %d", got, want)
			}
			for _, job := range page.Jobs {
				walkedJobIDs = append(walkedJobIDs, job.ID)
			}
			if !page.HasMore {
				break
			}
			jobQuery.Cursor = page.NextCursor
		}
		for index, job := range expectedJobs {
			if got, want := walkedJobIDs[index], job.ID; got != want {
				t.Fatalf("walked job %d = %q, want %q", index, got, want)
			}
		}

		SortTriggersForList(expectedTriggers)
		triggerQuery := TriggerListQuery{Search: "needle", Limit: 5}
		walkedTriggerIDs := make([]string, 0, len(expectedTriggers))
		for {
			page, err := manager.ListTriggers(h.ctx, triggerQuery)
			if err != nil {
				t.Fatalf("manager.ListTriggers(cursor=%q) error = %v", triggerQuery.Cursor, err)
			}
			if got, want := page.Total, len(expectedTriggers); got != want {
				t.Fatalf("manager.ListTriggers().Total = %d, want %d", got, want)
			}
			for _, trigger := range page.Triggers {
				walkedTriggerIDs = append(walkedTriggerIDs, trigger.ID)
			}
			if !page.HasMore {
				break
			}
			triggerQuery.Cursor = page.NextCursor
		}
		for index, trigger := range expectedTriggers {
			if got, want := walkedTriggerIDs[index], trigger.ID; got != want {
				t.Fatalf("walked trigger %d = %q, want %q", index, got, want)
			}
		}
	})
}

func TestAutomationJobResourceBuildDoesNotMutateLiveRuntime(t *testing.T) {
	t.Parallel()

	h := newManagerResourceHarness(t)
	manager := h.newResourceManager(t)
	if err := manager.Start(h.ctx); err != nil {
		t.Fatalf("manager.Start() error = %v", err)
	}
	t.Cleanup(func() {
		if err := manager.Shutdown(testutil.Context(t)); err != nil {
			t.Fatalf("manager.Shutdown() error = %v", err)
		}
	})

	record := h.putJobResource(t, "job-side-effect", "side-effect-job")
	plan, err := manager.BuildJobResourceState(h.ctx, []resources.Record[Job]{record})
	if err != nil {
		t.Fatalf("BuildJobResourceState() error = %v", err)
	}
	if plan.Kind() != JobResourceKind || plan.Revision() != record.Version || plan.OperationCount() != 1 {
		t.Fatalf(
			"job plan metadata = kind:%q revision:%d operations:%d",
			plan.Kind(),
			plan.Revision(),
			plan.OperationCount(),
		)
	}
	shutdownJobResourcePlan(t, manager, plan)

	jobs, err := manager.Jobs(h.ctx)
	if err != nil {
		t.Fatalf("manager.Jobs() error = %v", err)
	}
	if len(jobs) != 0 {
		t.Fatalf("manager.Jobs() after Build = %d, want 0", len(jobs))
	}
	if got := manager.scheduler.States(); len(got) != 0 {
		t.Fatalf("scheduler states after Build = %d, want 0", len(got))
	}
}

func TestAutomationTriggerResourceBuildDoesNotMutateLiveRuntime(t *testing.T) {
	t.Parallel()

	h := newManagerResourceHarness(t)
	manager := h.newResourceManager(t)
	if err := manager.Start(h.ctx); err != nil {
		t.Fatalf("manager.Start() error = %v", err)
	}
	t.Cleanup(func() {
		if err := manager.Shutdown(testutil.Context(t)); err != nil {
			t.Fatalf("manager.Shutdown() error = %v", err)
		}
	})

	record := h.putTriggerResource(t, "trigger-side-effect", "side-effect-trigger")
	plan, err := manager.BuildTriggerResourceState(h.ctx, []resources.Record[Trigger]{record})
	if err != nil {
		t.Fatalf("BuildTriggerResourceState() error = %v", err)
	}
	if plan.Kind() != TriggerResourceKind || plan.Revision() != record.Version || plan.OperationCount() != 1 {
		t.Fatalf(
			"trigger plan metadata = kind:%q revision:%d operations:%d",
			plan.Kind(),
			plan.Revision(),
			plan.OperationCount(),
		)
	}
	shutdownTriggerResourcePlan(t, manager, plan)

	triggers, err := manager.Triggers(h.ctx)
	if err != nil {
		t.Fatalf("manager.Triggers() error = %v", err)
	}
	if len(triggers) != 0 {
		t.Fatalf("manager.Triggers() after Build = %d, want 0", len(triggers))
	}
	manager.triggers.mu.RLock()
	registered := len(manager.triggers.registrations)
	manager.triggers.mu.RUnlock()
	if registered != 0 {
		t.Fatalf("trigger registrations after Build = %d, want 0", registered)
	}
}

func TestAutomationJobResourceApplyFailurePreservesPreviousRuntime(t *testing.T) {
	t.Parallel()

	h := newManagerResourceHarness(t)
	manager := h.newResourceManager(t)
	if err := manager.Start(h.ctx); err != nil {
		t.Fatalf("manager.Start() error = %v", err)
	}
	t.Cleanup(func() {
		if err := manager.Shutdown(testutil.Context(t)); err != nil {
			t.Fatalf("manager.Shutdown() error = %v", err)
		}
	})

	first := h.putJobResource(t, "job-previous", "previous-job")
	firstPlan, err := manager.BuildJobResourceState(h.ctx, []resources.Record[Job]{first})
	if err != nil {
		t.Fatalf("BuildJobResourceState(previous) error = %v", err)
	}
	if err := manager.ApplyJobResourceState(h.ctx, firstPlan); err != nil {
		t.Fatalf("ApplyJobResourceState(previous) error = %v", err)
	}

	next := h.putJobResource(t, "job-next", "next-job")
	nextPlan, err := manager.BuildJobResourceState(h.ctx, []resources.Record[Job]{next})
	if err != nil {
		t.Fatalf("BuildJobResourceState(next) error = %v", err)
	}
	canceledCtx, cancel := context.WithCancel(h.ctx)
	cancel()
	if err := manager.ApplyJobResourceState(canceledCtx, nextPlan); !errors.Is(err, context.Canceled) {
		t.Fatalf("ApplyJobResourceState(canceled) error = %v, want context.Canceled", err)
	}

	jobs, err := manager.Jobs(h.ctx)
	if err != nil {
		t.Fatalf("manager.Jobs() error = %v", err)
	}
	if got := findJobByID(jobs, first.ID); got == nil {
		t.Fatalf("previous job %q missing after failed Apply", first.ID)
	}
	if got := findJobByID(jobs, next.ID); got != nil {
		t.Fatalf("next job %q applied after failed Apply", next.ID)
	}
}

func TestAutomationTriggerResourceApplyFailurePreservesPreviousRuntime(t *testing.T) {
	t.Parallel()

	h := newManagerResourceHarness(t)
	manager := h.newResourceManager(t)
	if err := manager.Start(h.ctx); err != nil {
		t.Fatalf("manager.Start() error = %v", err)
	}
	t.Cleanup(func() {
		if err := manager.Shutdown(testutil.Context(t)); err != nil {
			t.Fatalf("manager.Shutdown() error = %v", err)
		}
	})

	first := h.putTriggerResource(t, "trigger-previous", "previous-trigger")
	firstPlan, err := manager.BuildTriggerResourceState(h.ctx, []resources.Record[Trigger]{first})
	if err != nil {
		t.Fatalf("BuildTriggerResourceState(previous) error = %v", err)
	}
	if err := manager.ApplyTriggerResourceState(h.ctx, firstPlan); err != nil {
		t.Fatalf("ApplyTriggerResourceState(previous) error = %v", err)
	}

	next := h.putTriggerResource(t, "trigger-next", "next-trigger")
	nextPlan, err := manager.BuildTriggerResourceState(h.ctx, []resources.Record[Trigger]{next})
	if err != nil {
		t.Fatalf("BuildTriggerResourceState(next) error = %v", err)
	}
	canceledCtx, cancel := context.WithCancel(h.ctx)
	cancel()
	if err := manager.ApplyTriggerResourceState(canceledCtx, nextPlan); !errors.Is(err, context.Canceled) {
		t.Fatalf("ApplyTriggerResourceState(canceled) error = %v, want context.Canceled", err)
	}

	triggers, err := manager.Triggers(h.ctx)
	if err != nil {
		t.Fatalf("manager.Triggers() error = %v", err)
	}
	if got := findTriggerByID(triggers, first.ID); got == nil {
		t.Fatalf("previous trigger %q missing after failed Apply", first.ID)
	}
	if got := findTriggerByID(triggers, next.ID); got != nil {
		t.Fatalf("next trigger %q applied after failed Apply", next.ID)
	}
}

func TestLegacyAutomationDefinitionWritesDoNotDriveResourceManager(t *testing.T) {
	t.Parallel()

	h := newManagerResourceHarness(t)
	manager := h.newResourceManager(t)
	if err := manager.Start(h.ctx); err != nil {
		t.Fatalf("manager.Start() error = %v", err)
	}
	t.Cleanup(func() {
		if err := manager.Shutdown(testutil.Context(t)); err != nil {
			t.Fatalf("manager.Shutdown() error = %v", err)
		}
	})

	legacyJob, err := h.db.CreateJob(h.ctx, testJob(AutomationScopeGlobal, "legacy-job", ""))
	if err != nil {
		t.Fatalf("CreateJob(legacy) error = %v", err)
	}
	if _, err := manager.GetJob(h.ctx, legacyJob.ID); !errors.Is(err, ErrJobNotFound) {
		t.Fatalf("manager.GetJob(legacy) error = %v, want ErrJobNotFound", err)
	}

	resourceRecord := h.putJobResource(t, "job-resource", "resource-job")
	plan, err := manager.BuildJobResourceState(h.ctx, []resources.Record[Job]{resourceRecord})
	if err != nil {
		t.Fatalf("BuildJobResourceState(resource) error = %v", err)
	}
	if err := manager.ApplyJobResourceState(h.ctx, plan); err != nil {
		t.Fatalf("ApplyJobResourceState(resource) error = %v", err)
	}
	if _, err := manager.GetJob(h.ctx, resourceRecord.ID); err != nil {
		t.Fatalf("manager.GetJob(resource) error = %v", err)
	}
}

func TestAutomationResourceProjectionRejectsInvalidInputs(t *testing.T) {
	t.Parallel()

	h := newManagerResourceHarness(t)
	manager := h.newResourceManager(t)

	if _, err := manager.BuildJobResourceState(nilAutomationResourceContext(), nil); err == nil {
		t.Fatal("BuildJobResourceState(nil context) error = nil, want error")
	}
	var nilManager *Manager
	if _, err := nilManager.BuildJobResourceState(h.ctx, nil); err == nil {
		t.Fatal("nil manager BuildJobResourceState() error = nil, want error")
	}
	if err := manager.ApplyJobResourceState(nilAutomationResourceContext(), nil); err == nil {
		t.Fatal("ApplyJobResourceState(nil context) error = nil, want error")
	}
	if err := nilManager.ApplyJobResourceState(h.ctx, nil); err == nil {
		t.Fatal("nil manager ApplyJobResourceState() error = nil, want error")
	}
	if err := manager.ApplyJobResourceState(h.ctx, &triggerResourceProjectionPlan{}); err == nil {
		t.Fatal("ApplyJobResourceState(wrong plan type) error = nil, want error")
	}
	if err := manager.ApplyJobResourceState(h.ctx, &jobResourceProjectionPlan{}); err == nil {
		t.Fatal("ApplyJobResourceState(missing scheduler) error = nil, want error")
	}

	if _, err := manager.BuildTriggerResourceState(nilAutomationResourceContext(), nil); err == nil {
		t.Fatal("BuildTriggerResourceState(nil context) error = nil, want error")
	}
	if _, err := nilManager.BuildTriggerResourceState(h.ctx, nil); err == nil {
		t.Fatal("nil manager BuildTriggerResourceState() error = nil, want error")
	}
	if err := manager.ApplyTriggerResourceState(nilAutomationResourceContext(), nil); err == nil {
		t.Fatal("ApplyTriggerResourceState(nil context) error = nil, want error")
	}
	if err := nilManager.ApplyTriggerResourceState(h.ctx, nil); err == nil {
		t.Fatal("nil manager ApplyTriggerResourceState() error = nil, want error")
	}
	if err := manager.ApplyTriggerResourceState(h.ctx, &jobResourceProjectionPlan{}); err == nil {
		t.Fatal("ApplyTriggerResourceState(wrong plan type) error = nil, want error")
	}
	if err := manager.ApplyTriggerResourceState(h.ctx, &triggerResourceProjectionPlan{}); err == nil {
		t.Fatal("ApplyTriggerResourceState(missing engine) error = nil, want error")
	}
}

func TestAutomationResourceManagerCRUDUsesTypedResourceStores(t *testing.T) {
	t.Parallel()

	t.Run("Should persist typed resource definitions and preserve Loop target execution", func(t *testing.T) {
		t.Parallel()

		h := newManagerResourceHarness(t)
		loopStarter := &recordingLoopStarter{}
		manager := h.newResourceManager(t, WithLoopStarter(loopStarter))
		if err := manager.Start(h.ctx); err != nil {
			t.Fatalf("manager.Start() error = %v", err)
		}
		t.Cleanup(func() {
			if err := manager.Shutdown(testutil.Context(t)); err != nil {
				t.Fatalf("manager.Shutdown() error = %v", err)
			}
		})

		createdJob, err := manager.CreateJob(h.ctx, testJob(AutomationScopeGlobal, "resource-crud-job", ""))
		if err != nil {
			t.Fatalf("CreateJob(resource) error = %v", err)
		}
		if createdJob.Source != JobSourceDynamic {
			t.Fatalf("created job source = %q, want %q", createdJob.Source, JobSourceDynamic)
		}
		jobRecord, err := h.jobStore.Get(h.ctx, h.actor, createdJob.ID)
		if err != nil {
			t.Fatalf("jobStore.Get(created) error = %v", err)
		}
		if jobRecord.Spec.Name != createdJob.Name {
			t.Fatalf("job resource name = %q, want %q", jobRecord.Spec.Name, createdJob.Name)
		}
		changedTarget := createdJob
		changedTarget.AgentName = "other-agent"
		if _, err := manager.UpdateJob(h.ctx, changedTarget); !errors.Is(err, ErrTargetIdentityImmutable) {
			t.Fatalf("UpdateJob(resource changed target) error = %v, want ErrTargetIdentityImmutable", err)
		}
		blockedLifecycle := createdJob
		blockedLifecycle.Prompt = "Run `agh daemon stop` now."
		if _, err := manager.UpdateJob(h.ctx, blockedLifecycle); !errors.Is(
			err,
			ErrDaemonLifecycleCommandBlocked,
		) {
			t.Fatalf(
				"UpdateJob(resource lifecycle command) error = %v, want ErrDaemonLifecycleCommandBlocked",
				err,
			)
		}
		jobRecord, err = h.jobStore.Get(h.ctx, h.actor, createdJob.ID)
		if err != nil {
			t.Fatalf("jobStore.Get(after blocked update) error = %v", err)
		}
		if jobRecord.Spec.Prompt != createdJob.Prompt {
			t.Fatalf(
				"job resource prompt after blocked update = %q, want unchanged %q",
				jobRecord.Spec.Prompt,
				createdJob.Prompt,
			)
		}

		nextJob := createdJob
		nextJob.Prompt = "Review the resource-backed scheduler"
		updatedJob, err := manager.UpdateJob(h.ctx, nextJob)
		if err != nil {
			t.Fatalf("UpdateJob(resource) error = %v", err)
		}
		if updatedJob.Prompt != nextJob.Prompt {
			t.Fatalf("updated job prompt = %q, want %q", updatedJob.Prompt, nextJob.Prompt)
		}
		disabledJob, err := manager.SetJobEnabled(h.ctx, createdJob.ID, false)
		if err != nil {
			t.Fatalf("SetJobEnabled(resource) error = %v", err)
		}
		if disabledJob.Enabled {
			t.Fatal("SetJobEnabled(resource) returned enabled job, want disabled")
		}
		jobRecord, err = h.jobStore.Get(h.ctx, h.actor, createdJob.ID)
		if err != nil {
			t.Fatalf("jobStore.Get(disabled) error = %v", err)
		}
		if jobRecord.Spec.Enabled {
			t.Fatal("resource job spec enabled = true, want false")
		}
		if err := manager.DeleteJob(h.ctx, createdJob.ID); err != nil {
			t.Fatalf("DeleteJob(resource) error = %v", err)
		}
		if _, err := manager.GetJob(h.ctx, createdJob.ID); !errors.Is(err, ErrJobNotFound) {
			t.Fatalf("GetJob(deleted resource) error = %v, want ErrJobNotFound", err)
		}
		if _, err := h.jobStore.Get(h.ctx, h.actor, createdJob.ID); !errors.Is(err, resources.ErrNotFound) {
			t.Fatalf("jobStore.Get(deleted) error = %v, want resources.ErrNotFound", err)
		}

		trigger := testTrigger(AutomationScopeGlobal, "resource-crud-trigger", "")
		trigger.Event = "session.stopped"
		trigger.WebhookID = ""
		createdTrigger, err := manager.CreateTrigger(h.ctx, trigger, WebhookSecretWrite{})
		if err != nil {
			t.Fatalf("CreateTrigger(resource) error = %v", err)
		}
		triggerRecord, err := h.triggerStore.Get(h.ctx, h.actor, createdTrigger.ID)
		if err != nil {
			t.Fatalf("triggerStore.Get(created) error = %v", err)
		}
		if triggerRecord.Spec.Event != "session.stopped" {
			t.Fatalf("trigger resource event = %q, want session.stopped", triggerRecord.Spec.Event)
		}

		nextTrigger := createdTrigger
		nextTrigger.Prompt = `Review stopped session {{ index .Data "session_id" }}`
		updatedTrigger, err := manager.UpdateTrigger(h.ctx, nextTrigger, nil)
		if err != nil {
			t.Fatalf("UpdateTrigger(resource) error = %v", err)
		}
		if updatedTrigger.Prompt != nextTrigger.Prompt {
			t.Fatalf("updated trigger prompt = %q, want %q", updatedTrigger.Prompt, nextTrigger.Prompt)
		}
		disabledTrigger, err := manager.SetTriggerEnabled(h.ctx, createdTrigger.ID, false)
		if err != nil {
			t.Fatalf("SetTriggerEnabled(resource) error = %v", err)
		}
		if disabledTrigger.Enabled {
			t.Fatal("SetTriggerEnabled(resource) returned enabled trigger, want disabled")
		}
		triggerRecord, err = h.triggerStore.Get(h.ctx, h.actor, createdTrigger.ID)
		if err != nil {
			t.Fatalf("triggerStore.Get(disabled) error = %v", err)
		}
		if triggerRecord.Spec.Enabled {
			t.Fatal("resource trigger spec enabled = true, want false")
		}
		if err := manager.DeleteTrigger(h.ctx, createdTrigger.ID); err != nil {
			t.Fatalf("DeleteTrigger(resource) error = %v", err)
		}
		if _, err := manager.GetTrigger(h.ctx, createdTrigger.ID); !errors.Is(err, ErrTriggerNotFound) {
			t.Fatalf("GetTrigger(deleted resource) error = %v, want ErrTriggerNotFound", err)
		}
		if _, err := h.triggerStore.Get(h.ctx, h.actor, createdTrigger.ID); !errors.Is(err, resources.ErrNotFound) {
			t.Fatalf("triggerStore.Get(deleted) error = %v, want resources.ErrNotFound", err)
		}

		loopTrigger := testTrigger(AutomationScopeWorkspace, "resource-crud-loop-trigger", h.workspace.ID)
		loopTrigger.AgentName = ""
		loopTrigger.Prompt = ""
		loopTrigger.Event = "ext.qa.loop-ready"
		loopTrigger.WebhookID = ""
		loopTrigger.WebhookSecretRef = ""
		loopTrigger.TargetKind = TargetKindLoop
		loopTrigger.LoopTarget = &LoopTarget{
			WorkspaceID: h.workspace.ID,
			LoopName:    "reviews-watch",
			Inputs:      map[string]any{"pr": float64(2)},
		}
		createdLoopTrigger, err := manager.CreateTrigger(h.ctx, loopTrigger, WebhookSecretWrite{})
		if err != nil {
			t.Fatalf("CreateTrigger(resource loop target) error = %v", err)
		}
		if createdLoopTrigger.TargetKind != TargetKindLoop || createdLoopTrigger.LoopTarget == nil {
			t.Fatalf(
				"created Loop trigger target = kind:%q target:%#v",
				createdLoopTrigger.TargetKind,
				createdLoopTrigger.LoopTarget,
			)
		}
		if got, want := createdLoopTrigger.LoopTarget.Inputs["pr"], float64(2); got != want {
			t.Fatalf("created Loop trigger input pr = %#v, want %#v", got, want)
		}
		loopTriggerRecord, err := h.triggerStore.Get(h.ctx, h.actor, createdLoopTrigger.ID)
		if err != nil {
			t.Fatalf("triggerStore.Get(created Loop target) error = %v", err)
		}
		if loopTriggerRecord.Spec.LoopTarget == nil ||
			loopTriggerRecord.Spec.LoopTarget.WorkspaceID != h.workspace.ID {
			t.Fatalf("stored Loop trigger target = %#v", loopTriggerRecord.Spec.LoopTarget)
		}
		loopStartErr := errors.New("Loop start integration probe")
		loopStarter.startErr = loopStartErr
		fired, err := manager.FireExtensionTrigger(h.ctx, ExtensionTriggerRequest{
			Event:       loopTrigger.Event,
			Scope:       AutomationScopeWorkspace,
			WorkspaceID: h.workspace.ID,
			Payload:     map[string]any{"source": "qa"},
		})
		if !errors.Is(err, loopStartErr) {
			t.Fatalf("FireExtensionTrigger(resource Loop target) error = %v, want Loop starter error", err)
		}
		if got, want := fired.Matched, 1; got != want {
			t.Fatalf("FireExtensionTrigger(resource Loop target).Matched = %d, want %d", got, want)
		}
		startCalls := loopStarter.startCallSnapshot()
		if got, want := len(startCalls), 1; got != want {
			t.Fatalf("len(StartLoop calls) = %d, want %d", got, want)
		}
		if got, want := startCalls[0].WorkspaceID, h.workspace.ID; got != want {
			t.Fatalf("StartLoop().WorkspaceID = %q, want %q", got, want)
		}
		if got, want := startCalls[0].Inputs["pr"], float64(2); got != want {
			t.Fatalf("StartLoop().Inputs[pr] = %#v, want %#v", got, want)
		}

		crossWorkspace := cloneTrigger(loopTrigger)
		crossWorkspace.ID = "trigger-resource-crud-loop-cross-workspace"
		crossWorkspace.Name = "resource-crud-loop-cross-workspace"
		crossWorkspace.LoopTarget.WorkspaceID = "ws-other"
		if _, err := manager.CreateTrigger(h.ctx, crossWorkspace, WebhookSecretWrite{}); err == nil {
			t.Fatal("CreateTrigger(cross-workspace Loop target) error = nil, want validation error")
		} else if !strings.Contains(
			err.Error(),
			"trigger.loop_target.workspace_id must equal automation workspace_id",
		) {
			t.Fatalf("CreateTrigger(cross-workspace Loop target) error = %v, want workspace mismatch", err)
		}
		if _, err := h.triggerStore.Get(h.ctx, h.actor, crossWorkspace.ID); !errors.Is(err, resources.ErrNotFound) {
			t.Fatalf("triggerStore.Get(cross-workspace Loop target) error = %v, want resources.ErrNotFound", err)
		}

		loopStarter.validateErr = errors.New("start_kind_not_allowed: trigger not declared")
		incompatible := cloneTrigger(loopTrigger)
		incompatible.ID = "trigger-resource-crud-loop-incompatible"
		incompatible.Name = "resource-crud-loop-incompatible"
		if _, err := manager.CreateTrigger(h.ctx, incompatible, WebhookSecretWrite{}); err == nil {
			t.Fatal("CreateTrigger(incompatible Loop target) error = nil, want start kind error")
		} else if !strings.Contains(err.Error(), "start_kind_not_allowed") {
			t.Fatalf("CreateTrigger(incompatible Loop target) error = %v, want start_kind_not_allowed", err)
		}
		if _, err := h.triggerStore.Get(h.ctx, h.actor, incompatible.ID); !errors.Is(err, resources.ErrNotFound) {
			t.Fatalf("triggerStore.Get(incompatible Loop target) error = %v, want resources.ErrNotFound", err)
		}
	})
}

func TestAutomationResourceManagerCRUDRollsBackCommittedMutationsOnApplyFailure(t *testing.T) {
	t.Parallel()

	t.Run("Should rollback created job resources when projection apply fails", func(t *testing.T) {
		t.Parallel()

		h := newManagerResourceHarness(t)
		ctx, cancel := context.WithCancel(h.ctx)
		jobStore := &cancelAfterMutationStore[Job]{
			store:       h.jobStore,
			cancel:      cancel,
			cancelOnPut: true,
		}
		manager := h.newResourceManager(t, WithResourceDefinitions(jobStore, h.triggerStore, h.actor, nil))

		job := testJob(AutomationScopeGlobal, "rollback-created-job", "")
		_, err := manager.CreateJob(ctx, job)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("manager.CreateJob() error = %v, want context.Canceled", err)
		}
		if _, err := h.jobStore.Get(h.ctx, h.actor, job.ID); !errors.Is(err, resources.ErrNotFound) {
			t.Fatalf("jobStore.Get(created rollback) error = %v, want resources.ErrNotFound", err)
		}
		if _, err := manager.GetJob(h.ctx, job.ID); !errors.Is(err, ErrJobNotFound) {
			t.Fatalf("manager.GetJob(created rollback) error = %v, want ErrJobNotFound", err)
		}
	})

	t.Run("Should rollback updated job resources when projection apply fails", func(t *testing.T) {
		t.Parallel()

		h := newManagerResourceHarness(t)
		jobStore := &cancelAfterMutationStore[Job]{store: h.jobStore}
		manager := h.newResourceManager(t, WithResourceDefinitions(jobStore, h.triggerStore, h.actor, nil))

		current := h.putJobResource(t, "job-rollback-update", "rollback-update-job")
		if err := manager.applyJobResourcesFromStore(h.ctx); err != nil {
			t.Fatalf("manager.applyJobResourcesFromStore() error = %v", err)
		}

		ctx, cancel := context.WithCancel(h.ctx)
		jobStore.cancel = cancel
		jobStore.cancelOnPut = true
		next := cloneJob(current.Spec)
		next.ID = current.ID
		next.Prompt = "Review the updated rollback prompt"
		_, err := manager.UpdateJob(ctx, next)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("manager.UpdateJob() error = %v, want context.Canceled", err)
		}

		stored, err := h.jobStore.Get(h.ctx, h.actor, current.ID)
		if err != nil {
			t.Fatalf("jobStore.Get(updated rollback) error = %v", err)
		}
		if stored.Spec.Prompt != current.Spec.Prompt {
			t.Fatalf("stored job prompt after rollback = %q, want %q", stored.Spec.Prompt, current.Spec.Prompt)
		}
		effective, err := manager.GetJob(h.ctx, current.ID)
		if err != nil {
			t.Fatalf("manager.GetJob(updated rollback) error = %v", err)
		}
		if effective.Prompt != current.Spec.Prompt {
			t.Fatalf("effective job prompt after rollback = %q, want %q", effective.Prompt, current.Spec.Prompt)
		}
	})

	t.Run("Should rollback deleted job resources when projection apply fails", func(t *testing.T) {
		t.Parallel()

		h := newManagerResourceHarness(t)
		jobStore := &cancelAfterMutationStore[Job]{store: h.jobStore}
		manager := h.newResourceManager(t, WithResourceDefinitions(jobStore, h.triggerStore, h.actor, nil))

		current := h.putJobResource(t, "job-rollback-delete", "rollback-delete-job")
		if err := manager.applyJobResourcesFromStore(h.ctx); err != nil {
			t.Fatalf("manager.applyJobResourcesFromStore() error = %v", err)
		}

		ctx, cancel := context.WithCancel(h.ctx)
		jobStore.cancel = cancel
		jobStore.cancelOnDelete = true
		err := manager.DeleteJob(ctx, current.ID)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("manager.DeleteJob() error = %v, want context.Canceled", err)
		}

		if _, err := h.jobStore.Get(h.ctx, h.actor, current.ID); err != nil {
			t.Fatalf("jobStore.Get(deleted rollback) error = %v", err)
		}
		if _, err := manager.GetJob(h.ctx, current.ID); err != nil {
			t.Fatalf("manager.GetJob(deleted rollback) error = %v", err)
		}
	})

	t.Run(
		"Should rollback created trigger resources and owned secrets when projection apply fails",
		func(t *testing.T) {
			t.Parallel()

			h := newManagerResourceHarness(t)
			ctx, cancel := context.WithCancel(h.ctx)
			triggerStore := &cancelAfterMutationStore[Trigger]{
				store:       h.triggerStore,
				cancel:      cancel,
				cancelOnPut: true,
			}
			manager := h.newResourceManager(t, WithResourceDefinitions(h.jobStore, triggerStore, h.actor, nil))

			trigger := testWebhookTrigger(AutomationScopeGlobal, "rollback-created-trigger", "")
			trigger.WebhookSecretRef = ""
			secretRef := defaultAutomationWebhookSecretRef(trigger.ID)
			_, err := manager.CreateTrigger(ctx, trigger, webhookSecretWrite("secret-v1"))
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("manager.CreateTrigger() error = %v, want context.Canceled", err)
			}
			if _, err := h.triggerStore.Get(h.ctx, h.actor, trigger.ID); !errors.Is(err, resources.ErrNotFound) {
				t.Fatalf("triggerStore.Get(created rollback) error = %v, want resources.ErrNotFound", err)
			}
			h.assertWebhookSecretMissing(t, secretRef)
			if _, err := manager.GetTrigger(h.ctx, trigger.ID); !errors.Is(err, ErrTriggerNotFound) {
				t.Fatalf("manager.GetTrigger(created rollback) error = %v, want ErrTriggerNotFound", err)
			}
		},
	)

	t.Run(
		"Should rollback updated trigger resources and owned secret deletion when projection apply fails",
		func(t *testing.T) {
			t.Parallel()

			h := newManagerResourceHarness(t)
			secretStore := &cancelAfterWebhookSecretStore{
				store:          h.webhookSecretStore(t),
				cancelOnDelete: false,
			}
			manager := h.newResourceManager(t, WithWebhookSecretStore(secretStore))

			current := h.putOwnedWebhookTriggerResource(
				t,
				"trigger-rollback-update",
				"rollback-update-trigger",
				"secret-v1",
			)
			if err := manager.applyTriggerResourcesFromStore(h.ctx); err != nil {
				t.Fatalf("manager.applyTriggerResourcesFromStore() error = %v", err)
			}

			ctx, cancel := context.WithCancel(h.ctx)
			secretStore.cancel = cancel
			secretStore.cancelOnDelete = true
			next := cloneTrigger(current.Spec)
			next.ID = current.ID
			next.Event = "session.stopped"
			next.EndpointSlug = ""
			next.WebhookID = ""
			next.WebhookSecretRef = ""
			_, err := manager.UpdateTrigger(ctx, next, nil)
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("manager.UpdateTrigger() error = %v, want context.Canceled", err)
			}

			stored, err := h.triggerStore.Get(h.ctx, h.actor, current.ID)
			if err != nil {
				t.Fatalf("triggerStore.Get(updated rollback) error = %v", err)
			}
			if stored.Spec.Event != current.Spec.Event ||
				stored.Spec.WebhookSecretRef != current.Spec.WebhookSecretRef {
				t.Fatalf(
					"stored trigger after rollback = %#v, want event %q secret ref %q",
					stored.Spec,
					current.Spec.Event,
					current.Spec.WebhookSecretRef,
				)
			}
			if got := h.resolveWebhookSecret(t, current.Spec.WebhookSecretRef); got != "secret-v1" {
				t.Fatalf("restored webhook secret = %q, want %q", got, "secret-v1")
			}
			effective, err := manager.GetTrigger(h.ctx, current.ID)
			if err != nil {
				t.Fatalf("manager.GetTrigger(updated rollback) error = %v", err)
			}
			if effective.Event != current.Spec.Event || effective.WebhookSecretRef != current.Spec.WebhookSecretRef {
				t.Fatalf(
					"effective trigger after rollback = %#v, want event %q secret ref %q",
					effective,
					current.Spec.Event,
					current.Spec.WebhookSecretRef,
				)
			}
		},
	)

	t.Run(
		"Should rollback deleted trigger resources and owned secret deletion when projection apply fails",
		func(t *testing.T) {
			t.Parallel()

			h := newManagerResourceHarness(t)
			secretStore := &cancelAfterWebhookSecretStore{
				store: h.webhookSecretStore(t),
			}
			manager := h.newResourceManager(t, WithWebhookSecretStore(secretStore))

			current := h.putOwnedWebhookTriggerResource(
				t,
				"trigger-rollback-delete",
				"rollback-delete-trigger",
				"secret-v1",
			)
			if err := manager.applyTriggerResourcesFromStore(h.ctx); err != nil {
				t.Fatalf("manager.applyTriggerResourcesFromStore() error = %v", err)
			}

			ctx, cancel := context.WithCancel(h.ctx)
			secretStore.cancel = cancel
			secretStore.cancelOnDelete = true
			err := manager.DeleteTrigger(ctx, current.ID)
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("manager.DeleteTrigger() error = %v, want context.Canceled", err)
			}

			if _, err := h.triggerStore.Get(h.ctx, h.actor, current.ID); err != nil {
				t.Fatalf("triggerStore.Get(deleted rollback) error = %v", err)
			}
			if got := h.resolveWebhookSecret(t, current.Spec.WebhookSecretRef); got != "secret-v1" {
				t.Fatalf("restored deleted webhook secret = %q, want %q", got, "secret-v1")
			}
			if _, err := manager.GetTrigger(h.ctx, current.ID); err != nil {
				t.Fatalf("manager.GetTrigger(deleted rollback) error = %v", err)
			}
		},
	)
}

func TestAutomationResourceSyncManagedDefinitionsPublishesAndPrunesSourceRecords(t *testing.T) {
	t.Parallel()

	h := newManagerResourceHarness(t)
	var triggered []resources.ResourceKind
	manager := h.newResourceManager(t, WithResourceDefinitions(
		h.jobStore,
		h.triggerStore,
		h.actor,
		func(_ context.Context, kind resources.ResourceKind, _ resources.ReconcileReason) error {
			triggered = append(triggered, kind.Normalize())
			return nil
		},
	))

	job := testJob(AutomationScopeGlobal, "managed-resource-job", "")
	trigger := testTrigger(AutomationScopeGlobal, "managed-resource-trigger", "")
	trigger.Event = "session.stopped"
	trigger.WebhookID = ""
	trigger.WebhookSecretRef = ""

	stats, err := manager.SyncManagedDefinitions(
		h.ctx,
		JobSourceConfig,
		[]Job{job},
		[]Trigger{trigger},
	)
	if err != nil {
		t.Fatalf("SyncManagedDefinitions(create) error = %v", err)
	}
	if stats.JobsSynced != 1 || stats.TriggersSynced != 1 || stats.JobsRemoved != 0 || stats.TriggersRemoved != 0 {
		t.Fatalf("create stats = %#v", stats)
	}

	configActor := manager.resourceActorForSource(JobSourceConfig)
	jobRecord, err := h.jobStore.Get(h.ctx, configActor, job.ID)
	if err != nil {
		t.Fatalf("jobStore.Get(config) error = %v", err)
	}
	if jobRecord.Spec.Source != JobSourceConfig || jobRecord.Spec.Name != job.Name {
		t.Fatalf("config job resource = %#v", jobRecord.Spec)
	}
	triggerRecord, err := h.triggerStore.Get(h.ctx, configActor, trigger.ID)
	if err != nil {
		t.Fatalf("triggerStore.Get(config) error = %v", err)
	}
	if triggerRecord.Spec.Source != JobSourceConfig || triggerRecord.Spec.Event != "session.stopped" {
		t.Fatalf("config trigger resource = %#v", triggerRecord.Spec)
	}

	job.Prompt = "Review the updated config resource"
	stats, err = manager.SyncManagedDefinitions(h.ctx, JobSourceConfig, []Job{job}, nil)
	if err != nil {
		t.Fatalf("SyncManagedDefinitions(update) error = %v", err)
	}
	if stats.JobsSynced != 1 || stats.TriggersSynced != 0 || stats.JobsRemoved != 0 || stats.TriggersRemoved != 1 {
		t.Fatalf("update stats = %#v", stats)
	}
	jobRecord, err = h.jobStore.Get(h.ctx, configActor, job.ID)
	if err != nil {
		t.Fatalf("jobStore.Get(updated config) error = %v", err)
	}
	if jobRecord.Spec.Prompt != job.Prompt {
		t.Fatalf("updated config job prompt = %q, want %q", jobRecord.Spec.Prompt, job.Prompt)
	}
	if _, err := h.triggerStore.Get(h.ctx, configActor, trigger.ID); !errors.Is(err, resources.ErrNotFound) {
		t.Fatalf("triggerStore.Get(pruned config) error = %v, want resources.ErrNotFound", err)
	}
	if len(triggered) != 4 {
		t.Fatalf("resource reconcile triggers = %#v, want two kinds for each sync", triggered)
	}
}

func TestAutomationResourceSyncManagedDefinitionsSkipsUnchangedTaskBackedJob(t *testing.T) {
	t.Parallel()

	h := newManagerResourceHarness(t)
	manager := h.newResourceManager(t)

	job := testJob(AutomationScopeWorkspace, "task-backed-resource-job", "ws-1")
	job.AgentName = ""
	job.Prompt = ""
	job.Retry = RetryConfig{Strategy: RetryStrategyNone}
	job.Task = &JobTaskConfig{
		Title:                "Run task-backed automation",
		Description:          "Exercise task equality in managed resource sync",
		NetworkParticipation: testNamedParticipation("builders"),
		Owner:                &taskpkg.Ownership{Kind: taskpkg.OwnerKindPool, Ref: "ops"},
	}

	if _, err := manager.SyncManagedDefinitions(h.ctx, JobSourceConfig, []Job{job}, nil); err != nil {
		t.Fatalf("SyncManagedDefinitions(first) error = %v", err)
	}
	configActor := manager.resourceActorForSource(JobSourceConfig)
	firstRecord, err := h.jobStore.Get(h.ctx, configActor, job.ID)
	if err != nil {
		t.Fatalf("jobStore.Get(first) error = %v", err)
	}

	stats, err := manager.SyncManagedDefinitions(h.ctx, JobSourceConfig, []Job{job}, nil)
	if err != nil {
		t.Fatalf("SyncManagedDefinitions(second) error = %v", err)
	}
	if stats.JobsSynced != 1 || stats.JobsRemoved != 0 || stats.TriggersSynced != 0 || stats.TriggersRemoved != 0 {
		t.Fatalf("second sync stats = %#v", stats)
	}
	secondRecord, err := h.jobStore.Get(h.ctx, configActor, job.ID)
	if err != nil {
		t.Fatalf("jobStore.Get(second) error = %v", err)
	}
	if secondRecord.Version != firstRecord.Version {
		t.Fatalf("unchanged task-backed job version = %d, want %d", secondRecord.Version, firstRecord.Version)
	}
	if secondRecord.Spec.Task == nil || secondRecord.Spec.Task.Owner == nil {
		t.Fatalf("task-backed job resource lost task owner: %#v", secondRecord.Spec.Task)
	}
}

func TestAutomationResourceConfigEnabledChangesUseOperationalOverlays(t *testing.T) {
	t.Parallel()

	h := newManagerResourceHarness(t)
	manager := h.newResourceManager(t)
	if err := manager.Start(h.ctx); err != nil {
		t.Fatalf("manager.Start() error = %v", err)
	}
	t.Cleanup(func() {
		if err := manager.Shutdown(testutil.Context(t)); err != nil {
			t.Fatalf("manager.Shutdown() error = %v", err)
		}
	})

	job := testJob(AutomationScopeGlobal, "config-resource-job", "")
	trigger := testTrigger(AutomationScopeGlobal, "config-resource-trigger", "")
	trigger.Event = "session.stopped"
	trigger.WebhookID = ""
	trigger.WebhookSecretRef = ""
	if _, err := manager.SyncManagedDefinitions(
		h.ctx,
		JobSourceConfig,
		[]Job{job},
		[]Trigger{trigger},
	); err != nil {
		t.Fatalf("SyncManagedDefinitions(config) error = %v", err)
	}
	if err := manager.applyJobResourcesFromStore(h.ctx); err != nil {
		t.Fatalf("applyJobResourcesFromStore() error = %v", err)
	}
	if err := manager.applyTriggerResourcesFromStore(h.ctx); err != nil {
		t.Fatalf("applyTriggerResourcesFromStore() error = %v", err)
	}

	disabledJob, err := manager.SetJobEnabled(h.ctx, job.ID, false)
	if err != nil {
		t.Fatalf("SetJobEnabled(config resource) error = %v", err)
	}
	if disabledJob.Enabled {
		t.Fatal("config resource job effective enabled = true, want false overlay")
	}
	configActor := manager.resourceActorForSource(JobSourceConfig)
	jobRecord, err := h.jobStore.Get(h.ctx, configActor, job.ID)
	if err != nil {
		t.Fatalf("jobStore.Get(config resource) error = %v", err)
	}
	if !jobRecord.Spec.Enabled {
		t.Fatal("config resource job spec enabled = false, want unchanged true")
	}
	jobOverlay, err := h.db.GetJobEnabledOverlay(h.ctx, job.ID)
	if err != nil {
		t.Fatalf("GetJobEnabledOverlay(config resource) error = %v", err)
	}
	if jobOverlay.EnabledOverride {
		t.Fatal("config job overlay enabled_override = true, want false")
	}

	disabledTrigger, err := manager.SetTriggerEnabled(h.ctx, trigger.ID, false)
	if err != nil {
		t.Fatalf("SetTriggerEnabled(config resource) error = %v", err)
	}
	if disabledTrigger.Enabled {
		t.Fatal("config resource trigger effective enabled = true, want false overlay")
	}
	triggerRecord, err := h.triggerStore.Get(h.ctx, configActor, trigger.ID)
	if err != nil {
		t.Fatalf("triggerStore.Get(config resource) error = %v", err)
	}
	if !triggerRecord.Spec.Enabled {
		t.Fatal("config resource trigger spec enabled = false, want unchanged true")
	}
	triggerOverlay, err := h.db.GetTriggerEnabledOverlay(h.ctx, trigger.ID)
	if err != nil {
		t.Fatalf("GetTriggerEnabledOverlay(config resource) error = %v", err)
	}
	if triggerOverlay.EnabledOverride {
		t.Fatal("config trigger overlay enabled_override = true, want false")
	}
}

type managerResourceHarness struct {
	*managerHarness
	jobStore     resources.Store[Job]
	triggerStore resources.Store[Trigger]
	actor        resources.MutationActor
}

func newManagerResourceHarness(t *testing.T) *managerResourceHarness {
	t.Helper()

	base := newManagerHarness(t)
	kernel, err := resources.NewKernel(base.db.DB())
	if err != nil {
		t.Fatalf("resources.NewKernel() error = %v", err)
	}
	jobCodec, err := NewJobResourceCodec()
	if err != nil {
		t.Fatalf("NewJobResourceCodec() error = %v", err)
	}
	jobStore, err := resources.NewStore(kernel, jobCodec)
	if err != nil {
		t.Fatalf("resources.NewStore(job) error = %v", err)
	}
	triggerCodec, err := NewTriggerResourceCodec()
	if err != nil {
		t.Fatalf("NewTriggerResourceCodec() error = %v", err)
	}
	triggerStore, err := resources.NewStore(kernel, triggerCodec)
	if err != nil {
		t.Fatalf("resources.NewStore(trigger) error = %v", err)
	}

	return &managerResourceHarness{
		managerHarness: base,
		jobStore:       jobStore,
		triggerStore:   triggerStore,
		actor: resources.MutationActor{
			Kind: resources.MutationActorKindDaemon,
			ID:   "automation-resource-test",
			Source: resources.ResourceSource{
				Kind: resources.ResourceSourceKind("daemon"),
				ID:   "automation-resource-test",
			},
			MaxScope: resources.ResourceScope{Kind: resources.ResourceScopeKindGlobal},
		},
	}
}

func (h *managerResourceHarness) newResourceManager(t *testing.T, opts ...Option) *Manager {
	t.Helper()

	resourceOpts := []Option{
		WithResourceDefinitions(h.jobStore, h.triggerStore, h.actor, nil),
	}
	resourceOpts = append(resourceOpts, opts...)
	return h.newManager(t, defaultAutomationTestConfig(), resourceOpts...)
}

func (h *managerResourceHarness) putJobResource(t *testing.T, id string, name string) resources.Record[Job] {
	t.Helper()
	return h.putJobResourceWithSource(t, id, name, JobSourceDynamic)
}

func (h *managerResourceHarness) putJobResourceWithSource(
	t *testing.T,
	id string,
	name string,
	source JobSource,
) resources.Record[Job] {
	t.Helper()

	runAt := time.Now().UTC().Add(time.Hour).Format(time.RFC3339)
	job := Job{
		Scope:     AutomationScopeGlobal,
		Name:      name,
		AgentName: "reviewer",
		Prompt:    "Review repository",
		Schedule:  &ScheduleSpec{Mode: ScheduleModeAt, Time: runAt},
		Enabled:   true,
		Retry:     DefaultRetryConfig(),
		FireLimit: DefaultFireLimitConfig(),
		Source:    source,
	}
	record, err := h.jobStore.Put(testutil.Context(t), h.actor, resources.Draft[Job]{
		ID:              id,
		Scope:           ResourceScopeForAutomation(job.Scope, job.WorkspaceID),
		ExpectedVersion: 0,
		Spec:            job,
	})
	if err != nil {
		t.Fatalf("jobStore.Put(%q) error = %v", id, err)
	}
	return record
}

func (h *managerResourceHarness) putTriggerResource(t *testing.T, id string, name string) resources.Record[Trigger] {
	t.Helper()
	return h.putTriggerResourceWithSource(t, id, name, JobSourceDynamic)
}

func (h *managerResourceHarness) putTriggerResourceWithSource(
	t *testing.T,
	id string,
	name string,
	source JobSource,
) resources.Record[Trigger] {
	t.Helper()

	trigger := Trigger{
		Scope:     AutomationScopeGlobal,
		Name:      name,
		AgentName: "reviewer",
		Prompt:    `Review {{ index .Data "session_id" }}`,
		Event:     "session.stopped",
		Enabled:   true,
		Retry:     DefaultRetryConfig(),
		FireLimit: DefaultFireLimitConfig(),
		Source:    source,
	}
	record, err := h.triggerStore.Put(testutil.Context(t), h.actor, resources.Draft[Trigger]{
		ID:              id,
		Scope:           ResourceScopeForAutomation(trigger.Scope, trigger.WorkspaceID),
		ExpectedVersion: 0,
		Spec:            trigger,
	})
	if err != nil {
		t.Fatalf("triggerStore.Put(%q) error = %v", id, err)
	}
	return record
}

func (h *managerResourceHarness) putOwnedWebhookTriggerResource(
	t *testing.T,
	id string,
	name string,
	secret string,
) resources.Record[Trigger] {
	t.Helper()

	trigger := testWebhookTrigger(AutomationScopeGlobal, name, "")
	trigger.ID = id
	trigger.Name = name
	trigger.WebhookID = "wbh_" + name
	trigger.WebhookSecretRef = defaultAutomationWebhookSecretRef(id)
	record, err := h.triggerStore.Put(testutil.Context(t), h.actor, resources.Draft[Trigger]{
		ID:              trigger.ID,
		Scope:           ResourceScopeForAutomation(trigger.Scope, trigger.WorkspaceID),
		ExpectedVersion: 0,
		Spec:            trigger,
	})
	if err != nil {
		t.Fatalf("triggerStore.Put(%q) error = %v", id, err)
	}
	h.putWebhookSecret(t, trigger.WebhookSecretRef, secret)
	return record
}

func defaultAutomationTestConfig() aghconfig.AutomationConfig {
	return aghconfig.AutomationConfig{
		Enabled:           true,
		Timezone:          DefaultTimezone,
		MaxConcurrentJobs: DefaultMaxConcurrentJobs,
		DefaultFireLimit:  DefaultFireLimitConfig(),
		Suggestions: aghconfig.AutomationSuggestionsConfig{
			PendingCap: DefaultSuggestionPendingCap,
		},
	}
}

func mustAutomationJSON(t *testing.T, value any) []byte {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	return raw
}

func nilAutomationResourceContext() context.Context {
	return nil
}

func shutdownJobResourcePlan(t *testing.T, manager *Manager, plan resources.ProjectionPlan) {
	t.Helper()
	typed, ok := plan.(*jobResourceProjectionPlan)
	if !ok || typed.scheduler == nil {
		return
	}
	if err := manager.shutdownRuntimeComponent(testutil.Context(t), "scheduler", typed.scheduler); err != nil {
		t.Fatalf("shutdown job resource plan scheduler error = %v", err)
	}
}

func shutdownTriggerResourcePlan(t *testing.T, manager *Manager, plan resources.ProjectionPlan) {
	t.Helper()
	typed, ok := plan.(*triggerResourceProjectionPlan)
	if !ok || typed.engine == nil {
		return
	}
	if err := manager.shutdownRuntimeComponent(testutil.Context(t), "trigger engine", typed.engine); err != nil {
		t.Fatalf("shutdown trigger resource plan engine error = %v", err)
	}
}

type cancelAfterMutationStore[T any] struct {
	store          resources.Store[T]
	cancel         context.CancelFunc
	cancelOnPut    bool
	cancelOnDelete bool
}

var _ resources.Store[Job] = (*cancelAfterMutationStore[Job])(nil)

func (s *cancelAfterMutationStore[T]) Put(
	ctx context.Context,
	actor resources.MutationActor,
	draft resources.Draft[T],
) (resources.Record[T], error) {
	record, err := s.store.Put(ctx, actor, draft)
	if err == nil && s.cancelOnPut && s.cancel != nil {
		s.cancel()
	}
	return record, err
}

func (s *cancelAfterMutationStore[T]) Delete(
	ctx context.Context,
	actor resources.MutationActor,
	id string,
	expectedVersion int64,
) error {
	err := s.store.Delete(ctx, actor, id, expectedVersion)
	if err == nil && s.cancelOnDelete && s.cancel != nil {
		s.cancel()
	}
	return err
}

func (s *cancelAfterMutationStore[T]) Get(
	ctx context.Context,
	actor resources.MutationActor,
	id string,
) (resources.Record[T], error) {
	return s.store.Get(ctx, actor, id)
}

func (s *cancelAfterMutationStore[T]) List(
	ctx context.Context,
	actor resources.MutationActor,
	filter resources.ResourceFilter,
) ([]resources.Record[T], error) {
	return s.store.List(ctx, actor, filter)
}

type cancelAfterWebhookSecretStore struct {
	store          WebhookSecretStore
	cancel         context.CancelFunc
	cancelOnPut    bool
	cancelOnDelete bool
}

var _ WebhookSecretStore = (*cancelAfterWebhookSecretStore)(nil)

func (s *cancelAfterWebhookSecretStore) ResolveRef(ctx context.Context, ref string) (string, error) {
	return s.store.ResolveRef(ctx, ref)
}

func (s *cancelAfterWebhookSecretStore) PutSecret(
	ctx context.Context,
	ref string,
	kind string,
	value string,
) (vault.Metadata, error) {
	metadata, err := s.store.PutSecret(ctx, ref, kind, value)
	if err == nil && s.cancelOnPut && s.cancel != nil {
		s.cancel()
	}
	return metadata, err
}

func (s *cancelAfterWebhookSecretStore) DeleteSecret(ctx context.Context, ref string) error {
	err := s.store.DeleteSecret(ctx, ref)
	if err == nil && s.cancelOnDelete && s.cancel != nil {
		s.cancel()
	}
	return err
}

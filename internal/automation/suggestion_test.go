package automation

import (
	"errors"
	"sync"
	"testing"

	"github.com/compozy/agh/internal/testutil"
)

func TestManagerAutomationSuggestions(t *testing.T) {
	t.Parallel()

	t.Run("Should seed the deterministic workspace catalog on first touch", func(t *testing.T) {
		t.Parallel()

		h := newManagerResourceHarness(t)
		manager := h.newResourceManager(t)
		first, err := manager.ListSuggestions(
			h.ctx,
			h.workspace.ID,
			SuggestionStatusPending,
		)
		if err != nil {
			t.Fatalf("ListSuggestions(first) error = %v", err)
		}
		second, err := manager.ListSuggestions(
			h.ctx,
			h.workspace.ID,
			SuggestionStatusPending,
		)
		if err != nil {
			t.Fatalf("ListSuggestions(second) error = %v", err)
		}
		if got, want := len(first), len(starterSuggestionCatalog); got != want {
			t.Fatalf("len(ListSuggestions(first)) = %d, want %d", got, want)
		}
		if got, want := len(second), len(first); got != want {
			t.Fatalf("len(ListSuggestions(second)) = %d, want %d", got, want)
		}
		for index, suggestion := range first {
			if suggestion.ID != second[index].ID || suggestion.DedupKey != second[index].DedupKey {
				t.Fatalf("seed identity changed at index %d: %#v != %#v", index, suggestion, second[index])
			}
			if suggestion.WorkspaceID != h.workspace.ID ||
				suggestion.Payload.WorkspaceID != h.workspace.ID ||
				suggestion.Payload.Scope != AutomationScopeWorkspace {
				t.Fatalf("suggestion workspace binding = %#v, want workspace %q", suggestion, h.workspace.ID)
			}
			if got, want := suggestion.Payload.AgentName, h.workspace.Config.Defaults.Agent; got != want {
				t.Fatalf("suggestion agent = %q, want effective default %q", got, want)
			}
			if suggestion.Payload.Schedule == nil ||
				suggestion.Payload.Schedule.CatchUpPolicy != SchedulerCatchUpPolicyRunOnce {
				t.Fatalf("suggestion schedule = %#v, want run-once catch-up", suggestion.Payload.Schedule)
			}
		}
	})

	t.Run("Should enforce the configured pending cap during catalog seeding", func(t *testing.T) {
		t.Parallel()

		h := newManagerResourceHarness(t)
		cfg := defaultAutomationTestConfig()
		cfg.Suggestions.PendingCap = 2
		manager := h.newResourceManager(t, WithConfig(cfg))
		suggestions, err := manager.ListSuggestions(
			h.ctx,
			h.workspace.ID,
			SuggestionStatusPending,
		)
		if err != nil {
			t.Fatalf("ListSuggestions() error = %v", err)
		}
		if got, want := len(suggestions), cfg.Suggestions.PendingCap; got != want {
			t.Fatalf("len(ListSuggestions()) = %d, want configured cap %d", got, want)
		}
	})

	t.Run("Should let exactly one concurrent acceptance create the resource Job", func(t *testing.T) {
		t.Parallel()

		h := newManagerResourceHarness(t)
		manager := h.newResourceManager(t)
		suggestions, err := manager.ListSuggestions(
			h.ctx,
			h.workspace.ID,
			SuggestionStatusPending,
		)
		if err != nil {
			t.Fatalf("ListSuggestions() error = %v", err)
		}
		selected := suggestions[0]
		results := make(chan SuggestionAcceptance, 2)
		errs := make(chan error, 2)
		var workers sync.WaitGroup
		for range 2 {
			workers.Go(func() {
				accepted, acceptErr := manager.AcceptSuggestion(
					testutil.Context(t),
					h.workspace.ID,
					selected.ID,
				)
				results <- accepted
				errs <- acceptErr
			})
		}
		workers.Wait()
		close(results)
		close(errs)

		winners := 0
		losers := 0
		for err := range errs {
			switch {
			case err == nil:
				winners++
			case errors.Is(err, ErrSuggestionResolved):
				losers++
			default:
				t.Fatalf("AcceptSuggestion(concurrent) unexpected error = %v", err)
			}
		}
		if winners != 1 || losers != 1 {
			t.Fatalf("AcceptSuggestion winners/losers = %d/%d, want 1/1", winners, losers)
		}
		for result := range results {
			if result.Job.ID == "" {
				continue
			}
			if got, want := result.Job.ID, suggestionJobID(selected.WorkspaceID, selected.DedupKey); got != want {
				t.Fatalf("accepted Job ID = %q, want %q", got, want)
			}
		}
		page, err := manager.ListJobs(h.ctx, JobListQuery{
			Scope:       AutomationScopeWorkspace,
			WorkspaceID: h.workspace.ID,
		})
		if err != nil {
			t.Fatalf("ListJobs() error = %v", err)
		}
		if got, want := len(page.Jobs), 1; got != want {
			t.Fatalf("len(ListJobs()) = %d, want %d", got, want)
		}
	})

	t.Run("Should reject a lifecycle command before resolving or creating", func(t *testing.T) {
		t.Parallel()

		h := newManagerResourceHarness(t)
		manager := h.newResourceManager(t)
		suggestion := suggestionForManagerTest(
			h.workspace.ID,
			"suggestion-lifecycle-guard",
			"catalog:v1:lifecycle-guard",
		)
		suggestion.Payload.Prompt = "Run `agh daemon restart` now."
		created, err := h.db.CreateSuggestion(h.ctx, suggestion, DefaultSuggestionPendingCap)
		if err != nil {
			t.Fatalf("CreateSuggestion() error = %v", err)
		}
		if _, err := manager.AcceptSuggestion(
			h.ctx,
			h.workspace.ID,
			created.ID,
		); !errors.Is(err, ErrDaemonLifecycleCommandBlocked) {
			t.Fatalf("AcceptSuggestion() error = %v, want ErrDaemonLifecycleCommandBlocked", err)
		}
		stored, err := h.db.GetSuggestion(h.ctx, h.workspace.ID, created.ID)
		if err != nil {
			t.Fatalf("GetSuggestion() error = %v", err)
		}
		if stored.Status != SuggestionStatusPending {
			t.Fatalf("suggestion status = %q, want pending", stored.Status)
		}
		if _, err := manager.authoritativeJobDefinition(
			h.ctx,
			suggestionJobID(created.WorkspaceID, created.DedupKey),
		); !errors.Is(err, ErrJobNotFound) {
			t.Fatalf("authoritativeJobDefinition() error = %v, want ErrJobNotFound", err)
		}
	})

	t.Run("Should recover an accepted suggestion before scheduler startup", func(t *testing.T) {
		t.Parallel()

		h := newManagerResourceHarness(t)
		suggestion := suggestionForManagerTest(
			h.workspace.ID,
			"suggestion-recovery",
			"catalog:v1:recovery",
		)
		created, err := h.db.CreateSuggestion(h.ctx, suggestion, DefaultSuggestionPendingCap)
		if err != nil {
			t.Fatalf("CreateSuggestion() error = %v", err)
		}
		accepted, err := h.db.ResolveSuggestion(
			h.ctx,
			h.workspace.ID,
			created.ID,
			SuggestionStatusAccepted,
		)
		if err != nil {
			t.Fatalf("ResolveSuggestion(accepted) error = %v", err)
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
		jobID := suggestionJobID(accepted.WorkspaceID, accepted.DedupKey)
		job, err := manager.GetJob(h.ctx, jobID)
		if err != nil {
			t.Fatalf("manager.GetJob(recovered) error = %v", err)
		}
		if job.WorkspaceID != h.workspace.ID || job.Name != accepted.Payload.Name {
			t.Fatalf("recovered Job = %#v, want accepted workspace payload", job)
		}
	})
}

func suggestionForManagerTest(workspaceID string, id string, dedupKey string) Suggestion {
	return Suggestion{
		ID:          id,
		WorkspaceID: workspaceID,
		Source:      SuggestionSourceCatalog,
		DedupKey:    dedupKey,
		Status:      SuggestionStatusPending,
		Payload: Job{
			ID:          suggestionJobID(workspaceID, dedupKey),
			Scope:       AutomationScopeWorkspace,
			Name:        "Suggested workspace review",
			TargetKind:  TargetKindAgent,
			AgentName:   "general",
			WorkspaceID: workspaceID,
			Prompt:      "Review the workspace and report priorities.",
			Schedule: &ScheduleSpec{
				Mode:          ScheduleModeCron,
				Expr:          "0 8 * * *",
				CatchUpPolicy: SchedulerCatchUpPolicyRunOnce,
			},
			Enabled:   true,
			Retry:     DefaultRetryConfig(),
			FireLimit: DefaultFireLimitConfig(),
			Source:    JobSourceDynamic,
		},
	}
}

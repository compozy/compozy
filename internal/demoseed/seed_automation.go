package demoseed

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	automationpkg "github.com/compozy/compozy/internal/automation"
	automation "github.com/compozy/compozy/internal/automation/model"
	"github.com/compozy/compozy/internal/resources"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb"
	"github.com/compozy/compozy/internal/workspaceaccess"
)

// automationResourceActor is the actor the automation manager writes operator-created
// definitions with (`resourceActorForSource(JobSourceDynamic)`), so the daemon owns them alike.
func automationResourceActor() resources.MutationActor {
	sourceID := "automation." + string(automation.JobSourceDynamic)
	return resources.MutationActor{
		Kind:     resources.MutationActorKindDaemon,
		ID:       sourceID,
		Source:   resources.ResourceSource{Kind: resources.ResourceSourceKind("daemon"), ID: sourceID},
		MaxScope: resources.ResourceScope{Kind: resources.ResourceScopeKindUser},
	}
}

// automationStores are the desired-state stores the daemon's automation manager lists
// definitions from; the legacy automation_jobs table is not read once they are wired.
type automationStores struct {
	jobs     resources.Store[automation.Job]
	triggers resources.Store[automation.Trigger]
}

func openAutomationStores(db *globaldb.GlobalDB, now func() time.Time) (automationStores, error) {
	kernel, err := resources.NewKernel(db.DB(), resources.WithReadDB(db.ReadDB()), resources.WithNow(now))
	if err != nil {
		return automationStores{}, fmt.Errorf("demo seed: open resource kernel: %w", err)
	}
	jobCodec, err := automationpkg.NewJobResourceCodec()
	if err != nil {
		return automationStores{}, fmt.Errorf("demo seed: automation job codec: %w", err)
	}
	triggerCodec, err := automationpkg.NewTriggerResourceCodec()
	if err != nil {
		return automationStores{}, fmt.Errorf("demo seed: automation trigger codec: %w", err)
	}
	jobs, err := resources.NewStore(kernel, jobCodec)
	if err != nil {
		return automationStores{}, fmt.Errorf("demo seed: automation job store: %w", err)
	}
	triggers, err := resources.NewStore(kernel, triggerCodec)
	if err != nil {
		return automationStores{}, fmt.Errorf("demo seed: automation trigger store: %w", err)
	}
	return automationStores{jobs: jobs, triggers: triggers}, nil
}

func automationJobCount() int     { return countAutomationDefinitions(true) }
func automationTriggerCount() int { return countAutomationDefinitions(false) }

func countAutomationDefinitions(jobs bool) int {
	count := 0
	for _, definition := range scenarioAutomationDefinitions(newTimeline(time.Now())) {
		if (definition.Job != nil) == jobs {
			count++
		}
	}
	return count
}

func automationRunCount() int { return len(scenarioAutomationRuns(newTimeline(time.Now()))) }

// cleanupWorkspaceAutomation removes this workspace's seed-owned definitions and runs
// before the workspace row goes, so a replace never collides with the previous seed.
func cleanupWorkspaceAutomation(
	ctx context.Context,
	db *globaldb.GlobalDB,
	state *scenario,
	workspaceKey string,
	workspaceID string,
) error {
	for _, run := range scenarioAutomationRuns(state.clock) {
		if run.WorkspaceKey != workspaceKey {
			continue
		}
		if err := db.DeleteRun(ctx, run.ID); err != nil && !errors.Is(err, automation.ErrRunNotFound) {
			return fmt.Errorf("demo seed: replace automation run %q: %w", run.ID, err)
		}
	}
	stores, err := openAutomationStores(db, state.writeClock.Now)
	if err != nil {
		return err
	}
	actor := automationResourceActor()
	for _, definition := range scenarioAutomationDefinitions(state.clock) {
		if definition.WorkspaceKey != workspaceKey {
			continue
		}
		if err := deleteAutomationDefinition(ctx, stores, actor, definition, workspaceID); err != nil {
			return err
		}
	}
	return nil
}

func deleteAutomationDefinition(
	ctx context.Context,
	stores automationStores,
	actor resources.MutationActor,
	definition automationDefinitionStory,
	workspaceID string,
) error {
	var (
		version int64
		owner   string
		err     error
	)
	if definition.Job != nil {
		var record resources.Record[automation.Job]
		record, err = stores.jobs.Get(ctx, actor, definition.id())
		version, owner = record.Version, record.Spec.WorkspaceID
	} else {
		var record resources.Record[automation.Trigger]
		record, err = stores.triggers.Get(ctx, actor, definition.id())
		version, owner = record.Version, record.Spec.WorkspaceID
	}
	if errors.Is(err, resources.ErrNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("demo seed: inspect automation %q: %w", definition.id(), err)
	}
	if owner != workspaceID {
		return fmt.Errorf(
			"demo seed: automation %q belongs to workspace %q, not %q",
			definition.id(),
			owner,
			workspaceID,
		)
	}
	if definition.Job != nil {
		err = stores.jobs.Delete(ctx, actor, definition.id(), version)
	} else {
		err = stores.triggers.Delete(ctx, actor, definition.id(), version)
	}
	if err != nil {
		return fmt.Errorf("demo seed: replace automation %q: %w", definition.id(), err)
	}
	return nil
}

// seedAutomation writes the operator-created schedules and triggers the way the
// automation manager stores them, then each recorded fire with the session it ran in.
// automationSeeded reports the run sessions the automation history wrote.
type automationSeeded struct {
	sessionIDs       []string
	transcriptEvents int
}

func seedAutomation(ctx context.Context, db *globaldb.GlobalDB, state *scenario) (automationSeeded, error) {
	defer state.writeClock.reset()
	seeded := automationSeeded{}
	stores, err := openAutomationStores(db, state.writeClock.Now)
	if err != nil {
		return seeded, err
	}
	definitions := map[string]automationDefinitionStory{}
	for _, definition := range scenarioAutomationDefinitions(state.clock) {
		if err := putAutomationDefinition(ctx, stores, state, definition); err != nil {
			return seeded, err
		}
		definitions[definition.id()] = definition
	}
	for _, run := range scenarioAutomationRuns(state.clock) {
		definition := definitions[run.JobID+run.TriggerID]
		sessionID, events, err := seedAutomationRun(ctx, db, state, definition, run)
		if err != nil {
			return seeded, err
		}
		seeded.sessionIDs = append(seeded.sessionIDs, sessionID)
		seeded.transcriptEvents += events
	}
	return seeded, nil
}

func putAutomationDefinition(
	ctx context.Context,
	stores automationStores,
	state *scenario,
	definition automationDefinitionStory,
) error {
	record, err := state.recordFor(definition.WorkspaceKey)
	if err != nil {
		return err
	}
	actor := automationResourceActor()
	scope := resources.ResourceScope{Kind: resources.ResourceScopeKindWorkspace, ID: record.ID}
	if job := definition.Job; job != nil {
		spec := *job
		spec.WorkspaceID = record.ID
		state.writeClock.at(spec.CreatedAt)
		if _, err := stores.jobs.Put(ctx, actor, resources.Draft[automation.Job]{
			ID: spec.ID, Scope: scope, Spec: spec,
		}); err != nil {
			return fmt.Errorf("demo seed: create automation job %q: %w", spec.ID, err)
		}
		return nil
	}
	spec := *definition.Trigger
	spec.WorkspaceID = record.ID
	state.writeClock.at(spec.CreatedAt)
	if _, err := stores.triggers.Put(ctx, actor, resources.Draft[automation.Trigger]{
		ID: spec.ID, Scope: scope, Spec: spec,
	}); err != nil {
		return fmt.Errorf("demo seed: create automation trigger %q: %w", spec.ID, err)
	}
	return nil
}

func seedAutomationRun(
	ctx context.Context,
	db *globaldb.GlobalDB,
	state *scenario,
	definition automationDefinitionStory,
	run automationRunStory,
) (string, int, error) {
	record, err := state.recordFor(run.WorkspaceKey)
	if err != nil {
		return "", 0, err
	}
	story := automationRunSession(definition, run)
	sessionDir := filepath.Join(state.paths.SessionsDir, story.ID)
	if err := os.RemoveAll(sessionDir); err != nil {
		return "", 0, fmt.Errorf("demo seed: reset automation session %q: %w", story.ID, err)
	}
	events, err := seedSession(ctx, db, state, story)
	if err != nil {
		return "", 0, err
	}
	if run.Status == automation.RunFailed && !run.isLatestFailure() {
		if err := dismissSeenFailure(ctx, db, story.ID, run.EndedAt.Add(30*time.Minute)); err != nil {
			return "", 0, err
		}
	}
	workspaceID := record.ID
	persisted := automation.Run{
		ID: run.ID, ProfileID: store.DefaultProfileID, WorkspaceID: &workspaceID, Attribution: "recorded",
		JobID: run.JobID, TriggerID: run.TriggerID, SessionID: story.ID, FireID: run.FireID,
		Status: run.Status, Attempt: run.Attempt, StartedAt: new(run.StartedAt), EndedAt: new(run.EndedAt),
		Error: run.Error, Metadata: run.Metadata,
	}
	if run.Attempt > 1 {
		// A retry reserves a fresh run; the fire id stays with the first attempt.
		persisted.FireID = ""
	}
	if !run.ScheduledAt.IsZero() {
		persisted.ScheduledAt = new(run.ScheduledAt)
	}
	if _, err := db.CreateRun(ctx, persisted); err != nil {
		return "", 0, fmt.Errorf("demo seed: create automation run %q: %w", run.ID, err)
	}
	return story.ID, events, nil
}

// isLatestFailure keeps only the newest exhausted fire in Needs you; the operator
// already looked at older failures, the way a working week leaves them.
func (r automationRunStory) isLatestFailure() bool {
	return strings.HasSuffix(r.ID, "_outage_a2")
}

func dismissSeenFailure(ctx context.Context, db *globaldb.GlobalDB, sessionID string, at time.Time) error {
	attention, err := db.GetSessionAttention(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("demo seed: read attention for %q: %w", sessionID, err)
	}
	if _, err := db.DismissSessionAttention(ctx, sessionID, attention.Fence(), at); err != nil {
		return fmt.Errorf("demo seed: mark failure seen for %q: %w", sessionID, err)
	}
	return nil
}

func automationRunSession(definition automationDefinitionStory, run automationRunStory) sessionStory {
	name, prompt := "", ""
	if definition.Job != nil {
		name, prompt = definition.Job.Name, definition.Job.Prompt
	} else {
		name, prompt = definition.Trigger.Name, definition.Trigger.Prompt
	}
	story := sessionStory{
		ID:   automationSessionPrefix + strings.TrimPrefix(run.ID, "autorun_northstar_"),
		Name: name, WorkspaceKey: run.WorkspaceKey, AgentName: run.AgentName,
		Provider: providerClaude, Model: modelClaude, SessionType: string(session.SessionTypeSystem),
		InitiatorKind: string(workspaceaccess.ActorAutomation), InitiatorID: definition.id(),
		StartedAt: run.StartedAt, EndedAt: run.EndedAt,
		Input: 1800 + int64(len(run.Summary))*6, Output: 220 + int64(len(run.Summary)),
		CostUSD: 0.03 + float64(len(run.Summary))/20000,
		Steps: []transcriptStep{
			{Kind: stepUser, Text: prompt},
			automationEvidenceStep(run),
			{Kind: stepAgent, Text: run.Summary},
		},
	}
	if run.Status == automation.RunFailed {
		story.StopReason = string(store.StopError)
		story.StopDetail = run.Error
		story.Failure = &sessionFailureStory{Kind: string(store.FailurePrompt), Summary: run.Error}
	} else {
		story.StopDetail = "Automation run finished."
	}
	return story
}

func automationEvidenceStep(run automationRunStory) transcriptStep {
	switch run.AgentName {
	case agentPlatformEngineer:
		result := `{"stdout":"worker  exhausted  last_error\nsettle-07  1  409 duplicate batch"}`
		if run.Status == automation.RunFailed {
			result = `{"stderr":"curl: (22) The requested URL returned error: 503 logs.northstar.internal"}`
		}
		return transcriptStep{
			Kind:       stepTool,
			ToolName:   toolBash,
			ToolKind:   toolKindExecute,
			ToolInput:  `{"command":"./scripts/settlement-workers.sh --since 1h --filter retry_exhausted"}`,
			ToolResult: result,
		}
	case agentDocsSteward:
		return transcriptStep{
			Kind: stepTool, ToolName: toolRead, ToolKind: toolKindRead,
			ToolInput:  `{"file_path":"docs/runbooks/release-window.md"}`,
			ToolResult: `{"content":"# Release window\n\nEvery promoted change names an owner and a rollback step."}`,
		}
	case agentSupportLead:
		return transcriptStep{
			Kind: stepTool, ToolName: toolGrep, ToolKind: toolKindSearch,
			ToolInput:  `{"pattern":"refund","path":"support/macros/br"}`,
			ToolResult: `{"stdout":"support/macros/br/BR-REFUND-01.md\nsupport/macros/br/BR-REFUND-03.md"}`,
		}
	default:
		return transcriptStep{
			Kind: stepTool, ToolName: toolTaskList, ToolKind: toolKindRead,
			ToolInput:  `{"workspace_id":"$WORKSPACE_ID","status":["blocked","ready","in_progress"]}`,
			ToolResult: `{"content":"Blocked: Mexico partner replay gap. Ready: rollback monitor."}`,
		}
	}
}

package demoseed

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/store/globaldb"
)

const cleanupTimeout = 10 * time.Second

// scenario carries everything one seed run needs after option resolution.
type scenario struct {
	paths      config.HomePaths
	clock      timeline
	replace    bool
	workspaces []workspaceStory
	records    map[string]workspaceRecord
	// writeClock is what the store stamps records with; stories move it to their own instant.
	writeClock *writeClock
	// automationSessionIDs are the sessions automation runs recorded.
	automationSessionIDs []string
}

// Seed writes the Northstar Pay launch scenario into one explicit Compozy home.
func Seed(ctx context.Context, opts Options) (result Result, err error) {
	if ctx == nil {
		return Result{}, errors.New("demo seed: context is required")
	}
	paths, clock, err := resolveOptions(opts)
	if err != nil {
		return Result{}, err
	}
	state := &scenario{
		paths: paths, clock: clock, replace: opts.Replace,
		workspaces: scenarioWorkspaces(clock),
		records:    map[string]workspaceRecord{},
		writeClock: newWriteClock(clock.Now()),
	}
	if err := preflightWorkspaceRoots(state); err != nil {
		return Result{}, err
	}
	if err := config.EnsureHomeLayout(paths); err != nil {
		return Result{}, fmt.Errorf("demo seed: ensure Compozy home: %w", err)
	}
	db, err := globaldb.OpenGlobalDB(ctx, paths.DatabaseFile, globaldb.WithClock(state.writeClock.Now))
	if err != nil {
		return Result{}, fmt.Errorf("demo seed: open global database: %w", err)
	}
	defer func() {
		closeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), cleanupTimeout)
		defer cancel()
		if closeErr := db.Close(closeCtx); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("demo seed: close global database: %w", closeErr))
		}
	}()
	return seedDatabase(ctx, db, state)
}

func seedDatabase(ctx context.Context, db *globaldb.GlobalDB, state *scenario) (Result, error) {
	if err := prepareGlobalState(ctx, db, state); err != nil {
		return Result{}, err
	}
	if err := registerWorkspaces(ctx, db, state); err != nil {
		return Result{}, err
	}
	if err := writeAgentDefinitions(state); err != nil {
		return Result{}, err
	}
	counts, err := seedScenarioContent(ctx, db, state)
	if err != nil {
		return Result{}, err
	}
	observability, err := seedObservability(ctx, db, state)
	if err != nil {
		return Result{}, err
	}
	counts.EventSummaries = observability.eventSummaries
	counts.TokenUsageDays = observability.tokenUsageDays
	for _, story := range state.workspaces {
		if err := writeSeedMarker(state.workspaceRoot(story), story.Key); err != nil {
			return Result{}, err
		}
	}
	onboardedAt := state.clock.daysAgo(34).Format(time.RFC3339Nano)
	if _, err := db.CompleteOnboarding(ctx, onboardedAt); err != nil {
		return Result{}, fmt.Errorf("demo seed: complete onboarding: %w", err)
	}
	return buildResult(state, counts), nil
}

func seedScenarioContent(ctx context.Context, db *globaldb.GlobalDB, state *scenario) (Counts, error) {
	sessions := scenarioSessions(state.clock)
	transcriptEvents, err := seedSessions(ctx, db, state, sessions)
	if err != nil {
		return Counts{}, err
	}
	tasks := scenarioTasks(state.clock)
	taskRuns, err := seedTasks(ctx, db, state, tasks)
	if err != nil {
		return Counts{}, err
	}
	loops, err := seedLoopDefinitions(ctx, db, state)
	if err != nil {
		return Counts{}, err
	}
	loopCounts, err := seedLoopRuns(ctx, db, state)
	if err != nil {
		return Counts{}, err
	}
	automationSeeded, err := seedAutomation(ctx, db, state)
	if err != nil {
		return Counts{}, err
	}
	state.automationSessionIDs = automationSeeded.sessionIDs
	worktrees, err := seedWorktree(ctx, db, state)
	if err != nil {
		return Counts{}, err
	}
	board, err := seedBoard(ctx, db, state)
	if err != nil {
		return Counts{}, err
	}
	alwaysOn, err := seedAlwaysOn(ctx, db, state)
	if err != nil {
		return Counts{}, err
	}
	sessionFolders, err := seedSessionFolders(ctx, db, state)
	if err != nil {
		return Counts{}, err
	}
	return Counts{
		SessionFolders:    sessionFolders,
		AlwaysOnInstances: alwaysOn.instances, CheckIns: alwaysOn.checkIns,
		InstanceReports: alwaysOn.reports, ProposalPicks: alwaysOn.picks,
		Subagents: board.subagents, Monitors: board.monitors,
		PullRequests: board.pullRequests, LaterSessions: board.later,
		Workspaces: len(state.workspaces), Agents: len(scenarioAgents()),
		Sessions: len(sessions) + board.sessions + alwaysOn.instances + len(automationSeeded.sessionIDs),
		TranscriptEvents: transcriptEvents + board.transcriptEvents + alwaysOn.transcriptEvents +
			automationSeeded.transcriptEvents,
		Tasks: len(tasks), TaskRuns: taskRuns,
		LoopDefinitions: loops,
		LoopRuns:        loopCounts.runs, LoopGenerations: loopCounts.generations,
		LoopRunEvents: loopCounts.events, GoalTurns: loopCounts.goalTurns,
		Worktrees:      worktrees,
		AutomationJobs: automationJobCount(), AutomationRuns: automationRunCount(),
		AutomationTriggers: automationTriggerCount(),
	}, nil
}

func buildResult(state *scenario, counts Counts) Result {
	primary := state.records[workspaceKeyLaunch]
	workspaceIDs := make([]string, 0, len(state.workspaces))
	for _, story := range state.workspaces {
		workspaceIDs = append(workspaceIDs, state.records[story.Key].ID)
	}
	taskIDs := make([]string, 0)
	for _, story := range scenarioTasks(state.clock) {
		taskIDs = append(taskIDs, story.ID)
	}
	return Result{
		HomeDir:      state.paths.HomeDir,
		DatabaseFile: state.paths.DatabaseFile,
		WorkspaceID:  primary.ID, WorkspaceRoot: primary.RootDir, WorkspaceName: primary.Name,
		WorkspaceIDs: workspaceIDs,
		SessionIDs: append(append(append([]string(nil), scenarioSessionIDs...), boardSessionIDs(state.clock)...),
			append(append(alwaysOnSessionIDs(), bundledSessionIDs()...), state.automationSessionIDs...)...),
		TaskIDs:  taskIDs,
		LoopName: loopLaunchReadiness, LoopNames: scenarioLoopNames(),
		LoopRunID: loopApprovalRunID, LoopRunIDs: scenarioLoopRunIDs(),
		AutomationJobID:  launchAutomationID,
		SuggestedWebPath: suggestedWebPaths(),
		Counts:           counts,
	}
}

func suggestedWebPaths() []string {
	return []string{
		"/",
		"/loop-runs",
		fmt.Sprintf("/loop-runs/%s", loopApprovalRunID),
		fmt.Sprintf("/loops/%s/editor", loopMarketRollout),
		"/tasks?mode=dashboard",
		fmt.Sprintf("/agents/%s/sessions/%s", agentProductLead, sessionLaunchDecisionID),
	}
}

func resolveOptions(opts Options) (config.HomePaths, timeline, error) {
	if opts.HomeDir == "" {
		return config.HomePaths{}, timeline{}, errors.New("demo seed: home directory is required")
	}
	paths, err := config.ResolveHomePathsFrom(opts.HomeDir)
	if err != nil {
		return config.HomePaths{}, timeline{}, fmt.Errorf("demo seed: resolve home: %w", err)
	}
	now := opts.Now
	if now.IsZero() {
		now = time.Now()
	}
	return paths, newTimeline(now), nil
}

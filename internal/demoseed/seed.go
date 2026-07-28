package demoseed

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/config"
	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/store/globaldb"
	compozyworkspace "github.com/compozy/compozy/internal/workspace"
)

const cleanupTimeout = 10 * time.Second

// Seed writes the Northstar Pay launch scenario into one explicit Compozy home.
func Seed(ctx context.Context, opts Options) (result Result, err error) {
	if ctx == nil {
		return Result{}, errors.New("demo seed: context is required")
	}
	paths, now, workspaceRoot, err := resolveOptions(opts)
	if err != nil {
		return Result{}, err
	}
	if err := preflightWorkspaceRoot(workspaceRoot, opts.Replace); err != nil {
		return Result{}, err
	}
	if err := config.EnsureHomeLayout(paths); err != nil {
		return Result{}, fmt.Errorf("demo seed: ensure Compozy home: %w", err)
	}

	db, err := globaldb.OpenGlobalDB(ctx, paths.DatabaseFile)
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
	result, err = seedDatabase(ctx, db, paths, workspaceRoot, now, opts.Replace)
	return result, err
}

func seedDatabase(
	ctx context.Context,
	db *globaldb.GlobalDB,
	paths config.HomePaths,
	workspaceRoot string,
	now time.Time,
	replace bool,
) (Result, error) {
	if err := prepareGlobalState(ctx, db, workspaceRoot, paths, replace); err != nil {
		return Result{}, err
	}
	if err := os.MkdirAll(workspaceRoot, 0o755); err != nil {
		return Result{}, fmt.Errorf("demo seed: create workspace root %q: %w", workspaceRoot, err)
	}
	identity, err := compozyworkspace.EnsureIdentity(ctx, workspaceRoot)
	if err != nil {
		return Result{}, fmt.Errorf("demo seed: ensure workspace identity: %w", err)
	}
	if err := cleanupAutomationState(ctx, db, identity.WorkspaceID, replace); err != nil {
		return Result{}, err
	}

	workspaceRecord := compozyworkspace.Workspace{
		ID:           identity.WorkspaceID,
		RootDir:      workspaceRoot,
		Name:         workspaceName,
		DefaultAgent: productLeadAgent,
		CreatedAt:    now.Add(-26 * time.Hour),
		UpdatedAt:    now.Add(-27 * time.Minute),
	}
	if err := db.InsertWorkspace(ctx, workspaceRecord); err != nil {
		return Result{}, fmt.Errorf("demo seed: register workspace: %w", err)
	}
	if err := writeAgentDefinitions(workspaceRoot, replace); err != nil {
		return Result{}, err
	}
	if err := writeLoopDefinition(workspaceRoot, replace); err != nil {
		return Result{}, err
	}

	sessions := scenarioSessions(now)
	if err := seedSessions(ctx, db, paths, workspaceRecord, sessions); err != nil {
		return Result{}, err
	}
	tasks := scenarioTasks(now)
	if err := seedTasks(ctx, db, workspaceRecord.ID, tasks); err != nil {
		return Result{}, err
	}
	messages := scenarioNetworkMessages(now)
	if err := seedNetwork(ctx, db, workspaceRecord.ID, messages); err != nil {
		return Result{}, err
	}
	if err := seedAutomation(ctx, db, workspaceRecord.ID, now); err != nil {
		return Result{}, err
	}
	if _, err := db.CompleteOnboarding(ctx, now.Add(-26*time.Hour).Format(time.RFC3339Nano)); err != nil {
		return Result{}, fmt.Errorf("demo seed: complete onboarding: %w", err)
	}

	return buildResult(paths, workspaceRecord, sessions, tasks, messages), nil
}

func buildResult(
	paths config.HomePaths,
	workspaceRecord compozyworkspace.Workspace,
	sessions []sessionStory,
	tasks []taskStory,
	messages []networkMessageStory,
) Result {
	taskIDs := make([]string, 0, len(tasks))
	for _, story := range tasks {
		taskIDs = append(taskIDs, story.ID)
	}
	return Result{
		HomeDir:         paths.HomeDir,
		DatabaseFile:    paths.DatabaseFile,
		WorkspaceID:     workspaceRecord.ID,
		WorkspaceRoot:   workspaceRecord.RootDir,
		WorkspaceName:   workspaceRecord.Name,
		SessionIDs:      append([]string(nil), scenarioSessionIDs...),
		TaskIDs:         taskIDs,
		NetworkChannel:  launchChannel,
		NetworkThreadID: launchThreadID,
		LoopName:        launchLoopName,
		AutomationJobID: launchAutomationID,
		SuggestedWebPath: []string{
			"/",
			"/tasks?mode=dashboard",
			fmt.Sprintf("/agents/%s/sessions/%s", productLeadAgent, scenarioSessionIDs[3]),
			fmt.Sprintf("/network/%s/%s/threads/%s", workspaceRecord.ID, launchChannel, launchThreadID),
			"/loops/launch-readiness",
		},
		Counts: Counts{
			Agents:           len(scenarioAgents()),
			Sessions:         len(sessions),
			Tasks:            len(tasks),
			NetworkMessages:  len(messages),
			AutomationJobs:   1,
			AutomationRuns:   1,
			TranscriptEvents: len(sessions) * 6,
		},
	}
}

func resolveOptions(opts Options) (config.HomePaths, time.Time, string, error) {
	if strings.TrimSpace(opts.HomeDir) == "" {
		return config.HomePaths{}, time.Time{}, "", errors.New("demo seed: home directory is required")
	}
	paths, err := config.ResolveHomePathsFrom(opts.HomeDir)
	if err != nil {
		return config.HomePaths{}, time.Time{}, "", fmt.Errorf("demo seed: resolve home: %w", err)
	}
	now := opts.Now
	if now.IsZero() {
		now = time.Now()
	}
	now = now.UTC().Truncate(time.Second)
	workspaceRoot := filepath.Join(paths.HomeDir, filepath.FromSlash(workspaceRelative))
	return paths, now, workspaceRoot, nil
}

func preflightWorkspaceRoot(workspaceRoot string, replace bool) error {
	info, err := os.Lstat(workspaceRoot)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("demo seed: inspect workspace root %q: %w", workspaceRoot, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("demo seed: workspace root %q must not be a symbolic link", workspaceRoot)
	}
	if !info.IsDir() {
		return fmt.Errorf("demo seed: workspace root %q is not a directory", workspaceRoot)
	}
	if !replace {
		return fmt.Errorf("%w at %s; rerun with --replace", ErrScenarioExists, workspaceRoot)
	}
	return nil
}

func prepareGlobalState(
	ctx context.Context,
	db *globaldb.GlobalDB,
	workspaceRoot string,
	paths config.HomePaths,
	replace bool,
) error {
	byPath, pathErr := db.GetWorkspaceByPath(ctx, workspaceRoot)
	if pathErr != nil && !errors.Is(pathErr, compozyworkspace.ErrWorkspaceNotFound) {
		return fmt.Errorf("demo seed: inspect registered workspace path: %w", pathErr)
	}
	byName, nameErr := db.GetWorkspaceByName(ctx, workspaceName)
	if nameErr != nil && !errors.Is(nameErr, compozyworkspace.ErrWorkspaceNotFound) {
		return fmt.Errorf("demo seed: inspect registered workspace name: %w", nameErr)
	}

	existing := byPath
	if errors.Is(pathErr, compozyworkspace.ErrWorkspaceNotFound) && nameErr == nil {
		if filepath.Clean(byName.RootDir) != filepath.Clean(workspaceRoot) {
			return fmt.Errorf(
				"demo seed: workspace name %q is already used at %s",
				workspaceName,
				byName.RootDir,
			)
		}
		existing = byName
	}
	if existing.ID != "" {
		if !replace {
			return fmt.Errorf("%w in %s; rerun with --replace", ErrScenarioExists, paths.DatabaseFile)
		}
		if err := db.DeleteWorkspace(ctx, existing.ID); err != nil {
			return fmt.Errorf("demo seed: replace registered workspace: %w", err)
		}
	}
	if !replace {
		return nil
	}
	for _, sessionID := range scenarioSessionIDs {
		sessionDir := filepath.Join(paths.SessionsDir, sessionID)
		if err := os.RemoveAll(sessionDir); err != nil {
			return fmt.Errorf("demo seed: remove prior session directory %q: %w", sessionDir, err)
		}
	}
	return nil
}

func writeAgentDefinitions(workspaceRoot string, overwrite bool) error {
	agentsRoot := filepath.Join(workspaceRoot, config.DirName, config.AgentsDirName)
	for _, story := range scenarioAgents() {
		path := filepath.Join(agentsRoot, story.Name, config.AgentDefinitionFileName)
		if _, err := config.CreateAgentDefFile(path, config.AgentDefinitionDraft{
			Name: story.Name, Provider: story.Provider, Permissions: story.Permissions,
			Tools: story.Tools, Prompt: story.Prompt,
		}, overwrite); err != nil {
			return fmt.Errorf("demo seed: write agent %q: %w", story.Name, err)
		}
	}
	return nil
}

func writeLoopDefinition(workspaceRoot string, overwrite bool) error {
	loopsRoot := filepath.Join(workspaceRoot, config.DirName, config.LoopsDirName)
	if _, _, err := looppkg.WriteDefinition(loopsRoot, []byte(launchLoopDefinition), looppkg.WriteDefinitionOptions{
		Source: looppkg.SourceUser, Overwrite: overwrite,
	}); err != nil {
		return fmt.Errorf("demo seed: write Loop %q: %w", launchLoopName, err)
	}
	return nil
}

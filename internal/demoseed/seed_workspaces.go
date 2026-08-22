package demoseed

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/fileutil"
	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/store/globaldb"
	compozyworkspace "github.com/compozy/compozy/internal/workspace"
)

// workspaceRecord is the registered identity a seeded workspace resolves to.
type workspaceRecord struct {
	ID      string
	Name    string
	RootDir string
}

func (s *scenario) workspaceRoot(story workspaceStory) string {
	return filepath.Join(s.paths.HomeDir, filepath.FromSlash(story.Relative))
}

func (s *scenario) recordFor(workspaceKey string) (workspaceRecord, error) {
	record, ok := s.records[workspaceKey]
	if !ok {
		return workspaceRecord{}, fmt.Errorf("demo seed: workspace %q is not registered", workspaceKey)
	}
	return record, nil
}

func preflightWorkspaceRoots(state *scenario) error {
	for _, story := range state.workspaces {
		if err := preflightWorkspaceRoot(state.workspaceRoot(story), state.replace); err != nil {
			return err
		}
	}
	return nil
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
	if _, err := readSeedMarker(workspaceRoot); err != nil {
		return fmt.Errorf(
			"demo seed: refusing to replace unowned workspace root %q: %w",
			workspaceRoot,
			err,
		)
	}
	return nil
}

func prepareGlobalState(ctx context.Context, db *globaldb.GlobalDB, state *scenario) error {
	for _, story := range state.workspaces {
		if err := prepareWorkspaceState(ctx, db, state, story); err != nil {
			return err
		}
	}
	if !state.replace {
		return nil
	}
	for _, sessionID := range scenarioSessionIDs {
		sessionDir := filepath.Join(state.paths.SessionsDir, sessionID)
		if err := os.RemoveAll(sessionDir); err != nil {
			return fmt.Errorf("demo seed: remove prior session directory %q: %w", sessionDir, err)
		}
	}
	return nil
}

func prepareWorkspaceState(
	ctx context.Context,
	db *globaldb.GlobalDB,
	state *scenario,
	story workspaceStory,
) error {
	workspaceRoot := state.workspaceRoot(story)
	existing, err := lookupExistingWorkspace(ctx, db, story.Name, workspaceRoot)
	if err != nil {
		return err
	}
	if existing.ID == "" {
		if !state.replace {
			return nil
		}
		marker, markerErr := readSeedMarker(workspaceRoot)
		if markerErr != nil {
			if errors.Is(markerErr, os.ErrNotExist) {
				return nil
			}
			return fmt.Errorf("demo seed: verify replace ownership for %q: %w", story.Name, markerErr)
		}
		if marker.WorkspaceKey != story.Key {
			return fmt.Errorf(
				"demo seed: workspace marker key %q does not match %q",
				marker.WorkspaceKey,
				story.Key,
			)
		}
		if err := cleanSeedManifest(workspaceRoot, marker); err != nil {
			return err
		}
		return nil
	}
	if !state.replace {
		return fmt.Errorf("%w in %s; rerun with --replace", ErrScenarioExists, state.paths.DatabaseFile)
	}
	marker, err := readSeedMarker(workspaceRoot)
	if err != nil {
		return fmt.Errorf("demo seed: verify replace ownership for %q: %w", story.Name, err)
	}
	if marker.WorkspaceKey != story.Key {
		return fmt.Errorf("demo seed: workspace marker key %q does not match %q", marker.WorkspaceKey, story.Key)
	}
	if story.Key == workspaceKeyPlatform {
		if err := cleanupSeedWorktree(ctx, db, state, workspaceRecord{
			ID: existing.ID, Name: existing.Name, RootDir: existing.RootDir,
		}); err != nil {
			return err
		}
	}
	if err := cleanSeedManifest(workspaceRoot, marker); err != nil {
		return err
	}
	// Loop runs are workspace-scoped but not workspace-owned, so they outlive the
	// workspace row and would collide with the next import.
	if err := deleteScenarioLoopRuns(ctx, db, state, story.Key, existing.ID); err != nil {
		return err
	}
	if err := cleanupWorkspaceAutomation(ctx, db, story.Key, existing.ID); err != nil {
		return err
	}
	// The daily usage rollup is additive, so a replace has to clear it first.
	if err := db.DeleteWorkspaceObservability(ctx, existing.ID); err != nil {
		return fmt.Errorf("demo seed: replace observability for %q: %w", story.Name, err)
	}
	if err := db.DeleteWorkspace(ctx, existing.ID); err != nil {
		return fmt.Errorf("demo seed: replace registered workspace %q: %w", story.Name, err)
	}
	return nil
}

func deleteScenarioLoopRuns(
	ctx context.Context,
	db *globaldb.GlobalDB,
	state *scenario,
	workspaceKey string,
	workspaceID string,
) error {
	for _, run := range scenarioLoopRuns(state.clock) {
		if run.WorkspaceKey != workspaceKey {
			continue
		}
		if err := db.DeleteRunHistory(
			ctx, looppkg.WorkspaceID(workspaceID), looppkg.RunID(run.ID),
		); err != nil {
			return fmt.Errorf("demo seed: replace Loop run %q: %w", run.ID, err)
		}
	}
	return nil
}

func lookupExistingWorkspace(
	ctx context.Context,
	db *globaldb.GlobalDB,
	name string,
	workspaceRoot string,
) (compozyworkspace.Workspace, error) {
	canonicalRoot, err := canonicalWorkspaceRoot(workspaceRoot)
	if err != nil {
		return compozyworkspace.Workspace{}, err
	}
	byPath, pathErr := db.GetWorkspaceByPath(ctx, canonicalRoot)
	if pathErr != nil && !errors.Is(pathErr, compozyworkspace.ErrWorkspaceNotFound) {
		return compozyworkspace.Workspace{}, fmt.Errorf("demo seed: inspect registered workspace path: %w", pathErr)
	}
	if pathErr == nil {
		return byPath, nil
	}
	byName, nameErr := db.GetWorkspaceByName(ctx, name)
	if nameErr != nil {
		if errors.Is(nameErr, compozyworkspace.ErrWorkspaceNotFound) {
			return compozyworkspace.Workspace{}, nil
		}
		return compozyworkspace.Workspace{}, fmt.Errorf("demo seed: inspect registered workspace name: %w", nameErr)
	}
	canonicalStoredRoot, err := canonicalWorkspaceRoot(byName.RootDir)
	if err != nil {
		return compozyworkspace.Workspace{}, err
	}
	if canonicalStoredRoot != canonicalRoot {
		return compozyworkspace.Workspace{}, fmt.Errorf(
			"demo seed: workspace name %q is already used at %s", name, byName.RootDir,
		)
	}
	return byName, nil
}

func canonicalWorkspaceRoot(root string) (string, error) {
	canonical, err := fileutil.CanonicalExistingDirectory(root)
	if err == nil {
		return canonical, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return filepath.Clean(root), nil
	}
	return "", fmt.Errorf("demo seed: canonicalize workspace root %q: %w", root, err)
}

func registerWorkspaces(ctx context.Context, db *globaldb.GlobalDB, state *scenario) error {
	for _, story := range state.workspaces {
		workspaceRoot := state.workspaceRoot(story)
		if err := os.MkdirAll(workspaceRoot, 0o755); err != nil {
			return fmt.Errorf("demo seed: create workspace root %q: %w", workspaceRoot, err)
		}
		workspaceRoot, err := canonicalWorkspaceRoot(workspaceRoot)
		if err != nil {
			return err
		}
		identity, err := compozyworkspace.EnsureIdentity(ctx, workspaceRoot)
		if err != nil {
			return fmt.Errorf("demo seed: ensure workspace identity for %q: %w", story.Name, err)
		}
		if err := db.InsertWorkspace(ctx, compozyworkspace.Workspace{
			ID: identity.WorkspaceID, RootDir: workspaceRoot, Name: story.Name,
			DefaultAgent: story.DefaultAgent,
			CreatedAt:    story.CreatedAt, UpdatedAt: story.UpdatedAt,
		}); err != nil {
			return fmt.Errorf("demo seed: register workspace %q: %w", story.Name, err)
		}
		state.records[story.Key] = workspaceRecord{
			ID: identity.WorkspaceID, Name: story.Name, RootDir: workspaceRoot,
		}
		if err := writeSeedMarker(workspaceRoot, story.Key); err != nil {
			return err
		}
	}
	return nil
}

func writeAgentDefinitions(state *scenario) error {
	for _, story := range scenarioAgents() {
		record, err := state.recordFor(story.WorkspaceKey)
		if err != nil {
			return err
		}
		path := filepath.Join(
			record.RootDir, config.DirName, config.AgentsDirName, story.Name, config.AgentDefinitionFileName,
		)
		if _, err := config.CreateAgentDefFile(path, config.AgentDefinitionDraft{
			Name: story.Name, Provider: story.Provider, Model: story.Model,
			Permissions: story.Permissions, Tools: story.Tools,
			CategoryPath: story.CategoryPath, Prompt: story.Prompt,
		}, state.replace); err != nil {
			return fmt.Errorf("demo seed: write agent %q: %w", story.Name, err)
		}
	}
	return nil
}

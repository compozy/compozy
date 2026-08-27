package demoseed

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/compozy/compozy/internal/network/participation"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb"
	"github.com/compozy/compozy/internal/store/sessiondb"
)

func seedSessions(
	ctx context.Context,
	db *globaldb.GlobalDB,
	state *scenario,
	stories []sessionStory,
) (int, error) {
	total := 0
	for _, story := range stories {
		written, err := seedSession(ctx, db, state, story)
		if err != nil {
			return 0, err
		}
		total += written
	}
	return total, nil
}

func seedSession(
	ctx context.Context,
	db *globaldb.GlobalDB,
	state *scenario,
	story sessionStory,
) (int, error) {
	record, err := state.recordFor(story.WorkspaceKey)
	if err != nil {
		return 0, err
	}
	sessionDir := filepath.Join(state.paths.SessionsDir, story.ID)
	if err := os.MkdirAll(sessionDir, 0o700); err != nil {
		return 0, fmt.Errorf("demo seed: create session directory %q: %w", sessionDir, err)
	}
	written, err := writeSessionTranscript(ctx, sessionDir, record.ID, story)
	if err != nil {
		return 0, err
	}
	permissions, err := sessionPermissions(story)
	if err != nil {
		return 0, err
	}
	if err := writeSessionRecords(ctx, db, sessionDir, record, story, permissions); err != nil {
		return 0, err
	}
	return written, nil
}

func writeSessionRecords(
	ctx context.Context,
	db *globaldb.GlobalDB,
	sessionDir string,
	record workspaceRecord,
	story sessionStory,
	permissions string,
) error {
	stopReason := store.StopReason(story.stopReason())
	networkSpec := participation.LocalSpec()
	lineage := sessionLineage(story)
	stopDetail := story.StopDetail
	meta := store.SessionMeta{
		ID: story.ID, Name: story.Name, AgentName: story.AgentName, Provider: story.Provider,
		ProfileID:   store.DefaultProfileID,
		Model:       story.Model,
		WorkspaceID: record.ID,
		SessionType: story.SessionType, State: string(session.StateStopped),
		RuntimeStatus: store.SessionRuntimeUnbound,
		StopReason:    &stopReason, StopDetail: stopDetail,
		Failure: sessionFailure(story),
		Lineage: lineage,
		SessionMetaPlacementState: store.NewSessionMetaPlacement(
			store.SessionScopeWorkspace,
			networkSpec,
		),
		CreatedAt: story.StartedAt, UpdatedAt: story.EndedAt,
	}
	meta.SetCWD(record.RootDir)
	meta.SetEffectivePermissions(permissions)
	if err := store.WriteSessionMeta(store.SessionMetaFile(sessionDir), meta); err != nil {
		return fmt.Errorf("demo seed: write metadata for session %q: %w", story.ID, err)
	}
	if err := db.RegisterSession(ctx, store.SessionInfo{
		ID: story.ID, ProfileID: store.DefaultProfileID,
		Name: story.Name, AgentName: story.AgentName, Provider: story.Provider,
		Model: story.Model, WorkspaceID: record.ID, SessionType: story.SessionType,
		Lineage:             lineage,
		SessionNetworkState: &store.SessionNetworkState{NetworkSpec: networkSpec},
		State:               string(session.StateStopped), RuntimeStatus: store.SessionRuntimeUnbound,
		StopReason: stopReason, StopDetail: stopDetail, Failure: sessionFailure(story),
		CreatedAt: story.StartedAt, UpdatedAt: story.EndedAt,
	}); err != nil {
		return fmt.Errorf("demo seed: register session %q: %w", story.ID, err)
	}
	return nil
}

func sessionPermissions(story sessionStory) (string, error) {
	for _, agent := range scenarioAgents() {
		if agent.WorkspaceKey == story.WorkspaceKey && agent.Name == story.AgentName {
			return agent.Permissions, nil
		}
	}
	return "", fmt.Errorf(
		"demo seed: agent %q is not defined in workspace %q",
		story.AgentName,
		story.WorkspaceKey,
	)
}

func sessionLineage(story sessionStory) *store.SessionLineage {
	if strings.TrimSpace(story.ParentID) == "" {
		return nil
	}
	return &store.SessionLineage{
		ParentSessionID: story.ParentID, RootSessionID: story.ParentID,
		SpawnDepth: 1, SpawnRole: story.SpawnRole, AutoStopOnParent: true,
	}
}

func sessionFailure(story sessionStory) *store.SessionFailure {
	if story.Failure == nil {
		return nil
	}
	return &store.SessionFailure{
		Kind: store.FailureKind(story.Failure.Kind), Summary: story.Failure.Summary,
	}
}

func writeSessionTranscript(
	ctx context.Context,
	sessionDir string,
	workspaceID string,
	story sessionStory,
) (written int, err error) {
	eventsDB, err := sessiondb.OpenSessionDB(ctx, store.SessionDBOwner{
		SessionID:   story.ID,
		WorkspaceID: workspaceID,
	}, store.SessionDBFile(sessionDir))
	if err != nil {
		return 0, fmt.Errorf("demo seed: open transcript for session %q: %w", story.ID, err)
	}
	defer func() {
		closeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), cleanupTimeout)
		defer cancel()
		if closeErr := eventsDB.Close(closeCtx); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("demo seed: close transcript for session %q: %w", story.ID, closeErr))
		}
	}()
	persisted, usage, err := buildTranscript(story, workspaceID)
	if err != nil {
		return 0, err
	}
	if _, err := eventsDB.RecordPersistedBatch(ctx, persisted); err != nil {
		return 0, fmt.Errorf("demo seed: write transcript for session %q: %w", story.ID, err)
	}
	if err := eventsDB.RecordTokenUsage(ctx, usage); err != nil {
		return 0, fmt.Errorf("demo seed: write transcript usage for session %q: %w", story.ID, err)
	}
	return len(persisted), nil
}

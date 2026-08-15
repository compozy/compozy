package session

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/fileutil"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/sessiondb"
)

// ClearConversation resets the persisted transcript and ACP conversation context
// for an existing session while preserving the same session id. The session is
// restarted on a fresh event store so subsequent prompts start from a clean
// conversation. Attachments are retained because they remain scoped to the
// unchanged session identity and may still be referenced outside the cleared epoch.
func (m *Manager) ClearConversation(ctx context.Context, id string) (_ *Session, err error) {
	if m == nil {
		return nil, errors.New("session: manager is required")
	}
	if ctx == nil {
		return nil, errors.New("session: clear conversation context is required")
	}
	if err := m.checkNewWorkAdmission(ctx); err != nil {
		return nil, err
	}

	target := strings.TrimSpace(id)
	if target == "" {
		return nil, errors.New("session: session id is required")
	}
	if m.isPending(target) {
		return nil, fmt.Errorf("%w: %s", ErrSessionNotActive, target)
	}
	ctx, unlockConversation, err := m.lockConversationOperation(ctx, target)
	if err != nil {
		return nil, err
	}
	defer unlockConversation()
	if m.isPending(target) {
		return nil, fmt.Errorf("%w: %s", ErrSessionNotActive, target)
	}

	if stopErr := m.stopActiveSessionForClear(ctx, target); stopErr != nil {
		return nil, stopErr
	}

	meta, err := m.readMetaWithContext(ctx, target)
	if err != nil {
		return nil, err
	}
	owner, err := m.resolveStoredSessionOwner(ctx, target, meta.WorkspaceID)
	if err != nil {
		return nil, fmt.Errorf("session: resolve catalog owner for clear %q: %w", target, err)
	}

	sanitized := clearedConversationMeta(meta, m.now())
	spec, err := m.prepareResumeStart(ctx, sanitized)
	if err != nil {
		return nil, err
	}
	spec.clearEventStoreOnOpen = true
	return m.clearStoppedConversation(ctx, target, owner, meta, sanitized, &spec)
}

func (m *Manager) restartClearedConversation(
	ctx context.Context,
	spec *sessionStartSpec,
	target string,
	dbPath string,
	clearGeneration string,
) (_ *Session, err error) {
	session, err := m.startSession(ctx, spec)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err == nil {
			return
		}
		if rollbackErr := m.rollbackClearedRestart(session, target); rollbackErr != nil {
			err = errors.Join(err, fmt.Errorf("session: rollback failed clear restart for %q: %w", target, rollbackErr))
		}
	}()
	targetEpoch, err := m.nextTranscriptEpoch(ctx, session, target)
	if err != nil {
		return nil, err
	}
	if commitErr := commitSessionDBClear(dbPath, clearGeneration, targetEpoch); commitErr != nil {
		return nil, fmt.Errorf("session: commit cleared event store for %q: %w", target, commitErr)
	}
	if _, err := m.ensureTranscriptEpoch(ctx, session, target, targetEpoch); err != nil {
		return nil, err
	}
	m.clearLogger().Info(
		"session.clear.restart_complete",
		"session_id",
		target,
		"db_path",
		dbPath,
	)

	return session, nil
}

func (m *Manager) rollbackClearedRestart(session *Session, target string) error {
	if session == nil {
		return nil
	}
	proc := session.processHandle()
	recorder := session.recorderHandle()
	err := m.rollbackActivation(session, proc, m.now())
	if recorder == nil {
		return err
	}
	closeCtx, cancel := m.lifecycleCleanupContext()
	defer cancel()
	if closeErr := recorder.Close(closeCtx); closeErr != nil {
		err = errors.Join(err, fmt.Errorf("session: close failed clear recorder for %q: %w", target, closeErr))
	}
	return err
}

func (m *Manager) stopActiveSessionForClear(ctx context.Context, target string) error {
	active, activeBefore := m.Get(target)
	if !activeBefore {
		return nil
	}
	if active.IsPrompting() {
		return fmt.Errorf("%w: %s", ErrPromptInProgress, target)
	}
	return m.StopWithCause(ctx, target, CauseClearConversation, "conversation cleared")
}

func (m *Manager) backupSessionDBForClear(
	ctx context.Context,
	lease *sessiondb.FamilyLease,
	owner store.SessionDBOwner,
	dbPath string,
	target string,
) (sessionDBClearManifest, error) {
	if err := m.recoverSessionDBClear(ctx, lease, owner, dbPath); err != nil {
		return sessionDBClearManifest{}, fmt.Errorf("session: recover previous clear state before backup: %w", err)
	}
	manifest, err := createSessionDBClearManifest(ctx, lease, owner, dbPath)
	if err != nil {
		return sessionDBClearManifest{}, err
	}
	if err := backupSessionDBManifestArtifacts(ctx, lease, owner, dbPath, manifest); err != nil {
		return sessionDBClearManifest{}, err
	}
	m.clearLogger().Info(
		"session.clear.backup_complete",
		"session_id",
		target,
		"backup_count",
		len(manifest.Artifacts),
		"db_path",
		dbPath,
	)
	return manifest, nil
}

func (m *Manager) clearLogger() *slog.Logger {
	if m != nil && m.logger != nil {
		return m.logger
	}
	return slog.Default()
}

func clearedConversationMeta(meta store.SessionMeta, now time.Time) store.SessionMeta {
	cleared := meta
	cleared.State = string(StateStopped)
	cleared.StopReason = nil
	cleared.StopDetail = ""
	cleared.ACPSessionID = nil
	cleared.UpdatedAt = now
	return cleared
}

type sessionDBClearCommit struct {
	Generation      string `json:"generation"`
	TranscriptEpoch int64  `json:"transcript_epoch"`
}

func commitSessionDBClear(path string, generation string, transcriptEpoch int64) error {
	cleanPath := strings.TrimSpace(path)
	if cleanPath == "" {
		return errors.New("session: session database path is required")
	}
	if strings.TrimSpace(generation) == "" {
		return errors.New("session: clear commit generation is required")
	}
	if transcriptEpoch <= 0 {
		return errors.New("session: clear commit transcript epoch must be positive")
	}
	manifest, present, err := readSessionDBClearManifest(cleanPath)
	if err != nil {
		return err
	}
	if !present {
		return errors.New("session: clear commit manifest is missing")
	}
	if manifest.Generation != generation {
		return fmt.Errorf(
			"%w: clear commit generation %q does not match manifest %q",
			sessiondb.ErrSessionDBFamilyChanged,
			generation,
			manifest.Generation,
		)
	}
	commit := sessionDBClearCommit{Generation: generation, TranscriptEpoch: transcriptEpoch}
	if err := writeSessionDBClearCommit(cleanPath, commit); err != nil {
		return err
	}
	manifest.Phase = sessionDBClearPhaseCommitted
	manifest.TranscriptEpoch = transcriptEpoch
	if err := writeSessionDBClearManifest(cleanPath, manifest); err != nil {
		return fmt.Errorf("session: persist committed clear manifest: %w", err)
	}
	return nil
}

func writeSessionDBClearCommit(path string, commit sessionDBClearCommit) error {
	if strings.TrimSpace(commit.Generation) == "" {
		return errors.New("session: clear commit generation is required")
	}
	if commit.TranscriptEpoch <= 0 {
		return errors.New("session: clear commit transcript epoch must be positive")
	}
	payload, err := json.Marshal(commit)
	if err != nil {
		return fmt.Errorf("session: marshal clear commit marker: %w", err)
	}
	payload = append(payload, '\n')
	if err := fileutil.AtomicWriteFile(sessionDBClearCommitPath(path), payload, 0o600); err != nil {
		return fmt.Errorf("session: write clear commit marker: %w", err)
	}
	return nil
}

func resolveSessionDBClearCommit(
	manifest sessionDBClearManifest,
	marker sessionDBClearCommit,
	markerPresent bool,
) (sessionDBClearCommit, bool, error) {
	if markerPresent && marker.Generation != manifest.Generation {
		return sessionDBClearCommit{}, false, fmt.Errorf(
			"%w: clear commit generation %q does not match manifest %q",
			sessiondb.ErrSessionDBFamilyChanged,
			marker.Generation,
			manifest.Generation,
		)
	}
	if manifest.Phase == sessionDBClearPhaseCommitted {
		committed := sessionDBClearCommit{
			Generation:      manifest.Generation,
			TranscriptEpoch: manifest.TranscriptEpoch,
		}
		if markerPresent && marker.TranscriptEpoch != committed.TranscriptEpoch {
			return sessionDBClearCommit{}, false, fmt.Errorf(
				"%w: clear commit transcript epoch %d does not match manifest %d",
				sessiondb.ErrSessionDBFamilyChanged,
				marker.TranscriptEpoch,
				committed.TranscriptEpoch,
			)
		}
		return committed, true, nil
	}
	if markerPresent {
		return marker, true, nil
	}
	return sessionDBClearCommit{}, false, nil
}

func readSessionDBClearCommit(path string) (sessionDBClearCommit, bool, error) {
	marker := sessionDBClearCommitPath(path)
	payload, _, err := fileutil.ReadRegularFile(marker)
	if errors.Is(err, os.ErrNotExist) {
		return sessionDBClearCommit{}, false, nil
	}
	if err != nil {
		return sessionDBClearCommit{}, false, fmt.Errorf("session: read clear commit marker %q: %w", marker, err)
	}
	var commit sessionDBClearCommit
	if err := json.Unmarshal(payload, &commit); err != nil {
		return sessionDBClearCommit{}, false, fmt.Errorf("session: decode clear commit marker %q: %w", marker, err)
	}
	if strings.TrimSpace(commit.Generation) == "" || commit.TranscriptEpoch <= 0 {
		return sessionDBClearCommit{}, false, errors.New("session: clear commit marker is incomplete")
	}
	return commit, true, nil
}

func discardSessionDBClearCommit(path string) error {
	marker := sessionDBClearCommitPath(path)
	if err := fileutil.AtomicRemoveFile(marker); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("session: remove clear commit marker %q: %w", marker, err)
	}
	return nil
}

func sessionDBClearCommitPath(path string) string {
	return strings.TrimSpace(path) + ".clear-committed"
}

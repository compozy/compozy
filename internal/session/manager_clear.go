package session

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/compozy/agh/internal/fileutil"
	"github.com/compozy/agh/internal/store"
)

type sessionDBBackup struct {
	original string
	backup   string
}

// ClearConversation resets the persisted transcript and ACP conversation context
// for an existing session while preserving the same session id. The session is
// restarted on a fresh event store so subsequent prompts start from a clean
// conversation.
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

	if stopErr := m.stopActiveSessionForClear(ctx, target); stopErr != nil {
		return nil, stopErr
	}

	meta, err := m.readMetaWithContext(ctx, target)
	if err != nil {
		return nil, err
	}

	sanitized := clearedConversationMeta(meta, m.now())
	spec, err := m.prepareResumeStart(ctx, sanitized)
	if err != nil {
		return nil, err
	}
	spec.clearEventStoreOnOpen = true

	dbPath := store.SessionDBFile(filepath.Join(m.homePaths.SessionsDir, target))
	metaPath := store.SessionMetaFile(filepath.Join(m.homePaths.SessionsDir, target))
	backups, err := m.backupSessionDBForClear(ctx, dbPath, target)
	if err != nil {
		return nil, err
	}

	defer func() {
		if err == nil {
			if cleanupErr := m.finalizeCommittedSessionDBClear(dbPath, backups); cleanupErr != nil {
				err = cleanupErr
			}
			return
		}
		if restoreErr := restoreClearedConversationFailure(backups, dbPath, metaPath, meta); restoreErr != nil {
			err = errors.Join(err, restoreErr)
		}
	}()

	if discardErr := m.discardMaterializedSessionLedger(ctx, meta, dbPath); discardErr != nil {
		return nil, discardErr
	}
	if writeErr := store.WriteSessionMeta(metaPath, sanitized); writeErr != nil {
		return nil, fmt.Errorf("session: persist cleared metadata for %q: %w", target, writeErr)
	}

	return m.restartClearedConversation(ctx, &spec, target, dbPath)
}

func (m *Manager) restartClearedConversation(
	ctx context.Context,
	spec *sessionStartSpec,
	target string,
	dbPath string,
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
	if commitErr := commitSessionDBClear(dbPath, targetEpoch); commitErr != nil {
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

func (m *Manager) finalizeCommittedSessionDBClear(dbPath string, backups []sessionDBBackup) error {
	if err := discardSessionDBBackup(backups); err != nil {
		return err
	}
	if err := discardSessionDBClearCommit(dbPath); err != nil {
		m.clearLogger().Warn(
			"session.clear.commit_marker_cleanup_failed",
			"db_path",
			dbPath,
			"error",
			err,
		)
	}
	return nil
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
	closeCtx, cancel := context.WithTimeout(context.Background(), defaultLifecycleTimeout)
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
	dbPath string,
	target string,
) ([]sessionDBBackup, error) {
	if err := m.recoverSessionDBClear(ctx, target, dbPath); err != nil {
		return nil, fmt.Errorf("session: recover previous clear state before backup: %w", err)
	}
	backups, err := backupSessionDBAfterRecovery(dbPath)
	if err != nil {
		return nil, err
	}
	m.clearLogger().Info(
		"session.clear.backup_complete",
		"session_id",
		target,
		"backup_count",
		len(backups),
		"db_path",
		dbPath,
	)
	return backups, nil
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

func backupSessionDB(path string) ([]sessionDBBackup, error) {
	cleanPath := strings.TrimSpace(path)
	if cleanPath == "" {
		return nil, errors.New("session: session database path is required")
	}
	if err := recoverSessionDBClear(cleanPath); err != nil {
		return nil, fmt.Errorf("session: recover previous clear state before backup: %w", err)
	}
	return backupSessionDBAfterRecovery(cleanPath)
}

func backupSessionDBAfterRecovery(path string) ([]sessionDBBackup, error) {
	cleanPath := strings.TrimSpace(path)
	if cleanPath == "" {
		return nil, errors.New("session: session database path is required")
	}
	paths := []string{cleanPath, cleanPath + "-wal", cleanPath + "-shm"}
	backups := make([]sessionDBBackup, 0, len(paths))

	for _, original := range paths {
		if _, err := os.Stat(original); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return nil, rollbackSessionDBBackup(
				backups,
				fmt.Errorf("session: stat event store artifact %q: %w", original, err),
			)
		}

		backup := original + ".clear-backup"
		if err := removeSessionDBArtifact(backup); err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, rollbackSessionDBBackup(
				backups,
				fmt.Errorf("session: remove stale clear backup %q: %w", backup, err),
			)
		}
		if err := os.Rename(original, backup); err != nil {
			return nil, rollbackSessionDBBackup(
				backups,
				fmt.Errorf("session: backup event store artifact %q: %w", original, err),
			)
		}
		backups = append(backups, sessionDBBackup{original: original, backup: backup})
		if err := syncSessionDBDir(original); err != nil {
			return nil, rollbackSessionDBBackup(
				backups,
				fmt.Errorf("session: sync clear backup directory for %q: %w", original, err),
			)
		}
	}

	return backups, nil
}

func rollbackSessionDBBackup(backups []sessionDBBackup, primary error) error {
	if len(backups) == 0 {
		return primary
	}
	if rollbackErr := restoreSessionDBArtifacts(backups); rollbackErr != nil {
		return errors.Join(primary, fmt.Errorf("session: rollback clear backup: %w", rollbackErr))
	}
	return primary
}

func discardSessionDBBackup(backups []sessionDBBackup) error {
	var errs []error
	for _, item := range backups {
		target := strings.TrimSpace(item.backup)
		if target == "" {
			continue
		}
		if err := removeSessionDBArtifact(target); err != nil && !errors.Is(err, os.ErrNotExist) {
			errs = append(errs, fmt.Errorf("session: remove clear backup %q: %w", target, err))
		}
	}
	return errors.Join(errs...)
}

func restoreClearedConversationFailure(
	backups []sessionDBBackup,
	dbPath string,
	metaPath string,
	meta store.SessionMeta,
) error {
	var errs []error

	errs = append(errs, restoreSessionDBArtifacts(backups))
	if err := discardSessionDBClearCommit(dbPath); err != nil {
		errs = append(errs, err)
	}

	if err := store.WriteSessionMeta(metaPath, meta); err != nil {
		errs = append(errs, fmt.Errorf("session: restore cleared metadata for %q: %w", meta.ID, err))
	}

	return errors.Join(errs...)
}

func restoreSessionDBArtifacts(backups []sessionDBBackup) error {
	var errs []error

	for _, item := range slices.Backward(backups) {
		if err := removeSessionDBArtifact(item.original); err != nil && !errors.Is(err, os.ErrNotExist) {
			errs = append(errs, fmt.Errorf("session: remove failed clear artifact %q: %w", item.original, err))
		}
		if err := os.Rename(item.backup, item.original); err != nil {
			errs = append(errs, fmt.Errorf("session: restore cleared artifact %q: %w", item.original, err))
			continue
		}
		if err := syncSessionDBDir(item.original); err != nil {
			errs = append(errs, fmt.Errorf("session: sync restored clear artifact %q: %w", item.original, err))
		}
	}

	return errors.Join(errs...)
}

func recoverSessionDBClear(path string) error {
	cleanPath := strings.TrimSpace(path)
	if cleanPath == "" {
		return errors.New("session: session database path is required")
	}

	backups, err := existingSessionDBClearBackups(cleanPath)
	if err != nil {
		return err
	}
	committed, err := hasSessionDBClearCommit(cleanPath)
	if err != nil {
		return err
	}
	if len(backups) == 0 {
		if committed {
			return discardSessionDBClearCommit(cleanPath)
		}
		return nil
	}
	if committed {
		if err := discardSessionDBBackup(backups); err != nil {
			return fmt.Errorf("session: discard committed clear backups: %w", err)
		}
		return discardSessionDBClearCommit(cleanPath)
	}
	if err := restoreSessionDBArtifacts(backups); err != nil {
		return fmt.Errorf("session: restore interrupted clear backups: %w", err)
	}
	return discardSessionDBClearCommit(cleanPath)
}

func (m *Manager) recoverSessionDBClear(ctx context.Context, sessionID string, path string) error {
	cleanPath := strings.TrimSpace(path)
	if cleanPath == "" {
		return errors.New("session: session database path is required")
	}
	marker, committed, err := readSessionDBClearCommit(cleanPath)
	if err != nil {
		return err
	}
	if committed && marker.TranscriptEpoch > 0 && m != nil && m.transcriptEpochStore != nil {
		if _, ensureErr := m.transcriptEpochStore.EnsureSessionTranscriptEpoch(ctx, store.SessionTranscriptEpochUpdate{
			SessionID: sessionID,
			Minimum:   marker.TranscriptEpoch,
		}); ensureErr != nil {
			return fmt.Errorf("session: reconcile transcript epoch for %q: %w", sessionID, ensureErr)
		}
	}
	return recoverSessionDBClear(cleanPath)
}

func existingSessionDBClearBackups(path string) ([]sessionDBBackup, error) {
	cleanPath := strings.TrimSpace(path)
	if cleanPath == "" {
		return nil, errors.New("session: session database path is required")
	}

	paths := []string{cleanPath, cleanPath + "-wal", cleanPath + "-shm"}
	backups := make([]sessionDBBackup, 0, len(paths))
	for _, original := range paths {
		backup := original + ".clear-backup"
		info, err := os.Lstat(backup)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return nil, fmt.Errorf("session: stat clear backup %q: %w", backup, err)
		}
		if !info.Mode().IsRegular() {
			continue
		}
		backups = append(backups, sessionDBBackup{original: original, backup: backup})
	}
	return backups, nil
}

type sessionDBClearCommit struct {
	TranscriptEpoch int64 `json:"transcript_epoch"`
}

func commitSessionDBClear(path string, transcriptEpoch int64) error {
	cleanPath := strings.TrimSpace(path)
	if cleanPath == "" {
		return errors.New("session: session database path is required")
	}
	payload, err := json.Marshal(sessionDBClearCommit{TranscriptEpoch: transcriptEpoch})
	if err != nil {
		return fmt.Errorf("session: marshal clear commit marker: %w", err)
	}
	payload = append(payload, '\n')
	if err := fileutil.AtomicWriteFile(sessionDBClearCommitPath(cleanPath), payload, 0o600); err != nil {
		return fmt.Errorf("session: write clear commit marker: %w", err)
	}
	return nil
}

func hasSessionDBClearCommit(path string) (bool, error) {
	_, committed, err := readSessionDBClearCommit(path)
	return committed, err
}

func readSessionDBClearCommit(path string) (sessionDBClearCommit, bool, error) {
	marker := sessionDBClearCommitPath(path)
	payload, err := os.ReadFile(marker)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return sessionDBClearCommit{}, false, nil
		}
		return sessionDBClearCommit{}, false, fmt.Errorf("session: read clear commit marker %q: %w", marker, err)
	}
	var commit sessionDBClearCommit
	if err := json.Unmarshal(payload, &commit); err != nil {
		return sessionDBClearCommit{}, false, fmt.Errorf("session: decode clear commit marker %q: %w", marker, err)
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

func removeSessionDBArtifact(path string) error {
	cleanPath := strings.TrimSpace(path)
	if cleanPath == "" {
		return errors.New("session: session database artifact path is required")
	}
	if err := fileutil.AtomicRemoveFile(cleanPath); err != nil {
		return err
	}
	return nil
}

func syncSessionDBDir(path string) error {
	return fileutil.SyncDir(filepath.Dir(path))
}

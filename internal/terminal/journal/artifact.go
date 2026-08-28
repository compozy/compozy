package journal

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/redact"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/workspacedb"
	terminalpkg "github.com/compozy/compozy/internal/terminal"
)

const (
	privateDirMode  = 0o700
	privateFileMode = 0o600
)

// WriteArtifact redacts and retains one content-addressed spill artifact.
func (s *Service) WriteArtifact(
	ctx context.Context,
	workspaceID, profileID, commandID string,
	terminalID *terminalpkg.ID,
	contents []byte,
	expiresAt time.Time,
) (terminalpkg.SpillRef, error) {
	s.artifactMu.Lock()
	defer s.artifactMu.Unlock()
	id, err := s.newArtifactID()
	if err != nil {
		return terminalpkg.SpillRef{}, err
	}
	path, digestText, retainedBytes, created, err := s.writeRedactedRetainedFile(
		workspaceID,
		"terminal-artifacts",
		".bin",
		contents,
	)
	if err != nil {
		return terminalpkg.SpillRef{}, err
	}
	db, err := s.databases.Open(ctx, workspaceID)
	if err != nil {
		cleanupErr := removeCreatedFile(path, created)
		return terminalpkg.SpillRef{}, errors.Join(
			fmt.Errorf("terminal journal: open workspace store: %w", err), cleanupErr,
		)
	}
	insert := func(artifactID string) error {
		return db.InsertTerminalArtifact(ctx, workspacedb.TerminalArtifactWrite{
			ID: artifactID, TerminalID: terminalIDString(terminalID), CommandID: commandID,
			ProfileID: profileID, Digest: digestText, Path: path,
			Bytes: retainedBytes, ExpiresAt: expiresAt.UnixMilli(),
		})
	}
	err = insert(id)
	if store.IsSQLiteIdentityConstraint(err) {
		id, err = s.newArtifactID()
		if err == nil {
			err = insert(id)
		}
	}
	if err != nil {
		return terminalpkg.SpillRef{}, errors.Join(
			fmt.Errorf("terminal journal: insert artifact %q: %w", id, err),
			removeCreatedFile(path, created),
		)
	}
	return terminalpkg.SpillRef{
		ArtifactID: id, Path: path, ProfileID: profileID, Bytes: retainedBytes,
	}, nil
}

// Artifact opens a profile-scoped spill artifact after containment validation.
func (s *Service) Artifact(
	ctx context.Context,
	workspaceID string,
	scope store.ReadScope,
	id string,
) (io.ReadCloser, error) {
	if err := scope.Validate(); err != nil {
		return nil, os.ErrNotExist
	}
	db, err := s.databases.Open(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("terminal journal: open workspace store: %w", err)
	}
	artifact, err := db.TerminalArtifact(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("terminal journal: get artifact %q: %w", id, err)
	}
	if !scope.Matches(artifact.ProfileID) {
		return nil, os.ErrNotExist
	}
	return s.openContained(workspaceID, "terminal-artifacts", artifact.Path)
}

// Recording opens a profile-scoped recording and its asciicast artifact.
func (s *Service) Recording(
	ctx context.Context,
	workspaceID string,
	scope store.ReadScope,
	id string,
) (*terminalpkg.RecordingRef, io.ReadCloser, error) {
	if err := scope.Validate(); err != nil {
		return nil, nil, os.ErrNotExist
	}
	db, err := s.databases.Open(ctx, workspaceID)
	if err != nil {
		return nil, nil, fmt.Errorf("terminal journal: open workspace store: %w", err)
	}
	recording, err := db.TerminalRecording(ctx, id)
	if err != nil {
		return nil, nil, fmt.Errorf("terminal journal: get recording %q: %w", id, err)
	}
	if !scope.Matches(recording.ProfileID) {
		return nil, nil, os.ErrNotExist
	}
	reader, err := s.openContained(workspaceID, "terminal-recordings", recording.Path)
	if err != nil {
		return nil, nil, err
	}
	ref := &terminalpkg.RecordingRef{
		ID: recording.ID, TerminalID: terminalpkg.ID(recording.TerminalID),
		ProfileID: recording.ProfileID, Digest: recording.Digest, Path: recording.Path,
		StartedAt: time.UnixMilli(recording.StartedAt).UTC(),
		StoppedAt: millisTimePointer(recording.StoppedAt), Bytes: recording.Bytes,
		ExpiresAt: time.UnixMilli(recording.ExpiresAt).UTC(),
	}
	return ref, reader, nil
}

func (s *Service) writeRecordingFile(
	workspaceID, id string,
	contents []byte,
) (string, string, int64, bool, error) {
	path, digest, retainedBytes, created, err := s.writeRedactedRetainedFile(
		workspaceID,
		"terminal-recordings",
		".cast",
		contents,
	)
	if err != nil {
		return "", "", 0, false, fmt.Errorf("terminal journal: write recording %q: %w", id, err)
	}
	return path, digest, retainedBytes, created, nil
}

func (s *Service) writeRedactedRetainedFile(
	workspaceID, kind, extension string,
	contents []byte,
) (string, string, int64, bool, error) {
	redacted := []byte(redact.String(string(contents)))
	digest := sha256.Sum256(redacted)
	digestText := hex.EncodeToString(digest[:])
	path, created, err := s.writeContained(workspaceID, kind, digestText+extension, redacted)
	if err != nil {
		return "", "", 0, false, err
	}
	return path, digestText, int64(len(redacted)), created, nil
}

// SweepExpired removes expired rows and their retained files.
func (s *Service) SweepExpired(ctx context.Context, workspaceID string, at time.Time) error {
	s.artifactMu.Lock()
	defer s.artifactMu.Unlock()
	db, err := s.databases.Open(ctx, workspaceID)
	if err != nil {
		return fmt.Errorf("terminal journal: open workspace store: %w", err)
	}
	if err := db.SweepExpiredTerminalFiles(ctx, at.UnixMilli(), removeRetainedFile); err != nil {
		return fmt.Errorf("terminal journal: sweep expired files: %w", err)
	}
	return nil
}

func (s *Service) writeContained(
	workspaceID, kind, name string,
	contents []byte,
) (string, bool, error) {
	root := filepath.Join(s.homeDir, "workspaces", workspaceID, kind)
	if err := os.MkdirAll(root, privateDirMode); err != nil {
		return "", false, fmt.Errorf("terminal journal: create artifact root: %w", err)
	}
	if err := os.Chmod(root, privateDirMode); err != nil {
		return "", false, fmt.Errorf("terminal journal: secure artifact root: %w", err)
	}
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", false, fmt.Errorf("terminal journal: resolve artifact root: %w", err)
	}
	path := filepath.Join(realRoot, filepath.Base(name))
	if err := requireContained(realRoot, path); err != nil {
		return "", false, err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, privateFileMode)
	if errors.Is(err, os.ErrExist) {
		return path, false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("terminal journal: create artifact: %w", err)
	}
	writeErr := writeAndClose(file, contents)
	if writeErr != nil {
		removeErr := os.Remove(path)
		if errors.Is(removeErr, os.ErrNotExist) {
			removeErr = nil
		}
		return "", false, errors.Join(writeErr, removeErr)
	}
	return path, true, nil
}

func removeRetainedFile(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("terminal journal: remove retained file %q: %w", path, err)
	}
	return nil
}

func removeCreatedFile(path string, created bool) error {
	if !created {
		return nil
	}
	return removeRetainedFile(path)
}

func (s *Service) removeWorkspaceFiles(workspaceID string) error {
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" || filepath.Base(workspaceID) != workspaceID {
		return errors.New("terminal journal: canonical workspace id is required for removal")
	}
	root := filepath.Join(s.homeDir, "workspaces", workspaceID)
	var errs []error
	for _, kind := range []string{"terminal-artifacts", "terminal-recordings"} {
		path := filepath.Join(root, kind)
		if err := os.RemoveAll(path); err != nil {
			errs = append(errs, fmt.Errorf("terminal journal: remove retained workspace files %q: %w", path, err))
		}
	}
	return errors.Join(errs...)
}

func (s *Service) openContained(workspaceID, kind, storedPath string) (io.ReadCloser, error) {
	root := filepath.Join(s.homeDir, "workspaces", workspaceID, kind)
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil, fmt.Errorf("terminal journal: resolve artifact root: %w", err)
	}
	realPath, err := filepath.EvalSymlinks(storedPath)
	if err != nil {
		return nil, fmt.Errorf("terminal journal: resolve artifact path: %w", err)
	}
	if err := requireContained(realRoot, realPath); err != nil {
		return nil, err
	}
	file, err := os.Open(realPath)
	if err != nil {
		return nil, fmt.Errorf("terminal journal: open artifact: %w", err)
	}
	return file, nil
}

func requireContained(root, path string) error {
	relative, err := filepath.Rel(root, path)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return errors.New("terminal journal: artifact path escapes workspace root")
	}
	return nil
}

func writeAndClose(file *os.File, contents []byte) error {
	_, writeErr := file.Write(contents)
	syncErr := file.Sync()
	closeErr := file.Close()
	return errors.Join(writeErr, syncErr, closeErr)
}

func millisTimePointer(value *int64) *time.Time {
	if value == nil {
		return nil
	}
	result := time.UnixMilli(*value).UTC()
	return &result
}

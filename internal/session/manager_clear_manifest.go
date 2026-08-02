package session

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/compozy/compozy/internal/fileutil"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/sessiondb"
)

const sessionDBClearManifestVersion = 2

type sessionDBClearPhase string

const (
	sessionDBClearPhasePrepared  sessionDBClearPhase = "prepared"
	sessionDBClearPhaseCommitted sessionDBClearPhase = "committed"
)

var sessionDBClearArtifactSuffixes = [...]string{"", "-wal", "-shm", "-journal"}

type sessionDBClearManifest struct {
	Version         int                      `json:"version"`
	Generation      string                   `json:"generation"`
	Phase           sessionDBClearPhase      `json:"phase"`
	TranscriptEpoch int64                    `json:"transcript_epoch"`
	Owner           sessionDBClearOwner      `json:"owner"`
	Artifacts       []sessionDBClearArtifact `json:"artifacts"`
}

type sessionDBClearOwner struct {
	SessionID   string `json:"session_id"`
	WorkspaceID string `json:"workspace_id"`
}

type sessionDBClearArtifact struct {
	Suffix string `json:"suffix"`
	SHA256 string `json:"sha256"`
}

func createSessionDBClearManifest(
	ctx context.Context,
	lease *sessiondb.FamilyLease,
	owner store.SessionDBOwner,
	dbPath string,
) (sessionDBClearManifest, error) {
	normalizedOwner, err := owner.Normalize()
	if err != nil {
		return sessionDBClearManifest{}, err
	}
	generation, err := store.NewID("clear")
	if err != nil {
		return sessionDBClearManifest{}, fmt.Errorf("session: reserve clear generation: %w", err)
	}
	manifest := sessionDBClearManifest{
		Version:    sessionDBClearManifestVersion,
		Generation: generation,
		Phase:      sessionDBClearPhasePrepared,
		Owner: sessionDBClearOwner{
			SessionID:   normalizedOwner.SessionID,
			WorkspaceID: normalizedOwner.WorkspaceID,
		},
		Artifacts: make([]sessionDBClearArtifact, 0, len(sessionDBClearArtifactSuffixes)),
	}
	for _, suffix := range sessionDBClearArtifactSuffixes {
		path := dbPath + suffix
		if _, err := os.Lstat(path); errors.Is(err, os.ErrNotExist) {
			if suffix == "" {
				return sessionDBClearManifest{}, sessiondb.ErrSessionDBOwnerMissing
			}
			continue
		} else if err != nil {
			return sessionDBClearManifest{}, fmt.Errorf("session: inspect clear artifact %q: %w", path, err)
		}
		bound, err := bindSessionDBClearArtifact(ctx, lease, normalizedOwner, path, suffix)
		if err != nil {
			return sessionDBClearManifest{}, err
		}
		digest, digestErr := bound.SHA256()
		closeErr := bound.Close()
		if digestErr != nil || closeErr != nil {
			return sessionDBClearManifest{}, errors.Join(digestErr, closeErr)
		}
		manifest.Artifacts = append(manifest.Artifacts, sessionDBClearArtifact{
			Suffix: suffix,
			SHA256: hex.EncodeToString(digest[:]),
		})
	}
	if err := writeSessionDBClearManifest(dbPath, manifest); err != nil {
		return sessionDBClearManifest{}, err
	}
	return manifest, nil
}

func writeSessionDBClearManifest(path string, manifest sessionDBClearManifest) error {
	if err := validateSessionDBClearManifest(manifest); err != nil {
		return err
	}
	payload, err := json.Marshal(manifest)
	if err != nil {
		return fmt.Errorf("session: marshal clear manifest: %w", err)
	}
	payload = append(payload, '\n')
	if err := fileutil.AtomicWriteFile(sessionDBClearManifestPath(path), payload, 0o600); err != nil {
		return fmt.Errorf("session: write clear manifest: %w", err)
	}
	return nil
}

func readSessionDBClearManifest(path string) (sessionDBClearManifest, bool, error) {
	payload, _, err := fileutil.ReadRegularFile(sessionDBClearManifestPath(path))
	if errors.Is(err, os.ErrNotExist) {
		return sessionDBClearManifest{}, false, nil
	}
	if err != nil {
		return sessionDBClearManifest{}, false, fmt.Errorf("session: read clear manifest: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	var manifest sessionDBClearManifest
	if err := decoder.Decode(&manifest); err != nil {
		return sessionDBClearManifest{}, false, fmt.Errorf("session: decode clear manifest: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return sessionDBClearManifest{}, false, errors.New("session: decode clear manifest: trailing JSON value")
	}
	if err := validateSessionDBClearManifest(manifest); err != nil {
		return sessionDBClearManifest{}, false, err
	}
	return manifest, true, nil
}

func validateSessionDBClearManifest(manifest sessionDBClearManifest) error {
	if manifest.Version != sessionDBClearManifestVersion {
		return fmt.Errorf(
			"session: clear manifest version = %d, want %d",
			manifest.Version,
			sessionDBClearManifestVersion,
		)
	}
	if strings.TrimSpace(manifest.Generation) == "" {
		return errors.New("session: clear manifest generation is required")
	}
	switch manifest.Phase {
	case sessionDBClearPhasePrepared:
		if manifest.TranscriptEpoch != 0 {
			return errors.New("session: prepared clear manifest cannot have a transcript epoch")
		}
	case sessionDBClearPhaseCommitted:
		if manifest.TranscriptEpoch <= 0 {
			return errors.New("session: committed clear manifest transcript epoch must be positive")
		}
	default:
		return fmt.Errorf("session: invalid clear manifest phase %q", manifest.Phase)
	}
	if _, err := manifest.owner().Normalize(); err != nil {
		return fmt.Errorf("session: validate clear manifest owner: %w", err)
	}
	seen := make(map[string]struct{}, len(manifest.Artifacts))
	for _, artifact := range manifest.Artifacts {
		if !isSessionDBClearArtifactSuffix(artifact.Suffix) {
			return fmt.Errorf("session: unsupported clear artifact suffix %q", artifact.Suffix)
		}
		if _, exists := seen[artifact.Suffix]; exists {
			return fmt.Errorf("session: duplicate clear artifact suffix %q", artifact.Suffix)
		}
		seen[artifact.Suffix] = struct{}{}
		digest, err := hex.DecodeString(artifact.SHA256)
		if err != nil || len(digest) != sha256.Size {
			return fmt.Errorf("session: invalid clear artifact digest for suffix %q", artifact.Suffix)
		}
	}
	if _, exists := seen[""]; !exists {
		return errors.New("session: clear manifest is missing the main database")
	}
	return nil
}

func (m sessionDBClearManifest) owner() store.SessionDBOwner {
	return store.SessionDBOwner{SessionID: m.Owner.SessionID, WorkspaceID: m.Owner.WorkspaceID}
}

func (m sessionDBClearManifest) requireOwner(owner store.SessionDBOwner) error {
	normalized, err := owner.Normalize()
	if err != nil {
		return err
	}
	if m.owner() != normalized {
		return fmt.Errorf(
			"%w: clear manifest expected session %q workspace %q, found session %q workspace %q",
			sessiondb.ErrSessionDBOwnerMismatch,
			normalized.SessionID,
			normalized.WorkspaceID,
			m.Owner.SessionID,
			m.Owner.WorkspaceID,
		)
	}
	return nil
}

func (m sessionDBClearManifest) hasArtifact(suffix string) bool {
	for _, artifact := range m.Artifacts {
		if artifact.Suffix == suffix {
			return true
		}
	}
	return false
}

func bindSessionDBClearArtifact(
	ctx context.Context,
	lease *sessiondb.FamilyLease,
	owner store.SessionDBOwner,
	path string,
	suffix string,
) (*sessiondb.FamilyFile, error) {
	if suffix == "" {
		return lease.BindOwnedFile(ctx, owner, path)
	}
	return lease.BindFile(path)
}

func verifySessionDBClearArtifact(
	ctx context.Context,
	lease *sessiondb.FamilyLease,
	owner store.SessionDBOwner,
	path string,
	artifact sessionDBClearArtifact,
) (*sessiondb.FamilyFile, error) {
	bound, err := bindSessionDBClearArtifact(ctx, lease, owner, path, artifact.Suffix)
	if err != nil {
		return nil, err
	}
	digest, err := bound.SHA256()
	if err != nil {
		return nil, errors.Join(err, bound.Close())
	}
	if hex.EncodeToString(digest[:]) != artifact.SHA256 {
		return nil, errors.Join(
			fmt.Errorf("%w: clear artifact %q digest changed", sessiondb.ErrSessionDBFamilyChanged, path),
			bound.Close(),
		)
	}
	return bound, nil
}

func isSessionDBClearArtifactSuffix(suffix string) bool {
	for _, candidate := range sessionDBClearArtifactSuffixes {
		if suffix == candidate {
			return true
		}
	}
	return false
}

func discardSessionDBClearManifest(path string) error {
	err := fileutil.AtomicRemoveFile(sessionDBClearManifestPath(path))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("session: remove clear manifest: %w", err)
	}
	return nil
}

func sessionDBClearManifestPath(path string) string {
	return strings.TrimSpace(path) + ".clear-manifest"
}

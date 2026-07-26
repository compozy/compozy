package tools

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/compozy/agh/internal/fileutil"
)

const artifactFileMode = 0o600

type artifactWriteFile func(path string, content []byte, perm os.FileMode) error

type artifactStoreOptions struct {
	now       func() time.Time
	writeFile artifactWriteFile
}

// FilesystemToolArtifactStore retains immutable content-addressed result envelopes.
type FilesystemToolArtifactStore struct {
	root      string
	retention ToolArtifactRetention
	now       func() time.Time
	writeFile artifactWriteFile
	mu        sync.Mutex
}

var _ ToolArtifactStore = (*FilesystemToolArtifactStore)(nil)

// OpenFilesystemToolArtifactStore opens the canonical store and applies retention before use.
func OpenFilesystemToolArtifactStore(
	ctx context.Context,
	root string,
	retention ToolArtifactRetention,
	opts ...func(*artifactStoreOptions),
) (*FilesystemToolArtifactStore, error) {
	if strings.TrimSpace(root) == "" {
		return nil, fmt.Errorf("tool artifact root is required")
	}
	if err := retention.Validate(); err != nil {
		return nil, err
	}
	settings := artifactStoreOptions{now: time.Now, writeFile: fileutil.AtomicWriteFile}
	for _, opt := range opts {
		opt(&settings)
	}
	store := &FilesystemToolArtifactStore{
		root:      filepath.Clean(root),
		retention: retention,
		now:       settings.now,
		writeFile: settings.writeFile,
	}
	if err := store.ensureRoot(); err != nil {
		return nil, err
	}
	if err := store.Sweep(ctx); err != nil {
		return nil, err
	}
	return store, nil
}

// Put publishes canonical bytes once and returns their opaque reference.
func (s *FilesystemToolArtifactStore) Put(
	ctx context.Context,
	workspaceID string,
	content []byte,
) (ArtifactRef, error) {
	if err := artifactContextErr(ctx); err != nil {
		return ArtifactRef{}, err
	}
	if strings.TrimSpace(workspaceID) == "" {
		return ArtifactRef{}, fmt.Errorf("%w: workspace is required", ErrToolArtifactPersistence)
	}
	if int64(len(content)) > s.retention.MaxBytes {
		return ArtifactRef{}, fmt.Errorf(
			"%w: artifact bytes %d exceed retention max_bytes %d",
			ErrToolArtifactPersistence,
			len(content),
			s.retention.MaxBytes,
		)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.sweepLocked(ctx, 0, 0); err != nil {
		return ArtifactRef{}, err
	}

	id := artifactID(content)
	workspaceDir, err := s.ensureWorkspaceDir(workspaceID)
	if err != nil {
		return ArtifactRef{}, err
	}
	path := filepath.Join(workspaceDir, id+".json")
	if existing, readErr := readArtifactFile(path, id); readErr == nil {
		if !bytes.Equal(existing, content) {
			return ArtifactRef{}, fmt.Errorf("%w: digest collision for %s", ErrToolArtifactCorrupt, id)
		}
		return toolArtifactRef(id, int64(len(existing))), nil
	} else if !errors.Is(readErr, ErrToolArtifactNotFound) {
		return ArtifactRef{}, readErr
	}

	if err := s.sweepLocked(ctx, 1, int64(len(content))); err != nil {
		return ArtifactRef{}, err
	}
	if err := s.writeFile(path, content, artifactFileMode); err != nil {
		return ArtifactRef{}, s.cleanupFailedPublish(path, err)
	}
	if err := persistArtifactCreatedAt(path, s.now().UTC()); err != nil {
		return ArtifactRef{}, s.cleanupFailedPublish(path, err)
	}
	stored, err := readArtifactFile(path, id)
	if err != nil {
		return ArtifactRef{}, s.cleanupFailedPublish(path, err)
	}
	if !bytes.Equal(stored, content) {
		return ArtifactRef{}, s.cleanupFailedPublish(path, ErrToolArtifactCorrupt)
	}
	return toolArtifactRef(id, int64(len(stored))), nil
}

// ReadPage returns one exact bounded byte range after enforcing workspace scope and retention.
func (s *FilesystemToolArtifactStore) ReadPage(
	ctx context.Context,
	workspaceID string,
	uri string,
	offset int64,
	limit int64,
) (ToolArtifactPage, error) {
	if err := artifactContextErr(ctx); err != nil {
		return ToolArtifactPage{}, err
	}
	if strings.TrimSpace(workspaceID) == "" {
		return ToolArtifactPage{}, ErrToolArtifactNotFound
	}
	if offset < 0 {
		return ToolArtifactPage{}, fmt.Errorf(
			"%w: artifact offset must be greater than or equal to zero",
			ErrToolArtifactInvalidRange,
		)
	}
	if limit == 0 {
		limit = DefaultToolArtifactPageBytes
	}
	if limit < 1 || limit > MaxToolArtifactPageBytes {
		return ToolArtifactPage{}, fmt.Errorf(
			"%w: artifact limit must be between 1 and %d",
			ErrToolArtifactInvalidRange,
			MaxToolArtifactPageBytes,
		)
	}
	id, err := ParseToolArtifactURI(uri)
	if err != nil {
		return ToolArtifactPage{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.sweepLocked(ctx, 0, 0); err != nil {
		return ToolArtifactPage{}, err
	}
	path := filepath.Join(s.workspaceDir(workspaceID), id+".json")
	content, err := readArtifactFile(path, id)
	if err != nil {
		return ToolArtifactPage{}, err
	}
	total := int64(len(content))
	if offset > total {
		return ToolArtifactPage{}, fmt.Errorf(
			"%w: artifact offset %d exceeds total bytes %d",
			ErrToolArtifactInvalidRange,
			offset,
			total,
		)
	}
	end := min(total, offset+limit)
	pageBytes := content[offset:end]
	return ToolArtifactPage{
		Artifact:   toolArtifactRef(id, total),
		Offset:     offset,
		Bytes:      int64(len(pageBytes)),
		TotalBytes: total,
		DataBase64: base64.StdEncoding.EncodeToString(pageBytes),
		NextOffset: end,
		EOF:        end == total,
	}, nil
}

// Sweep removes expired and over-budget artifacts in deterministic creation order.
func (s *FilesystemToolArtifactStore) Sweep(ctx context.Context) error {
	if err := artifactContextErr(ctx); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sweepLocked(ctx, 0, 0)
}

func artifactID(content []byte) string {
	digest := sha256.Sum256(content)
	return "art_" + hex.EncodeToString(digest[:])
}

func (s *FilesystemToolArtifactStore) ensureRoot() error {
	if err := os.MkdirAll(s.root, 0o700); err != nil {
		return fmt.Errorf("%w: create artifact root: %w", ErrToolArtifactPersistence, err)
	}
	if err := os.Chmod(s.root, 0o700); err != nil {
		return fmt.Errorf("%w: secure artifact root: %w", ErrToolArtifactPersistence, err)
	}
	info, err := os.Lstat(s.root)
	if err != nil {
		return fmt.Errorf("%w: stat artifact root: %w", ErrToolArtifactPersistence, err)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%w: artifact root is not a private directory", ErrToolArtifactPersistence)
	}
	return nil
}

func (s *FilesystemToolArtifactStore) ensureWorkspaceDir(workspaceID string) (string, error) {
	dir := s.workspaceDir(workspaceID)
	if err := os.Mkdir(dir, 0o700); err != nil && !errors.Is(err, os.ErrExist) {
		return "", fmt.Errorf("%w: create artifact workspace: %w", ErrToolArtifactPersistence, err)
	}
	info, err := os.Lstat(dir)
	if err != nil {
		return "", fmt.Errorf("%w: stat artifact workspace: %w", ErrToolArtifactPersistence, err)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("%w: artifact workspace is not a directory", ErrToolArtifactPersistence)
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return "", fmt.Errorf("%w: secure artifact workspace: %w", ErrToolArtifactPersistence, err)
	}
	return dir, nil
}

func (s *FilesystemToolArtifactStore) workspaceDir(workspaceID string) string {
	digest := sha256.Sum256([]byte(strings.TrimSpace(workspaceID)))
	return filepath.Join(s.root, hex.EncodeToString(digest[:]))
}

func (s *FilesystemToolArtifactStore) cleanupFailedPublish(path string, cause error) error {
	cleanupErr := os.Remove(path)
	if cleanupErr != nil && !errors.Is(cleanupErr, os.ErrNotExist) {
		return errors.Join(
			fmt.Errorf("%w: publish artifact: %w", ErrToolArtifactPersistence, cause),
			fmt.Errorf("remove failed artifact %q: %w", path, cleanupErr),
		)
	}
	return fmt.Errorf("%w: publish artifact: %w", ErrToolArtifactPersistence, cause)
}

func readArtifactFile(path string, id string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrToolArtifactNotFound
		}
		return nil, fmt.Errorf("read tool artifact: %w", err)
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("%w: artifact is not a regular file", ErrToolArtifactCorrupt)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read tool artifact: %w", err)
	}
	if artifactID(content) != id {
		return nil, fmt.Errorf("%w: content digest mismatch", ErrToolArtifactCorrupt)
	}
	return content, nil
}

func persistArtifactCreatedAt(path string, createdAt time.Time) error {
	if err := os.Chtimes(path, createdAt, createdAt); err != nil {
		return fmt.Errorf("set artifact creation time: %w", err)
	}
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open artifact for metadata sync: %w", err)
	}
	syncErr := file.Sync()
	closeErr := file.Close()
	if syncErr != nil || closeErr != nil {
		return errors.Join(syncErr, closeErr)
	}
	if err := fileutil.SyncDir(filepath.Dir(path)); err != nil {
		return fmt.Errorf("sync artifact creation time: %w", err)
	}
	return nil
}

func artifactContextErr(ctx context.Context) error {
	if ctx == nil {
		return errors.New("tool artifact context is required")
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

type retainedArtifact struct {
	path      string
	createdAt time.Time
	bytes     int64
}

func (s *FilesystemToolArtifactStore) sweepLocked(
	ctx context.Context,
	reserveCount int,
	reserveBytes int64,
) error {
	artifacts, err := s.listArtifacts(ctx)
	if err != nil {
		return err
	}
	now := s.now().UTC()
	retained := artifacts[:0]
	for _, artifact := range artifacts {
		if now.Sub(artifact.createdAt) > s.retention.MaxAge {
			if err := fileutil.AtomicRemoveFile(artifact.path); err != nil {
				return fmt.Errorf("%w: remove expired artifact: %w", ErrToolArtifactPersistence, err)
			}
			continue
		}
		retained = append(retained, artifact)
	}
	slices.SortFunc(retained, compareRetainedArtifacts)
	totalBytes := int64(0)
	for _, artifact := range retained {
		totalBytes += artifact.bytes
	}
	for len(retained)+reserveCount > s.retention.MaxCount || totalBytes+reserveBytes > s.retention.MaxBytes {
		oldest := retained[0]
		if err := fileutil.AtomicRemoveFile(oldest.path); err != nil {
			return fmt.Errorf("%w: evict artifact: %w", ErrToolArtifactPersistence, err)
		}
		totalBytes -= oldest.bytes
		retained = retained[1:]
	}
	return nil
}

func (s *FilesystemToolArtifactStore) listArtifacts(ctx context.Context) ([]retainedArtifact, error) {
	workspaceEntries, err := os.ReadDir(s.root)
	if err != nil {
		return nil, fmt.Errorf("%w: list artifact root: %w", ErrToolArtifactPersistence, err)
	}
	artifacts := make([]retainedArtifact, 0)
	for _, workspaceEntry := range workspaceEntries {
		if err := artifactContextErr(ctx); err != nil {
			return nil, err
		}
		if !workspaceEntry.IsDir() || len(workspaceEntry.Name()) != sha256.Size*2 {
			continue
		}
		workspacePath := filepath.Join(s.root, workspaceEntry.Name())
		entries, err := os.ReadDir(workspacePath)
		if err != nil {
			return nil, fmt.Errorf("%w: list artifact workspace: %w", ErrToolArtifactPersistence, err)
		}
		for _, entry := range entries {
			id := strings.TrimSuffix(entry.Name(), ".json")
			if entry.IsDir() || entry.Name() != id+".json" || !toolArtifactIDPattern.MatchString(id) {
				continue
			}
			path := filepath.Join(workspacePath, entry.Name())
			info, err := entry.Info()
			if err != nil {
				return nil, fmt.Errorf("%w: stat retained artifact: %w", ErrToolArtifactPersistence, err)
			}
			if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
				return nil, fmt.Errorf("%w: retained artifact is not a regular file", ErrToolArtifactCorrupt)
			}
			artifacts = append(artifacts, retainedArtifact{
				path:      path,
				createdAt: info.ModTime().UTC(),
				bytes:     info.Size(),
			})
		}
	}
	return artifacts, nil
}

func compareRetainedArtifacts(left retainedArtifact, right retainedArtifact) int {
	if compared := left.createdAt.Compare(right.createdAt); compared != 0 {
		return compared
	}
	return strings.Compare(left.path, right.path)
}

func withArtifactStoreNow(now func() time.Time) func(*artifactStoreOptions) {
	return func(options *artifactStoreOptions) {
		options.now = now
	}
}

func withArtifactStoreWriteFile(writeFile artifactWriteFile) func(*artifactStoreOptions) {
	return func(options *artifactStoreOptions) {
		options.writeFile = writeFile
	}
}

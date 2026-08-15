package attachments

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
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/compozy/compozy/internal/diagnostics"
	"github.com/compozy/compozy/internal/fileutil"
)

const attachmentFileMode = 0o600

type attachmentWriteFile func(path string, content []byte, perm os.FileMode) error

type attachmentStoreOptions struct {
	now       func() time.Time
	writeFile attachmentWriteFile
}

// FilesystemAttachmentStore retains immutable content-addressed session attachments.
type FilesystemAttachmentStore struct {
	root          string
	retention     AttachmentRetention
	maxFileBytes  int64
	allowedMIME   []string
	now           func() time.Time
	writeFile     attachmentWriteFile
	retentionPins RetentionPinSource
	deletedScopes map[string]struct{}
	mu            sync.Mutex
}

var _ Store = (*FilesystemAttachmentStore)(nil)
var _ ScopeLease = (*FilesystemAttachmentStore)(nil)

// AcquireScopeLease blocks attachment mutations until the caller releases the lease.
// An empty session ID reserves the whole workspace; the filesystem store currently
// serializes all scopes through one mutex so workspace deletion cannot race a Put.
func (s *FilesystemAttachmentStore) AcquireScopeLease(
	ctx context.Context,
	workspaceID string,
	sessionID string,
) (ScopeLeaseGuard, error) {
	if err := attachmentContextErr(ctx); err != nil {
		return nil, err
	}
	if _, err := sanitizeScopeID(workspaceID, "workspace"); err != nil {
		return nil, err
	}
	if sessionID != "" {
		if _, err := sanitizeScopeID(sessionID, "session"); err != nil {
			return nil, err
		}
	}
	s.mu.Lock()
	if err := attachmentContextErr(ctx); err != nil {
		s.mu.Unlock()
		return nil, err
	}
	return &filesystemScopeLeaseGuard{
		store: s,
		key:   attachmentScopeKey(workspaceID, sessionID),
	}, nil
}

type filesystemScopeLeaseGuard struct {
	store   *FilesystemAttachmentStore
	key     string
	deleted bool
	once    sync.Once
}

func (g *filesystemScopeLeaseGuard) MarkDeleted() {
	if g == nil || g.store == nil {
		return
	}
	g.deleted = true
}

func (g *filesystemScopeLeaseGuard) Release() {
	if g == nil || g.store == nil {
		return
	}
	g.once.Do(func() {
		if g.deleted {
			g.store.deletedScopes[g.key] = struct{}{}
		}
		g.store.mu.Unlock()
	})
}

func attachmentScopeKey(workspaceID string, sessionID string) string {
	return workspaceID + "\x00" + sessionID
}

// OpenFilesystemAttachmentStore opens the canonical store and applies retention before use.
func OpenFilesystemAttachmentStore(
	ctx context.Context,
	root string,
	retention AttachmentRetention,
	limits StoreLimits,
	retentionPins RetentionPinSource,
	opts ...func(*attachmentStoreOptions),
) (*FilesystemAttachmentStore, error) {
	if strings.TrimSpace(root) == "" {
		return nil, fmt.Errorf("attachment root is required")
	}
	if err := retention.Validate(); err != nil {
		return nil, err
	}
	if err := ValidateSize(0, limits.MaxFileBytes); err != nil {
		return nil, err
	}
	allowed, err := normalizeAllowedMIME(limits.AllowedMIME)
	if err != nil {
		return nil, err
	}
	settings := attachmentStoreOptions{now: time.Now, writeFile: fileutil.AtomicWriteFile}
	for _, opt := range opts {
		opt(&settings)
	}
	store := &FilesystemAttachmentStore{
		root:          filepath.Clean(root),
		retention:     retention,
		maxFileBytes:  limits.MaxFileBytes,
		allowedMIME:   allowed,
		now:           settings.now,
		writeFile:     settings.writeFile,
		retentionPins: retentionPins,
		deletedScopes: make(map[string]struct{}),
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
func (s *FilesystemAttachmentStore) Put(
	ctx context.Context,
	workspaceID string,
	sessionID string,
	name string,
	data []byte,
) (AttachmentRef, error) {
	if err := attachmentContextErr(ctx); err != nil {
		return AttachmentRef{}, err
	}
	workspaceID, sessionID, err := sanitizeScopeIDs(workspaceID, sessionID)
	if err != nil {
		return AttachmentRef{}, err
	}
	if err := ValidateSize(int64(len(data)), s.maxFileBytes); err != nil {
		return AttachmentRef{}, err
	}
	if int64(len(data)) > s.retention.MaxBytes {
		return AttachmentRef{}, fmt.Errorf(
			"%w: %d exceeds retention max_bytes %d",
			ErrTooLarge,
			len(data),
			s.retention.MaxBytes,
		)
	}
	mime, kind, width, height, err := SniffMIME(data, name)
	if err != nil {
		return AttachmentRef{}, err
	}
	if err := mimeAllowed(mime, s.allowedMIME); err != nil {
		return AttachmentRef{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if _, deleted := s.deletedScopes[attachmentScopeKey(workspaceID, sessionID)]; deleted {
		return AttachmentRef{}, ErrNotFound
	}
	if _, deleted := s.deletedScopes[attachmentScopeKey(workspaceID, "")]; deleted {
		return AttachmentRef{}, ErrNotFound
	}

	id := attachmentID(data)
	sessionDir, err := s.ensureSessionDir(workspaceID, sessionID)
	if err != nil {
		return AttachmentRef{}, err
	}
	contentPath := filepath.Join(sessionDir, id)
	if existing, readErr := s.readMeta(contentPath, id); readErr == nil {
		stored, storedErr := readAttachmentFile(contentPath, id)
		if storedErr != nil {
			return AttachmentRef{}, storedErr
		}
		if !bytes.Equal(stored, data) {
			return AttachmentRef{}, fmt.Errorf("%w: digest collision for %s", ErrCorrupt, id)
		}
		return attachmentRef(id, existing), nil
	} else if !errors.Is(readErr, ErrNotFound) {
		return AttachmentRef{}, readErr
	}

	if err := s.sweepLocked(ctx, 1, int64(len(data))); err != nil {
		return AttachmentRef{}, err
	}
	meta := attachmentMeta{
		Name:      diagnostics.Redact(strings.TrimSpace(name)),
		MIMEType:  mime,
		Bytes:     int64(len(data)),
		SHA256:    attachmentSHA256(id),
		Kind:      kind,
		Width:     width,
		Height:    height,
		CreatedAt: s.now().UTC(),
	}
	if err := s.publish(contentPath, data, meta); err != nil {
		return AttachmentRef{}, err
	}
	return attachmentRef(id, meta), nil
}

// Open returns a reader for retained bytes after enforcing workspace and session scope.
func (s *FilesystemAttachmentStore) Open(
	ctx context.Context,
	workspaceID string,
	sessionID string,
	id string,
) (io.ReadCloser, AttachmentRef, error) {
	ref, contentPath, err := s.statLocked(ctx, workspaceID, sessionID, id)
	if err != nil {
		return nil, AttachmentRef{}, err
	}
	file, err := os.Open(contentPath)
	if err != nil {
		return nil, AttachmentRef{}, fmt.Errorf("open attachment: %w", err)
	}
	if err := verifyAttachmentDigest(file, id); err != nil {
		closeErr := file.Close()
		if closeErr != nil {
			return nil, AttachmentRef{}, errors.Join(err, fmt.Errorf("close attachment: %w", closeErr))
		}
		return nil, AttachmentRef{}, err
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		closeErr := file.Close()
		if closeErr != nil {
			return nil, AttachmentRef{}, errors.Join(
				fmt.Errorf("rewind attachment: %w", err),
				fmt.Errorf("close attachment: %w", closeErr),
			)
		}
		return nil, AttachmentRef{}, fmt.Errorf("rewind attachment: %w", err)
	}
	return file, ref, nil
}

// Stat returns retained metadata after enforcing workspace and session scope.
func (s *FilesystemAttachmentStore) Stat(
	ctx context.Context,
	workspaceID string,
	sessionID string,
	id string,
) (AttachmentRef, error) {
	ref, _, err := s.statLocked(ctx, workspaceID, sessionID, id)
	return ref, err
}

// Delete removes one retained attachment and its sidecar metadata.
func (s *FilesystemAttachmentStore) Delete(
	ctx context.Context,
	workspaceID string,
	sessionID string,
	id string,
) error {
	if err := attachmentContextErr(ctx); err != nil {
		return err
	}
	workspaceID, sessionID, err := sanitizeScopeIDs(workspaceID, sessionID)
	if err != nil {
		return err
	}
	if !attachmentIDPattern.MatchString(id) {
		return fmt.Errorf("%w: %q", ErrInvalidID, id)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	contentPath := s.contentPath(workspaceID, sessionID, id)
	if _, err := s.readMeta(contentPath, id); err != nil {
		return err
	}
	return s.removePair(contentPath)
}

// Sweep removes expired and over-budget attachments in deterministic creation order.
func (s *FilesystemAttachmentStore) Sweep(ctx context.Context) error {
	if err := attachmentContextErr(ctx); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sweepLocked(ctx, 0, 0)
}

func (s *FilesystemAttachmentStore) statLocked(
	ctx context.Context,
	workspaceID string,
	sessionID string,
	id string,
) (AttachmentRef, string, error) {
	if err := attachmentContextErr(ctx); err != nil {
		return AttachmentRef{}, "", err
	}
	workspaceID, sessionID, err := sanitizeScopeIDs(workspaceID, sessionID)
	if err != nil {
		return AttachmentRef{}, "", err
	}
	if !attachmentIDPattern.MatchString(id) {
		return AttachmentRef{}, "", fmt.Errorf("%w: %q", ErrInvalidID, id)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.sweepLocked(ctx, 0, 0); err != nil {
		return AttachmentRef{}, "", err
	}
	contentPath := s.contentPath(workspaceID, sessionID, id)
	meta, err := s.readMeta(contentPath, id)
	if err != nil {
		return AttachmentRef{}, "", err
	}
	return attachmentRef(id, meta), contentPath, nil
}

func (s *FilesystemAttachmentStore) publish(contentPath string, data []byte, meta attachmentMeta) error {
	if err := s.writeFile(contentPath, data, attachmentFileMode); err != nil {
		return s.cleanupFailedPublish(contentPath, err)
	}
	encoded, err := json.Marshal(meta)
	if err != nil {
		return s.cleanupFailedPublish(contentPath, err)
	}
	if err := s.writeFile(metaPath(contentPath), encoded, attachmentFileMode); err != nil {
		return s.cleanupFailedPublish(contentPath, err)
	}
	stored, err := readAttachmentFile(contentPath, filepath.Base(contentPath))
	if err != nil {
		return s.cleanupFailedPublish(contentPath, err)
	}
	if !bytes.Equal(stored, data) {
		return s.cleanupFailedPublish(contentPath, ErrCorrupt)
	}
	return nil
}

func (s *FilesystemAttachmentStore) cleanupFailedPublish(contentPath string, cause error) error {
	cleanupErr := s.removePair(contentPath)
	if cleanupErr != nil && !errors.Is(cleanupErr, os.ErrNotExist) && !errors.Is(cleanupErr, ErrNotFound) {
		return errors.Join(
			fmt.Errorf("%w: publish attachment: %w", ErrPersistence, cause),
			fmt.Errorf("remove failed attachment %q: %w", contentPath, cleanupErr),
		)
	}
	return fmt.Errorf("%w: publish attachment: %w", ErrPersistence, cause)
}

func attachmentID(content []byte) string {
	digest := sha256.Sum256(content)
	return "att_" + hex.EncodeToString(digest[:])
}

func attachmentContextErr(ctx context.Context) error {
	if ctx == nil {
		return errors.New("attachment context is required")
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

func withAttachmentStoreNow(now func() time.Time) func(*attachmentStoreOptions) {
	return func(options *attachmentStoreOptions) {
		options.now = now
	}
}

func withAttachmentStoreWriteFile(writeFile attachmentWriteFile) func(*attachmentStoreOptions) {
	return func(options *attachmentStoreOptions) {
		options.writeFile = writeFile
	}
}

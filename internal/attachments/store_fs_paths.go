package attachments

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/compozy/compozy/internal/fileutil"
)

// userScopeAttachmentDir holds attachments of User-scope sessions, which have no workspace
// (USW BR-1). Workspace ids never take this value, and retention is keyed by session.
const userScopeAttachmentDir = "user"

// ScopeDir is the attachment directory of a workspace; the User scope's empty workspace maps to
// its reserved directory. Session deletion stages and recovers through the same name.
func ScopeDir(workspaceID string) string {
	return scopeWorkspaceID(workspaceID)
}

// scopeWorkspaceID maps the User scope's empty workspace to its reserved attachment directory.
func scopeWorkspaceID(workspaceID string) string {
	if workspaceID == "" {
		return userScopeAttachmentDir
	}
	return workspaceID
}

func sanitizeScopeIDs(workspaceID string, sessionID string) (string, string, error) {
	workspaceID, err := sanitizeScopeID(scopeWorkspaceID(workspaceID), "workspace")
	if err != nil {
		return "", "", err
	}
	sessionID, err = sanitizeScopeID(sessionID, "session")
	if err != nil {
		return "", "", err
	}
	return workspaceID, sessionID, nil
}

func sanitizeScopeID(id string, kind string) (string, error) {
	if strings.ContainsRune(id, 0) {
		return "", fmt.Errorf("%w: %s id contains NUL byte", ErrInvalidID, kind)
	}
	trimmed := strings.TrimSpace(id)
	if trimmed == "" || trimmed != id || !scopeIDPattern.MatchString(trimmed) {
		return "", fmt.Errorf("%w: %s id %q", ErrInvalidID, kind, id)
	}
	return trimmed, nil
}

func (s *FilesystemAttachmentStore) contentPath(workspaceID string, sessionID string, id string) string {
	return filepath.Join(s.root, workspaceID, sessionID, id)
}

func metaPath(contentPath string) string {
	return contentPath + ".json"
}

func (s *FilesystemAttachmentStore) ensureRoot(ctx context.Context) error {
	if err := attachmentContextErr(ctx); err != nil {
		return err
	}
	if err := os.MkdirAll(s.root, attachmentDirMode); err != nil {
		return fmt.Errorf("%w: create attachment root: %w", ErrPersistence, err)
	}
	if err := attachmentContextErr(ctx); err != nil {
		return err
	}
	if err := os.Chmod(s.root, attachmentDirMode); err != nil {
		return fmt.Errorf("%w: secure attachment root: %w", ErrPersistence, err)
	}
	if err := attachmentContextErr(ctx); err != nil {
		return err
	}
	info, err := os.Lstat(s.root)
	if err != nil {
		return fmt.Errorf("%w: stat attachment root: %w", ErrPersistence, err)
	}
	if err := attachmentContextErr(ctx); err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%w: attachment root is not a private directory", ErrPersistence)
	}
	return nil
}

func (s *FilesystemAttachmentStore) ensureSessionDir(
	ctx context.Context,
	workspaceID string,
	sessionID string,
) (string, error) {
	workspaceDir := filepath.Join(s.root, workspaceID)
	if err := ensurePrivateDir(ctx, workspaceDir, "attachment workspace"); err != nil {
		return "", err
	}
	sessionDir := filepath.Join(workspaceDir, sessionID)
	if err := ensurePrivateDir(ctx, sessionDir, "attachment session"); err != nil {
		return "", err
	}
	return sessionDir, nil
}

func ensurePrivateDir(ctx context.Context, dir string, label string) error {
	if err := attachmentContextErr(ctx); err != nil {
		return err
	}
	if err := os.Mkdir(dir, attachmentDirMode); err != nil && !errors.Is(err, os.ErrExist) {
		return fmt.Errorf("%w: create %s: %w", ErrPersistence, label, err)
	}
	if err := attachmentContextErr(ctx); err != nil {
		return err
	}
	info, err := os.Lstat(dir)
	if err != nil {
		return fmt.Errorf("%w: stat %s: %w", ErrPersistence, label, err)
	}
	if err := attachmentContextErr(ctx); err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%w: %s is not a directory", ErrPersistence, label)
	}
	if err := attachmentContextErr(ctx); err != nil {
		return err
	}
	if err := os.Chmod(dir, attachmentDirMode); err != nil {
		return fmt.Errorf("%w: secure %s: %w", ErrPersistence, label, err)
	}
	if err := attachmentContextErr(ctx); err != nil {
		return err
	}
	return nil
}

func (s *FilesystemAttachmentStore) readMeta(
	ctx context.Context,
	contentPath string,
	id string,
) (attachmentMeta, error) {
	content, err := readAttachmentFile(ctx, contentPath, id)
	if err != nil {
		return attachmentMeta{}, err
	}
	return readMetaForContent(ctx, contentPath, id, content)
}

func readMetaForContent(
	ctx context.Context,
	contentPath string,
	id string,
	content []byte,
) (attachmentMeta, error) {
	meta, err := readSidecar(ctx, contentPath)
	if err != nil {
		return attachmentMeta{}, err
	}
	mime, kind, width, height, err := SniffMIME(content, meta.Name)
	if err != nil {
		return attachmentMeta{}, fmt.Errorf("%w: sniff retained content: %w", ErrCorrupt, err)
	}
	if meta.SHA256 != attachmentSHA256(id) ||
		meta.Bytes != int64(len(content)) ||
		meta.MIMEType != mime ||
		meta.Kind != kind ||
		meta.Width != width ||
		meta.Height != height ||
		meta.CreatedAt.IsZero() {
		return attachmentMeta{}, fmt.Errorf("%w: sidecar metadata mismatch", ErrCorrupt)
	}
	return meta, nil
}

func readSidecar(ctx context.Context, contentPath string) (attachmentMeta, error) {
	encoded, err := readMetadataFile(ctx, metaPath(contentPath))
	if err != nil {
		return attachmentMeta{}, err
	}
	var meta attachmentMeta
	if err := json.Unmarshal(encoded, &meta); err != nil {
		return attachmentMeta{}, fmt.Errorf("%w: decode sidecar: %w", ErrCorrupt, err)
	}
	return meta, nil
}

func readMetadataFile(ctx context.Context, path string) ([]byte, error) {
	if err := attachmentContextErr(ctx); err != nil {
		return nil, err
	}
	info, err := os.Lstat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("%w: missing sidecar", ErrCorrupt)
		}
		return nil, fmt.Errorf("stat attachment metadata: %w", err)
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("%w: sidecar is not a regular file", ErrCorrupt)
	}
	if err := attachmentContextErr(ctx); err != nil {
		return nil, err
	}
	encoded, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read attachment metadata: %w", err)
	}
	if err := attachmentContextErr(ctx); err != nil {
		return nil, err
	}
	return encoded, nil
}

func (s *FilesystemAttachmentStore) removePair(contentPath string) error {
	contentErr := fileutil.AtomicRemoveFile(contentPath)
	metaErr := fileutil.AtomicRemoveFile(metaPath(contentPath))
	if contentErr != nil && !errors.Is(contentErr, os.ErrNotExist) {
		return contentErr
	}
	if metaErr != nil && !errors.Is(metaErr, os.ErrNotExist) {
		return metaErr
	}
	if errors.Is(contentErr, os.ErrNotExist) && errors.Is(metaErr, os.ErrNotExist) {
		return ErrNotFound
	}
	return nil
}

func readAttachmentFile(ctx context.Context, path string, id string) ([]byte, error) {
	if err := attachmentContextErr(ctx); err != nil {
		return nil, err
	}
	info, err := os.Lstat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("read attachment: %w", err)
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("%w: attachment is not a regular file", ErrCorrupt)
	}
	if err := attachmentContextErr(ctx); err != nil {
		return nil, err
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read attachment: %w", err)
	}
	if err := attachmentContextErr(ctx); err != nil {
		return nil, err
	}
	if attachmentID(content) != id {
		return nil, fmt.Errorf("%w: content digest mismatch", ErrCorrupt)
	}
	return content, nil
}

// readOpenAttachment validates the same descriptor that Open returns to its caller.
func readOpenAttachment(ctx context.Context, file *os.File, id string) ([]byte, error) {
	if err := attachmentContextErr(ctx); err != nil {
		return nil, err
	}
	info, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat open attachment: %w", err)
	}
	if !info.Mode().IsRegular() || info.Size() < 0 || info.Size() > math.MaxInt-bytes.MinRead {
		return nil, fmt.Errorf("%w: attachment is not a readable regular file", ErrCorrupt)
	}
	var content bytes.Buffer
	// Leave room for ReadFrom's final EOF probe without growing the full payload.
	// #nosec G115 -- the size is bounded by MaxInt above.
	content.Grow(int(info.Size()) + bytes.MinRead)
	if _, err := content.ReadFrom(file); err != nil {
		return nil, fmt.Errorf("read attachment: %w", err)
	}
	if err := attachmentContextErr(ctx); err != nil {
		return nil, err
	}
	if attachmentID(content.Bytes()) != id {
		return nil, fmt.Errorf("%w: content digest mismatch", ErrCorrupt)
	}
	return content.Bytes(), nil
}

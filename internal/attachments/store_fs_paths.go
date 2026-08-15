package attachments

import (
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

	"github.com/compozy/compozy/internal/fileutil"
)

func sanitizeScopeIDs(workspaceID string, sessionID string) (string, string, error) {
	workspaceID, err := sanitizeScopeID(workspaceID, "workspace")
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

func (s *FilesystemAttachmentStore) readMeta(ctx context.Context, contentPath string, id string) (attachmentMeta, error) {
	content, err := readAttachmentFile(ctx, contentPath, id)
	if err != nil {
		return attachmentMeta{}, err
	}
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

func verifyAttachmentDigest(ctx context.Context, reader io.Reader, id string) error {
	if err := attachmentContextErr(ctx); err != nil {
		return err
	}
	digest := sha256.New()
	if _, err := io.Copy(digest, reader); err != nil {
		return fmt.Errorf("hash attachment: %w", err)
	}
	if err := attachmentContextErr(ctx); err != nil {
		return err
	}
	got := "att_" + hex.EncodeToString(digest.Sum(nil))
	if got != id {
		return fmt.Errorf("%w: content digest mismatch", ErrCorrupt)
	}
	return nil
}

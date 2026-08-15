package attachments

import (
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

func (s *FilesystemAttachmentStore) ensureRoot() error {
	if err := os.MkdirAll(s.root, 0o700); err != nil {
		return fmt.Errorf("%w: create attachment root: %w", ErrPersistence, err)
	}
	if err := os.Chmod(s.root, 0o700); err != nil {
		return fmt.Errorf("%w: secure attachment root: %w", ErrPersistence, err)
	}
	info, err := os.Lstat(s.root)
	if err != nil {
		return fmt.Errorf("%w: stat attachment root: %w", ErrPersistence, err)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%w: attachment root is not a private directory", ErrPersistence)
	}
	return nil
}

func (s *FilesystemAttachmentStore) ensureSessionDir(workspaceID string, sessionID string) (string, error) {
	workspaceDir := filepath.Join(s.root, workspaceID)
	if err := ensurePrivateDir(workspaceDir, "attachment workspace"); err != nil {
		return "", err
	}
	sessionDir := filepath.Join(workspaceDir, sessionID)
	if err := ensurePrivateDir(sessionDir, "attachment session"); err != nil {
		return "", err
	}
	return sessionDir, nil
}

func ensurePrivateDir(dir string, label string) error {
	if err := os.Mkdir(dir, 0o700); err != nil && !errors.Is(err, os.ErrExist) {
		return fmt.Errorf("%w: create %s: %w", ErrPersistence, label, err)
	}
	info, err := os.Lstat(dir)
	if err != nil {
		return fmt.Errorf("%w: stat %s: %w", ErrPersistence, label, err)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%w: %s is not a directory", ErrPersistence, label)
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return fmt.Errorf("%w: secure %s: %w", ErrPersistence, label, err)
	}
	return nil
}

func (s *FilesystemAttachmentStore) readMeta(contentPath string, id string) (attachmentMeta, error) {
	content, err := readAttachmentFile(contentPath, id)
	if err != nil {
		return attachmentMeta{}, err
	}
	meta, err := readSidecar(contentPath)
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

func readSidecar(contentPath string) (attachmentMeta, error) {
	encoded, err := readMetadataFile(metaPath(contentPath))
	if err != nil {
		return attachmentMeta{}, err
	}
	var meta attachmentMeta
	if err := json.Unmarshal(encoded, &meta); err != nil {
		return attachmentMeta{}, fmt.Errorf("%w: decode sidecar: %w", ErrCorrupt, err)
	}
	return meta, nil
}

func readMetadataFile(path string) ([]byte, error) {
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
	encoded, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read attachment metadata: %w", err)
	}
	return encoded, nil
}

func (s *FilesystemAttachmentStore) removePair(contentPath string) error {
	contentErr := fileutilRemove(contentPath)
	metaErr := fileutilRemove(metaPath(contentPath))
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

func fileutilRemove(path string) error {
	return fileutil.AtomicRemoveFile(path)
}

func readAttachmentFile(path string, id string) ([]byte, error) {
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
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read attachment: %w", err)
	}
	if attachmentID(content) != id {
		return nil, fmt.Errorf("%w: content digest mismatch", ErrCorrupt)
	}
	return content, nil
}

func verifyAttachmentDigest(reader io.Reader, id string) error {
	digest := sha256.New()
	if _, err := io.Copy(digest, reader); err != nil {
		return fmt.Errorf("hash attachment: %w", err)
	}
	got := "att_" + hex.EncodeToString(digest.Sum(nil))
	if got != id {
		return fmt.Errorf("%w: content digest mismatch", ErrCorrupt)
	}
	return nil
}

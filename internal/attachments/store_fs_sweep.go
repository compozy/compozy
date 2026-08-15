package attachments

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/fileutil"
)

type retainedAttachment struct {
	contentPath string
	id          string
	sessionID   string
	createdAt   time.Time
	bytes       int64
	pinned      bool
}

func (s *FilesystemAttachmentStore) sweepLocked(
	ctx context.Context,
	reserveCount int,
	reserveBytes int64,
) error {
	items, err := s.listAttachments(ctx)
	if err != nil {
		return err
	}
	now := s.now().UTC()
	retained := items[:0]
	pinsBySession := make(map[string]map[string]struct{})
	for index := range items {
		item := &items[index]
		if s.retentionPins != nil {
			pins, ok := pinsBySession[item.sessionID]
			if !ok {
				pins, err = s.retentionPins.PinnedAttachmentIDs(ctx, item.sessionID)
				if err != nil {
					return fmt.Errorf("read attachment retention pins for session %q: %w", item.sessionID, err)
				}
				pinsBySession[item.sessionID] = pins
			}
			_, item.pinned = pins[item.id]
		}
		if !item.pinned && now.Sub(item.createdAt) > s.retention.MaxAge {
			if err := s.removePair(item.contentPath); err != nil {
				return fmt.Errorf("%w: remove expired attachment: %w", ErrPersistence, err)
			}
			continue
		}
		retained = append(retained, *item)
	}
	slices.SortFunc(retained, compareRetainedAttachments)
	totalBytes := int64(0)
	for _, item := range retained {
		totalBytes += item.bytes
	}
	for len(retained)+reserveCount > s.retention.MaxCount || totalBytes+reserveBytes > s.retention.MaxBytes {
		if len(retained) == 0 {
			return fmt.Errorf(
				"%w: reserved attachment exceeds retention bounds",
				ErrPersistence,
			)
		}
		evictIndex := slices.IndexFunc(retained, func(item retainedAttachment) bool { return !item.pinned })
		if evictIndex < 0 {
			if reserveCount == 0 && reserveBytes == 0 {
				return nil
			}
			return fmt.Errorf("%w: retained attachments are pinned by pending input", ErrPersistence)
		}
		oldest := retained[evictIndex]
		if err := s.removePair(oldest.contentPath); err != nil {
			return fmt.Errorf("%w: evict attachment: %w", ErrPersistence, err)
		}
		totalBytes -= oldest.bytes
		retained = slices.Delete(retained, evictIndex, evictIndex+1)
	}
	return nil
}

func (s *FilesystemAttachmentStore) listAttachments(ctx context.Context) ([]retainedAttachment, error) {
	if err := attachmentContextErr(ctx); err != nil {
		return nil, err
	}
	workspaceEntries, err := os.ReadDir(s.root)
	if err != nil {
		return nil, fmt.Errorf("%w: list attachment root: %w", ErrPersistence, err)
	}
	items := make([]retainedAttachment, 0)
	for _, workspaceEntry := range workspaceEntries {
		if err := attachmentContextErr(ctx); err != nil {
			return nil, err
		}
		if !workspaceEntry.IsDir() || !scopeIDPattern.MatchString(workspaceEntry.Name()) {
			continue
		}
		workspacePath := filepath.Join(s.root, workspaceEntry.Name())
		if err := attachmentContextErr(ctx); err != nil {
			return nil, err
		}
		sessionEntries, err := os.ReadDir(workspacePath)
		if err != nil {
			return nil, fmt.Errorf("%w: list attachment workspace: %w", ErrPersistence, err)
		}
		for _, sessionEntry := range sessionEntries {
			if err := attachmentContextErr(ctx); err != nil {
				return nil, err
			}
			if !sessionEntry.IsDir() || !scopeIDPattern.MatchString(sessionEntry.Name()) {
				continue
			}
			sessionPath := filepath.Join(workspacePath, sessionEntry.Name())
			listed, err := s.listSessionAttachments(ctx, sessionPath)
			if err != nil {
				return nil, err
			}
			items = append(items, listed...)
		}
	}
	return items, nil
}

func (s *FilesystemAttachmentStore) listSessionAttachments(
	ctx context.Context,
	sessionPath string,
) ([]retainedAttachment, error) {
	if err := attachmentContextErr(ctx); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(sessionPath)
	if err != nil {
		return nil, fmt.Errorf("%w: list attachment session: %w", ErrPersistence, err)
	}
	items := make([]retainedAttachment, 0)
	for _, entry := range entries {
		if err := attachmentContextErr(ctx); err != nil {
			return nil, err
		}
		item, err := s.retainedAttachmentFromEntry(ctx, sessionPath, entry)
		if err != nil {
			return nil, err
		}
		if item != nil {
			items = append(items, *item)
		}
	}
	return items, nil
}

func (s *FilesystemAttachmentStore) retainedAttachmentFromEntry(
	ctx context.Context,
	sessionPath string,
	entry os.DirEntry,
) (*retainedAttachment, error) {
	id := entry.Name()
	if entry.IsDir() {
		return nil, nil
	}
	if isAttachmentSidecarName(id) {
		return nil, removeOrphanedAttachmentSidecar(ctx, sessionPath, id)
	}
	if !attachmentIDPattern.MatchString(id) {
		return nil, nil
	}
	return s.readRetainedAttachment(ctx, sessionPath, id, entry)
}

func isAttachmentSidecarName(name string) bool {
	return strings.HasSuffix(name, ".json") &&
		attachmentIDPattern.MatchString(strings.TrimSuffix(name, ".json"))
}

func removeOrphanedAttachmentSidecar(ctx context.Context, sessionPath string, sidecarName string) error {
	contentPath := filepath.Join(sessionPath, strings.TrimSuffix(sidecarName, ".json"))
	if _, err := os.Lstat(contentPath); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("%w: inspect attachment content for sidecar: %w", ErrPersistence, err)
	}
	if err := attachmentContextErr(ctx); err != nil {
		return err
	}
	if err := fileutil.AtomicRemoveFile(
		filepath.Join(sessionPath, sidecarName),
	); err != nil &&
		!errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("%w: remove incomplete attachment sidecar: %w", ErrPersistence, err)
	}
	return nil
}

func (s *FilesystemAttachmentStore) readRetainedAttachment(
	ctx context.Context,
	sessionPath string,
	id string,
	entry os.DirEntry,
) (*retainedAttachment, error) {
	contentPath := filepath.Join(sessionPath, id)
	info, err := entry.Info()
	if err != nil {
		return nil, fmt.Errorf("%w: stat retained attachment: %w", ErrPersistence, err)
	}
	if err := attachmentContextErr(ctx); err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("%w: retained attachment is not a regular file", ErrCorrupt)
	}
	meta, err := readSidecar(ctx, contentPath)
	if err != nil {
		removed, cleanupErr := s.removeIncompleteAttachment(contentPath, err)
		if cleanupErr != nil {
			return nil, cleanupErr
		}
		if removed {
			return nil, nil
		}
		return nil, err
	}
	if meta.SHA256 != attachmentSHA256(id) || meta.Bytes != info.Size() || meta.CreatedAt.IsZero() {
		return nil, fmt.Errorf("%w: sidecar metadata mismatch", ErrCorrupt)
	}
	return &retainedAttachment{
		contentPath: contentPath,
		id:          id,
		sessionID:   filepath.Base(sessionPath),
		createdAt:   meta.CreatedAt.UTC(),
		bytes:       info.Size(),
	}, nil
}

func (s *FilesystemAttachmentStore) removeIncompleteAttachment(
	contentPath string,
	readErr error,
) (bool, error) {
	if !errors.Is(readErr, ErrCorrupt) {
		return false, nil
	}
	if _, err := os.Lstat(metaPath(contentPath)); err == nil {
		return false, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return false, fmt.Errorf("%w: inspect attachment sidecar: %w", ErrPersistence, err)
	}
	if err := s.removePair(contentPath); err != nil && !errors.Is(err, ErrNotFound) {
		return false, fmt.Errorf("%w: remove incomplete attachment: %w", ErrPersistence, err)
	}
	return true, nil
}

func compareRetainedAttachments(left retainedAttachment, right retainedAttachment) int {
	if compared := left.createdAt.Compare(right.createdAt); compared != 0 {
		return compared
	}
	return strings.Compare(left.contentPath, right.contentPath)
}

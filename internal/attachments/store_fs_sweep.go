package attachments

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
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
		sessionEntries, err := os.ReadDir(workspacePath)
		if err != nil {
			return nil, fmt.Errorf("%w: list attachment workspace: %w", ErrPersistence, err)
		}
		for _, sessionEntry := range sessionEntries {
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
	entries, err := os.ReadDir(sessionPath)
	if err != nil {
		return nil, fmt.Errorf("%w: list attachment session: %w", ErrPersistence, err)
	}
	items := make([]retainedAttachment, 0)
	for _, entry := range entries {
		if err := attachmentContextErr(ctx); err != nil {
			return nil, err
		}
		id := entry.Name()
		if entry.IsDir() || !attachmentIDPattern.MatchString(id) {
			continue
		}
		contentPath := filepath.Join(sessionPath, id)
		info, err := entry.Info()
		if err != nil {
			return nil, fmt.Errorf("%w: stat retained attachment: %w", ErrPersistence, err)
		}
		if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("%w: retained attachment is not a regular file", ErrCorrupt)
		}
		meta, err := readSidecar(contentPath)
		if err != nil {
			return nil, err
		}
		if meta.SHA256 != attachmentSHA256(id) || meta.Bytes != info.Size() || meta.CreatedAt.IsZero() {
			return nil, fmt.Errorf("%w: sidecar metadata mismatch", ErrCorrupt)
		}
		items = append(items, retainedAttachment{
			contentPath: contentPath,
			id:          id,
			sessionID:   filepath.Base(sessionPath),
			createdAt:   meta.CreatedAt.UTC(),
			bytes:       info.Size(),
		})
	}
	return items, nil
}

func compareRetainedAttachments(left retainedAttachment, right retainedAttachment) int {
	if compared := left.createdAt.Compare(right.createdAt); compared != 0 {
		return compared
	}
	return strings.Compare(left.contentPath, right.contentPath)
}

package attachments

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/compozy/compozy/internal/diagnostics"
)

// Put publishes canonical bytes once and returns their opaque reference.
func (s *FilesystemAttachmentStore) Put(ctx context.Context, request PutRequest) (AttachmentRef, error) {
	workspaceID, sessionID, mime, kind, width, height, err := s.validatePutRequest(ctx, request)
	if err != nil {
		return AttachmentRef{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.scopeDeleted(workspaceID, sessionID) {
		return AttachmentRef{}, ErrNotFound
	}
	return s.putLocked(ctx, request, workspaceID, sessionID, mime, kind, width, height)
}

func (s *FilesystemAttachmentStore) validatePutRequest(
	ctx context.Context,
	request PutRequest,
) (string, string, string, string, int, int, error) {
	if err := attachmentContextErr(ctx); err != nil {
		return "", "", "", "", 0, 0, err
	}
	workspaceID, sessionID, err := sanitizeScopeIDs(request.WorkspaceID, request.SessionID)
	if err != nil {
		return "", "", "", "", 0, 0, err
	}
	if err := ValidateSize(int64(len(request.Data)), s.maxFileBytes); err != nil {
		return "", "", "", "", 0, 0, err
	}
	if int64(len(request.Data)) > s.retention.MaxBytes {
		return "", "", "", "", 0, 0, fmt.Errorf(
			"%w: %d exceeds retention max_bytes %d",
			ErrTooLarge,
			len(request.Data),
			s.retention.MaxBytes,
		)
	}
	mime, kind, width, height, err := SniffMIME(request.Data, request.Name)
	if err != nil {
		return "", "", "", "", 0, 0, err
	}
	if err := mimeAllowed(mime, s.allowedMIME); err != nil {
		return "", "", "", "", 0, 0, err
	}
	return workspaceID, sessionID, mime, kind, width, height, nil
}

func (s *FilesystemAttachmentStore) putLocked(
	ctx context.Context,
	request PutRequest,
	workspaceID string,
	sessionID string,
	mime string,
	kind string,
	width int,
	height int,
) (AttachmentRef, error) {
	id := attachmentID(request.Data)
	sessionDir, err := s.ensureSessionDir(ctx, workspaceID, sessionID)
	if err != nil {
		return AttachmentRef{}, err
	}
	if err := s.sweepLocked(ctx, 0, 0); err != nil {
		return AttachmentRef{}, err
	}
	contentPath := filepath.Join(sessionDir, id)
	if existing, found, err := s.existingRef(ctx, contentPath, id, request.Data); err != nil || found {
		return existing, err
	}
	if err := s.sweepLocked(ctx, 1, int64(len(request.Data))); err != nil {
		return AttachmentRef{}, err
	}
	meta := s.newAttachmentMeta(request, id, mime, kind, width, height)
	if err := s.publish(ctx, contentPath, request.Data, meta); err != nil {
		return AttachmentRef{}, err
	}
	return attachmentRef(id, meta), nil
}

func (s *FilesystemAttachmentStore) existingRef(
	ctx context.Context,
	contentPath string,
	id string,
	data []byte,
) (AttachmentRef, bool, error) {
	existing, err := s.readMeta(ctx, contentPath, id)
	if errors.Is(err, ErrNotFound) {
		return AttachmentRef{}, false, nil
	}
	if err != nil {
		return AttachmentRef{}, false, err
	}
	stored, err := readAttachmentFile(ctx, contentPath, id)
	if err != nil {
		return AttachmentRef{}, false, err
	}
	if !bytes.Equal(stored, data) {
		return AttachmentRef{}, false, fmt.Errorf("%w: digest collision for %s", ErrCorrupt, id)
	}
	return attachmentRef(id, existing), true, nil
}

func (s *FilesystemAttachmentStore) newAttachmentMeta(
	request PutRequest,
	id string,
	mime string,
	kind string,
	width int,
	height int,
) attachmentMeta {
	return attachmentMeta{
		Name:      diagnostics.Redact(strings.TrimSpace(request.Name)),
		MIMEType:  mime,
		Bytes:     int64(len(request.Data)),
		SHA256:    attachmentSHA256(id),
		Kind:      kind,
		Width:     width,
		Height:    height,
		CreatedAt: s.now().UTC(),
	}
}

func (s *FilesystemAttachmentStore) scopeDeleted(workspaceID string, sessionID string) bool {
	if _, deleted := s.deletedScopes[attachmentScopeKey(workspaceID, sessionID)]; deleted {
		return true
	}
	_, deleted := s.deletedScopes[attachmentScopeKey(workspaceID, "")]
	return deleted
}

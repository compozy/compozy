package daemon

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	attachmentspkg "github.com/compozy/compozy/internal/attachments"
	"github.com/compozy/compozy/internal/session"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

func (n *daemonNativeTools) resolveNativePromptAttachments(
	ctx context.Context,
	toolID toolspkg.ToolID,
	workspaceID string,
	sessionID string,
	values []string,
) ([]session.AttachmentMeta, error) {
	if len(values) == 0 {
		return nil, nil
	}
	maxFiles := n.deps.Config.Session.Attachments.MaxFilesPerPrompt
	if len(values) > maxFiles {
		return nil, nativeNetworkInputError(
			toolID,
			fmt.Errorf("attachments exceeds max_files_per_prompt %d", maxFiles),
		)
	}
	if n.deps.SessionAttachments == nil {
		return nil, errors.New("daemon: session attachment store is unavailable")
	}

	resolved := make([]session.AttachmentMeta, 0, len(values))
	for index, raw := range values {
		value := strings.TrimSpace(raw)
		if value == "" {
			return nil, nativeNetworkInputError(toolID, fmt.Errorf("attachments[%d] is required", index))
		}
		ref, err := n.resolveNativePromptAttachment(ctx, workspaceID, sessionID, value)
		if err != nil {
			return nil, nativeNetworkInputError(
				toolID,
				fmt.Errorf("resolve attachments[%d]: %w", index, err),
			)
		}
		resolved = append(resolved, nativeAttachmentMeta(ref))
	}
	return resolved, nil
}

func (n *daemonNativeTools) resolveNativePromptAttachment(
	ctx context.Context,
	workspaceID string,
	sessionID string,
	value string,
) (attachmentspkg.AttachmentRef, error) {
	if strings.HasPrefix(value, "att_") {
		return n.deps.SessionAttachments.Stat(ctx, workspaceID, sessionID, value)
	}
	data, err := readNativeAttachmentPath(value, n.deps.Config.Session.Attachments.MaxFileBytes)
	if err != nil {
		return attachmentspkg.AttachmentRef{}, err
	}
	return n.deps.SessionAttachments.Put(ctx, workspaceID, sessionID, filepath.Base(value), data)
}

func readNativeAttachmentPath(path string, maxBytes int64) (data []byte, retErr error) {
	if maxBytes <= 0 {
		return nil, errors.New("session.attachments.max_file_bytes must be greater than zero")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open attachment path: %w", err)
	}
	defer func() {
		retErr = errors.Join(retErr, file.Close())
	}()
	info, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat attachment path: %w", err)
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("attachment path must identify a regular file")
	}
	if err := attachmentspkg.ValidateSize(info.Size(), maxBytes); err != nil {
		return nil, err
	}
	data, err = io.ReadAll(io.LimitReader(file, maxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read attachment path: %w", err)
	}
	if err := attachmentspkg.ValidateSize(int64(len(data)), maxBytes); err != nil {
		return nil, err
	}
	return data, nil
}

func nativeAttachmentMeta(ref attachmentspkg.AttachmentRef) session.AttachmentMeta {
	return session.AttachmentMeta{
		ID:       ref.ID,
		Name:     ref.Name,
		MIMEType: ref.MIMEType,
		Bytes:    ref.Bytes,
		SHA256:   ref.SHA256,
		Kind:     ref.Kind,
		Width:    ref.Width,
		Height:   ref.Height,
	}
}

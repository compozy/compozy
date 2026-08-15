package daemon

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
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
		resolved = append(resolved, session.AttachmentMetaFromRef(ref))
	}
	return resolved, nil
}

func (n *daemonNativeTools) resolveNativePromptAttachment(
	ctx context.Context,
	workspaceID string,
	sessionID string,
	value string,
) (attachmentspkg.AttachmentRef, error) {
	if id, err := attachmentspkg.ParseAttachmentID(value); err == nil {
		return n.deps.SessionAttachments.Stat(ctx, workspaceID, sessionID, id)
	}
	data, err := readNativeAttachmentPath(ctx, value, n.deps.Config.Session.Attachments.MaxFileBytes)
	if err != nil {
		return attachmentspkg.AttachmentRef{}, err
	}
	return n.deps.SessionAttachments.Put(ctx, workspaceID, sessionID, filepath.Base(value), data)
}

func readNativeAttachmentPath(ctx context.Context, path string, maxBytes int64) (data []byte, retErr error) {
	if ctx == nil {
		return nil, errors.New("attachment path context is required")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
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
	reader := io.Reader(&nativeAttachmentContextReader{ctx: ctx, reader: file})
	if maxBytes < math.MaxInt64 {
		reader = io.LimitReader(reader, maxBytes+1)
	}
	data, err = io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("read attachment path: %w", err)
	}
	if err := attachmentspkg.ValidateSize(int64(len(data)), maxBytes); err != nil {
		return nil, err
	}
	return data, nil
}

type nativeAttachmentContextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r *nativeAttachmentContextReader) Read(buffer []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(buffer)
}

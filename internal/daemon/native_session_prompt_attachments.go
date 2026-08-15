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
	registrypkg "github.com/compozy/compozy/internal/registry"
	"github.com/compozy/compozy/internal/session"
	skillmarketplace "github.com/compozy/compozy/internal/skills/marketplace"
	toolspkg "github.com/compozy/compozy/internal/tools"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

func (n *daemonNativeTools) resolveNativePromptAttachments(
	ctx context.Context,
	toolID toolspkg.ToolID,
	workspaceID string,
	workspaceRoots []string,
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
		ref, err := n.resolveNativePromptAttachment(
			ctx,
			workspaceID,
			workspaceRoots,
			sessionID,
			value,
		)
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
	workspaceRoots []string,
	sessionID string,
	value string,
) (attachmentspkg.AttachmentRef, error) {
	if id, err := attachmentspkg.ParseAttachmentID(value); err == nil {
		return n.deps.SessionAttachments.Stat(ctx, workspaceID, sessionID, id)
	}
	name, data, err := readNativeAttachmentPath(
		ctx,
		workspaceRoots,
		value,
		n.deps.Config.Session.Attachments.MaxFileBytes,
	)
	if err != nil {
		return attachmentspkg.AttachmentRef{}, err
	}
	return n.deps.SessionAttachments.Put(ctx, attachmentspkg.PutRequest{
		WorkspaceID: workspaceID,
		SessionID:   sessionID,
		Name:        name,
		Data:        data,
	})
}

func readNativeAttachmentPath(
	ctx context.Context,
	workspaceRoots []string,
	path string,
	maxBytes int64,
) (name string, data []byte, retErr error) {
	if ctx == nil {
		return "", nil, errors.New("attachment path context is required")
	}
	if err := ctx.Err(); err != nil {
		return "", nil, err
	}
	if maxBytes <= 0 {
		return "", nil, errors.New("session.attachments.max_file_bytes must be greater than zero")
	}
	resolvedPath, err := resolveNativeAttachmentPath(workspaceRoots, path)
	if err != nil {
		return "", nil, err
	}
	file, err := os.Open(resolvedPath)
	if err != nil {
		return "", nil, fmt.Errorf("open attachment path: %w", err)
	}
	defer func() {
		retErr = errors.Join(retErr, file.Close())
	}()
	info, err := file.Stat()
	if err != nil {
		return "", nil, fmt.Errorf("stat attachment path: %w", err)
	}
	if !info.Mode().IsRegular() {
		return "", nil, errors.New("attachment path must identify a regular file")
	}
	if err := attachmentspkg.ValidateSize(info.Size(), maxBytes); err != nil {
		return "", nil, err
	}
	reader := io.Reader(&nativeAttachmentContextReader{ctx: ctx, reader: file})
	if maxBytes < math.MaxInt64 {
		reader = io.LimitReader(reader, maxBytes+1)
	}
	data, err = io.ReadAll(reader)
	if err != nil {
		return "", nil, fmt.Errorf("read attachment path: %w", err)
	}
	if err := attachmentspkg.ValidateSize(int64(len(data)), maxBytes); err != nil {
		return "", nil, err
	}
	return filepath.Base(resolvedPath), data, nil
}

func resolveNativeAttachmentPath(workspaceRoots []string, path string) (string, error) {
	for _, root := range workspaceRoots {
		resolvedPath, err := skillmarketplace.PathInsideRoot(root, path)
		switch {
		case err == nil:
			return resolvedPath, nil
		case errors.Is(err, registrypkg.ErrPathOutsideRoot):
			continue
		default:
			return "", fmt.Errorf("resolve attachment path: %w", err)
		}
	}
	return "", errors.New("attachment path must be inside the workspace")
}

func nativeWorkspaceAttachmentRoots(resolved workspacepkg.ResolvedWorkspace) []string {
	roots := make([]string, 0, 1+len(resolved.AdditionalDirs))
	for _, root := range append([]string{resolved.RootDir}, resolved.AdditionalDirs...) {
		if root = strings.TrimSpace(root); root != "" {
			roots = append(roots, root)
		}
	}
	return roots
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

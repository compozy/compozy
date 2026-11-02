package daemon

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"syscall"

	"github.com/compozy/compozy/internal/heartbeat"
)

const workLogReadLimit = 64 * 1024

// openValidatedWorkspaceRoot refuses a symlink swapped in after validation, which OpenRoot would follow.
func openValidatedWorkspaceRoot(path string, validated os.FileInfo, logger *slog.Logger) (*os.Root, bool) {
	workspace, err := os.OpenRoot(path)
	if err != nil {
		logger.Warn("work_log: open workspace", "error", err)
		return nil, false
	}
	opened, err := workspace.Stat(".")
	if err == nil && os.SameFile(validated, opened) {
		return workspace, true
	}
	logger.Warn("work_log: path outside workspace", "error", err)
	if closeErr := workspace.Close(); closeErr != nil {
		logger.Warn("work_log: close workspace", "error", closeErr)
	}
	return nil, false
}

func readWorkspaceWorkLog(root string, logger *slog.Logger) ([]heartbeat.WorkLogFact, bool) {
	if logger == nil {
		logger = slog.Default()
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		logger.Warn("work_log: resolve workspace", "error", err)
		return nil, false
	}
	validatedRoot, err := os.Stat(resolvedRoot)
	if err != nil {
		logger.Warn("work_log: stat workspace", "error", err)
		return nil, false
	}
	path, err := realpathDeepestExistingPath(filepath.Join(resolvedRoot, ".compozy", "work-log.md"))
	if err != nil {
		logger.Warn("work_log: resolve path", "error", err)
		return nil, false
	}
	if err := requirePathWithinResolvedRoot(resolvedRoot, path); err != nil {
		logger.Warn("work_log: path outside workspace")
		return nil, false
	}
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, false
	}
	if err != nil || !info.Mode().IsRegular() {
		logger.Warn("work_log: unavailable regular file", "error", err)
		return nil, false
	}
	workspace, ok := openValidatedWorkspaceRoot(resolvedRoot, validatedRoot, logger)
	if !ok {
		return nil, false
	}
	defer func() {
		if err := workspace.Close(); err != nil {
			logger.Warn("work_log: close workspace", "error", err)
		}
	}()
	file, err := openWorkspaceWorkLogFile(workspace)
	if errors.Is(err, os.ErrNotExist) {
		return nil, false
	}
	if err != nil {
		logger.Warn("work_log: open", "error", err)
		return nil, false
	}
	defer func() {
		if err := file.Close(); err != nil {
			logger.Warn("work_log: close", "error", err)
		}
	}()
	info, err = file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		logger.Warn("work_log: unavailable regular file", "error", err)
		return nil, false
	}
	content, err := io.ReadAll(io.LimitReader(file, workLogReadLimit+1))
	if err != nil {
		logger.Warn("work_log: read", "error", err)
		return nil, false
	}
	truncated := len(content) > workLogReadLimit
	if truncated {
		content = content[:workLogReadLimit]
		if end := bytes.LastIndexByte(content, '\n'); end >= 0 {
			content = content[:end+1]
		} else {
			content = nil
		}
	}
	facts, malformed := heartbeat.ParseWorkLog(string(content))
	for _, line := range malformed {
		logger.Warn(fmt.Sprintf("work_log: ignored line %d", line))
	}
	return facts, truncated
}

func openWorkspaceWorkLogFile(workspace *os.Root) (*os.File, error) {
	return workspace.OpenFile(".compozy/work-log.md", os.O_RDONLY|syscall.O_NONBLOCK, 0)
}

package daemon

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/compozy/compozy/internal/heartbeat"
)

const workLogReadLimit = 64 * 1024

func readWorkspaceWorkLog(root string, logger *slog.Logger) ([]heartbeat.WorkLogFact, bool) {
	if logger == nil {
		logger = slog.Default()
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		logger.Warn("work_log: resolve workspace", "error", err)
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
	file, err := os.Open(path)
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
	content, err := io.ReadAll(io.LimitReader(file, workLogReadLimit))
	if err != nil {
		logger.Warn("work_log: read", "error", err)
		return nil, false
	}
	truncated := info.Size() > workLogReadLimit
	if truncated {
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

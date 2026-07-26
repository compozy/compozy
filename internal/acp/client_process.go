package acp

import (
	"context"

	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"

	"strings"
	"time"

	aghconfig "github.com/compozy/agh/internal/config"

	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/toolruntime"
)

func (p *AgentProcess) waitForExit(ctx context.Context, processRecordTimeout time.Duration) {
	var waitErr error
	var groupWaitErr error
	switch {
	case p.handle != nil:
		waitErr = p.handle.Wait()
	case p.managed != nil:
		waitErr = p.managed.Wait()
	case p.cmd != nil:
		waitErr = p.cmd.Wait()
		if p.stopWasRequested() {
			groupWaitErr = forceManagedProcessGroupExit(p.cmd, time.Second)
		}
	default:
		waitErr = nil
	}
	if p.stopWasRequested() {
		waitErr = nil
		if groupWaitErr != nil {
			waitErr = fmt.Errorf("acp: wait for subprocess tree exit: %w", groupWaitErr)
		}
	} else if waitErr != nil {
		waitErr = WrapFailure(
			store.FailureProcess,
			"ACP subprocess exited unexpectedly",
			fmt.Errorf("acp: subprocess exited: %w", attachStderr(waitErr, p.Stderr())),
		)
	}
	p.setWaitError(waitErr)
	if p.processRecord != nil {
		recordCtx, cancelRecord := processRecordContext(ctx, processRecordTimeout)
		err := p.processRecord.Complete(recordCtx, toolruntime.ProcessCompletion{Err: waitErr})
		cancelRecord()
		if err != nil {
			slog.Default().Warn("acp: complete process record", "pid", p.PID, "error", err)
		}
	}
	if p.cancelProcess != nil {
		p.cancelProcess()
	}
	if p.terminals != nil {
		p.terminals.closeAll()
	}
	close(p.done)
}

func processRecordContext(parent context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		timeout = defaultProcessRecordTimeout
	}
	if parent == nil {
		parent = context.Background()
	}
	return context.WithTimeout(context.WithoutCancel(parent), timeout)
}

func normalizeStartOpts(opts StartOpts) (StartOpts, error) {
	if err := opts.Validate(); err != nil {
		return StartOpts{}, err
	}

	cwd, err := normalizeWorkspaceDir(opts.Cwd, "cwd")
	if err != nil {
		return StartOpts{}, err
	}

	normalized := opts
	normalized.Cwd = cwd
	additionalDirs, err := normalizeAdditionalDirs(cwd, opts.AdditionalDirs)
	if err != nil {
		return StartOpts{}, err
	}
	normalized.AdditionalDirs = additionalDirs
	if normalized.Permissions == "" {
		normalized.Permissions = aghconfig.PermissionModeApproveReads
	}
	if normalized.AdditionalDirs != nil {
		normalized.AdditionalDirs = append([]string(nil), normalized.AdditionalDirs...)
	}
	if normalized.Env != nil {
		normalized.Env = append([]string(nil), normalized.Env...)
	}
	if normalized.MCPServers != nil {
		normalized.MCPServers = append([]aghconfig.MCPServer(nil), normalized.MCPServers...)
	}
	normalized.SystemPrompt = strings.TrimSpace(normalized.SystemPrompt)
	if strings.TrimSpace(normalized.SystemPrompt) == "" {
		normalized.SystemPromptDelivery = ""
	} else if normalized.SystemPromptDelivery == "" {
		normalized.SystemPromptDelivery = SystemPromptDeliveryFirstTurnPrefix
	}
	normalized.PreferredModel = strings.TrimSpace(normalized.PreferredModel)
	normalized.ReasoningEffort = strings.TrimSpace(normalized.ReasoningEffort)

	return normalized, nil
}

func daemonMatchedEnv(base []string) []string {
	env := append([]string(nil), base...)
	if len(env) == 0 {
		env = os.Environ()
	}

	executable, err := os.Executable()
	if err != nil {
		return env
	}
	if resolved, resolveErr := filepath.EvalSymlinks(
		executable,
	); resolveErr == nil &&
		strings.TrimSpace(resolved) != "" {
		executable = resolved
	}
	executable = strings.TrimSpace(executable)
	if executable == "" {
		return env
	}

	env = setEnvValue(env, "AGH_BIN", executable)

	binDir := strings.TrimSpace(filepath.Dir(executable))
	if binDir == "" {
		return env
	}

	pathValue, _ := envValue(env, "PATH")
	env = setEnvValue(env, "PATH", prependPathEntry(pathValue, binDir))
	return env
}

func prependPathEntry(pathValue string, entry string) string {
	cleanEntry := strings.TrimSpace(entry)
	if cleanEntry == "" {
		return pathValue
	}

	separator := string(os.PathListSeparator)
	segments := strings.Split(pathValue, separator)
	filtered := make([]string, 0, len(segments))
	for _, segment := range segments {
		trimmed := strings.TrimSpace(segment)
		if trimmed == "" || trimmed == cleanEntry {
			continue
		}
		filtered = append(filtered, trimmed)
	}
	return strings.Join(append([]string{cleanEntry}, filtered...), separator)
}

func envValue(env []string, key string) (string, bool) {
	prefix := key + "="
	for _, variable := range slices.Backward(env) {
		if strings.HasPrefix(variable, prefix) {
			return variable[len(prefix):], true
		}
	}
	return "", false
}

func setEnvValue(env []string, key string, value string) []string {
	prefix := key + "="
	entry := prefix + value
	filtered := env[:0]
	for _, variable := range env {
		if strings.HasPrefix(variable, prefix) {
			continue
		}
		filtered = append(filtered, variable)
	}
	return append(filtered, entry)
}

func normalizeWorkspaceDir(path string, field string) (string, error) {
	target := strings.TrimSpace(path)
	absPath, err := filepath.Abs(target)
	if err != nil {
		return "", fmt.Errorf("acp: resolve %s %q: %w", field, path, err)
	}
	info, err := os.Stat(absPath)
	if err != nil {
		return "", fmt.Errorf("acp: stat %s %q: %w", field, absPath, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("acp: %s %q is not a directory", field, absPath)
	}
	resolvedPath, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		return "", fmt.Errorf("acp: evaluate %s %q: %w", field, absPath, err)
	}
	canonicalPath, err := filepath.Abs(resolvedPath)
	if err != nil {
		return "", fmt.Errorf("acp: resolve canonical %s %q: %w", field, resolvedPath, err)
	}
	return filepath.Clean(canonicalPath), nil
}

func normalizeAdditionalDirs(rootDir string, dirs []string) ([]string, error) {
	if len(dirs) == 0 {
		return nil, nil
	}

	normalized := make([]string, 0, len(dirs))
	seen := make(map[string]struct{}, len(dirs))

	for i, dir := range dirs {
		trimmed := strings.TrimSpace(dir)
		if trimmed == "" {
			continue
		}

		canonicalDir, err := normalizeWorkspaceDir(trimmed, fmt.Sprintf("additional_dirs[%d]", i))
		if err != nil {
			return nil, err
		}
		if canonicalDir == rootDir {
			continue
		}
		if _, ok := seen[canonicalDir]; ok {
			continue
		}

		seen[canonicalDir] = struct{}{}
		normalized = append(normalized, canonicalDir)
	}

	return normalized, nil
}

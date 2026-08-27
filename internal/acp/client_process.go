package acp

import (
	"context"
	"errors"

	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"

	"strings"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/fileutil"

	speedpkg "github.com/compozy/compozy/internal/speed"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/subprocess"
	"github.com/compozy/compozy/internal/toolruntime"
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
		groupWaitErr = forceManagedProcessGroupExit(p.cmd, time.Second)
	default:
		waitErr = nil
	}
	if p.stopWasRequested() {
		waitErr = nil
		if groupWaitErr != nil {
			waitErr = fmt.Errorf("acp: wait for subprocess tree exit: %w", groupWaitErr)
		}
	} else {
		if waitErr != nil {
			waitErr = WrapFailure(
				store.FailureProcess,
				"ACP subprocess exited unexpectedly",
				fmt.Errorf("acp: subprocess exited: %w", attachStderr(waitErr, p.Stderr())),
			)
		}
		if groupWaitErr != nil {
			waitErr = errors.Join(waitErr, fmt.Errorf("acp: release subprocess tree: %w", groupWaitErr))
		}
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
	p.closeAndWaitChildTasks()
	if p.terminals != nil {
		p.terminals.closeAll()
	}
	close(p.done)
}

func processRecordContext(parent context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		timeout = defaultProcessRecordTimeout
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
	normalized.launchIdentity = clonePreparedLaunchIdentity(opts.launchIdentity)
	normalized.Cwd = cwd
	additionalDirs, err := normalizeAdditionalDirs(cwd, opts.AdditionalDirs)
	if err != nil {
		return StartOpts{}, err
	}
	normalized.AdditionalDirs = additionalDirs
	if normalized.Permissions == "" {
		normalized.Permissions = compozyconfig.PermissionModeApproveReads
	}
	if normalized.AdditionalDirs != nil {
		normalized.AdditionalDirs = append([]string(nil), normalized.AdditionalDirs...)
	}
	if normalized.Env != nil {
		normalized.Env = append([]string(nil), normalized.Env...)
	}
	if normalized.MCPServers != nil {
		normalized.MCPServers = append([]compozyconfig.MCPServer(nil), normalized.MCPServers...)
	}
	normalized.ACPOptions, err = normalizeSessionConfigOptionSelections(normalized.ACPOptions)
	if err != nil {
		return StartOpts{}, fmt.Errorf("acp: normalize start ACP options: %w", err)
	}
	normalized.SystemPrompt = strings.TrimSpace(normalized.SystemPrompt)
	if strings.TrimSpace(normalized.SystemPrompt) == "" {
		normalized.SystemPromptDelivery = ""
	} else if normalized.SystemPromptDelivery == "" {
		normalized.SystemPromptDelivery = SystemPromptDeliveryFirstTurnPrefix
	}
	normalized.PreferredModel = strings.TrimSpace(normalized.PreferredModel)
	normalized.ExpectedTransportModel = strings.TrimSpace(normalized.ExpectedTransportModel)
	if normalized.RuntimeStrategy == "" {
		normalized.RuntimeStrategy = RuntimeApplicationSessionConfig
	}
	normalized.ReasoningEffort = strings.TrimSpace(normalized.ReasoningEffort)
	if normalized.Speed != "" {
		parsedSpeed, parseErr := speedpkg.Parse(string(normalized.Speed))
		if parseErr != nil {
			return StartOpts{}, fmt.Errorf("acp: validate speed: %w", parseErr)
		}
		normalized.Speed = parsedSpeed
	}

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

	env = setEnvValue(env, "COMPOZY_BIN", executable)

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
		if trimmed == "" || samePathEntry(trimmed, cleanEntry) {
			continue
		}
		filtered = append(filtered, trimmed)
	}
	return strings.Join(append([]string{cleanEntry}, filtered...), separator)
}

func samePathEntry(left string, right string) bool {
	left = filepath.Clean(left)
	right = filepath.Clean(right)
	if runtime.GOOS == terminalWindowsKey {
		return strings.EqualFold(left, right)
	}
	return left == right
}

func envValue(env []string, key string) (string, bool) {
	return subprocess.LookupEnv(env, key)
}

func setEnvValue(env []string, key string, value string) []string {
	return subprocess.SetEnvValue(env, key, value)
}

func normalizeWorkspaceDir(path string, field string) (string, error) {
	canonicalPath, err := fileutil.CanonicalExistingDirectory(path)
	if err != nil {
		return "", fmt.Errorf("acp: canonicalize %s %q: %w", field, path, err)
	}
	return canonicalPath, nil
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

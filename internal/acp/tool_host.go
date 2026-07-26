package acp

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	acpsdk "github.com/coder/acp-go-sdk"
	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/sandbox"
	"github.com/compozy/agh/internal/toolruntime"
)

// ToolHost abstracts ACP file, permission, and terminal operations for a runtime.
type ToolHost = sandbox.ToolHost

type permissionOperation = sandbox.PermissionOperation

const (
	permissionReadTextFile     = sandbox.PermissionOperationReadTextFile
	permissionWriteTextFile    = sandbox.PermissionOperationWriteTextFile
	permissionCreateTerminal   = sandbox.PermissionOperationCreateTerminal
	permissionRequestToolGrant = sandbox.PermissionOperationRequestToolGrant
)

type permissionDecision = sandbox.PermissionDecision

const (
	decisionPending      = sandbox.PermissionDecisionPending
	decisionAllowOnce    = sandbox.PermissionDecisionAllowOnce
	decisionAllowAlways  = sandbox.PermissionDecisionAllowAlways
	decisionRejectOnce   = sandbox.PermissionDecisionRejectOnce
	decisionRejectAlways = sandbox.PermissionDecisionRejectAlways
)

var _ sandbox.ToolHost = (*localToolHost)(nil)

type localToolHost struct {
	cwd         string
	permissions permissionPolicy
	terminals   *terminalManager
}

type localRuntimeConfig struct {
	processRegistry *toolruntime.Registry
	additionalRoots []string
}

// LocalRuntimeOption customizes local ACP runtime helpers.
type LocalRuntimeOption func(*localRuntimeConfig)

// WithLocalProcessRegistry injects the shared tool process registry.
func WithLocalProcessRegistry(registry *toolruntime.Registry) LocalRuntimeOption {
	return func(cfg *localRuntimeConfig) {
		cfg.processRegistry = registry
	}
}

// WithLocalAdditionalRoots authorizes local tool-host paths outside the primary root.
func WithLocalAdditionalRoots(roots ...string) LocalRuntimeOption {
	snapshot := append([]string(nil), roots...)
	return func(cfg *localRuntimeConfig) {
		cfg.additionalRoots = append(cfg.additionalRoots, snapshot...)
	}
}

// NewLocalToolHost returns the local daemon-host file, permission, and terminal host.
func NewLocalToolHost(
	ctx context.Context,
	root string,
	mode aghconfig.PermissionMode,
	logger *slog.Logger,
	opts ...LocalRuntimeOption,
) (sandbox.ToolHost, error) {
	return newLocalToolHost(ctx, root, mode, logger, opts...)
}

func newLocalToolHost(
	ctx context.Context,
	root string,
	mode aghconfig.PermissionMode,
	logger *slog.Logger,
	opts ...LocalRuntimeOption,
) (*localToolHost, error) {
	cfg := localRuntimeConfig{}
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}
	policy, err := newPermissionPolicy(mode, root, cfg.additionalRoots...)
	if err != nil {
		return nil, err
	}
	return newLocalToolHostFromPolicy(ctx, root, policy, logger, opts...), nil
}

func newLocalToolHostFromPolicy(
	ctx context.Context,
	root string,
	policy permissionPolicy,
	logger *slog.Logger,
	opts ...LocalRuntimeOption,
) *localToolHost {
	if ctx == nil {
		ctx = context.Background()
	}
	if logger == nil {
		logger = slog.Default()
	}
	cfg := localRuntimeConfig{}
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}
	return &localToolHost{
		cwd:         root,
		permissions: policy,
		terminals:   newTerminalManager(ctx, logger, cfg.processRegistry),
	}
}

func (h *localToolHost) ReadTextFile(_ context.Context, path string) (string, error) {
	if err := h.Authorize(permissionReadTextFile); err != nil {
		return "", err
	}
	resolvedPath, err := h.ResolvePath(path)
	if err != nil {
		return "", err
	}
	content, err := os.ReadFile(resolvedPath)
	if err != nil {
		return "", fmt.Errorf("acp: read %q: %w", resolvedPath, err)
	}
	return string(content), nil
}

func (h *localToolHost) WriteTextFile(_ context.Context, path string, content string) error {
	if err := h.Authorize(permissionWriteTextFile); err != nil {
		return err
	}
	resolvedPath, err := h.ResolvePath(path)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(resolvedPath), 0o755); err != nil {
		return fmt.Errorf("acp: create parent directories for %q: %w", resolvedPath, err)
	}
	if err := os.WriteFile(resolvedPath, []byte(content), 0o600); err != nil {
		return fmt.Errorf("acp: write %q: %w", resolvedPath, err)
	}
	return nil
}

func (h *localToolHost) ResolvePath(path string) (string, error) {
	return h.permissions.resolvePath(path)
}

func (h *localToolHost) Authorize(op sandbox.PermissionOperation) error {
	return h.permissions.authorize(op)
}

func (h *localToolHost) PermissionDecision(
	req acpsdk.RequestPermissionRequest,
) (sandbox.PermissionDecision, bool) {
	return h.permissions.permissionDecision(req)
}

func (h *localToolHost) CreateTerminal(
	ctx context.Context,
	req acpsdk.CreateTerminalRequest,
) (acpsdk.CreateTerminalResponse, error) {
	return h.createTerminal(ctx, req, terminalOwnership{})
}

func (h *localToolHost) createTerminal(
	ctx context.Context,
	req acpsdk.CreateTerminalRequest,
	ownership terminalOwnership,
) (acpsdk.CreateTerminalResponse, error) {
	if err := h.Authorize(permissionCreateTerminal); err != nil {
		return acpsdk.CreateTerminalResponse{}, err
	}
	cwd := h.cwd
	if req.Cwd != nil {
		cwd = *req.Cwd
	}
	resolvedCwd, err := h.ResolvePath(cwd)
	if err != nil {
		return acpsdk.CreateTerminalResponse{}, err
	}
	return h.terminals.create(ctx, resolvedCwd, req, ownership)
}

func (h *localToolHost) KillTerminal(id string) error {
	return h.terminals.kill(id)
}

func (h *localToolHost) TerminalOutput(id string) (string, error) {
	output, _, _, err := h.terminalOutputStatus(id)
	return output, err
}

func (h *localToolHost) terminalOutputStatus(
	id string,
) (string, bool, *acpsdk.TerminalExitStatus, error) {
	return h.terminals.output(id)
}

func (h *localToolHost) WaitForTerminalExit(ctx context.Context, id string) (int, error) {
	exitStatus, err := h.waitForTerminalExitStatus(ctx, id)
	if err != nil {
		return 0, err
	}
	if exitStatus == nil {
		return 1, fmt.Errorf("acp: terminal %q exited without an exit code", id)
	}
	if exitStatus.ExitCode == nil {
		if exitStatus.Signal != nil && strings.TrimSpace(*exitStatus.Signal) != "" {
			return 1, fmt.Errorf("acp: terminal %q exited due to signal: %s", id, strings.TrimSpace(*exitStatus.Signal))
		}
		return 1, fmt.Errorf("acp: terminal %q exited without an exit code", id)
	}
	return *exitStatus.ExitCode, nil
}

func (h *localToolHost) waitForTerminalExitStatus(
	ctx context.Context,
	id string,
) (*acpsdk.TerminalExitStatus, error) {
	return h.terminals.wait(ctx, id)
}

func (h *localToolHost) ReleaseTerminal(id string) error {
	return h.terminals.release(id)
}

func (h *localToolHost) terminalOwnership(id string) (terminalOwnership, error) {
	term, err := h.terminals.lookup(id)
	if err != nil {
		return terminalOwnership{}, err
	}
	return terminalOwnership{
		networkOwned:   term.networkOwned,
		ownerSessionID: term.ownerSessionID,
		ownerTurnID:    term.ownerTurnID,
	}, nil
}

func (h *localToolHost) Close() {
	if h == nil || h.terminals == nil {
		return
	}
	h.terminals.closeAll()
}

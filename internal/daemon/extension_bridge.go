package daemon

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strings"

	extensions "github.com/compozy/compozy/internal/core/extension"
	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/reviews"
)

const daemonExtensionPresentationMode = "detach"

type extensionBridge struct {
	runManager    *RunManager
	workspaceRoot string
	token         string
}

var _ extensions.DaemonHostBridge = (*extensionBridge)(nil)

func newExtensionBridge(runManager *RunManager, workspaceRoot string) (*extensionBridge, error) {
	if runManager == nil {
		return nil, fmt.Errorf("daemon: extension bridge run manager is required")
	}

	token, err := newExtensionHostCapabilityToken()
	if err != nil {
		return nil, err
	}

	return &extensionBridge{
		runManager:    runManager,
		workspaceRoot: strings.TrimSpace(workspaceRoot),
		token:         token,
	}, nil
}

func (b *extensionBridge) HostCapabilityToken() string {
	if b == nil {
		return ""
	}
	return strings.TrimSpace(b.token)
}

func (b *extensionBridge) StartRun(
	ctx context.Context,
	runtimeCfg *model.RuntimeConfig,
) (*extensions.RunHandle, error) {
	normalized, err := b.normalizeRuntime(runtimeCfg)
	if err != nil {
		return nil, err
	}

	var runID string
	switch normalized.Mode {
	case model.ExecutionModePRDTasks:
		runID, err = b.startTaskRun(ctx, normalized)
	case model.ExecutionModePRReview:
		runID, err = b.startReviewRun(ctx, normalized)
	case model.ExecutionModeExec:
		runID, err = b.startExecRun(ctx, normalized)
	default:
		err = fmt.Errorf("daemon: unsupported child run mode %q", normalized.Mode)
	}
	if err != nil {
		return nil, err
	}

	return &extensions.RunHandle{
		RunID:       runID,
		ParentRunID: strings.TrimSpace(normalized.ParentRunID),
	}, nil
}

func (b *extensionBridge) normalizeRuntime(runtimeCfg *model.RuntimeConfig) (*model.RuntimeConfig, error) {
	if b == nil {
		return nil, fmt.Errorf("daemon: extension bridge is required")
	}
	if runtimeCfg == nil {
		return nil, fmt.Errorf("daemon: child runtime config is required")
	}

	normalized := runtimeCfg.Clone()
	if normalized == nil {
		return nil, fmt.Errorf("daemon: child runtime config is required")
	}
	if strings.TrimSpace(normalized.WorkspaceRoot) == "" {
		normalized.WorkspaceRoot = b.workspaceRoot
	}
	normalized.ApplyDefaults()
	normalized.TUI = false
	normalized.DaemonOwned = true
	if normalized.Mode == model.ExecutionModeExec {
		normalized.Persist = true
	}

	if err := validateDaemonRuntimeConfig(normalized); err != nil {
		return nil, err
	}
	return normalized, nil
}

func (b *extensionBridge) startTaskRun(ctx context.Context, runtimeCfg *model.RuntimeConfig) (string, error) {
	slug := strings.TrimSpace(runtimeCfg.Name)
	if slug == "" {
		return "", fmt.Errorf("daemon: child task run workflow name is required")
	}
	if strings.TrimSpace(runtimeCfg.TasksDir) == "" {
		runtimeCfg.TasksDir = model.TaskDirectoryForWorkspace(runtimeCfg.WorkspaceRoot, slug)
	}
	if err := requireDirectory(runtimeCfg.TasksDir); err != nil {
		return "", err
	}

	workspaceRow, workflowID, _, err := b.runManager.resolveWorkflowContext(
		detachContext(ctx),
		runtimeCfg.WorkspaceRoot,
		slug,
	)
	if err != nil {
		return "", err
	}

	run, err := b.runManager.startRun(ctx, startRunSpec{
		workspace:        workspaceRow,
		workflowID:       workflowID,
		workflowSlug:     slug,
		workflowRoot:     runtimeCfg.TasksDir,
		mode:             runModeTask,
		presentationMode: daemonExtensionPresentationMode,
		runtimeCfg:       runtimeCfg,
	})
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(run.RunID), nil
}

func (b *extensionBridge) startReviewRun(ctx context.Context, runtimeCfg *model.RuntimeConfig) (string, error) {
	slug := strings.TrimSpace(runtimeCfg.Name)
	if slug == "" {
		return "", fmt.Errorf("daemon: child review run workflow name is required")
	}
	if runtimeCfg.Round <= 0 {
		return "", fmt.Errorf("daemon: child review run round must be positive")
	}
	if strings.TrimSpace(runtimeCfg.ReviewsDir) == "" {
		runtimeCfg.ReviewsDir = reviewDirForWorkflow(runtimeCfg.WorkspaceRoot, slug, runtimeCfg.Round)
	}
	if err := requireDirectory(runtimeCfg.ReviewsDir); err != nil {
		return "", err
	}

	workspaceRow, workflowID, _, err := b.runManager.resolveWorkflowContext(
		detachContext(ctx),
		runtimeCfg.WorkspaceRoot,
		slug,
	)
	if err != nil {
		return "", err
	}

	run, err := b.runManager.startRun(ctx, startRunSpec{
		workspace:        workspaceRow,
		workflowID:       workflowID,
		workflowSlug:     slug,
		workflowRoot:     filepath.Dir(runtimeCfg.ReviewsDir),
		mode:             runModeReview,
		presentationMode: daemonExtensionPresentationMode,
		runtimeCfg:       runtimeCfg,
	})
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(run.RunID), nil
}

func (b *extensionBridge) startExecRun(ctx context.Context, runtimeCfg *model.RuntimeConfig) (string, error) {
	workspaceRow, err := b.runManager.globalDB.ResolveOrRegister(detachContext(ctx), runtimeCfg.WorkspaceRoot)
	if err != nil {
		return "", err
	}

	run, err := b.runManager.startRun(ctx, startRunSpec{
		workspace:        workspaceRow,
		mode:             runModeExec,
		presentationMode: daemonExtensionPresentationMode,
		runtimeCfg:       runtimeCfg,
	})
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(run.RunID), nil
}

func newExtensionHostCapabilityToken() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", fmt.Errorf("daemon: generate extension host capability token: %w", err)
	}
	return hex.EncodeToString(raw[:]), nil
}

func reviewDirForWorkflow(workspaceRoot string, slug string, round int) string {
	return filepath.Join(model.TaskDirectoryForWorkspace(workspaceRoot, slug), reviews.RoundDirName(round))
}

package acp

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/diagnostics"
	authproviders "github.com/compozy/compozy/internal/providers"
	"github.com/compozy/compozy/internal/store"
)

const (
	startOutcomeSucceeded = "succeeded"
	startOutcomeSkipped   = "skipped"
	startOutcomeFailed    = "failed"
)

// ProviderPreStarter classifies provider readiness before ACP process launch.
// The daemon owns the concrete cache instance and injects it at composition.
type ProviderPreStarter interface {
	PreStart(context.Context, compozyconfig.ProviderConfig, *authproviders.ProbeEnv) authproviders.PreStartReport
}

// Start launches a subprocess, completes ACP initialization, and creates or resumes a session.
func (d *Driver) Start(ctx context.Context, opts StartOpts) (process *AgentProcess, startErr error) {
	startedAt := time.Now()
	defer func() {
		outcome := startOutcomeSucceeded
		if startErr != nil {
			outcome = startOutcomeFailed
		}
		d.logStartStage(opts, process, "total", outcome, startedAt)
	}()

	if ctx == nil {
		return nil, errors.New("acp: context is required")
	}

	normalized, err := normalizeStartOpts(opts)
	if err != nil {
		return nil, WrapFailure(store.FailureStartup, "invalid ACP start options", err)
	}
	normalized, launchIdentityErr := d.prepareLaunchIdentity(ctx, normalized)
	if launchIdentityErr != nil && ctx.Err() != nil {
		return nil, ctx.Err()
	}

	stageStartedAt := time.Now()
	err = d.runProviderPreStart(ctx, normalized)
	d.logStartStage(
		normalized,
		nil,
		"provider_pre_start",
		stageOutcome(err, normalized.ProviderName == ""),
		stageStartedAt,
	)
	if err != nil {
		return nil, err
	}
	if launchIdentityErr != nil {
		return nil, WrapFailure(
			store.FailureStartup,
			"agent executable resolution failed",
			launchIdentityErr,
		)
	}

	stageStartedAt = time.Now()
	err = validateReasoningApplication(normalized)
	d.logStartStage(
		normalized,
		nil,
		"reasoning_validation",
		stageOutcome(err, normalized.ReasoningEffort == ""),
		stageStartedAt,
	)
	if err != nil {
		return nil, WrapFailure(store.FailureProtocol, "ACP reasoning strategy validation failed", err)
	}

	stageStartedAt = time.Now()
	process, err = d.launchAgentProcess(ctx, normalized)
	d.logStartStage(normalized, process, "process_launch", stageOutcome(err, false), stageStartedAt)
	if err != nil {
		return nil, WrapFailure(store.FailureStartup, "agent subprocess startup failed", err)
	}

	stageStartedAt = time.Now()
	err = d.initializeConnection(ctx, process, normalized.AgentName)
	d.logStartStage(normalized, process, "initialize", stageOutcome(err, false), stageStartedAt)
	if err != nil {
		return nil, d.cleanupFailedStart(process, err)
	}
	stageStartedAt = time.Now()
	err = activateMCPServers(ctx, normalized)
	d.logStartStage(normalized, process, "mcp_activation", stageOutcome(err, normalized.ActivateMCPServers == nil), stageStartedAt)
	if err != nil {
		return nil, d.cleanupFailedStart(process, err)
	}
	if err := d.negotiateSession(ctx, process, normalized); err != nil {
		return nil, d.cleanupFailedStart(process, err)
	}
	return process, nil
}

func activateMCPServers(ctx context.Context, opts StartOpts) error {
	if opts.ActivateMCPServers == nil {
		return nil
	}
	if err := opts.ActivateMCPServers(ctx); err != nil {
		return WrapFailure(store.FailureStartup, "hosted MCP activation failed", err)
	}
	return nil
}

func (d *Driver) runProviderPreStart(ctx context.Context, opts StartOpts) error {
	if strings.TrimSpace(opts.ProviderName) == "" {
		return nil
	}
	if d == nil || d.providerPreStarter == nil {
		return WrapFailure(
			store.FailureProviderAuth,
			"provider auth pre-start probe unavailable",
			errors.New("acp: provider pre-starter is required"),
		)
	}
	provider := compozyconfig.ProviderConfig{}
	if opts.ProviderConfig != nil {
		provider = *opts.ProviderConfig
	}
	report := d.providerPreStarter.PreStart(ctx, provider, opts.ProviderAuthEnv)
	if report.Item == nil && report.Cause == nil {
		return nil
	}
	item := report.Item
	if item == nil {
		classification := authproviders.ClassifyError(report.Cause)
		generated := authproviders.DiagnosticItem(opts.ProviderName, classification)
		item = &generated
	}
	message := "provider auth pre-start probe failed"
	code := strings.TrimSpace(item.Code)
	itemMessage := strings.TrimSpace(item.Message)
	switch {
	case code != "" && itemMessage != "":
		message = code + ": " + itemMessage
	case code != "":
		message = code
	case itemMessage != "":
		message = itemMessage
	}
	err := fmt.Errorf(
		"acp: provider auth pre-start probe for %q failed: %s",
		strings.TrimSpace(opts.ProviderName),
		message,
	)
	cause := report.Cause
	if cause == nil {
		cause = err
	}
	return WrapFailure(
		store.FailureProviderAuth,
		"provider auth pre-start probe failed",
		diagnostics.NewStructuredError(*item, cause),
	)
}

func stageOutcome(err error, skipped bool) string {
	if err != nil {
		return startOutcomeFailed
	}
	if skipped {
		return startOutcomeSkipped
	}
	return startOutcomeSucceeded
}

func (d *Driver) logStartStage(
	opts StartOpts,
	process *AgentProcess,
	stage string,
	outcome string,
	startedAt time.Time,
) {
	logger := slog.Default()
	if d != nil && d.logger != nil {
		logger = d.logger
	}
	sessionID := strings.TrimSpace(opts.ResumeSessionID)
	if process != nil && strings.TrimSpace(process.SessionID) != "" {
		sessionID = strings.TrimSpace(process.SessionID)
	}
	logger.Info(
		"acp.start.stage",
		"stage", stage,
		"outcome", outcome,
		"duration_ms", time.Since(startedAt).Milliseconds(),
		"agent", strings.TrimSpace(opts.AgentName),
		"provider", strings.TrimSpace(opts.ProviderName),
		"session_id", sessionID,
	)
}

package acp

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/diagnostics"
	authproviders "github.com/compozy/agh/internal/providers"
	"github.com/compozy/agh/internal/store"
)

const (
	startOutcomeSucceeded = "succeeded"
	startOutcomeSkipped   = "skipped"
	startOutcomeFailed    = "failed"
)

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

	stageStartedAt := time.Now()
	err = runProviderPreStart(ctx, normalized)
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
	if err := d.negotiateSession(ctx, process, normalized); err != nil {
		return nil, d.cleanupFailedStart(process, err)
	}
	return process, nil
}

func runProviderPreStart(ctx context.Context, opts StartOpts) error {
	if strings.TrimSpace(opts.ProviderName) == "" {
		return nil
	}
	provider := aghconfig.ProviderConfig{}
	if opts.ProviderConfig != nil {
		provider = *opts.ProviderConfig
	}
	report := authproviders.PreStart(ctx, provider, opts.ProviderAuthEnv)
	if report.Item == nil {
		return nil
	}
	message := "provider auth pre-start probe failed"
	code := strings.TrimSpace(report.Item.Code)
	itemMessage := strings.TrimSpace(report.Item.Message)
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
	return WrapFailure(
		store.FailureProviderAuth,
		"provider auth pre-start probe failed",
		diagnostics.NewStructuredError(*report.Item, err),
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

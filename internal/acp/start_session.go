package acp

import (
	"context"
	"fmt"
	"time"

	acpsdk "github.com/coder/acp-go-sdk"
	"github.com/compozy/agh/internal/store"
)

func (d *Driver) negotiateSession(ctx context.Context, process *AgentProcess, normalized StartOpts) error {
	if normalized.ResumeSessionID != "" {
		return d.loadSession(ctx, process, normalized)
	}
	return d.createSession(ctx, process, normalized)
}

func (d *Driver) loadSession(ctx context.Context, process *AgentProcess, normalized StartOpts) error {
	stageStartedAt := time.Now()
	if !process.CapsSnapshot().SupportsLoadSession {
		err := WrapFailure(store.FailureLoad, "ACP session/load is not supported", fmt.Errorf(
			"%w: agent %q does not support session/load for resume %q",
			ErrAgentDoesNotSupportSession,
			normalized.AgentName,
			normalized.ResumeSessionID,
		))
		d.logStartStage(normalized, process, "session_load", startOutcomeFailed, stageStartedAt)
		return err
	}

	loadRequest := acpsdk.LoadSessionRequest{
		Cwd:        normalized.Cwd,
		McpServers: toSDKMCPServers(normalized.MCPServers),
		SessionId:  acpsdk.SessionId(normalized.ResumeSessionID),
	}
	loadWireRequest := wireLoadSessionRequest{
		Cwd:            loadRequest.Cwd,
		McpServers:     loadRequest.McpServers,
		AdditionalDirs: append([]string(nil), normalized.AdditionalDirs...),
		SessionID:      loadRequest.SessionId,
	}
	loadResponse, err := acpsdk.SendRequest[acpsdk.LoadSessionResponse](
		process.conn,
		ctx,
		acpsdk.AgentMethodSessionLoad,
		loadWireRequest,
	)
	d.logStartStage(normalized, process, "session_load", stageOutcome(err, false), stageStartedAt)
	if err != nil {
		return WrapFailure(store.FailureLoad, "ACP session/load failed", fmt.Errorf(
			"%w: load session %q for %q: %w",
			ErrLoadSessionFailed,
			normalized.ResumeSessionID,
			normalized.AgentName,
			err,
		))
	}

	process.SessionID = normalized.ResumeSessionID
	if err := process.checkpointProcessOwner(ctx); err != nil {
		return err
	}
	process.setCaps(captureCaps(
		process.CapsSnapshot().SupportsLoadSession,
		loadResponse.Modes,
		loadResponse.ConfigOptions,
	))
	return d.applySessionConfiguration(ctx, process, normalized)
}

func (d *Driver) createSession(ctx context.Context, process *AgentProcess, normalized StartOpts) error {
	stageStartedAt := time.Now()
	newRequest := acpsdk.NewSessionRequest{
		Cwd:        normalized.Cwd,
		McpServers: toSDKMCPServers(normalized.MCPServers),
	}
	newWireRequest := wireNewSessionRequest{
		Cwd:            newRequest.Cwd,
		McpServers:     newRequest.McpServers,
		AdditionalDirs: append([]string(nil), normalized.AdditionalDirs...),
	}
	newResponse, err := acpsdk.SendRequest[acpsdk.NewSessionResponse](
		process.conn,
		ctx,
		acpsdk.AgentMethodSessionNew,
		newWireRequest,
	)
	if err != nil {
		d.logStartStage(normalized, process, "session_new", startOutcomeFailed, stageStartedAt)
		return WrapFailure(
			store.FailureProtocol,
			"ACP session/new failed",
			fmt.Errorf("acp: create session for %q: %w", normalized.AgentName, err),
		)
	}

	process.SessionID = string(newResponse.SessionId)
	d.logStartStage(normalized, process, "session_new", startOutcomeSucceeded, stageStartedAt)
	if err := process.checkpointProcessOwner(ctx); err != nil {
		return err
	}
	process.setCaps(captureCaps(
		process.CapsSnapshot().SupportsLoadSession,
		newResponse.Modes,
		newResponse.ConfigOptions,
	))
	return d.applySessionConfiguration(ctx, process, normalized)
}

func (d *Driver) applySessionConfiguration(
	ctx context.Context,
	process *AgentProcess,
	normalized StartOpts,
) error {
	stageStartedAt := time.Now()
	applied, err := d.applySessionMode(ctx, process, normalized.Permissions)
	d.logStartStage(normalized, process, "set_mode", stageOutcome(err, !applied), stageStartedAt)
	if err != nil {
		return WrapFailure(
			store.FailureProtocol,
			"ACP session mode negotiation failed",
			fmt.Errorf("acp: set session mode for %q: %w", normalized.AgentName, err),
		)
	}

	stageStartedAt = time.Now()
	applied, err = d.applySessionModel(ctx, process, normalized.PreferredModel)
	d.logStartStage(normalized, process, "set_model", stageOutcome(err, !applied), stageStartedAt)
	if err != nil {
		return WrapFailure(
			store.FailureProtocol,
			"ACP session model negotiation failed",
			fmt.Errorf("acp: set session model for %q: %w", normalized.AgentName, err),
		)
	}

	stageStartedAt = time.Now()
	applied, err = d.applySessionReasoningEffort(ctx, process, normalized.ReasoningEffort)
	d.logStartStage(normalized, process, "set_reasoning", stageOutcome(err, !applied), stageStartedAt)
	if err != nil {
		return WrapFailure(
			store.FailureProtocol,
			"ACP session reasoning negotiation failed",
			fmt.Errorf("acp: set session reasoning effort for %q: %w", normalized.AgentName, err),
		)
	}
	return nil
}

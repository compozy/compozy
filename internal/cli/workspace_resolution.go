package cli

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/compozy/compozy/internal/agentidentity"
	"github.com/compozy/compozy/internal/api/contract"
	"github.com/spf13/cobra"
)

const (
	workspaceResolutionPositional      = "positional"
	workspaceResolutionFlag            = "flag"
	workspaceResolutionEnv             = "env"
	workspaceResolutionSessionIdentity = "session_identity"
	workspaceResolutionCrossAttempt    = "cross_workspace_attempt"
	workspaceResolutionCWD             = "cwd"
	workspaceEnvName                   = "COMPOZY_WORKSPACE"
	workspaceFlagName                  = "workspace"
)

var errWorkspaceReferenceRequired = errors.New("cli: workspace reference is required")

type workspaceLookupClient interface {
	GetWorkspace(context.Context, string) (WorkspaceDetailRecord, error)
}

type workspaceResolutionRequest struct {
	PositionalRef string
	FlagRef       string
}

type workspaceResolution struct {
	ID            string
	Root          string
	Source        string
	IdentityAgent string
	Detail        WorkspaceDetailRecord
}

type workspaceResolutionContextKey struct{}

type workspaceResolutionError struct {
	CWD   string
	Cause error
}

func (e *workspaceResolutionError) Error() string {
	if e == nil {
		return ""
	}
	message := fmt.Sprintf(
		"cli: workspace resolution failed after trying positional ref, --workspace, %s, "+
			"session identity, and cwd discovery at %q; register the directory with "+
			"`compozy workspace add %q` or pass --workspace <id|name|path>",
		workspaceEnvName,
		e.CWD,
		e.CWD,
	)
	if e.Cause != nil {
		return message + ": " + e.Cause.Error()
	}
	return message
}

func (e *workspaceResolutionError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

func (e *workspaceResolutionError) errorPayload() contract.ErrorPayload {
	if e == nil {
		return contract.ErrorPayload{}
	}
	return contract.ErrorPayload{Error: e.Error()}
}

func resolveCommandWorkspace(
	ctx context.Context,
	cmd *cobra.Command,
	deps commandDeps,
	client workspaceLookupClient,
	request workspaceResolutionRequest,
) (workspaceResolution, error) {
	if client == nil {
		return workspaceResolution{}, errors.New("cli: workspace lookup client is required")
	}
	if commandWorkspaceFlagIsBlank(cmd, request.FlagRef) {
		return workspaceResolution{}, errWorkspaceReferenceRequired
	}

	identityRef, identityAgent, err := workspaceRefFromSessionIdentity(ctx, cmd, deps, client)
	if err != nil {
		return workspaceResolution{}, err
	}
	candidates := []struct {
		ref    string
		source string
	}{
		{ref: request.PositionalRef, source: workspaceResolutionPositional},
		{ref: request.FlagRef, source: workspaceResolutionFlag},
		{ref: deps.getenv(workspaceEnvName), source: workspaceResolutionEnv},
	}
	for _, candidate := range candidates {
		if strings.TrimSpace(candidate.ref) == "" {
			continue
		}
		resolved, err := resolveWorkspaceCandidate(ctx, cmd, client, candidate.ref, candidate.source)
		if err != nil {
			return workspaceResolution{}, err
		}
		resolved = markCrossWorkspaceAttempt(cmd, resolved, identityRef)
		return resolved, nil
	}

	if identityRef != "" {
		resolved, err := resolveWorkspaceCandidate(
			ctx,
			cmd,
			client,
			identityRef,
			workspaceResolutionSessionIdentity,
		)
		if err == nil {
			resolved.IdentityAgent = identityAgent
			return resolved, nil
		}
		return workspaceResolution{}, err
	}

	cwd, err := currentWorkingDirectory(deps)
	if err != nil {
		return workspaceResolution{}, &workspaceResolutionError{Cause: err}
	}
	resolved, err := resolveWorkspaceCandidate(ctx, cmd, client, cwd, workspaceResolutionCWD)
	if err != nil {
		return workspaceResolution{}, &workspaceResolutionError{CWD: cwd, Cause: err}
	}
	return resolved, nil
}

func resolveWorkspaceOverrideOnly(
	ctx context.Context,
	cmd *cobra.Command,
	deps commandDeps,
	client workspaceLookupClient,
	flagRef string,
) (workspaceResolution, bool, error) {
	if commandWorkspaceFlagIsBlank(cmd, flagRef) {
		return workspaceResolution{}, true, errWorkspaceReferenceRequired
	}
	identityRef, _, err := workspaceRefFromSessionIdentity(ctx, cmd, deps, client)
	if err != nil {
		return workspaceResolution{}, false, err
	}
	candidates := []struct {
		ref    string
		source string
	}{
		{ref: flagRef, source: workspaceResolutionFlag},
		{ref: deps.getenv(workspaceEnvName), source: workspaceResolutionEnv},
	}
	for _, candidate := range candidates {
		if strings.TrimSpace(candidate.ref) == "" {
			continue
		}
		resolution, err := resolveWorkspaceCandidate(ctx, cmd, client, candidate.ref, candidate.source)
		if err == nil {
			resolution = markCrossWorkspaceAttempt(cmd, resolution, identityRef)
		}
		return resolution, true, err
	}
	return workspaceResolution{}, false, nil
}

func commandWorkspaceFlagIsBlank(cmd *cobra.Command, value string) bool {
	if cmd == nil || strings.TrimSpace(value) != "" {
		return false
	}
	flag := cmd.Flags().Lookup(workspaceFlagName)
	return flag != nil && flag.Changed
}

func resolveOptionalWorkspaceOverride(
	ctx context.Context,
	cmd *cobra.Command,
	deps commandDeps,
	client workspaceLookupClient,
	flagRef string,
) (string, error) {
	resolution, resolved, err := resolveWorkspaceOverrideOnly(ctx, cmd, deps, client, flagRef)
	if err != nil {
		return "", err
	}
	if !resolved {
		return "", nil
	}
	return resolution.ID, nil
}

func resolveWorkspaceFlagOverride(
	cmd *cobra.Command,
	deps commandDeps,
	client workspaceLookupClient,
	required bool,
) (string, error) {
	workspaceRef, err := commandWorkspaceFlag(cmd)
	if err != nil {
		return "", fmt.Errorf("parse workspace override: %w", err)
	}
	if required {
		resolution, err := resolveCommandWorkspace(
			cmd.Context(),
			cmd,
			deps,
			client,
			workspaceResolutionRequest{FlagRef: workspaceRef},
		)
		if err != nil {
			return "", fmt.Errorf("resolve workspace override: %w", err)
		}
		return resolution.ID, nil
	}
	workspace, err := resolveOptionalWorkspaceOverride(
		cmd.Context(),
		cmd,
		deps,
		client,
		workspaceRef,
	)
	if err != nil {
		return "", fmt.Errorf("resolve workspace override: %w", err)
	}
	return workspace, nil
}

func resolveContextualCommandWorkspace(
	ctx context.Context,
	cmd *cobra.Command,
	deps commandDeps,
	client workspaceLookupClient,
	flagRef string,
) (workspaceResolution, bool, error) {
	resolution, explicit, err := resolveWorkspaceOverrideOnly(ctx, cmd, deps, client, flagRef)
	if err != nil || explicit {
		return resolution, explicit, err
	}

	resolution, err = resolveCommandWorkspace(
		ctx,
		cmd,
		deps,
		client,
		workspaceResolutionRequest{},
	)
	if err == nil {
		return resolution, true, nil
	}
	if workspaceResolutionAllowsGlobalFallback(err) {
		return workspaceResolution{}, false, nil
	}
	return workspaceResolution{}, false, err
}

func workspaceResolutionAllowsGlobalFallback(err error) bool {
	pathErr, ok := errors.AsType[*workspacePathNotRegisteredError](err)
	if ok && pathErr != nil {
		return true
	}
	apiErr, ok := errors.AsType[*daemonAPIError](err)
	return ok && apiErr.statusCode == http.StatusNotFound
}

func resolveCommandWorkspaceRoot(
	cmd *cobra.Command,
	deps commandDeps,
	flagRef string,
) (string, error) {
	client, err := clientFromDeps(deps)
	if err != nil {
		return "", err
	}
	resolution, err := resolveCommandWorkspace(
		cmd.Context(),
		cmd,
		deps,
		client,
		workspaceResolutionRequest{FlagRef: flagRef},
	)
	if err != nil {
		return "", err
	}
	if resolution.Root == "" {
		return "", fmt.Errorf("cli: resolved workspace %q has no root directory", resolution.ID)
	}
	return resolution.Root, nil
}

func resolveWorkspaceCandidate(
	ctx context.Context,
	cmd *cobra.Command,
	client workspaceLookupClient,
	ref string,
	source string,
) (workspaceResolution, error) {
	trimmed := strings.TrimSpace(ref)
	if trimmed == "" {
		return workspaceResolution{}, errWorkspaceReferenceRequired
	}
	detail, err := client.GetWorkspace(ctx, trimmed)
	if err != nil {
		return workspaceResolution{}, fmt.Errorf("cli: resolve workspace %q from %s: %w", trimmed, source, err)
	}
	id := strings.TrimSpace(detail.Workspace.ID)
	if id == "" {
		return workspaceResolution{}, fmt.Errorf(
			"cli: resolve workspace %q from %s: missing workspace id",
			trimmed,
			source,
		)
	}
	resolution := workspaceResolution{
		ID:     id,
		Root:   strings.TrimSpace(detail.Workspace.RootDir),
		Source: source,
		Detail: detail,
	}
	recordWorkspaceResolution(cmd, resolution)
	return resolution, nil
}

func workspaceRefFromSessionIdentity(
	ctx context.Context,
	cmd *cobra.Command,
	deps commandDeps,
	client workspaceLookupClient,
) (string, string, error) {
	credentials := agentCredentialsFromEnv(deps)
	if credentials.SessionID == "" || credentials.AgentName == "" {
		return "", "", nil
	}
	sessionClient, ok := client.(agentSessionClient)
	if !ok {
		return "", "", fmt.Errorf(
			"cli: workspace session identity lookup is unavailable: %w",
			agentidentity.ErrIdentityLookupUnavailable,
		)
	}
	originRef := "workspace.resolve"
	if cmd != nil {
		originRef = cmd.CommandPath()
	}
	caller, err := resolveAgentCallerFromEnv(ctx, deps, sessionClient, originRef)
	if err != nil {
		return "", "", err
	}
	return strings.TrimSpace(caller.Session.WorkspaceID), strings.TrimSpace(caller.Session.AgentName), nil
}

func recordWorkspaceResolution(cmd *cobra.Command, resolution workspaceResolution) {
	if cmd == nil || strings.TrimSpace(resolution.Source) == "" {
		return
	}
	cmd.SetContext(context.WithValue(cmd.Context(), workspaceResolutionContextKey{}, resolution))
}

func markCrossWorkspaceAttempt(
	cmd *cobra.Command,
	resolution workspaceResolution,
	identityWorkspaceID string,
) workspaceResolution {
	identityWorkspaceID = strings.TrimSpace(identityWorkspaceID)
	if identityWorkspaceID == "" || identityWorkspaceID == strings.TrimSpace(resolution.ID) {
		return resolution
	}
	resolution.Source = workspaceResolutionCrossAttempt
	recordWorkspaceResolution(cmd, resolution)
	return resolution
}

func commandWorkspaceResolution(cmd *cobra.Command) (workspaceResolution, bool) {
	if cmd == nil || cmd.Context() == nil {
		return workspaceResolution{}, false
	}
	resolution, ok := cmd.Context().Value(workspaceResolutionContextKey{}).(workspaceResolution)
	return resolution, ok && strings.TrimSpace(resolution.Source) != ""
}

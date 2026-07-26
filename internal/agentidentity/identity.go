// Package agentidentity resolves daemon-validated caller identity for
// agent-facing CLI and UDS operations.
package agentidentity

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/session"
	taskpkg "github.com/compozy/agh/internal/task"
)

const (
	// EnvSessionID is the daemon-issued session identifier visible inside agent sessions.
	EnvSessionID = "AGH_SESSION_ID"
	// EnvAgent is the daemon-issued agent name visible inside agent sessions.
	EnvAgent = "AGH_AGENT"

	// HeaderSessionID carries EnvSessionID over the local UDS HTTP transport.
	HeaderSessionID = "X-AGH-Session-ID"
	// HeaderAgent carries EnvAgent over the local UDS HTTP transport.
	HeaderAgent = "X-AGH-Agent"
	// HeaderWorkspaceID optionally narrows an agent request to the caller workspace.
	HeaderWorkspaceID = "X-AGH-Workspace-ID"
)

// ResolveOptions configures agent caller resolution.
type ResolveOptions struct {
	Credentials         Credentials
	Lookup              SessionLookup
	ExpectedWorkspaceID string
	OriginKind          taskpkg.OriginKind
	OriginRef           string
}

// Caller is a validated agent-session caller and its task-domain actor context.
type Caller struct {
	Credentials Credentials
	Session     SessionSnapshot
	Actor       taskpkg.ActorContext
}

// Resolve validates untrusted caller credentials against the daemon session lookup.
func Resolve(ctx context.Context, opts ResolveOptions) (Caller, error) {
	creds := normalizeCredentials(opts.Credentials)
	if err := validateResolveInputs(ctx, opts.Lookup, creds); err != nil {
		return Caller{}, err
	}
	snapshot, err := lookupSessionSnapshot(ctx, opts.Lookup, creds)
	if err != nil {
		return Caller{}, err
	}
	if err := validateWorkspace(snapshot, opts.ExpectedWorkspaceID, creds.WorkspaceID); err != nil {
		return Caller{}, err
	}
	actor, err := deriveActorContext(snapshot.ID, snapshot.WorkspaceID, opts.OriginKind, opts.OriginRef)
	if err != nil {
		return Caller{}, fmt.Errorf("agent identity: derive actor context: %w", err)
	}

	return Caller{
		Credentials: creds,
		Session:     snapshot,
		Actor:       actor,
	}, nil
}

func validateResolveInputs(ctx context.Context, lookup SessionLookup, creds Credentials) error {
	if ctx == nil {
		return identityError(
			ErrIdentityLookupUnavailable,
			"identity_lookup_unavailable",
			"agent identity cannot be validated",
			"retry after the daemon is reachable",
		)
	}
	if creds.SessionID == "" {
		return identityError(
			ErrIdentityRequired,
			"identity_required",
			EnvSessionID+" is required for agent commands",
			"run this command from an AGH-managed agent session",
		)
	}
	if creds.AgentName == "" {
		return identityError(
			ErrIdentityRequired,
			"identity_required",
			EnvAgent+" is required for agent commands",
			"run this command from an AGH-managed agent session",
		)
	}
	if lookup == nil {
		return identityError(
			ErrIdentityLookupUnavailable,
			"identity_lookup_unavailable",
			"agent identity cannot be validated",
			"retry after the daemon is reachable",
		)
	}
	return nil
}

func lookupSessionSnapshot(ctx context.Context, lookup SessionLookup, creds Credentials) (SessionSnapshot, error) {
	snapshot, err := lookup(ctx, creds.SessionID)
	if err != nil {
		if errors.Is(err, ErrIdentityLookupUnavailable) ||
			errors.Is(err, context.Canceled) ||
			errors.Is(err, context.DeadlineExceeded) {
			return SessionSnapshot{}, identityError(
				ErrIdentityLookupUnavailable,
				"identity_lookup_unavailable",
				"agent identity cannot be validated",
				"retry after the daemon is reachable",
			)
		}
		if !errors.Is(err, session.ErrSessionNotFound) {
			return SessionSnapshot{}, identityError(
				ErrIdentityLookupUnavailable,
				"identity_lookup_unavailable",
				"agent identity cannot be validated",
				"retry after the daemon is reachable",
			)
		}
		return SessionSnapshot{}, identityError(
			ErrIdentityStale,
			"identity_stale",
			"agent session identity is not known to the daemon",
			"start or resume the AGH session, then retry",
		)
	}
	snapshot = normalizeSessionSnapshot(snapshot)
	if snapshot.ID == "" || snapshot.State != session.StateActive {
		return SessionSnapshot{}, identityError(
			ErrIdentityStale,
			"identity_stale",
			"agent session identity is not active",
			"start or resume the AGH session, then retry",
		)
	}
	if snapshot.ID != creds.SessionID {
		return SessionSnapshot{}, identityError(
			ErrIdentityMismatch,
			"identity_mismatch",
			"agent session lookup returned a different session",
			"clear stale AGH identity environment variables and retry",
		)
	}
	if snapshot.AgentName != creds.AgentName {
		return SessionSnapshot{}, identityError(
			ErrIdentityMismatch,
			"identity_mismatch",
			EnvAgent+" does not match the daemon session agent",
			"clear stale AGH identity environment variables and retry",
		)
	}
	return snapshot, nil
}

func validateWorkspace(snapshot SessionSnapshot, expectedWorkspaceID string, credentialsWorkspaceID string) error {
	expectedWorkspaceID = strings.TrimSpace(expectedWorkspaceID)
	credentialsWorkspaceID = strings.TrimSpace(credentialsWorkspaceID)
	if expectedWorkspaceID != "" && credentialsWorkspaceID != "" && expectedWorkspaceID != credentialsWorkspaceID {
		return identityError(
			ErrIdentityUnauthorized,
			"identity_unauthorized",
			"agent session does not belong to the requested workspace",
			"use the session workspace or start a session in the requested workspace",
		)
	}
	if expectedWorkspaceID == "" {
		expectedWorkspaceID = credentialsWorkspaceID
	}
	if expectedWorkspaceID == "" || snapshot.WorkspaceID == expectedWorkspaceID {
		return nil
	}
	return identityError(
		ErrIdentityUnauthorized,
		"identity_unauthorized",
		"agent session does not belong to the requested workspace",
		"use the session workspace or start a session in the requested workspace",
	)
}

func deriveActorContext(
	sessionID string,
	workspaceID string,
	originKind taskpkg.OriginKind,
	originRef string,
) (taskpkg.ActorContext, error) {
	originKind = originKind.Normalize()
	if originKind == "" {
		originKind = taskpkg.OriginKindAgentSession
	}
	originRef = strings.TrimSpace(originRef)
	if originRef == "" {
		originRef = originRefForKind(originKind)
	}
	return taskpkg.DeriveAgentSessionActorContextForOrigin(sessionID, workspaceID, originKind, originRef)
}

func originRefForKind(kind taskpkg.OriginKind) string {
	switch kind.Normalize() {
	case taskpkg.OriginKindCLI:
		return "agent.cli"
	case taskpkg.OriginKindUDS:
		return "agent.uds"
	default:
		return "agent.session"
	}
}

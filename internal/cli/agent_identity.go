package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/agentidentity"
	diagnosticspkg "github.com/compozy/compozy/internal/diagnostics"
	taskpkg "github.com/compozy/compozy/internal/task"
)

type agentSessionClient interface {
	GetSession(context.Context, string) (SessionRecord, error)
}

func agentCredentialsFromEnv(deps commandDeps) agentidentity.Credentials {
	return agentidentity.Credentials{
		SessionID: strings.TrimSpace(deps.getenv(agentidentity.EnvSessionID)),
		AgentName: strings.TrimSpace(deps.getenv(agentidentity.EnvAgent)),
	}
}

func resolveAgentCallerFromEnv(
	ctx context.Context,
	deps commandDeps,
	client agentSessionClient,
	originRef string,
) (agentidentity.Caller, error) {
	if client == nil {
		return agentidentity.Resolve(ctx, agentidentity.ResolveOptions{
			Credentials: agentCredentialsFromEnv(deps),
			OriginKind:  taskpkg.OriginKindCLI,
			OriginRef:   originRef,
		})
	}
	return agentidentity.Resolve(ctx, agentidentity.ResolveOptions{
		Credentials: agentCredentialsFromEnv(deps),
		Lookup:      agentSessionLookup(client),
		OriginKind:  taskpkg.OriginKindCLI,
		OriginRef:   originRef,
	})
}

func agentSessionLookup(client agentSessionClient) agentidentity.SessionLookup {
	return func(ctx context.Context, sessionID string) (agentidentity.SessionSnapshot, error) {
		record, err := client.GetSession(ctx, sessionID)
		if err != nil {
			return agentidentity.SessionSnapshot{}, fmt.Errorf("cli: lookup agent session %q: %w", sessionID, err)
		}
		return agentidentity.SessionSnapshot{
			ID:            record.ID,
			Name:          record.Name,
			AgentName:     record.AgentName,
			Provider:      sessionRuntimeProvider(record),
			WorkspaceID:   record.WorkspaceID,
			WorkspacePath: record.WorkspacePath,
			Type:          record.Type,
			State:         record.State,
			CreatedAt:     record.CreatedAt,
			UpdatedAt:     record.UpdatedAt,
		}, nil
	}
}

func cliExitCodeForError(err error) int {
	if err == nil {
		return 0
	}
	if apiErr, ok := errors.AsType[interface {
		error
		cliExitCode() int
	}](err); ok {
		return apiErr.cliExitCode()
	}
	code := agentidentity.ExitCodeForError(err)
	if code == agentidentity.ExitUnavailable {
		if _, ok := diagnosticspkg.ItemFromError(err); ok {
			return code
		}
		return 1
	}
	return code
}

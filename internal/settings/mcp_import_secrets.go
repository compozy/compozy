package settings

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/mcppolicy"
	"github.com/compozy/compozy/internal/runtimemcp"
)

func mcpHeaderSecretRef(prefix, name string) string {
	return prefix + "headers/" + hex.EncodeToString([]byte(strings.ToLower(name)))
}

func prepareMCPSecretHeaderValues(
	prefix string,
	server *config.MCPServer,
	values map[string]string,
) ([]preparedSecretWrite, error) {
	if len(values) == 0 {
		return nil, nil
	}
	if server.EffectiveTransport() != config.MCPServerTransportHTTP {
		return nil, validationError(fmt.Errorf("settings: secret headers require HTTP transport"))
	}
	if err := mcppolicy.ValidateHeaders(
		nil,
		values,
		mcppolicy.SourceOperatorSecret,
		server.Auth.Enabled(),
	); err != nil {
		return nil, validationError(err)
	}
	if server.SecretHeaders == nil {
		server.SecretHeaders = map[string]string{}
	}
	writes := make([]preparedSecretWrite, 0, len(values))
	for _, name := range slices.Sorted(maps.Keys(values)) {
		ref := mcpHeaderSecretRef(prefix, name)
		server.SecretHeaders[name] = ref
		writes = append(
			writes,
			preparedSecretWrite{ref: ref, kind: mcpHeaderSecretKind, value: values[name], description: "MCP header"},
		)
	}
	return writes, nil
}

func mcpImportCleanupWarning(name string) string {
	return name + " was copied, but CompozyOS couldn't confirm its secrets were cleaned up. Check Settings → MCP servers → " + name + "."
}

func (s *service) rollbackUnreferencedMCPSecrets(
	ctx context.Context,
	scope ScopeKind,
	workspaceID, profile string,
	mutations []mcpSecretMutation,
) error {
	if len(mutations) == 0 {
		return nil
	}
	cleanupCtx, cancel := mcpSecretRollbackContext(ctx)
	defer cancel()
	_, sources, err := s.resolveMCPTargetContext(cleanupCtx, scope, workspaceID, profile)
	if err != nil {
		return err
	}
	referenced := map[string]struct{}{}
	for _, entries := range sources {
		for _, entry := range entries {
			maps.Copy(referenced, referencedMCPSecretRefs(entry.Server))
		}
	}
	safe := make([]mcpSecretMutation, 0, len(mutations))
	for _, mutation := range mutations {
		if _, found := referenced[mutation.ref]; mutation.created && found {
			continue
		}
		safe = append(safe, mutation)
	}
	return s.rollbackMCPSecretMutations(cleanupCtx, safe)
}

func (s *service) validateMCPImportName(
	ctx context.Context,
	scope ScopeKind,
	workspaceID, profile, name string,
	sources map[string][]mcpSourceEntry,
) error {
	_, occupied := preferredMCPDeleteTarget(scope, name, sources)
	err := s.validateManualMCPName(
		ctx,
		MCPAuthTargetRequest{Scope: scope, WorkspaceID: workspaceID, ProfileName: profile, Name: name},
	)
	if occupied || errors.Is(err, ErrMCPServerNameTaken) {
		return conflictError(
			&runtimemcp.ImportError{
				Code:    "mcp_import_name_taken",
				Message: fmt.Sprintf("A CompozyOS server named %s already exists in %s", name, importScopeLabel(scope)),
			},
		)
	}
	return err
}

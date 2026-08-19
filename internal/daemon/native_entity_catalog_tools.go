package daemon

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/api/core"
	toolspkg "github.com/compozy/compozy/internal/tools"
	"github.com/compozy/compozy/internal/vault"
)

type agentListInput struct {
	Workspace string `json:"workspace,omitempty"`
}

type vaultListInput struct {
	Prefix string `json:"prefix,omitempty"`
}

func (n *daemonNativeTools) agentList(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input agentListInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	workspaceID := nativeCallerWorkspaceInput(input.Workspace, scope)
	var entries []core.AgentCatalogEntry
	if workspaceID == "" {
		listed, err := n.deps.AgentCatalog.ListAgents(ctx)
		if err != nil {
			return toolspkg.ToolResult{}, fmt.Errorf("daemon: list agent catalog: %w", err)
		}
		entries = listed
	} else {
		resolved, err := n.deps.Workspaces.Resolve(ctx, workspaceID)
		if err != nil {
			return toolspkg.ToolResult{}, err
		}
		listed, err := n.workspaceAgents(ctx, &resolved)
		if err != nil {
			return toolspkg.ToolResult{}, err
		}
		entries = listed
	}
	payload := core.AgentPayloadsFromEntries(entries)
	return structuredResult(
		map[string]any{nativeToolsAgentsKey: payload},
		fmt.Sprintf("%d agents", len(payload)),
	)
}

func (n *daemonNativeTools) vaultList(
	ctx context.Context,
	_ toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input vaultListInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	prefix := vault.NormalizeRef(input.Prefix)
	if err := vault.ValidateSecretRefPrefix(prefix); err != nil {
		return toolspkg.ToolResult{}, toolspkg.NewToolError(
			toolspkg.ErrorCodeInvalidInput,
			req.ToolID,
			"vault prefix is invalid",
			fmt.Errorf("%w: %w", toolspkg.ErrToolInvalidInput, err),
			toolspkg.ReasonSchemaInvalid,
		)
	}
	rows, err := n.deps.Vault.ListMetadata(ctx, prefix)
	if err != nil {
		return toolspkg.ToolResult{}, fmt.Errorf("daemon: list Vault metadata: %w", err)
	}
	secrets, err := core.VaultSecretPayloadsFromMetadata(rows)
	if err != nil {
		return toolspkg.ToolResult{}, fmt.Errorf("daemon: project Vault metadata: %w", err)
	}
	return structuredResult(
		contract.VaultSecretsResponse{Secrets: secrets},
		fmt.Sprintf("%d Vault refs", len(secrets)),
	)
}

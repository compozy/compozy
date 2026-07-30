package settings

import (
	"context"
	"errors"
	"fmt"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	diagnosticcontract "github.com/compozy/compozy/internal/diagnosticcontract"
	"github.com/compozy/compozy/internal/marketplace"
	"github.com/compozy/compozy/internal/vault"
)

// MCPCatalogInstallNextStep directs the caller to the only valid follow-up state.
type MCPCatalogInstallNextStep string

const (
	MCPCatalogInstallNextStepNone      MCPCatalogInstallNextStep = "none"
	MCPCatalogInstallNextStepAuthorize MCPCatalogInstallNextStep = "authorize"
)

// MCPSecretInput is one write-only typed value or one existing Vault binding.
type MCPSecretInput struct {
	Value    string
	VaultRef string
}

// MCPCatalogInstallValues contains operator-supplied catalog input bindings.
type MCPCatalogInstallValues struct {
	Inputs map[string]MCPSecretInput
}

// MCPCatalogInstallRequest identifies one feed entry and its target settings scope.
type MCPCatalogInstallRequest struct {
	EntryID     string
	Name        string
	Scope       ScopeKind
	WorkspaceID string
	Values      MCPCatalogInstallValues
}

// MCPCatalogInstallResult returns the persisted item and its config-apply outcome.
type MCPCatalogInstallResult struct {
	Item     MCPServerItem
	Apply    ApplyResult
	NextStep MCPCatalogInstallNextStep
	Warnings []diagnosticcontract.DiagnosticItem
}

// InstallMCPCatalog validates and persists one feed-locked MCP template.
func (s *service) InstallMCPCatalog(
	ctx context.Context,
	req MCPCatalogInstallRequest,
) (MCPCatalogInstallResult, error) {
	normalized, entry, detail, err := s.prepareMCPCatalogInstall(ctx, req)
	if err != nil {
		return MCPCatalogInstallResult{}, err
	}
	server, secrets, err := s.mcpServerFromCatalog(ctx, normalized, entry, detail)
	if err != nil {
		return MCPCatalogInstallResult{}, err
	}
	if _, err := s.prepareMCPSecretWrites(
		normalized.Scope,
		normalized.WorkspaceID,
		server.Name,
		&server,
		secrets,
	); err != nil {
		return MCPCatalogInstallResult{}, err
	}
	applyResult, err := s.ApplyCollectionItem(ctx, CollectionItemPutRequest{
		CollectionRequest: CollectionRequest{
			Collection:  CollectionMCPServers,
			Scope:       normalized.Scope,
			WorkspaceID: normalized.WorkspaceID,
		},
		Name:       server.Name,
		Target:     TargetAuto,
		MCPServer:  &server,
		MCPSecrets: secrets,
	})
	if err != nil {
		return MCPCatalogInstallResult{}, err
	}
	if applyResult.MCPServer == nil {
		return MCPCatalogInstallResult{}, errors.New("settings: committed MCP catalog item is unavailable")
	}
	item := cloneMCPServerItem(*applyResult.MCPServer)
	nextStep := MCPCatalogInstallNextStepNone
	if detail.Auth != nil {
		nextStep = MCPCatalogInstallNextStepAuthorize
	}
	result := MCPCatalogInstallResult{Item: item, Apply: applyResult, NextStep: nextStep}
	if warning := s.notifyMCPCatalogInstalled(ctx, normalized.EntryID); warning != nil {
		result.Warnings = []diagnosticcontract.DiagnosticItem{*warning}
	}
	return result, nil
}

func (s *service) prepareMCPCatalogInstall(
	ctx context.Context,
	req MCPCatalogInstallRequest,
) (MCPCatalogInstallRequest, *marketplace.Entry, *marketplace.MCPEntryDetails, error) {
	if strings.TrimSpace(req.EntryID) == "" {
		return MCPCatalogInstallRequest{}, nil, nil, validationError(
			errors.New("settings: MCP catalog entry_id is required"),
		)
	}
	if s.mcpCatalog == nil {
		return MCPCatalogInstallRequest{}, nil, nil, unavailableError(
			errors.New("settings: MCP catalog is not configured"),
		)
	}
	entryID := strings.TrimSpace(req.EntryID)
	entry, err := s.mcpCatalog.Detail(ctx, marketplace.KindMCP, entryID)
	if errors.Is(err, marketplace.ErrEntryNotFound) {
		return MCPCatalogInstallRequest{}, nil, nil, notFoundError(
			fmt.Errorf("settings: MCP catalog entry %q not found", entryID),
		)
	}
	if err != nil {
		return MCPCatalogInstallRequest{}, nil, nil, fmt.Errorf(
			"settings: load MCP catalog entry %q: %w",
			entryID,
			err,
		)
	}
	if entry == nil {
		return MCPCatalogInstallRequest{}, nil, nil, notFoundError(
			fmt.Errorf("settings: MCP catalog entry %q not found", entryID),
		)
	}
	projected, err := marketplace.ProjectEntry(*entry)
	if err != nil {
		return MCPCatalogInstallRequest{}, nil, nil, unprocessableError(
			fmt.Errorf("settings: project MCP catalog entry %q: %w", entryID, err),
		)
	}
	if projected.MCP == nil {
		return MCPCatalogInstallRequest{}, nil, nil, unprocessableError(
			fmt.Errorf("settings: catalog entry %q does not project an MCP server", entryID),
		)
	}
	scope := req.Scope
	if scope == "" {
		scope = ScopeKind(strings.TrimSpace(projected.MCP.DefaultScope))
	}
	scope, workspaceID, err := s.normalizeReadScope(scope, req.WorkspaceID)
	if err != nil {
		return MCPCatalogInstallRequest{}, nil, nil, err
	}
	if scope == ScopeWorkspace && workspaceID == "" {
		return MCPCatalogInstallRequest{}, nil, nil, validationError(
			errors.New("settings: MCP catalog workspace scope requires workspace_id"),
		)
	}
	if scope == ScopeAgent {
		return MCPCatalogInstallRequest{}, nil, nil, validationError(
			errors.New("settings: MCP catalog install does not support agent scope"),
		)
	}
	normalized := req
	normalized.EntryID = entryID
	normalized.Scope = scope
	normalized.WorkspaceID = workspaceID
	normalized.Name = strings.TrimSpace(req.Name)
	if normalized.Name == "" {
		normalized.Name = strings.TrimSpace(entry.Name)
	}
	if normalized.Name == "" {
		return MCPCatalogInstallRequest{}, nil, nil, validationError(
			errors.New("settings: MCP catalog install name is required"),
		)
	}
	return normalized, entry, projected.MCP, nil
}

func (s *service) mcpServerFromCatalog(
	ctx context.Context,
	req MCPCatalogInstallRequest,
	entry *marketplace.Entry,
	detail *marketplace.MCPEntryDetails,
) (compozyconfig.MCPServer, MCPSecretValues, error) {
	server, err := mcpServerFromCatalogLaunch(req.Name, entry, detail)
	if err != nil {
		return compozyconfig.MCPServer{}, MCPSecretValues{}, err
	}
	secrets, err := s.applyCatalogInputs(ctx, &server, detail, req, req.Values.Inputs)
	if err != nil {
		return compozyconfig.MCPServer{}, MCPSecretValues{}, err
	}
	if err := finalizeMCPCatalogLaunch(&server, detail); err != nil {
		return compozyconfig.MCPServer{}, MCPSecretValues{}, err
	}
	return server, secrets, nil
}

type mcpSecretInputMode uint8

const (
	mcpSecretInputValue mcpSecretInputMode = iota + 1
	mcpSecretInputVaultRef
)

func validateMCPSecretInput(path string, input MCPSecretInput) (mcpSecretInputMode, error) {
	hasValue := strings.TrimSpace(input.Value) != ""
	hasVaultRef := strings.TrimSpace(input.VaultRef) != ""
	if hasValue == hasVaultRef {
		return 0, validationError(fmt.Errorf(
			"settings: %s must set exactly one of value or vault_ref",
			path,
		))
	}
	if hasVaultRef {
		return mcpSecretInputVaultRef, nil
	}
	if err := validateMCPCatalogInputValue(input.Value); err != nil {
		return 0, validationError(fmt.Errorf("settings: %s: %w", path, err))
	}
	return mcpSecretInputValue, nil
}

func (s *service) validateCatalogInputVaultRef(
	ctx context.Context,
	path string,
	_ MCPCatalogInstallRequest,
	_ string,
	rawRef string,
) (string, error) {
	return s.validateExistingMCPRef(ctx, path, rawRef)
}

func (s *service) validateExistingMCPRef(
	ctx context.Context,
	path string,
	rawRef string,
) (string, error) {
	ref := vault.NormalizeRef(rawRef)
	if err := vault.ValidateSecretRefNamespace(ref, "mcp"); err != nil {
		return "", validationError(fmt.Errorf("settings: %s.vault_ref must use vault:mcp/**: %w", path, err))
	}
	if s.providerSecrets == nil {
		return "", validationError(errors.New("settings: secret store is not available"))
	}
	metadata, err := s.providerSecrets.GetMetadata(ctx, ref)
	if errors.Is(err, vault.ErrSecretNotFound) || (err == nil && !metadata.Present) {
		return "", validationError(fmt.Errorf("settings: %s.vault_ref is not present", path))
	}
	if err != nil {
		return "", fmt.Errorf("settings: inspect %s.vault_ref: %w", path, err)
	}
	return ref, nil
}

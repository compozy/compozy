package settings

import (
	"context"
	"errors"
	"fmt"
	"strings"

	aghconfig "github.com/compozy/agh/internal/config"
	diagnosticcontract "github.com/compozy/agh/internal/diagnosticcontract"
	"github.com/compozy/agh/internal/marketplace"
	"github.com/compozy/agh/internal/vault"
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

// MCPCatalogInstallValues contains operator-supplied feed fields.
type MCPCatalogInstallValues struct {
	Env               map[string]MCPSecretInput
	OAuthClientSecret *MCPSecretInput
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
	if detail.OAuth != nil {
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
	if req.Scope == "" {
		return MCPCatalogInstallRequest{}, nil, nil, validationError(
			errors.New("settings: MCP catalog install scope is required"),
		)
	}
	scope, workspaceID, err := s.normalizeReadScope(req.Scope, req.WorkspaceID)
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
) (aghconfig.MCPServer, MCPSecretValues, error) {
	server := aghconfig.MCPServer{
		Name:           req.Name,
		Transport:      aghconfig.MCPServerTransport(strings.TrimSpace(detail.Transport)),
		Command:        strings.TrimSpace(detail.Command),
		Args:           append([]string(nil), detail.Args...),
		URL:            strings.TrimSpace(detail.URL),
		CatalogEntry:   strings.TrimSpace(entry.EntryID),
		CatalogVersion: strings.TrimSpace(entry.Version),
	}
	if detail.OAuth != nil {
		server.Auth = aghconfig.MCPAuthConfig{
			Type:             aghconfig.MCPAuthTypeOAuth2PKCE,
			IssuerURL:        strings.TrimSpace(detail.OAuth.IssuerURL),
			AuthorizationURL: strings.TrimSpace(detail.OAuth.AuthorizationURL),
			TokenURL:         strings.TrimSpace(detail.OAuth.TokenURL),
			ClientID:         strings.TrimSpace(detail.OAuth.ClientID),
			Scopes:           append([]string(nil), detail.OAuth.Scopes...),
		}
	}

	secrets := MCPSecretValues{}
	inputs, err := normalizeMCPEnvInputs(req.Values.Env)
	if err != nil {
		return aghconfig.MCPServer{}, MCPSecretValues{}, err
	}
	declared := make(map[string]struct{}, len(detail.Env))
	for _, field := range detail.Env {
		name := strings.TrimSpace(field.Name)
		declared[name] = struct{}{}
		input, supplied := inputs[name]
		if field.Secret {
			if err := s.applyCatalogSecretEnv(ctx, &server, &secrets, field, input, supplied); err != nil {
				return aghconfig.MCPServer{}, MCPSecretValues{}, err
			}
			continue
		}
		if err := applyCatalogPlainEnv(&server, field, input, supplied); err != nil {
			return aghconfig.MCPServer{}, MCPSecretValues{}, err
		}
	}
	for name := range inputs {
		if _, ok := declared[name]; !ok {
			return aghconfig.MCPServer{}, MCPSecretValues{}, validationError(
				fmt.Errorf("settings: values.env.%s is not declared by catalog entry %q", name, req.EntryID),
			)
		}
	}
	if err := s.applyCatalogOAuthSecret(ctx, &server, &secrets, req.Values.OAuthClientSecret); err != nil {
		return aghconfig.MCPServer{}, MCPSecretValues{}, err
	}
	return server, secrets, nil
}

func normalizeMCPEnvInputs(values map[string]MCPSecretInput) (map[string]MCPSecretInput, error) {
	normalized := make(map[string]MCPSecretInput, len(values))
	for rawName, input := range values {
		name := strings.TrimSpace(rawName)
		if !vault.EnvNamePattern.MatchString(name) {
			return nil, validationError(fmt.Errorf("settings: values.env key %q is invalid", name))
		}
		if _, exists := normalized[name]; exists {
			return nil, validationError(fmt.Errorf("settings: values.env key %q is duplicated", name))
		}
		normalized[name] = input
	}
	return normalized, nil
}

func applyCatalogPlainEnv(
	server *aghconfig.MCPServer,
	field marketplace.MCPEnvFieldDetails,
	input MCPSecretInput,
	supplied bool,
) error {
	name := strings.TrimSpace(field.Name)
	value := field.Default
	if supplied {
		mode, err := validateMCPSecretInput("values.env."+name, input)
		if err != nil {
			return err
		}
		if mode != mcpSecretInputValue {
			return validationError(fmt.Errorf("settings: values.env.%s must use value for a non-secret field", name))
		}
		value = input.Value
	}
	if field.Required && strings.TrimSpace(value) == "" {
		return validationError(fmt.Errorf("settings: values.env.%s is required by the catalog entry", name))
	}
	if !supplied && strings.TrimSpace(value) == "" {
		return nil
	}
	if server.Env == nil {
		server.Env = make(map[string]string)
	}
	server.Env[name] = value
	return nil
}

func (s *service) applyCatalogSecretEnv(
	ctx context.Context,
	server *aghconfig.MCPServer,
	secrets *MCPSecretValues,
	field marketplace.MCPEnvFieldDetails,
	input MCPSecretInput,
	supplied bool,
) error {
	name := strings.TrimSpace(field.Name)
	if !supplied {
		if field.Required {
			return validationError(fmt.Errorf("settings: values.env.%s is required by the catalog entry", name))
		}
		return nil
	}
	mode, err := validateMCPSecretInput("values.env."+name, input)
	if err != nil {
		return err
	}
	if mode == mcpSecretInputVaultRef {
		ref, err := s.validateExistingMCPRef(ctx, "values.env."+name, input.VaultRef)
		if err != nil {
			return err
		}
		setMCPSecretEnvRef(server, name, ref)
		return nil
	}
	if secrets.SecretEnv == nil {
		secrets.SecretEnv = make(map[string]string)
	}
	secrets.SecretEnv[name] = input.Value
	return nil
}

func (s *service) applyCatalogOAuthSecret(
	ctx context.Context,
	server *aghconfig.MCPServer,
	secrets *MCPSecretValues,
	input *MCPSecretInput,
) error {
	if input == nil {
		return nil
	}
	if server.Auth.IsZero() {
		return validationError(errors.New(
			"settings: values.oauth_client_secret is only valid for a catalog entry with OAuth",
		))
	}
	mode, err := validateMCPSecretInput("values.oauth_client_secret", *input)
	if err != nil {
		return err
	}
	if mode == mcpSecretInputVaultRef {
		ref, err := s.validateExistingMCPRef(ctx, "values.oauth_client_secret", input.VaultRef)
		if err != nil {
			return err
		}
		server.Auth.ClientSecretRef = ref
		return nil
	}
	value := input.Value
	secrets.OAuthClientSecret = &value
	return nil
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
	return mcpSecretInputValue, nil
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

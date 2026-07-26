package settings

import (
	"context"
	"errors"
	"fmt"
	"os"

	"sort"
	"strings"

	aghconfig "github.com/compozy/agh/internal/config"

	"github.com/compozy/agh/internal/providerauth"
	authproviders "github.com/compozy/agh/internal/providers"
	"github.com/compozy/agh/internal/vault"
)

const settingsCredentialSourceEnv = "env"

func (s *service) ListCollection(ctx context.Context, req CollectionRequest) (CollectionEnvelope, error) {
	scope, workspaceID, err := s.normalizeReadScope(req.Scope, req.WorkspaceID)
	if err != nil {
		return CollectionEnvelope{}, fmt.Errorf("settings: list collection %q: %w", req.Collection, err)
	}
	if scope == ScopeAgent {
		return CollectionEnvelope{}, conflictError(
			fmt.Errorf("settings: collection %q does not support agent scope", req.Collection),
		)
	}
	if req.Collection != CollectionMCPServers && scope == ScopeWorkspace {
		return CollectionEnvelope{}, conflictError(
			fmt.Errorf("settings: collection %q does not support workspace scope", req.Collection),
		)
	}

	cfg, resolved, err := s.loadConfig(ctx, scope, workspaceID)
	if err != nil {
		return CollectionEnvelope{}, fmt.Errorf("settings: load collection %q config: %w", req.Collection, err)
	}

	envelope := CollectionEnvelope{
		Collection:  req.Collection,
		Scope:       scope,
		WorkspaceID: workspaceID,
	}

	switch req.Collection {
	case CollectionProviders:
		envelope.AvailableScopes = []ScopeKind{ScopeGlobal}
		items, buildErr := s.buildProviderItems(ctx, &cfg)
		if buildErr != nil {
			return CollectionEnvelope{}, buildErr
		}
		envelope.Providers = items
	case CollectionMCPServers:
		envelope.AvailableScopes = []ScopeKind{ScopeGlobal, ScopeWorkspace}
		items, buildErr := s.buildMCPServerItems(ctx, scope, workspaceID, resolved)
		if buildErr != nil {
			return CollectionEnvelope{}, buildErr
		}
		envelope.MCPServers = items
	case CollectionSandboxes:
		envelope.AvailableScopes = []ScopeKind{ScopeGlobal}
		items, buildErr := s.buildSandboxItems(ctx, &cfg)
		if buildErr != nil {
			return CollectionEnvelope{}, buildErr
		}
		envelope.Sandboxes = items
	case CollectionHooks:
		envelope.AvailableScopes = []ScopeKind{ScopeGlobal}
		envelope.Hooks = buildHookItems(cfg.Hooks.Declarations)
	default:
		return CollectionEnvelope{}, notFoundError(fmt.Errorf("settings: unknown collection %q", req.Collection))
	}

	return envelope, nil
}

func (s *service) PutCollectionItem(ctx context.Context, req CollectionItemPutRequest) (MutationResult, error) {
	finalize := func(result MutationResult, err error) (MutationResult, error) {
		return s.finalizeCommittedCollectionMutation(ctx, result, err, "replace", true)
	}

	scope, workspaceID, err := s.normalizeReadScope(req.Scope, req.WorkspaceID)
	if err != nil {
		return MutationResult{}, fmt.Errorf("settings: put collection item %q: %w", req.Collection, err)
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return MutationResult{}, validationError(errors.New("settings: collection item name is required"))
	}
	if scope == ScopeAgent {
		return MutationResult{}, conflictError(
			fmt.Errorf("settings: collection %q does not support agent scope", req.Collection),
		)
	}

	switch req.Collection {
	case CollectionProviders:
		if scope != ScopeGlobal {
			return MutationResult{}, conflictError(errors.New("settings: providers do not support workspace scope"))
		}
		if req.Provider == nil {
			return MutationResult{}, validationError(errors.New("settings: provider payload is required"))
		}
		return finalize(s.putProvider(
			ctx,
			name,
			*req.Provider,
			req.ProviderModelCuration,
			req.ProviderSecrets,
		))
	case CollectionMCPServers:
		return finalize(s.putMCPCollectionItem(ctx, scope, workspaceID, name, req))
	case CollectionSandboxes:
		if scope != ScopeGlobal {
			return MutationResult{}, conflictError(
				errors.New("settings: sandboxes do not support workspace scope"),
			)
		}
		if req.Sandbox == nil {
			return MutationResult{}, validationError(errors.New("settings: sandbox payload is required"))
		}
		return finalize(s.putSandbox(name, *req.Sandbox))
	case CollectionHooks:
		if scope != ScopeGlobal {
			return MutationResult{}, conflictError(errors.New("settings: hooks do not support workspace scope"))
		}
		if req.Hook == nil {
			return MutationResult{}, validationError(errors.New("settings: hook payload is required"))
		}
		return finalize(s.putHook(name, *req.Hook))
	default:
		return MutationResult{}, notFoundError(fmt.Errorf("settings: unknown collection %q", req.Collection))
	}
}

func (s *service) DeleteCollectionItem(ctx context.Context, req CollectionItemDeleteRequest) (MutationResult, error) {
	finalize := func(result MutationResult, err error) (MutationResult, error) {
		return s.finalizeCommittedCollectionMutation(ctx, result, err, "delete", false)
	}

	scope, workspaceID, err := s.normalizeReadScope(req.Scope, req.WorkspaceID)
	if err != nil {
		return MutationResult{}, fmt.Errorf("settings: delete collection item %q: %w", req.Collection, err)
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return MutationResult{}, validationError(errors.New("settings: collection item name is required"))
	}
	if scope == ScopeAgent {
		return MutationResult{}, conflictError(
			fmt.Errorf("settings: collection %q does not support agent scope", req.Collection),
		)
	}

	switch req.Collection {
	case CollectionProviders:
		if scope != ScopeGlobal {
			return MutationResult{}, conflictError(errors.New("settings: providers do not support workspace scope"))
		}
		return finalize(s.deleteProvider(name))
	case CollectionMCPServers:
		return finalize(s.deleteMCPServer(ctx, scope, workspaceID, name, req.Target))
	case CollectionSandboxes:
		if scope != ScopeGlobal {
			return MutationResult{}, conflictError(
				errors.New("settings: sandboxes do not support workspace scope"),
			)
		}
		return finalize(s.deleteSandbox(name))
	case CollectionHooks:
		if scope != ScopeGlobal {
			return MutationResult{}, conflictError(errors.New("settings: hooks do not support workspace scope"))
		}
		return finalize(s.deleteHook(name))
	default:
		return MutationResult{}, notFoundError(fmt.Errorf("settings: unknown collection %q", req.Collection))
	}
}

func (s *service) buildProviderItems(ctx context.Context, cfg *aghconfig.Config) ([]ProviderItem, error) {
	builtins := aghconfig.BuiltinProviders()
	names := make([]string, 0, len(builtins)+len(cfg.Providers))
	seen := make(map[string]struct{}, len(builtins)+len(cfg.Providers))
	for name := range builtins {
		names = append(names, name)
		seen[name] = struct{}{}
	}
	for name := range cfg.Providers {
		if _, ok := seen[name]; ok {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)

	items := make([]ProviderItem, 0, len(names))
	for _, name := range names {
		resolved, err := cfg.ResolveProvider(name)
		if err != nil {
			return nil, fmt.Errorf("settings: resolve provider %q: %w", name, err)
		}

		settings, err := s.providerSettingsFromConfig(ctx, name, resolved)
		if err != nil {
			return nil, err
		}
		credentials, err := s.providerCredentialStatuses(ctx, resolved)
		if err != nil {
			return nil, fmt.Errorf("settings: provider %q credential status: %w", name, err)
		}
		authStatus, err := providerAuthStatus(s.homePaths, name, resolved, credentials, s.commandLookPath)
		if err != nil {
			return nil, fmt.Errorf("settings: provider %q auth status: %w", name, err)
		}
		item := ProviderItem{
			Name:             name,
			Settings:         settings,
			Default:          strings.TrimSpace(cfg.Defaults.Provider) == name,
			CommandAvailable: s.commandAvailable(resolved.Command),
			Credentials:      credentials,
			AuthStatus:       authStatus,
		}

		if overlay, ok := cfg.Providers[name]; ok {
			item.SourceMetadata = SourceMetadata{
				EffectiveSource:  sourceRefForWriteTarget(WriteTargetGlobalConfig, ""),
				AvailableTargets: []WriteTargetKind{WriteTargetGlobalConfig},
			}
			if builtin, builtinOK := builtins[name]; builtinOK {
				item.SourceMetadata.ShadowedSources = []SourceRef{builtinProviderSource()}
				item.Fallback = providerFallbackFromBuiltin(name, builtin)
			}
			if strings.TrimSpace(overlay.Command) == "" && item.Settings.Command == "" {
				item.CommandAvailable = false
			}
		} else {
			item.SourceMetadata = SourceMetadata{
				EffectiveSource:  builtinProviderSource(),
				AvailableTargets: []WriteTargetKind{WriteTargetGlobalConfig},
			}
		}

		items = append(items, cloneProviderItem(&item))
	}
	return items, nil
}

func providerAuthStatus(
	homePaths aghconfig.HomePaths,
	providerName string,
	provider aghconfig.ProviderConfig,
	credentials []ProviderCredentialStatus,
	lookPath func(string) (string, error),
) (ProviderAuthStatus, error) {
	status := ProviderAuthStatus{
		Mode:       provider.EffectiveAuthMode(),
		EnvPolicy:  provider.EffectiveEnvPolicy(),
		HomePolicy: provider.EffectiveHomePolicy(),
		StatusCmd:  strings.TrimSpace(provider.AuthStatusCmd),
		LoginCmd:   strings.TrimSpace(provider.AuthLoginCmd),
	}
	classification, err := authproviders.ClassifyDeclared(context.Background(), provider, providerAuthStatusProbeEnv(
		providerName,
		credentials,
		lookPath,
	))
	if err != nil {
		return ProviderAuthStatus{}, err
	}
	status.State = string(classification.State)
	status.Code = classification.Code
	status.Message = classification.Message
	switch status.Mode {
	case aghconfig.ProviderAuthModeBoundSecret:
		return status, nil
	case aghconfig.ProviderAuthModeNone:
		return status, nil
	default:
		nativeCLI, err := providerauth.NativeCLIStatusForProvider(provider, lookPath)
		if err != nil {
			return ProviderAuthStatus{}, err
		}
		status.NativeCLI = nativeCLI
		loginEnv, err := providerauth.NativeCLILoginEnv(homePaths, providerName, provider, os.Environ())
		if err != nil {
			return ProviderAuthStatus{}, err
		}
		status.LoginEnv = loginEnv
	}
	return status, nil
}

func providerAuthStatusProbeEnv(
	providerName string,
	credentials []ProviderCredentialStatus,
	lookPath func(string) (string, error),
) *authproviders.ProbeEnv {
	return &authproviders.ProbeEnv{
		ProviderName: strings.TrimSpace(providerName),
		LookPath:     lookPath,
		LookupEnv: func(key string) (string, bool) {
			for _, credential := range credentials {
				if credential.Source != settingsCredentialSourceEnv || !credential.Present {
					continue
				}
				envName, err := vault.EnvNameFromRef(credential.SecretRef)
				if err == nil && envName == key {
					return "present", true
				}
			}
			return "", false
		},
		Vault: providerAuthStatusCredentialVault(credentials),
	}
}

type providerAuthStatusCredentialVault []ProviderCredentialStatus

func (v providerAuthStatusCredentialVault) GetMetadata(_ context.Context, ref string) (vault.Metadata, error) {
	normalized := vault.NormalizeRef(ref)
	for _, credential := range v {
		if vault.NormalizeRef(credential.SecretRef) != normalized {
			continue
		}
		if !credential.Present {
			return vault.Metadata{}, vault.ErrSecretNotFound
		}
		return vault.Metadata{Ref: normalized, Present: true, Kind: strings.TrimSpace(credential.Kind)}, nil
	}
	return vault.Metadata{}, vault.ErrSecretNotFound
}

func (s *service) providerCredentialStatuses(
	ctx context.Context,
	provider aghconfig.ProviderConfig,
) ([]ProviderCredentialStatus, error) {
	slots := provider.EffectiveCredentialSlots()
	if len(slots) == 0 {
		return nil, nil
	}
	statuses := make([]ProviderCredentialStatus, 0, len(slots))
	for _, slot := range slots {
		status, err := s.providerCredentialStatus(ctx, slot)
		if err != nil {
			return nil, err
		}
		statuses = append(statuses, status)
	}
	return statuses, nil
}

func (s *service) providerCredentialStatus(
	ctx context.Context,
	slot aghconfig.ProviderCredentialSlot,
) (ProviderCredentialStatus, error) {
	secretRef := vault.NormalizeRef(slot.SecretRef)
	status := ProviderCredentialStatus{
		Name:      strings.TrimSpace(slot.Name),
		TargetEnv: strings.TrimSpace(slot.TargetEnv),
		SecretRef: secretRef,
		Kind:      strings.TrimSpace(slot.Kind),
		Required:  slot.Required,
	}
	switch {
	case vault.IsEnvRef(secretRef):
		status.Source = settingsCredentialSourceEnv
		status.Present = s.envPresent(strings.TrimSpace(strings.TrimPrefix(secretRef, "env:")))
		return status, nil
	case vault.IsSecretRef(secretRef):
		status.Source = "vault"
		if s.providerSecrets == nil {
			return status, nil
		}
		metadata, err := s.providerSecrets.GetMetadata(ctx, secretRef)
		if err != nil {
			if errors.Is(err, vault.ErrSecretNotFound) {
				return status, nil
			}
			return ProviderCredentialStatus{}, err
		}
		status.Present = metadata.Present
		return status, nil
	default:
		status.Source = "unsupported"
		return status, nil
	}
}

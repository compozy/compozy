package settings

import (
	"context"
	"errors"
	"fmt"

	"sort"
	"strings"

	aghconfig "github.com/compozy/agh/internal/config"

	hookspkg "github.com/compozy/agh/internal/hooks"

	"github.com/compozy/agh/internal/vault"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

func (s *service) buildSandboxItems(
	ctx context.Context,
	cfg *aghconfig.Config,
) ([]SandboxItem, error) {
	usage := make(map[string]int)
	if s.workspaceResolver != nil {
		workspaces, err := s.workspaceResolver.List(ctx)
		if err != nil {
			return nil, fmt.Errorf("settings: list workspaces for sandbox usage: %w", err)
		}
		for _, workspace := range workspaces {
			ref := strings.TrimSpace(workspace.SandboxRef)
			if ref == "" {
				continue
			}
			usage[ref]++
		}
	}

	names := make([]string, 0, len(cfg.Sandboxes))
	for name := range cfg.Sandboxes {
		names = append(names, name)
	}
	sort.Strings(names)

	items := make([]SandboxItem, 0, len(names))
	for _, name := range names {
		item := SandboxItem{
			Name:                name,
			Profile:             cfg.Sandboxes[name],
			WorkspaceUsageCount: usage[name],
			SourceMetadata:      globalConfigSourceMetadata(),
		}
		items = append(items, cloneSandboxItem(item))
	}
	return items, nil
}

func buildHookItems(declarations []hookspkg.HookDecl) []HookItem {
	items := make([]HookItem, 0, len(declarations))
	for _, decl := range declarations {
		item := HookItem{
			Name:           strings.TrimSpace(decl.Name),
			Declaration:    decl,
			SourceMetadata: globalConfigSourceMetadata(),
		}
		items = append(items, cloneHookItem(&item))
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].Name < items[j].Name
	})
	return items
}

func (s *service) buildMCPServerItems(
	ctx context.Context,
	scope ScopeKind,
	workspaceID string,
	resolved *workspacepkg.ResolvedWorkspace,
) ([]MCPServerItem, error) {
	root := ""
	if resolved != nil {
		root = resolved.RootDir
	}

	sources, err := s.loadMCPSources(workspaceID, root, scope)
	if err != nil {
		return nil, err
	}

	names := make([]string, 0, len(sources))
	for name := range sources {
		names = append(names, name)
	}
	sort.Strings(names)

	items := make([]MCPServerItem, 0, len(names))
	for _, name := range names {
		entries := sources[name]
		if len(entries) == 0 {
			continue
		}
		effective := entries[len(entries)-1]
		item := baseMCPServerItem(effective, entries, scope, workspaceID)
		if err := s.populateMCPServerStatuses(ctx, &item, effective); err != nil {
			return nil, err
		}
		items = append(items, cloneMCPServerItem(item))
	}
	return items, nil
}

func (s *service) putProvider(
	ctx context.Context,
	name string,
	settings ProviderSettings,
	modelCuration *ProviderModelCurationRequest,
	secrets []ProviderSecretWrite,
) (MutationResult, error) {
	reconciledSettings, err := s.reconcileProviderCuratedWrite(ctx, name, settings, modelCuration)
	if err != nil {
		return MutationResult{}, err
	}
	settings = reconciledSettings
	values := providerSettingsMap(settings)
	if len(values) == 0 && len(secrets) == 0 {
		return MutationResult{}, validationError(errors.New("settings: provider overlay requires at least one field"))
	}
	secretWrites, err := s.prepareProviderSecretWrites(name, secrets)
	if err != nil {
		return MutationResult{}, err
	}
	var target aghconfig.WriteTarget
	classification := providerWriteClassification{}
	if len(values) != 0 {
		target, err = aghconfig.ResolveConfigWriteTarget(s.homePaths, "", aghconfig.WriteScopeGlobal)
		if err != nil {
			return MutationResult{}, err
		}
		classification, err = s.classifyProviderWrite(ctx, name, settings)
		if err != nil {
			return MutationResult{}, fmt.Errorf("settings: write provider %q: %w", name, err)
		}
	}
	if classification.noOp && len(secretWrites) == 0 && modelCuration == nil {
		result := mutationResultForProvider(target.Kind(), true)
		result.Warnings = []string{sectionsNoChangesValue}
		return result, nil
	}
	if err := s.storePreparedSecrets(ctx, secretWrites); err != nil {
		return MutationResult{}, err
	}
	if len(values) == 0 {
		return mutationResultForCollection(CollectionProviders, ScopeGlobal, "", WriteTargetGlobalConfig), nil
	}

	if _, err := aghconfig.EditConfigOverlay(s.homePaths, "", target, func(editor *aghconfig.OverlayEditor) error {
		path := []string{string(CollectionProviders), name}
		if err := editor.Delete(path); err != nil {
			return err
		}
		return editor.SetTable(path, values)
	}); err != nil {
		return MutationResult{}, fmt.Errorf("settings: write provider %q: %w", name, err)
	}

	return mutationResultForProvider(target.Kind(), classification.modelOnly && len(secretWrites) == 0), nil
}

type preparedSecretWrite struct {
	description string
	ref         string
	kind        string
	value       string
}

func (s *service) prepareProviderSecretWrites(
	providerName string,
	secrets []ProviderSecretWrite,
) ([]preparedSecretWrite, error) {
	if len(secrets) == 0 {
		return nil, nil
	}
	if s.providerSecrets == nil {
		return nil, validationError(errors.New("settings: secret store is not available"))
	}
	prefix, err := vaultSecretOwnerPrefix("providers", providerName)
	if err != nil {
		return nil, validationError(err)
	}
	writes := make([]preparedSecretWrite, 0, len(secrets))
	for _, secret := range secrets {
		ref := vault.NormalizeRef(secret.SecretRef)
		if ref == "" {
			return nil, validationError(errors.New("settings: provider secret ref is required"))
		}
		if err := vault.ValidateSecretRefNamespace(ref, "providers"); err != nil {
			return nil, validationError(
				fmt.Errorf("%w: provider secret refs must use vault:providers/<provider>/<slot>", err),
			)
		}
		if !strings.HasPrefix(ref, prefix) {
			return nil, validationError(fmt.Errorf(
				"settings: provider secret ref %q must be scoped under %s",
				ref,
				strings.TrimSuffix(prefix, "/"),
			))
		}
		if strings.TrimSpace(secret.Value) == "" {
			return nil, validationError(errors.New("settings: provider secret value is required"))
		}
		writes = append(writes, preparedSecretWrite{
			description: fmt.Sprintf("provider secret %q", strings.TrimSpace(secret.Name)),
			ref:         ref,
			kind:        strings.TrimSpace(secret.Kind),
			value:       secret.Value,
		})
	}
	return writes, nil
}

func providerConfigFromSettings(settings ProviderSettings) aghconfig.ProviderConfig {
	return aghconfig.ProviderConfig{
		Command:         strings.TrimSpace(settings.Command),
		DisplayName:     strings.TrimSpace(settings.DisplayName),
		Models:          providerModelsConfigFromSettings(settings.Models),
		Harness:         settings.Harness,
		RuntimeProvider: strings.TrimSpace(settings.RuntimeProvider),
		Transport:       strings.TrimSpace(settings.Transport),
		BaseURL:         strings.TrimSpace(settings.BaseURL),
		AuthMode:        settings.AuthMode,
		EnvPolicy:       settings.EnvPolicy,
		HomePolicy:      settings.HomePolicy,
		AuthStatusCmd:   strings.TrimSpace(settings.AuthStatusCmd),
		AuthLoginCmd:    strings.TrimSpace(settings.AuthLoginCmd),
		CredentialSlots: providerCredentialSlotsFromSettings(settings.CredentialSlots),
	}
}

func providerCredentialSlotsFromSettings(
	slots []aghconfig.ProviderCredentialSlot,
) []aghconfig.ProviderCredentialSlot {
	values := make([]aghconfig.ProviderCredentialSlot, 0, len(slots))
	for _, slot := range slots {
		normalized := aghconfig.ProviderCredentialSlot{
			Name:      strings.TrimSpace(slot.Name),
			TargetEnv: strings.TrimSpace(slot.TargetEnv),
			SecretRef: strings.TrimSpace(slot.SecretRef),
			Kind:      strings.TrimSpace(slot.Kind),
			Required:  slot.Required,
		}
		if normalized.Name == "" && normalized.TargetEnv == "" && normalized.SecretRef == "" && normalized.Kind == "" {
			continue
		}
		values = append(values, normalized)
	}
	if len(values) == 0 {
		return nil
	}
	return values
}

func (s *service) storePreparedSecrets(ctx context.Context, writes []preparedSecretWrite) error {
	if len(writes) == 0 {
		return nil
	}
	if s.providerSecrets == nil {
		return validationError(errors.New("settings: secret store is not available"))
	}
	for _, write := range writes {
		if _, err := s.providerSecrets.PutSecret(ctx, write.ref, write.kind, write.value); err != nil {
			return fmt.Errorf("settings: store %s: %w", write.description, err)
		}
	}
	return nil
}

func vaultSecretOwnerPrefix(namespace string, owner string) (string, error) {
	normalizedNamespace := strings.Trim(strings.TrimSpace(namespace), "/")
	normalizedOwner := strings.Trim(strings.TrimSpace(owner), "/")
	if normalizedNamespace == "" {
		return "", errors.New("settings: secret namespace is required")
	}
	if normalizedOwner == "" {
		return "", errors.New("settings: secret owner is required")
	}
	prefix := "vault:" + normalizedNamespace + "/" + normalizedOwner + "/"
	if err := vault.ValidateSecretRef(prefix + "value"); err != nil {
		return "", fmt.Errorf("settings: invalid secret owner %q: %w", normalizedOwner, err)
	}
	return prefix, nil
}

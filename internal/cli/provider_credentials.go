package cli

import (
	"context"
	"errors"
	"os"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/providerauth"
	authproviders "github.com/compozy/compozy/internal/providers"
	"github.com/compozy/compozy/internal/vault"
)

func providerCredentialStatuses(
	ctx context.Context,
	provider compozyconfig.ProviderConfig,
	env *authproviders.ProbeEnv,
) ([]providerCredentialStatusItem, error) {
	statuses, err := authproviders.CredentialStatuses(ctx, provider, env)
	if err != nil {
		return nil, err
	}
	return providerCredentialStatusItems(statuses), nil
}

func providerCredentialStatusItems(statuses []authproviders.CredentialStatus) []providerCredentialStatusItem {
	if len(statuses) == 0 {
		return nil
	}
	items := make([]providerCredentialStatusItem, 0, len(statuses))
	for _, status := range statuses {
		items = append(items, providerCredentialStatusItem{
			Name:      status.Name,
			TargetEnv: status.TargetEnv,
			SecretRef: status.SecretRef,
			Kind:      status.Kind,
			Required:  status.Required,
			Present:   status.Present,
			Source:    status.Source,
		})
	}
	return items
}

func labelProviderCredentialSources(statuses []providerCredentialStatusItem, profileName string) {
	profileName = strings.TrimSpace(profileName)
	if profileName == "" {
		return
	}
	for index := range statuses {
		parsed, err := vault.ParseProfileSecretRef(statuses[index].SecretRef)
		if err == nil && parsed.ProfileName == profileName {
			statuses[index].Source = "profile " + profileName + " (override)"
			continue
		}
		statuses[index].Source = providerDefaultSource
	}
}

func providerAuthProbeEnv(
	homePaths compozyconfig.HomePaths,
	providerName string,
	provider compozyconfig.ProviderConfig,
	deps commandDeps,
) (authproviders.ProbeEnv, error) {
	commandEnv, err := providerauth.CommandEnv(homePaths, providerName, provider, os.Environ())
	if err != nil {
		return authproviders.ProbeEnv{}, err
	}
	// The disposable ACP probe launches the provider server through session
	// base validation: carry the operator working directory so the request
	// reaches the server instead of failing with cwd-required.
	commandDir, err := currentWorkingDirectory(deps)
	if err != nil {
		return authproviders.ProbeEnv{}, err
	}
	return authproviders.ProbeEnv{
		ProviderName:   providerName,
		HomePaths:      homePaths,
		ResolveCommand: deps.resolveProviderAuthCommand,
		LookupEnv: func(key string) (string, bool) {
			value := deps.getenv(key)
			return value, strings.TrimSpace(value) != ""
		},
		Vault:      cliProviderVaultMetadataResolver{homePaths: homePaths},
		CommandEnv: commandEnv,
		CommandDir: commandDir,
		RunCommand: deps.runProviderAuthCommand,
	}, nil
}

type cliProviderVaultMetadataResolver struct {
	homePaths compozyconfig.HomePaths
}

func (r cliProviderVaultMetadataResolver) GetMetadata(ctx context.Context, ref string) (vault.Metadata, error) {
	reader, err := openLocalVaultReader(ctx, r.homePaths.DatabaseFile, "provider auth")
	if err != nil {
		return vault.Metadata{}, err
	}
	metadata, readErr := vault.ReadMetadata(ctx, reader, ref)
	if err := errors.Join(readErr, reader.Close()); err != nil {
		return vault.Metadata{}, err
	}
	return metadata, nil
}

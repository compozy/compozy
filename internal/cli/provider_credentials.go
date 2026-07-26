package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/providerauth"
	authproviders "github.com/compozy/agh/internal/providers"
	"github.com/compozy/agh/internal/vault"
)

func providerCredentialStatuses(
	ctx context.Context,
	provider aghconfig.ProviderConfig,
	env *authproviders.ProbeEnv,
) ([]providerCredentialStatusItem, error) {
	statuses, err := authproviders.CredentialStatuses(ctx, provider, env)
	if err != nil {
		return nil, err
	}
	if len(statuses) == 0 {
		return nil, nil
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
	return items, nil
}

func providerAuthProbeEnv(
	homePaths aghconfig.HomePaths,
	providerName string,
	provider aghconfig.ProviderConfig,
	deps commandDeps,
) (authproviders.ProbeEnv, error) {
	commandEnv, err := providerauth.CommandEnv(homePaths, providerName, provider, os.Environ())
	if err != nil {
		return authproviders.ProbeEnv{}, err
	}
	return authproviders.ProbeEnv{
		ProviderName: providerName,
		HomePaths:    homePaths,
		LookPath:     deps.lookPath,
		LookupEnv: func(key string) (string, bool) {
			value := deps.getenv(key)
			return value, strings.TrimSpace(value) != ""
		},
		Vault:      cliProviderVaultMetadataResolver{homePaths: homePaths, getenv: deps.getenv},
		CommandEnv: commandEnv,
		RunCommand: deps.runProviderAuthCommand,
	}, nil
}

type cliProviderVaultMetadataResolver struct {
	homePaths aghconfig.HomePaths
	getenv    func(string) string
}

func (r cliProviderVaultMetadataResolver) GetMetadata(ctx context.Context, ref string) (vault.Metadata, error) {
	db, err := openLocalGlobalDatabase(ctx, r.homePaths.DatabaseFile, "provider auth")
	if err != nil {
		return vault.Metadata{}, err
	}
	lookupEnv := func(key string) (string, bool) {
		value := r.getenv(key)
		return value, strings.TrimSpace(value) != ""
	}
	service, err := vault.NewService(
		db,
		vault.NewFileKeyProvider(r.homePaths.HomeDir, lookupEnv),
		vault.WithLookupEnv(lookupEnv),
	)
	if err != nil {
		closeErr := db.Close(ctx)
		if closeErr != nil {
			return vault.Metadata{}, fmt.Errorf(
				"cli: initialize provider auth vault status: %w; close global DB: %v",
				err,
				closeErr,
			)
		}
		return vault.Metadata{}, fmt.Errorf("cli: initialize provider auth vault status: %w", err)
	}
	metadata, err := service.GetMetadata(ctx, ref)
	closeErr := db.Close(ctx)
	if err != nil {
		if errors.Is(err, vault.ErrSecretNotFound) {
			if closeErr != nil {
				return vault.Metadata{}, fmt.Errorf("cli: close provider auth vault status DB: %w", closeErr)
			}
			return vault.Metadata{}, err
		}
		if closeErr != nil {
			return vault.Metadata{}, fmt.Errorf(
				"cli: read provider credential metadata: %w; close global DB: %v",
				err,
				closeErr,
			)
		}
		return vault.Metadata{}, fmt.Errorf("cli: read provider credential metadata: %w", err)
	}
	if closeErr != nil {
		return vault.Metadata{}, fmt.Errorf("cli: close provider auth vault status DB: %w", closeErr)
	}
	return metadata, nil
}

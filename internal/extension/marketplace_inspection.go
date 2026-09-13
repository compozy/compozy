package extensionpkg

import (
	"context"
	"errors"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/marketplace"
)

// InspectPluginPackage uses the install manifest and resource loaders without publishing or starting resources.
func InspectPluginPackage(ctx context.Context, root string) (marketplace.PluginInspection, error) {
	manifest, err := LoadManifest(root)
	if err != nil {
		return marketplace.PluginInspection{}, err
	}
	if manifest.Format != FormatAgentPlugin {
		return marketplace.PluginInspection{}, errors.New(
			"extension: marketplace plugin requires an Agent Plugins manifest",
		)
	}
	contents, err := InspectPackageContents(ctx, &Extension{
		Info: ExtensionInfo{Name: manifest.Name, Version: manifest.Version}, Manifest: manifest, RootDir: root,
	}, "")
	if err != nil {
		return marketplace.PluginInspection{}, err
	}
	return marketplace.PluginInspection{
		Inputs: manifest.Inputs,
		Contents: marketplace.PluginContents{
			Skills: contents.Skills, MCPServers: contents.MCPServers, Hooks: contents.Hooks,
			Loops: contents.Loops, Agents: contents.Agents, Bridges: contents.Bridges,
		},
	}, nil
}

// InspectMarketplacePackage reads pinned artifact bytes without install consent, execution, or registry writes.
// It shares download, digest verification, extraction and manifest loading with installation, but cannot commit.
func InspectMarketplacePackage(
	ctx context.Context,
	homePaths compozyconfig.HomePaths,
	req MarketplaceInstallRequest,
	profileName string,
) (_ contract.MarketplaceExtensionDetailPayload, resultErr error) {
	if !hasCuratedMarketplaceArtifact(req.Trust) || strings.TrimSpace(req.Trust.ArchiveDigestSHA256) == "" ||
		strings.TrimSpace(req.Slug) == "" {
		return contract.MarketplaceExtensionDetailPayload{}, errors.New(
			"extension: inspection requires a pinned catalog artifact",
		)
	}
	if err := ValidateExpectedDigest(req.Trust.ArchiveDigestSHA256); err != nil {
		return contract.MarketplaceExtensionDetailPayload{}, err
	}
	if err := CheckExpectedDigest(req.ExpectedDigest, req.Trust.ArchiveDigestSHA256); err != nil {
		return contract.MarketplaceExtensionDetailPayload{}, err
	}
	req.ExpectedDigest = req.Trust.ArchiveDigestSHA256
	req.ObserveDigestVerification = nil
	install, err := prepareMarketplacePackage(ctx, homePaths, nil, req, strings.TrimSpace(req.Slug))
	if err != nil {
		return contract.MarketplaceExtensionDetailPayload{}, err
	}
	prepared := &PreparedMarketplaceManagedInstall{install: install}
	defer func() { resultErr = errors.Join(resultErr, prepared.Close()) }()
	ext := &Extension{
		Info:     ExtensionInfo{Name: install.manifest.Name, Version: install.manifest.Version, Enabled: true},
		Manifest: install.manifest,
		RootDir:  install.installPath,
	}
	contents, err := InspectPackageContents(ctx, ext, profileName)
	if err != nil {
		return contract.MarketplaceExtensionDetailPayload{}, err
	}
	servers := extensionServerPayloads(ext, profileName)
	for i := range servers {
		servers[i].Status = ""
		servers[i].Scope = firstNonEmpty(install.manifest.Resources.MCPServers[servers[i].Name].DefaultScope, "global")
	}
	result := contract.MarketplaceExtensionDetailPayload{
		InstallSlug:  req.Slug,
		ArtifactURL:  req.Trust.ArtifactURL,
		DigestSHA256: install.archiveDigest,
		Repository:   req.Trust.Repository,
		Contents:     contents,
		MCPServers:   servers,
		Inputs:       []contract.MarketplaceInputPayload{},
	}
	for _, input := range install.manifest.Inputs {
		result.Inputs = append(
			result.Inputs,
			contract.MarketplaceInputPayload{
				ID:       input.ID,
				Prompt:   input.Prompt,
				Type:     input.Type,
				Required: input.Required,
				Default:  input.Default,
				Binding: contract.MarketplaceInputBindingPayload{
					Type: input.Binding.Type,
					Name: input.Binding.Name,
				},
			},
		)
	}
	return result, nil
}

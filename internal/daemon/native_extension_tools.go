package daemon

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	core "github.com/compozy/compozy/internal/api/core"
	compozyconfig "github.com/compozy/compozy/internal/config"
	registrypkg "github.com/compozy/compozy/internal/registry"
	registrygithub "github.com/compozy/compozy/internal/registry/github"
	registrygit "github.com/compozy/compozy/internal/registry/gitsrc"
	"github.com/compozy/compozy/internal/store"
	taskpkg "github.com/compozy/compozy/internal/task"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

const (
	nativeExtensionToolsExtensionKey  = "extension"
	nativeExtensionToolsExtensionsKey = "extensions"
)

var (
	errExtensionMarketplaceNotConfigured = errors.New("daemon: extensions marketplace is not configured")
	errExtensionRegistryUnsupported      = errors.New("daemon: unsupported extension registry")
)

type extensionMarketplaceSourceLoader func(
	context.Context,
	compozyconfig.ExtensionsConfig,
) ([]registrypkg.Source, error)

type extensionNameInput struct {
	Name string `json:"name"`
}

type extensionInstallInput struct {
	Source               contract.InstallExtensionSource `json:"source"`
	Ref                  string                          `json:"ref"`
	Version              string                          `json:"version"`
	Asset                string                          `json:"asset"`
	AllowUnverified      bool                            `json:"allow_unverified"`
	ConfirmNetworkDigest string                          `json:"confirm_network_digest"`
}

type extensionUpdateInput struct {
	Name                 string `json:"name"`
	All                  bool   `json:"all"`
	CheckOnly            bool   `json:"check_only"`
	Version              string `json:"version"`
	AllowUnverified      bool   `json:"allow_unverified"`
	ConfirmNetworkDigest string `json:"confirm_network_digest"`
}

func (n *daemonNativeTools) extensionToolBindings(
	availability toolspkg.NativeAvailabilityFunc,
) map[toolspkg.ToolID]nativeToolBinding {
	return map[toolspkg.ToolID]nativeToolBinding{
		toolspkg.ToolIDExtensionsInit: {
			call:         n.extensionInit,
			availability: availability,
		},
		toolspkg.ToolIDExtensionsBuild: {
			call:         n.extensionBuild,
			availability: availability,
		},
		toolspkg.ToolIDExtensionsValidate: {
			call:         n.extensionValidate,
			availability: availability,
		},
		toolspkg.ToolIDExtensionsDev: {
			call:         n.extensionDev,
			availability: availability,
		},
		toolspkg.ToolIDExtensionsReload: {
			call:         n.extensionReload,
			availability: availability,
		},
		toolspkg.ToolIDExtensionsLogs: {
			call:         n.extensionLogs,
			availability: availability,
		},
		toolspkg.ToolIDExtensionsSearch: {
			call:         n.extensionSearch,
			availability: availability,
		},
		toolspkg.ToolIDExtensionsProvenance: {
			call:         n.extensionProvenance,
			availability: availability,
		},
		toolspkg.ToolIDExtensionsPublish: {
			call:         n.extensionPublish,
			availability: availability,
		},
		toolspkg.ToolIDExtensionsList: {
			call:         n.extensionList,
			availability: availability,
		},
		toolspkg.ToolIDExtensionsInfo: {
			call:         n.extensionInfo,
			availability: availability,
		},
		toolspkg.ToolIDExtensionsInventory: {
			call:         n.extensionInventory,
			availability: availability,
		},
		toolspkg.ToolIDExtensionsInstall: {
			call:         n.extensionInstall,
			availability: availability,
		},
		toolspkg.ToolIDExtensionsUpdate: {
			call:         n.extensionUpdate,
			availability: availability,
		},
		toolspkg.ToolIDExtensionsRemove: {
			call:         n.extensionRemove,
			availability: availability,
		},
		toolspkg.ToolIDExtensionsEnable: {
			call:         n.extensionEnable,
			availability: availability,
		},
		toolspkg.ToolIDExtensionsDisable: {
			call:         n.extensionDisable,
			availability: availability,
		},
	}
}

func (n *daemonNativeTools) extensionList(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input struct{}
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	service := n.extensionService()
	actor, err := nativeExtensionScopedActorContext(scope, req)
	if err != nil {
		return toolspkg.ToolResult{}, nativeExtensionToolError(req.ToolID, err)
	}
	var items []contract.ExtensionPayload
	if strings.TrimSpace(actor.Scope.WorkspaceID) == "" {
		items, err = service.List(ctx)
	} else {
		items, err = service.ListScoped(ctx, actor)
	}
	if err != nil {
		return toolspkg.ToolResult{}, nativeExtensionToolError(req.ToolID, err)
	}
	return structuredResult(
		map[string]any{nativeExtensionToolsExtensionsKey: items},
		fmt.Sprintf("%d installed extensions", len(items)),
	)
}

func (n *daemonNativeTools) extensionInfo(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input extensionNameInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	name, err := requiredNativeString(req.ToolID, "name", input.Name)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	actor, err := nativeExtensionScopedActorContext(scope, req)
	if err != nil {
		return toolspkg.ToolResult{}, nativeExtensionToolError(req.ToolID, err)
	}
	var item contract.ExtensionPayload
	if strings.TrimSpace(actor.Scope.WorkspaceID) == "" {
		item, err = n.extensionService().Status(ctx, name)
	} else {
		item, err = n.extensionService().StatusScoped(ctx, name, actor)
	}
	if err != nil {
		return toolspkg.ToolResult{}, nativeExtensionToolError(req.ToolID, err)
	}
	return structuredResult(map[string]any{nativeExtensionToolsExtensionKey: item}, item.Name)
}

func (n *daemonNativeTools) extensionInstall(
	ctx context.Context,
	_ toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input extensionInstallInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}

	if err := input.validate(); err != nil {
		return toolspkg.ToolResult{}, nativeExtensionValidationError(req.ToolID, err)
	}
	actor, err := nativeExtensionActorContext(req)
	if err != nil {
		return toolspkg.ToolResult{}, nativeExtensionToolError(req.ToolID, err)
	}
	item, err := n.extensionService().Install(ctx, contract.InstallExtensionRequest(input), actor)
	if err != nil {
		return toolspkg.ToolResult{}, nativeExtensionToolError(req.ToolID, err)
	}
	return structuredResult(map[string]any{nativeExtensionToolsExtensionKey: item}, item.Name)
}

func (n *daemonNativeTools) extensionUpdate(
	ctx context.Context,
	_ toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input extensionUpdateInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	name := strings.TrimSpace(input.Name)
	if input.All && name != "" {
		return toolspkg.ToolResult{}, nativeExtensionValidationError(
			req.ToolID,
			errors.New("extension update accepts name or all, not both"),
		)
	}
	if input.All && strings.TrimSpace(input.ConfirmNetworkDigest) != "" {
		return toolspkg.ToolResult{}, nativeExtensionValidationError(
			req.ToolID,
			errors.New("confirm_network_digest applies only to a single extension update"),
		)
	}
	actor, err := nativeExtensionActorContext(req)
	if err != nil {
		return toolspkg.ToolResult{}, nativeExtensionToolError(req.ToolID, err)
	}

	if name != "" {
		item, updateErr := n.extensionService().Update(ctx, name, contract.UpdateExtensionRequest{
			Version: input.Version, CheckOnly: input.CheckOnly, AllowUnverified: input.AllowUnverified,
			ConfirmNetworkDigest: strings.TrimSpace(input.ConfirmNetworkDigest),
		}, actor)
		if updateErr != nil {
			return toolspkg.ToolResult{}, nativeExtensionToolError(req.ToolID, updateErr)
		}
		return structuredResult(
			map[string]any{"updates": []contract.ManagedExtensionUpdatePayload{item}},
			"1 extension update",
		)
	}
	items, err := n.extensionService().UpdateBatch(ctx, contract.UpdateExtensionsRequest{
		All:             input.All,
		CheckOnly:       input.CheckOnly,
		Version:         input.Version,
		AllowUnverified: input.AllowUnverified,
	}, actor)
	if err != nil {
		return toolspkg.ToolResult{}, nativeExtensionUpdateToolError(req.ToolID, items, err)
	}
	return structuredResult(map[string]any{"updates": items}, fmt.Sprintf("%d extension updates", len(items)))
}

func (n *daemonNativeTools) extensionRemove(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input extensionNameInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	name, err := requiredNativeString(req.ToolID, "name", input.Name)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	actor, err := nativeExtensionScopedActorContext(scope, req)
	if err != nil {
		return toolspkg.ToolResult{}, nativeExtensionToolError(req.ToolID, err)
	}

	var item contract.ManagedExtensionRemovePayload
	if strings.TrimSpace(actor.Scope.WorkspaceID) == "" {
		item, err = n.extensionService().Remove(ctx, name, actor)
	} else {
		item, err = n.extensionService().RemoveScoped(ctx, name, actor)
	}
	if err != nil {
		return toolspkg.ToolResult{}, nativeExtensionToolError(req.ToolID, err)
	}
	return structuredResult(map[string]any{nativeExtensionToolsExtensionKey: item}, item.Name)
}

func (n *daemonNativeTools) extensionEnable(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input extensionNameInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	name, err := requiredNativeString(req.ToolID, "name", input.Name)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	actor, err := nativeExtensionScopedActorContext(scope, req)
	if err != nil {
		return toolspkg.ToolResult{}, nativeExtensionToolError(req.ToolID, err)
	}
	item, err := n.setNativeExtensionEnablement(ctx, scope, name, true, actor)
	if err != nil {
		return toolspkg.ToolResult{}, nativeExtensionToolError(req.ToolID, err)
	}
	return structuredResult(item, name)
}

func (n *daemonNativeTools) extensionDisable(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input extensionNameInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	name, err := requiredNativeString(req.ToolID, "name", input.Name)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	actor, err := nativeExtensionScopedActorContext(scope, req)
	if err != nil {
		return toolspkg.ToolResult{}, nativeExtensionToolError(req.ToolID, err)
	}
	item, err := n.setNativeExtensionEnablement(ctx, scope, name, false, actor)
	if err != nil {
		return toolspkg.ToolResult{}, nativeExtensionToolError(req.ToolID, err)
	}
	return structuredResult(item, name)
}

func (n *daemonNativeTools) setNativeExtensionEnablement(
	ctx context.Context,
	scope toolspkg.Scope,
	name string,
	enabled bool,
	actor taskpkg.ActorContext,
) (contract.ExtensionEnablementPayload, error) {
	service := n.extensionService()
	if service == nil || service.profiles == nil {
		return contract.ExtensionEnablementPayload{}, errors.New(
			"daemon: profile manager is required for extension enablement",
		)
	}
	profileID := strings.TrimSpace(scope.ProfileID)
	if profileID == "" {
		profileID = store.DefaultProfileID
	}
	profileName, err := service.profiles.ProfileName(ctx, profileID)
	if err != nil {
		return contract.ExtensionEnablementPayload{}, err
	}
	return service.SetEnablement(ctx, name, contract.SetExtensionEnablementRequest{
		Profile: profileName,
		Enabled: enabled,
	}, actor)
}

func (n *daemonNativeTools) extensionService() *daemonExtensionService {
	if service, ok := n.extensionDependency().(*daemonExtensionService); ok && service != nil {
		return service
	}
	runtime := extensionRuntime(nil)
	if n.deps.ExtensionRuntime != nil {
		runtime = n.deps.ExtensionRuntime()
	}
	var automation extensionAutomationPreviewer
	if previewer, supportsPreview := n.deps.Automation.(extensionAutomationPreviewer); supportsPreview {
		automation = previewer
	}
	service, ok := newDaemonExtensionService(&daemonExtensionServiceDeps{
		Registry:     n.deps.ExtensionRegistry,
		Runtime:      runtime,
		HookBindings: n.deps.HookBindings,
		AgentSkill:   n.deps.agentSkills(),
		ToolMCP:      n.deps.ToolMCP,
		Loops:        n.deps.LoopResources,
		Profiles:     n.deps.ProfileManager,
		HomePaths:    n.deps.HomePaths,
	},
		withDaemonExtensionMarketplace(n.deps.ExtensionConfig, n.deps.ExtensionSources),
		withDaemonExtensionEventWriter(n.deps.ExtensionEvents),
		withDaemonExtensionWorkspaceResolver(n.deps.WorkspaceResolver),
		withDaemonExtensionAutomation(automation),
	).(*daemonExtensionService)
	if !ok {
		return nil
	}
	return service
}

func (n *daemonNativeTools) extensionCoreService() core.ExtensionService {
	if service := n.extensionDependency(); service != nil {
		return service
	}
	return n.extensionService()
}

func (n *daemonNativeTools) extensionDependency() core.ExtensionService {
	if n == nil || n.deps == nil || n.deps.Extensions == nil {
		return nil
	}
	return n.deps.Extensions()
}

func defaultDaemonExtensionMarketplaceSourceLoader(
	_ context.Context,
	cfg compozyconfig.ExtensionsConfig,
) ([]registrypkg.Source, error) {
	github := cfg.Sources.GitHub
	if github.Enabled && strings.TrimSpace(github.BaseURL) == "" {
		return nil, fmt.Errorf("%w: GitHub source base URL is required", errExtensionMarketplaceNotConfigured)
	}
	sources := make([]registrypkg.Source, 0, 2)
	if github.Enabled {
		sources = append(sources, registrygithub.NewClient(github.BaseURL))
	}
	if cfg.Sources.Git.Enabled {
		sources = append(sources, registrygit.NewClient())
	}
	if len(sources) == 0 {
		return nil, errExtensionMarketplaceNotConfigured
	}
	return sources, nil
}

func (i extensionInstallInput) validate() error {
	if strings.TrimSpace(i.Ref) == "" {
		return errors.New("extension install requires ref")
	}
	switch i.Source {
	case contract.InstallExtensionSourceCurated,
		contract.InstallExtensionSourceGitHub,
		contract.InstallExtensionSourceGit,
		contract.InstallExtensionSourceLocalPath:
		return nil
	default:
		return fmt.Errorf("unsupported extension install source %q", i.Source)
	}
}

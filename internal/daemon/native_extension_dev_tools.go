package daemon

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	taskpkg "github.com/compozy/compozy/internal/task"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

type extensionInitInput struct {
	Name      string `json:"name"`
	Template  string `json:"template"`
	Directory string `json:"directory"`
}

type extensionBuildInput struct {
	SourceDir    string   `json:"source_dir"`
	OutputDir    string   `json:"output_dir"`
	BuildCommand []string `json:"build_command"`
	TimeoutMS    int64    `json:"timeout_ms"`
}

type extensionValidateInput struct {
	Path string `json:"path"`
}

type extensionDevInput struct {
	OriginPath     string `json:"origin_path"`
	GenerationHash string `json:"generation_hash"`
}

type extensionReloadInput struct {
	Name           string `json:"name"`
	GenerationHash string `json:"generation_hash"`
}

type extensionLogsInput struct {
	Name  string `json:"name"`
	After int64  `json:"after"`
}

func (n *daemonNativeTools) extensionInit(
	_ context.Context,
	_ toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input extensionInitInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	template := extensionpkg.ScaffoldTemplate(strings.TrimSpace(input.Template))
	if template == "" {
		template = extensionpkg.ScaffoldTemplateToolProviderTS
	}
	result, err := extensionpkg.ScaffoldExtension(extensionpkg.ScaffoldRequest{
		Name:      input.Name,
		Template:  template,
		Directory: input.Directory,
	})
	if err != nil {
		return toolspkg.ToolResult{}, nativeExtensionValidationError(req.ToolID, err)
	}
	return structuredResult(map[string]any{"scaffold": result}, result.Directory)
}

func (n *daemonNativeTools) extensionBuild(
	ctx context.Context,
	_ toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input extensionBuildInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	timeout := time.Duration(input.TimeoutMS) * time.Millisecond
	result, err := extensionpkg.BuildBundle(ctx, extensionpkg.BuildRequest{
		SourceDir: strings.TrimSpace(input.SourceDir),
		OutputDir: strings.TrimSpace(input.OutputDir),
		BuildCmd:  append([]string(nil), input.BuildCommand...),
		Timeout:   timeout,
	})
	if err != nil {
		return toolspkg.ToolResult{}, nativeExtensionValidationError(req.ToolID, err)
	}
	return structuredResult(map[string]any{"build": result}, result.GenerationHash)
}

func (n *daemonNativeTools) extensionValidate(
	_ context.Context,
	_ toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input extensionValidateInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	report, err := extensionpkg.ValidateBundleReport(strings.TrimSpace(input.Path))
	if err != nil {
		return toolspkg.ToolResult{}, nativeExtensionValidationError(req.ToolID, err)
	}
	return structuredResult(map[string]any{"validation": report}, "extension validation complete")
}

func (n *daemonNativeTools) extensionDev(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input extensionDevInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	actor, err := nativeExtensionScopedActorContext(scope, req)
	if err != nil {
		return toolspkg.ToolResult{}, nativeExtensionToolError(req.ToolID, err)
	}
	item, err := n.extensionService().Dev(ctx, contract.DevLinkExtensionRequest{
		OriginPath:     input.OriginPath,
		GenerationHash: input.GenerationHash,
	}, actor)
	if err != nil {
		return toolspkg.ToolResult{}, nativeExtensionToolError(req.ToolID, err)
	}
	return structuredResult(map[string]any{nativeExtensionToolsExtensionKey: item}, item.Name)
}

func (n *daemonNativeTools) extensionReload(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input extensionReloadInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	actor, err := nativeExtensionScopedActorContext(scope, req)
	if err != nil {
		return toolspkg.ToolResult{}, nativeExtensionToolError(req.ToolID, err)
	}
	item, err := n.extensionService().ReloadDev(ctx, input.Name, contract.ReloadExtensionRequest{
		GenerationHash: input.GenerationHash,
	}, actor)
	if err != nil {
		return toolspkg.ToolResult{}, nativeExtensionToolError(req.ToolID, err)
	}
	return structuredResult(map[string]any{nativeExtensionToolsExtensionKey: item}, item.Name)
}

func (n *daemonNativeTools) extensionLogs(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input extensionLogsInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	actor, err := nativeExtensionScopedActorContext(scope, req)
	if err != nil {
		return toolspkg.ToolResult{}, nativeExtensionToolError(req.ToolID, err)
	}
	logs, err := n.extensionService().ExtensionLogs(ctx, input.Name, input.After, actor)
	if err != nil {
		return toolspkg.ToolResult{}, nativeExtensionToolError(req.ToolID, err)
	}
	return structuredResult(map[string]any{"logs": logs}, "extension logs")
}

func nativeExtensionScopedActorContext(
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (taskpkg.ActorContext, error) {
	workspaceID := strings.TrimSpace(scope.WorkspaceID)
	if reqWorkspace := strings.TrimSpace(req.WorkspaceID); reqWorkspace != "" && reqWorkspace != workspaceID {
		return taskpkg.ActorContext{}, extensionpkg.ErrExtensionWorkspaceDenied
	}
	if strings.TrimSpace(req.SessionID) != "" {
		if workspaceID == "" {
			return taskpkg.ActorContext{}, extensionpkg.ErrExtensionWorkspaceDenied
		}
		return taskpkg.DeriveAgentSessionActorContextForOrigin(
			req.SessionID,
			workspaceID,
			taskpkg.OriginKindAgentSession,
			strings.TrimSpace(string(req.ToolID)),
		)
	}
	if scope.Operator {
		return taskpkg.DeriveHumanActorContextForWorkspace(
			"native-tools",
			workspaceID,
			taskpkg.OriginKindCLI,
			strings.TrimSpace(string(req.ToolID)),
		)
	}
	actor, err := taskpkg.DeriveDaemonActorContext("native-tools", strings.TrimSpace(string(req.ToolID)))
	if err != nil {
		return taskpkg.ActorContext{}, err
	}
	actor.Scope.WorkspaceID = workspaceID
	if err := actor.Validate(); err != nil {
		return taskpkg.ActorContext{}, errors.Join(err, extensionpkg.ErrExtensionWorkspaceDenied)
	}
	return actor, nil
}

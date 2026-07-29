package daemon

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/diagnostics"
	eventspkg "github.com/compozy/compozy/internal/events"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	registrygithub "github.com/compozy/compozy/internal/registry/github"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

type extensionSearchInput struct {
	Query   string   `json:"query"`
	Sources []string `json:"sources"`
	Limit   int      `json:"limit"`
	Cursor  string   `json:"cursor"`
}

type extensionPublishInput struct {
	GenerationDir string `json:"generation_dir"`
	Repository    string `json:"repository"`
	TagName       string `json:"tag_name"`
	Draft         bool   `json:"draft"`
}

func (n *daemonNativeTools) extensionSearch(
	ctx context.Context,
	_ toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input extensionSearchInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	limit := input.Limit
	if limit == 0 {
		limit = defaultExtensionRegistrySearchLimit
	}
	result, err := n.extensionService().Search(ctx, contract.ExtensionSearchRequest{
		Query: input.Query, Sources: input.Sources, Limit: limit, Cursor: input.Cursor,
	})
	if err != nil {
		return toolspkg.ToolResult{}, nativeExtensionToolError(req.ToolID, err)
	}
	return structuredResult(result, fmt.Sprintf("%d extension search results", len(result.Items)))
}

func (n *daemonNativeTools) extensionProvenance(
	ctx context.Context,
	_ toolspkg.Scope,
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
	provenance, err := n.extensionService().Provenance(ctx, name)
	if err != nil {
		return toolspkg.ToolResult{}, nativeExtensionToolError(req.ToolID, err)
	}
	return structuredResult(map[string]any{"provenance": provenance}, name)
}

func (n *daemonNativeTools) extensionPublish(
	ctx context.Context,
	_ toolspkg.Scope,
	req toolspkg.CallRequest,
) (_ toolspkg.ToolResult, err error) {
	var input extensionPublishInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	for _, required := range []struct {
		field string
		value string
	}{
		{field: "generation_dir", value: input.GenerationDir},
		{field: "repository", value: input.Repository},
		{field: "tag_name", value: input.TagName},
	} {
		if _, err := requiredNativeString(req.ToolID, required.field, required.value); err != nil {
			return toolspkg.ToolResult{}, err
		}
	}
	manifest, err := extensionpkg.LoadManifest(strings.TrimSpace(input.GenerationDir))
	if err != nil {
		return toolspkg.ToolResult{}, nativeExtensionToolError(req.ToolID, err)
	}
	actor, err := nativeExtensionActorContext(req)
	if err != nil {
		return toolspkg.ToolResult{}, nativeExtensionToolError(req.ToolID, err)
	}
	event := extensionpkg.LifecycleEvent{
		Type: eventspkg.ExtensionPublishFailed, ExtensionName: manifest.Name,
	}
	defer func() {
		if err == nil {
			event.Type = eventspkg.ExtensionPublishCompleted
		}
		service := n.extensionService()
		if service != nil {
			err = errors.Join(err, service.recordCanonicalExtensionLifecycleEvent(ctx, actor, event))
		}
	}()
	token, err := n.resolveExtensionPublishCredential(ctx)
	if err != nil {
		return toolspkg.ToolResult{}, nativeExtensionToolError(req.ToolID, err)
	}
	cleanupSecret := diagnostics.RegisterDynamicSecret(token)
	defer cleanupSecret()
	client := registrygithub.NewClient(
		n.deps.ExtensionConfig.Sources.GitHub.BaseURL,
		registrygithub.WithToken(token),
	)
	defer func() {
		err = errors.Join(err, client.Close())
	}()
	result, err := extensionpkg.PublishRelease(ctx, client, extensionpkg.PublishRequest{
		GenerationDir: strings.TrimSpace(input.GenerationDir),
		Repository:    strings.TrimSpace(input.Repository),
		TagName:       strings.TrimSpace(input.TagName),
		Draft:         input.Draft,
	})
	if err != nil {
		return toolspkg.ToolResult{}, nativeExtensionToolError(req.ToolID, err)
	}
	return structuredResult(map[string]any{"publish": result}, result.ReleaseURL)
}

func (n *daemonNativeTools) resolveExtensionPublishCredential(ctx context.Context) (string, error) {
	if n == nil || n.deps == nil || n.deps.ExtensionSecrets == nil {
		return "", fmt.Errorf(
			"%w: configure GITHUB_TOKEN or vault:github/publish",
			extensionpkg.ErrExtensionPublishCredentialUnavailable,
		)
	}
	for _, ref := range []string{"env:GITHUB_TOKEN", "vault:github/publish"} {
		value, err := n.deps.ExtensionSecrets.ResolveRef(ctx, ref)
		if err == nil && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value), nil
		}
	}
	return "", fmt.Errorf(
		"%w: configure GITHUB_TOKEN or vault:github/publish",
		extensionpkg.ErrExtensionPublishCredentialUnavailable,
	)
}

const defaultExtensionRegistrySearchLimit = 20

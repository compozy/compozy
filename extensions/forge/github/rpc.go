package github

import (
	"context"
	"errors"
	"fmt"
	"io"

	extensioncontract "github.com/compozy/compozy/internal/extension/contract"
	compozysdk "github.com/compozy/compozy/sdk/go"
)

// RunProvider serves the bundled provider over the public extension SDK protocol.
func RunProvider(ctx context.Context, stdin io.Reader, stdout io.Writer, stderr io.Writer) error {
	if ctx == nil {
		return errors.New("github forge: context is required")
	}
	if stdin == nil || stdout == nil || stderr == nil {
		return errors.New("github forge: stdio is required")
	}
	provider := newProvider()
	extension := compozysdk.NewExtension(
		providerDefinition(),
		compozysdk.WithStdio(stdin, stdout),
		compozysdk.WithStderr(stderr),
	)
	if err := compozysdk.ForgeProvider(extension, forgeProviderHandlers(provider)); err != nil {
		return fmt.Errorf("github forge: register provider: %w", err)
	}
	if err := extension.Run(ctx); err != nil {
		return fmt.Errorf("github forge: run extension protocol: %w", err)
	}
	return nil
}

func forgeProviderHandlers(provider *Provider) compozysdk.ForgeProviderHandlers {
	return compozysdk.ForgeProviderHandlers{
		Capabilities: func(
			ctx context.Context,
			_ compozysdk.ExtensionContext,
			request compozysdk.ForgeCapabilitiesRequest,
		) (compozysdk.ForgeCapabilitiesResponse, error) {
			response, err := provider.Capabilities(ctx, extensioncontract.ForgeCapabilitiesRequest(request))
			return compozysdk.ForgeCapabilitiesResponse(response), err
		},
		Status: func(
			ctx context.Context,
			_ compozysdk.ExtensionContext,
			request compozysdk.ForgeStatusRequest,
		) (compozysdk.ForgeStatusResponse, error) {
			response, err := provider.Status(ctx, extensioncontract.ForgeStatusRequest(request))
			return compozysdk.ForgeStatusResponse(response), err
		},
		CreatePR: func(
			ctx context.Context,
			_ compozysdk.ExtensionContext,
			request compozysdk.ForgePRCreateRequest,
		) (compozysdk.ForgePRCreateResponse, error) {
			response, err := provider.CreatePR(ctx, extensioncontract.ForgePRCreateRequest(request))
			return compozysdk.ForgePRCreateResponse(response), err
		},
	}
}

func providerDefinition() compozysdk.ExtensionDefinition {
	return compozysdk.ExtensionDefinition{
		Name:        Name,
		Version:     "0.1.0",
		Description: "First-party GitHub pull request provider for worktree exit actions.",
		RequiresEnv: []string{"GITHUB_TOKEN"},
		Capabilities: compozysdk.CapabilitiesConfig{
			Provides: []string{compozysdk.CapabilityProvideForgeProvider},
		},
		Subprocess: compozysdk.DescribeSubprocess{
			Command: "{{compozy_executable}}",
			Args:    []string{"__internal", "extension-provider", Name},
		},
	}
}

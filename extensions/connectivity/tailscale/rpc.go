package tailscale

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"

	compozysdk "github.com/compozy/compozy/sdk/go"
)

const providerConfigurationRPCErrorCode = -32000

// RunProvider serves the bundled provider over the public extension SDK protocol.
func RunProvider(ctx context.Context, stdin io.Reader, stdout io.Writer, stderr io.Writer) error {
	if ctx == nil {
		return errors.New("tailscale: context is required")
	}
	if stdin == nil {
		return errors.New("tailscale: stdin is required")
	}
	if stdout == nil {
		return errors.New("tailscale: stdout is required")
	}
	if stderr == nil {
		return errors.New("tailscale: stderr is required")
	}
	stateDir, err := defaultStateDirectory()
	if err != nil {
		return fmt.Errorf("tailscale: prepare provider state: %w", err)
	}
	logger := slog.New(slog.NewTextHandler(stderr, nil))
	provider, err := NewProvider(stateDir, logger)
	if err != nil {
		return fmt.Errorf("tailscale: create provider: %w", err)
	}
	extension := compozysdk.NewExtension(
		providerDefinition(),
		compozysdk.WithStdio(stdin, stdout),
		compozysdk.WithStderr(stderr),
	)
	if err := compozysdk.ConnectivityProvider(extension, compozysdk.ConnectivityProviderHandlers{
		Establish: func(
			ctx context.Context,
			extensionContext compozysdk.ExtensionContext,
			request compozysdk.ConnectivityEstablishRequest,
		) (compozysdk.ConnectivityReachability, error) {
			reachability, establishErr := provider.Establish(ctx, extensionContext, request)
			return reachability, providerRPCError(establishErr)
		},
		Status:   provider.Status,
		Teardown: provider.Teardown,
	}); err != nil {
		return fmt.Errorf("tailscale: register provider services: %w", err)
	}
	runErr := extension.Run(ctx)
	if runErr != nil {
		runErr = fmt.Errorf("tailscale: run extension protocol: %w", runErr)
	}
	stopCtx, cancel := context.WithTimeout(context.Background(), providerShutdownTimeout)
	defer cancel()
	closeErr := provider.Close(stopCtx)
	if closeErr != nil {
		closeErr = fmt.Errorf("tailscale: close provider: %w", closeErr)
	}
	return errors.Join(runErr, closeErr)
}

func providerRPCError(err error) error {
	if !errors.Is(err, errAuthKeyBindingRequired) {
		return err
	}
	return compozysdk.NewRPCError(
		providerConfigurationRPCErrorCode,
		"TS_AUTHKEY binding is required",
		nil,
	)
}

func providerDefinition() compozysdk.ExtensionDefinition {
	return compozysdk.ExtensionDefinition{
		Name:        Name,
		Version:     "0.1.0",
		Description: "First-party private tailnet and public Funnel connectivity for CompozyOS gateways.",
		RequiresEnv: []string{"TS_AUTHKEY"},
		Capabilities: compozysdk.CapabilitiesConfig{
			Provides: []string{compozysdk.CapabilityProvideConnectivityProvider},
		},
		Subprocess: compozysdk.DescribeSubprocess{
			Command: "{{compozy_executable}}",
			Args:    []string{"__internal", "extension-provider", Name},
			Env:     map[string]string{"COMPOZY_HOME": "{{env:COMPOZY_HOME}}"},
		},
		NetworkParticipation: &compozysdk.NetworkParticipationRequirement{
			Required:      true,
			Mode:          "live",
			ChannelScopes: []string{"gateway.private", "gateway.public"},
		},
	}
}

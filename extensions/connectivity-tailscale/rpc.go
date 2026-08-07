package connectivitytailscale

import (
	"context"
	"errors"
	"io"
	"log/slog"

	compozysdk "github.com/compozy/compozy/sdk/go"
)

// RunProvider serves the bundled provider over the public extension SDK protocol.
func RunProvider(ctx context.Context, stdin io.Reader, stdout io.Writer, stderr io.Writer) error {
	if ctx == nil {
		return errors.New("connectivity-tailscale: context is required")
	}
	if stdin == nil {
		return errors.New("connectivity-tailscale: stdin is required")
	}
	if stdout == nil {
		return errors.New("connectivity-tailscale: stdout is required")
	}
	if stderr == nil {
		return errors.New("connectivity-tailscale: stderr is required")
	}
	stateDir, err := defaultStateDirectory()
	if err != nil {
		return err
	}
	logger := slog.New(slog.NewTextHandler(stderr, nil))
	provider, err := NewProvider(stateDir, logger)
	if err != nil {
		return err
	}
	extension := compozysdk.NewExtension(
		providerDefinition(),
		compozysdk.WithStdio(stdin, stdout),
		compozysdk.WithStderr(stderr),
	)
	if err := compozysdk.ConnectivityProvider(extension, compozysdk.ConnectivityProviderHandlers{
		Establish: provider.Establish,
		Status:    provider.Status,
		Teardown:  provider.Teardown,
	}); err != nil {
		return err
	}
	runErr := extension.Run(ctx)
	stopCtx, cancel := context.WithTimeout(context.Background(), providerShutdownTimeout)
	defer cancel()
	return errors.Join(runErr, provider.Close(stopCtx))
}

func providerDefinition() compozysdk.ExtensionDefinition {
	return compozysdk.ExtensionDefinition{
		Name:        Name,
		Version:     "0.1.0",
		Description: "First-party private tailnet and public Funnel connectivity for Compozy gateways.",
		RequiresEnv: []string{"TS_AUTHKEY"},
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

package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	compozysdk "github.com/compozy/compozy/sdk/go"
)

func main() {
	extension := compozysdk.NewExtension(compozysdk.ExtensionDefinition{
		Name: "__EXTENSION_NAME__", Version: "0.1.0",
		Description: "Provide verified daemon reachability",
		Subprocess:  compozysdk.DescribeSubprocess{Command: "./bin"},
		Capabilities: compozysdk.CapabilitiesConfig{
			Provides: []string{compozysdk.CapabilityProvideConnectivityProvider},
		},
		NetworkParticipation: &compozysdk.NetworkParticipationRequirement{
			Required: true, Mode: "live",
			ChannelScopes: []string{"gateway.private", "gateway.public"},
		},
	})
	if err := compozysdk.ConnectivityProvider(extension, compozysdk.ConnectivityProviderHandlers{
		Establish: establish,
		Status:    status,
		Teardown:  teardown,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "register connectivity provider: %v\n", err)
		os.Exit(1)
	}
	if err := extension.Run(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "run extension: %v\n", err)
		os.Exit(1)
	}
}

func establish(
	context.Context,
	compozysdk.ExtensionContext,
	compozysdk.ConnectivityEstablishRequest,
) (compozysdk.ConnectivityReachability, error) {
	return compozysdk.ConnectivityReachability{}, errors.New("implement provider forwarding")
}

func status(
	context.Context,
	compozysdk.ExtensionContext,
	compozysdk.ConnectivityStatusRequest,
) (compozysdk.ConnectivityReachability, error) {
	return compozysdk.ConnectivityReachability{}, errors.New("implement provider health reporting")
}

func teardown(
	context.Context,
	compozysdk.ExtensionContext,
	compozysdk.ConnectivityTeardownRequest,
) (compozysdk.ConnectivityTeardownResponse, error) {
	return compozysdk.ConnectivityTeardownResponse{Stopped: true}, nil
}

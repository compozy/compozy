package main

import (
	"context"
	"fmt"
	"os"

	sdk "github.com/compozy/compozy/sdk/go"
)

func main() {
	extension := sdk.NewExtension(
		sdk.ExtensionDefinition{
			Name:        "__EXTENSION_NAME__",
			Version:     "0.1.0",
			Description: "Poll check-in source items",
			Subprocess:  sdk.DescribeSubprocess{Command: "./bin"},
		},
	)
	if err := extension.HeartbeatSource(
		"updates",
		sdk.HeartbeatSourceKind{Title: "New updates"},
		func(_ context.Context, req sdk.HeartbeatSourceRequest) (sdk.HeartbeatSourceResult, error) {
			return sdk.HeartbeatSourceResult{Items: []sdk.HeartbeatSourceItem{}, State: req.State}, nil
		},
	); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := extension.Run(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

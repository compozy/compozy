package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	compozysdk "github.com/compozy/compozy/sdk/go"
)

type reviewWatchSpec struct {
	Kind  string `json:"kind"`
	Query string `json:"query,omitempty"`
}

func main() {
	extension := compozysdk.NewExtension(compozysdk.ExtensionDefinition{
		Name:        "__EXTENSION_NAME__",
		Version:     "0.1.0",
		Description: "Poll a review watch source",
		Subprocess:  compozysdk.DescribeSubprocess{Command: "./bin"},
		Capabilities: compozysdk.CapabilitiesConfig{
			Provides: []string{compozysdk.CapabilityProvideWatchSource},
		},
	})
	if err := compozysdk.WatchSource[reviewWatchSpec](
		extension,
		"reviews",
		compozysdk.WatchSourceOptions{},
		func(
			_ context.Context,
			req compozysdk.WatchSourceRequest[reviewWatchSpec],
		) (compozysdk.WatchPollResponse, error) {
			payload, err := json.Marshal(map[string]string{"kind": req.Spec.Kind, "query": req.Spec.Query})
			if err != nil {
				return compozysdk.WatchPollResponse{}, fmt.Errorf("encode watch payload: %w", err)
			}
			return compozysdk.WatchPollResponse{
				Ready:       false,
				StateDigest: "manual:" + req.Spec.Query,
				Payload:     payload,
			}, nil
		},
	); err != nil {
		fmt.Fprintf(os.Stderr, "register watch source: %v\n", err)
		os.Exit(1)
	}
	if err := extension.Run(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "run extension: %v\n", err)
		os.Exit(1)
	}
}

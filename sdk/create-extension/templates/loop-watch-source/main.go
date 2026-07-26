package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	aghsdk "github.com/compozy/agh/sdk/go"
)

type ReviewWatchSpec struct {
	Kind  string `json:"kind"`
	Query string `json:"query,omitempty"`
}

func main() {
	extension := aghsdk.NewExtension(aghsdk.ExtensionDefinition{
		Name:    "__EXTENSION_NAME__",
		Version: "0.1.0",
		Capabilities: aghsdk.CapabilitiesConfig{
			Provides: []string{"loop.watch_source"},
		},
	})

	if err := aghsdk.WatchSource[ReviewWatchSpec](
		extension,
		"reviews",
		aghsdk.WatchSourceOptions{},
		func(_ context.Context, req aghsdk.WatchSourceRequest[ReviewWatchSpec]) (aghsdk.WatchPollResponse, error) {
			payload, err := json.Marshal(map[string]string{
				"kind":  req.Spec.Kind,
				"query": req.Spec.Query,
			})
			if err != nil {
				return aghsdk.WatchPollResponse{}, fmt.Errorf("encode watch payload: %w", err)
			}
			return aghsdk.WatchPollResponse{
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

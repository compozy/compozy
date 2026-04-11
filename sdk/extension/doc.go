// Package extension provides the public Go SDK for Compozy executable
// extensions.
//
// An executable extension runs as a subprocess alongside Compozy and
// communicates over line-delimited JSON-RPC 2.0 on stdin/stdout. The SDK
// handles protocol negotiation, capability exchange, hook dispatch, event
// delivery, health checks, and graceful shutdown.
//
// # Quick start
//
//	ext := extension.New("my-ext", "0.1.0").
//		OnRunPostShutdown(func(
//			ctx context.Context,
//			hook extension.HookContext,
//			payload extension.RunPostShutdownPayload,
//		) error {
//			fmt.Fprintf(os.Stderr, "run %s finished\n", payload.RunID)
//			return nil
//		})
//	if err := ext.Start(context.Background()); err != nil {
//		log.Fatal(err)
//	}
//
// # Host API
//
// After initialization, extension handlers can call back into the host
// through [Extension.Host]. The [HostAPI] exposes typed clients for events,
// tasks, runs, artifacts, prompts, and memory.
//
// # Testing
//
// Import the [github.com/compozy/compozy/sdk/extension/testing] package for
// [exttesting.TestHarness] and [exttesting.MockTransport], which simulate the
// host side of the protocol for in-process unit tests.
package extension

// Package exttesting provides in-process test utilities for Compozy extension
// authors.
//
// [TestHarness] simulates the host side of the extension subprocess protocol,
// letting authors exercise their extension logic without a running Compozy
// instance. [MockTransport] provides connected in-memory transport pairs for
// unit tests that operate at the message level.
//
// # Quick start
//
//	harness := exttesting.NewTestHarness(exttesting.HarnessOptions{
//		GrantedCapabilities: []extension.Capability{extension.CapabilityRunMutate},
//	})
//	ext := extension.New("my-ext", "0.1.0").
//		OnRunPostShutdown(func(
//			ctx context.Context,
//			hook extension.HookContext,
//			payload extension.RunPostShutdownPayload,
//		) error {
//			return nil
//		})
//	errCh := harness.Run(context.Background(), ext)
//	resp, _ := harness.Initialize(ctx, extension.InitializeRequestIdentity{
//		Name: "my-ext", Version: "0.1.0", Source: "workspace",
//	})
//	_ = harness.Shutdown(ctx, extension.ShutdownRequest{Reason: "test"})
//	<-errCh
package exttesting

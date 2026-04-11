# Compozy Extension SDK for Go

Package `extension` provides the public Go SDK for building Compozy executable extensions.

An executable extension runs as a subprocess alongside Compozy and communicates over line-delimited JSON-RPC 2.0 on stdin/stdout. The SDK handles protocol negotiation, capability exchange, hook dispatch, event delivery, health checks, and graceful shutdown.

## Install

```bash
go get github.com/compozy/compozy/sdk/extension
```

## Quick start

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/compozy/compozy/sdk/extension"
)

func main() {
	ext := extension.New("hello-ext", "0.1.0").
		OnRunPostShutdown(func(ctx context.Context, hook extension.HookContext, payload extension.RunPostShutdownPayload) error {
			fmt.Fprintf(os.Stderr, "run %s finished with %s\n", payload.RunID, payload.Summary.Status)
			return nil
		})

	if err := ext.Start(context.Background()); err != nil {
		log.Fatal(err)
	}
}
```

## Public surface

- `Extension` manages initialize, hook dispatch, event delivery, health checks, and shutdown.
- `HostAPI` exposes typed clients for `host.events.*`, `host.tasks.*`, `host.runs.*`, `host.artifacts.*`, `host.prompts.render`, and `host.memory.*`.
- 29 typed hook registration methods (`OnPlanPreDiscover` through `OnArtifactPostWrite`) with strongly-typed payload and patch parameters.
- Constants for all 19 capabilities, 28 hook names, execution modes, and output formats.
- Protocol version `1`.

## Capabilities

Extensions declare the capabilities they require. The host grants or denies each capability during initialization. Common capabilities:

| Capability                           | Grants                                         |
| ------------------------------------ | ---------------------------------------------- |
| `events.read`                        | Subscribe to and receive forwarded events      |
| `plan.mutate`                        | Register `plan.*` hooks                        |
| `prompt.mutate`                      | Register `prompt.*` hooks                      |
| `run.mutate`                         | Register `run.*` hooks                         |
| `artifacts.read` / `artifacts.write` | Read or write workspace artifacts via Host API |
| `tasks.read` / `tasks.create`        | List, get, or create tasks via Host API        |
| `memory.read` / `memory.write`       | Read or write workflow memory via Host API     |

## Testing

```bash
go get github.com/compozy/compozy/sdk/extension/testing
```

The `exttesting` package provides `TestHarness` and `MockTransport` for in-process SDK tests without a running Compozy instance.

```go
package myext_test

import (
	"context"
	"testing"

	"github.com/compozy/compozy/sdk/extension"
	exttesting "github.com/compozy/compozy/sdk/extension/testing"
)

func TestExtension(t *testing.T) {
	harness := exttesting.NewTestHarness(exttesting.HarnessOptions{
		GrantedCapabilities: []extension.Capability{extension.CapabilityRunMutate},
	})

	ext := extension.New("my-ext", "0.1.0").
		OnRunPostShutdown(func(ctx context.Context, hook extension.HookContext, payload extension.RunPostShutdownPayload) error {
			return nil
		})

	ctx := context.Background()
	errCh := harness.Run(ctx, ext)
	_, err := harness.Initialize(ctx, extension.InitializeRequestIdentity{
		Name: "my-ext", Version: "0.1.0", Source: "workspace",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := harness.Shutdown(ctx, extension.ShutdownRequest{Reason: "test"}); err != nil {
		t.Fatal(err)
	}
	if err := <-errCh; err != nil {
		t.Fatal(err)
	}
}
```

## Documentation

- [Author guide](../../.compozy/docs/extensibility/index.md)
- [Getting started](../../.compozy/docs/extensibility/getting-started.md)
- [Hello world in Go](../../.compozy/docs/extensibility/hello-world-go.md)
- [Hook reference](../../.compozy/docs/extensibility/hook-reference.md)
- [Host API reference](../../.compozy/docs/extensibility/host-api-reference.md)
- [Capability reference](../../.compozy/docs/extensibility/capability-reference.md)
- [Testing guide](../../.compozy/docs/extensibility/testing.md)

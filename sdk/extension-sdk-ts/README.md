# @compozy/extension-sdk

TypeScript SDK for Compozy executable extensions.

The package mirrors the public Go SDK in [`sdk/extension`](../extension) and speaks the Compozy extension protocol over line-delimited JSON-RPC 2.0 on stdin/stdout.

## Install

```bash
npm install @compozy/extension-sdk
```

Node 18+ is required.

## Quick start

```ts
import { Extension } from "@compozy/extension-sdk";

const extension = new Extension("hello-ext", "0.1.0").onRunPostShutdown(
  async (_context, payload) => {
    process.stderr.write(`run ${payload.run_id} finished with ${payload.summary.status}\n`);
  }
);

extension.start().catch(error => {
  process.stderr.write(`${error instanceof Error ? error.message : String(error)}\n`);
  process.exitCode = 1;
});
```

## Public surface

- `Extension` manages initialize, hook dispatch, event delivery, health checks, and shutdown.
- `HostAPI` exposes typed clients for `host.events.*`, `host.tasks.*`, `host.runs.*`, `host.artifacts.*`, `host.prompts.render`, and `host.memory.*`.
- `HOOKS`, `CAPABILITIES`, and the exported payload and patch interfaces match protocol version `1`.
- `@compozy/extension-sdk/testing` exposes `MockTransport` and `TestHarness` for author-side tests.

## Starter templates

The published package also ships the starter templates used by `@compozy/create-extension`:

- `lifecycle-observer`
- `prompt-decorator`
- `review-provider`
- `skill-pack`

## Documentation

- [Author docs](../../.compozy/docs/extensibility/index.md)
- [Getting started](../../.compozy/docs/extensibility/getting-started.md)
- [Hook reference](../../.compozy/docs/extensibility/hook-reference.md)
- [Host API reference](../../.compozy/docs/extensibility/host-api-reference.md)
- [Testing guide](../../.compozy/docs/extensibility/testing.md)

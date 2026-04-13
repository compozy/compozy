# Testing Extensions

Test the extension at two levels:

- fast in-process tests against the SDK harness
- at least one real subprocess smoke test against the Compozy runtime

## In-process tests with `@compozy/extension-sdk/testing`

The testing package exposes:

- `TestHarness` for host-driven initialize, hook dispatch, event delivery, health checks, and shutdown
- `MockTransport` for lower-level transport tests

Example:

```ts
import test from "node:test";
import assert from "node:assert/strict";

import { CAPABILITIES, HOOKS } from "@compozy/extension-sdk";
import { TestHarness } from "@compozy/extension-sdk/testing";

import { createPromptDecoratorExtension } from "../src/extension.js";

test("decorates prompt text", async () => {
  const harness = new TestHarness({
    granted_capabilities: [CAPABILITIES.promptMutate],
  });
  const extension = createPromptDecoratorExtension();

  const run = harness.run(extension);
  await harness.initialize({ name: "demo", version: "0.1.0", source: "workspace" });

  const response = await harness.dispatchHook(
    "hook-1",
    {
      name: HOOKS.promptPostBuild,
      event: HOOKS.promptPostBuild,
      mutable: true,
      required: false,
      priority: 500,
      timeout_ms: 5000,
    },
    {
      run_id: "run-1",
      job_id: "job-1",
      prompt_text: "hello",
      batch_params: { name: "demo" },
    }
  );

  assert.deepEqual(response, {
    patch: { prompt_text: "hello\n\nDecorated by demo." },
  });

  await harness.shutdown({ reason: "run_completed", deadline_ms: 1000 });
  await run;
});
```

## Transport tests

Use `MockTransport` when you need to assert:

- request and response ID correlation
- out-of-order response handling
- structured JSON-RPC error propagation
- shutdown behavior and EOF handling

## Real runtime smoke tests

Add a subprocess test when any of these are true:

- the extension depends on real stdin/stdout framing
- the extension reads environment variables at runtime
- the extension relies on actual manifest-driven command execution
- you want coverage for startup and shutdown against the real manager

The lifecycle-observer starter template in this repository has that kind of smoke coverage on the Go side.

## Suggested test matrix

- initialize succeeds with the expected accepted capabilities
- initialize fails on unsupported protocol versions
- each mutable hook returns the patch shape you expect
- event subscriptions are narrowed correctly
- shutdown exits cleanly
- any Host API call you rely on round-trips through the harness

## Coverage guidance

Aim for:

- unit coverage on transport, initialize negotiation, and Host API helpers
- template-local tests in the scaffolded project
- one real manager-driven smoke test per executable template family

## Common mistakes

- testing only the patch shape and never running the extension subprocess
- writing artifacts or memory files directly instead of exercising the Host API
- assuming observe-only hook delivery order is strictly serialized

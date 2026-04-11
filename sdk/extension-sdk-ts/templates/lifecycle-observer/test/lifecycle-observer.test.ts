import assert from "node:assert/strict";
import test from "node:test";

import { HOOKS } from "@compozy/extension-sdk";
import { TestHarness } from "@compozy/extension-sdk/testing";
import { CAPABILITIES } from "@compozy/extension-sdk";

import { createLifecycleObserverExtension } from "../src/extension.js";

test("lifecycle observer acknowledges run.post_shutdown", async () => {
  const extension = createLifecycleObserverExtension();
  const harness = new TestHarness({
    granted_capabilities: [CAPABILITIES.runMutate],
  });

  const runPromise = harness.run(extension);
  const response = await harness.initialize({
    name: "__EXTENSION_NAME__",
    version: "__EXTENSION_VERSION__",
    source: "workspace",
  });

  assert.deepEqual(response.accepted_capabilities, [CAPABILITIES.runMutate]);
  assert.deepEqual(response.supported_hook_events, [HOOKS.runPostShutdown]);

  const hookResponse = await harness.dispatchHook(
    "hook-1",
    {
      name: HOOKS.runPostShutdown,
      event: HOOKS.runPostShutdown,
      mutable: false,
      required: false,
      priority: 500,
      timeout_ms: 5000,
    },
    {
      run_id: "run-1",
      reason: "run_completed",
      summary: {
        status: "completed",
        jobs_total: 1,
      },
    }
  );

  assert.deepEqual(hookResponse, {});

  const shutdown = await harness.shutdown({
    reason: "run_completed",
    deadline_ms: 1000,
  });
  assert.equal(shutdown.acknowledged, true);

  await runPromise;
});

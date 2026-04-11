import assert from "node:assert/strict";
import test from "node:test";

import { CAPABILITIES, HOOKS } from "@compozy/extension-sdk";
import { TestHarness } from "@compozy/extension-sdk/testing";

import { createPromptDecoratorExtension } from "../src/extension.js";

test("prompt decorator appends text to the rendered prompt", async () => {
  const extension = createPromptDecoratorExtension();
  const harness = new TestHarness({
    granted_capabilities: [CAPABILITIES.promptMutate],
  });

  const runPromise = harness.run(extension);
  await harness.initialize({
    name: "__EXTENSION_NAME__",
    version: "__EXTENSION_VERSION__",
    source: "workspace",
  });

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
      prompt_text: "Base prompt",
      batch_params: {
        name: "demo",
      },
    }
  );

  assert.deepEqual(response, {
    patch: {
      prompt_text: "Base prompt\n\nDecorated by __EXTENSION_NAME__.",
    },
  });

  await harness.shutdown({
    reason: "run_completed",
    deadline_ms: 1000,
  });
  await runPromise;
});

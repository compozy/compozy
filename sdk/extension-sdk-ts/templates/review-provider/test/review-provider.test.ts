import assert from "node:assert/strict";
import test from "node:test";

import { CAPABILITIES } from "@compozy/extension-sdk";
import { TestHarness } from "@compozy/extension-sdk/testing";

import { createReviewProviderExtension } from "../src/extension.js";

test("review provider template registers and serves executable review handlers", async () => {
  const extension = createReviewProviderExtension();
  const harness = new TestHarness({
    granted_capabilities: [CAPABILITIES.providersRegister],
  });

  const runPromise = harness.run(extension);
  const initialize = await harness.initialize({
    name: "__EXTENSION_NAME__",
    version: "__EXTENSION_VERSION__",
    source: "workspace",
  });
  assert.deepEqual(initialize.registered_review_providers, ["__EXTENSION_NAME__-review"]);

  const fetched = await harness.call<Array<Record<string, unknown>>>("fetch_reviews", {
    provider: "__EXTENSION_NAME__-review",
    pr: "123",
    include_nitpicks: true,
  });
  assert.deepEqual(fetched, [
    {
      title: "Fetched review for 123",
      file: "README.md",
      body: "Handled by __EXTENSION_NAME__-review",
      provider_ref: "thread-ts-1",
    },
  ]);

  await assert.doesNotReject(() =>
    harness.call("resolve_issues", {
      provider: "__EXTENSION_NAME__-review",
      pr: "123",
      issues: [{ file_path: "issue_001.md", provider_ref: "thread-ts-1" }],
    })
  );

  await harness.shutdown({
    reason: "run_completed",
    deadline_ms: 1000,
  });
  await runPromise;
});

import { describe, expect, it } from "vitest";

import { Extension } from "../src/extension.js";
import { CAPABILITIES, HOOKS } from "../src/types.js";
import { TestHarness } from "../src/testing/test_harness.js";

describe("Extension fluent hook registration", () => {
  it("registers every public hook helper and negotiates the required capabilities", async () => {
    const extension = new Extension("sdk-ext", "1.0.0")
      .onPlanPreDiscover(async () => ({}))
      .onPlanPostDiscover(async () => ({}))
      .onPlanPreGroup(async () => ({}))
      .onPlanPostGroup(async () => ({}))
      .onPlanPrePrepareJobs(async () => ({}))
      .onPlanPostPrepareJobs(async () => ({}))
      .onPromptPreBuild(async () => ({}))
      .onPromptPostBuild(async () => ({}))
      .onPromptPreSystem(async () => ({}))
      .onAgentPreSessionCreate(async () => ({}))
      .onAgentPostSessionCreate(async () => {})
      .onAgentPreSessionResume(async () => ({}))
      .onAgentOnSessionUpdate(async () => {})
      .onAgentPostSessionEnd(async () => {})
      .onJobPreExecute(async () => ({}))
      .onJobPostExecute(async () => {})
      .onJobPreRetry(async () => ({}))
      .onRunPreStart(async () => ({}))
      .onRunPostStart(async () => {})
      .onRunPreShutdown(async () => {})
      .onRunPostShutdown(async () => {})
      .onReviewPreFetch(async () => ({}))
      .onReviewPostFetch(async () => ({}))
      .onReviewPreBatch(async () => ({}))
      .onReviewPostFix(async () => {})
      .onReviewPreResolve(async () => ({}))
      .onArtifactPreWrite(async () => ({}))
      .onArtifactPostWrite(async () => {})
      .onEvent(async () => {}, "run.completed")
      .onHealthCheck(async () => ({ healthy: true }))
      .onShutdown(async () => {});

    const harness = new TestHarness({
      granted_capabilities: [
        CAPABILITIES.eventsRead,
        CAPABILITIES.planMutate,
        CAPABILITIES.promptMutate,
        CAPABILITIES.agentMutate,
        CAPABILITIES.jobMutate,
        CAPABILITIES.runMutate,
        CAPABILITIES.reviewMutate,
        CAPABILITIES.artifactsWrite,
      ],
    });
    harness.handleHostMethod("host.events.subscribe", async () => ({
      subscription_id: "sub-1",
    }));

    const runPromise = harness.run(extension);
    const response = await harness.initialize({
      name: "sdk-ext",
      version: "1.0.0",
      source: "workspace",
    });

    expect(response.supported_hook_events).toEqual(Object.values(HOOKS).sort());
    expect(response.accepted_capabilities).toEqual([
      CAPABILITIES.agentMutate,
      CAPABILITIES.artifactsWrite,
      CAPABILITIES.eventsRead,
      CAPABILITIES.jobMutate,
      CAPABILITIES.planMutate,
      CAPABILITIES.promptMutate,
      CAPABILITIES.reviewMutate,
      CAPABILITIES.runMutate,
    ]);
    await eventually(() => {
      expect(harness.hostCalls().map(call => call.method)).toContain("host.events.subscribe");
    });

    const shutdown = await harness.shutdown({
      reason: "run_completed",
      deadline_ms: 1000,
    });
    expect(shutdown.acknowledged).toBe(true);
    await expect(runPromise).resolves.toBeUndefined();
  });
});

async function eventually(assertion: () => void): Promise<void> {
  let lastError: unknown;
  for (let attempt = 0; attempt < 25; attempt += 1) {
    try {
      assertion();
      return;
    } catch (error) {
      lastError = error;
      await new Promise(resolve => setTimeout(resolve, 10));
    }
  }
  throw lastError;
}

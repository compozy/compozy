import { describe, expect, it } from "vitest";

import { Extension } from "../src/extension.js";
import { CAPABILITIES, HOOKS } from "../src/types.js";
import { RPCError } from "../src/transport.js";
import { TestHarness } from "../src/testing/test_harness.js";

describe("Extension", () => {
  it("processes initialize and records negotiated state", async () => {
    const extension = new Extension("sdk-ext", "1.0.0").onPromptPostBuild(async () => ({
      prompt_text: "patched",
    }));
    const harness = new TestHarness({
      granted_capabilities: [CAPABILITIES.promptMutate],
    });

    const runPromise = harness.run(extension);
    const response = await harness.initialize({
      name: "sdk-ext",
      version: "1.0.0",
      source: "workspace",
    });

    expect(response.protocol_version).toBe("1");
    expect(response.accepted_capabilities).toEqual([CAPABILITIES.promptMutate]);
    expect(response.supported_hook_events).toEqual([HOOKS.promptPostBuild]);
    expect(extension.initializeRequest()?.extension.name).toBe("sdk-ext");
    expect(extension.acceptedCapabilitiesList()).toEqual([CAPABILITIES.promptMutate]);

    const shutdown = await harness.shutdown({
      reason: "run_completed",
      deadline_ms: 1000,
    });
    expect(shutdown.acknowledged).toBe(true);

    await expect(runPromise).resolves.toBeUndefined();
  });

  it("rejects unsupported protocol versions", async () => {
    const extension = new Extension("sdk-ext", "1.0.0");
    const harness = new TestHarness({
      protocol_version: "9",
      supported_protocol_versions: ["9"],
    });

    const runPromise = harness.run(extension);
    await expect(
      harness.initialize({
        name: "sdk-ext",
        version: "1.0.0",
        source: "workspace",
      })
    ).rejects.toMatchObject<RPCError>({
      code: -32602,
      data: { reason: "unsupported_protocol_version" },
    });
    await expect(runPromise).rejects.toMatchObject<RPCError>({
      code: -32602,
    });
  });

  it("rejects initialize when granted capabilities are insufficient", async () => {
    const extension = new Extension("sdk-ext", "1.0.0").withCapabilities(CAPABILITIES.promptMutate);
    const harness = new TestHarness();

    const runPromise = harness.run(extension);
    await expect(
      harness.initialize({
        name: "sdk-ext",
        version: "1.0.0",
        source: "workspace",
      })
    ).rejects.toMatchObject<RPCError>({
      code: -32001,
      message: "Capability denied",
    });
    await expect(runPromise).rejects.toMatchObject<RPCError>({
      code: -32001,
    });
  });

  it("filters on_event deliveries and subscribes through the host API", async () => {
    const received: string[] = [];
    const extension = new Extension("sdk-ext", "1.0.0").onEvent(async event => {
      received.push(event.kind);
    }, "run.completed");
    const harness = new TestHarness({
      granted_capabilities: [CAPABILITIES.eventsRead],
    });
    harness.handleHostMethod("host.events.subscribe", async () => ({
      subscription_id: "sub-1",
    }));

    const runPromise = harness.run(extension);
    await harness.initialize({
      name: "sdk-ext",
      version: "1.0.0",
      source: "workspace",
    });

    await eventually(() => {
      expect(harness.hostCalls().map(call => call.method)).toContain("host.events.subscribe");
    });

    await harness.sendEvent({
      schema_version: "1.0",
      run_id: "run-1",
      seq: 1,
      ts: new Date().toISOString(),
      kind: "run.completed",
      payload: {},
    });
    await harness.sendEvent({
      schema_version: "1.0",
      run_id: "run-1",
      seq: 2,
      ts: new Date().toISOString(),
      kind: "job.failed",
      payload: {},
    });

    expect(received).toEqual(["run.completed"]);

    await harness.shutdown({
      reason: "run_completed",
      deadline_ms: 1000,
    });
    await expect(runPromise).resolves.toBeUndefined();
  });

  it("dispatches review.post_fetch using the issues patch shape", async () => {
    const extension = new Extension("sdk-ext", "1.0.0").onReviewPostFetch(
      async (_context, payload) => ({
        issues: [...(payload.issues ?? []), { name: "issue_002.md" }],
      })
    );
    const harness = new TestHarness({
      granted_capabilities: [CAPABILITIES.reviewMutate],
    });

    const runPromise = harness.run(extension);
    await harness.initialize({
      name: "sdk-ext",
      version: "1.0.0",
      source: "workspace",
    });

    const response = await harness.dispatchHook(
      "hook-review-1",
      {
        name: HOOKS.reviewPostFetch,
        event: HOOKS.reviewPostFetch,
        mutable: true,
        required: false,
        priority: 500,
        timeout_ms: 5000,
      },
      {
        run_id: "run-1",
        pr: "123",
        issues: [{ name: "issue_001.md" }],
      }
    );

    expect(response).toEqual({
      patch: {
        issues: [{ name: "issue_001.md" }, { name: "issue_002.md" }],
      },
    });

    await harness.shutdown({
      reason: "run_completed",
      deadline_ms: 1000,
    });
    await expect(runPromise).resolves.toBeUndefined();
  });

  it("registers and dispatches review watch fluent hooks", async () => {
    const seenFinished: unknown[] = [];
    const extension = new Extension("sdk-ext", "1.0.0")
      .onReviewWatchPreRound(async (_context, payload) => ({
        runtime_overrides: {
          auto_commit: payload.runtime_overrides !== undefined,
          model: "gpt-5.5",
        },
        continue: true,
      }))
      .onReviewWatchPostRound(async (_context, payload) => {
        expect(payload.child_run_id).toBe("child-1");
      })
      .onReviewWatchPrePush(async (_context, payload) => ({
        remote: `${payload.remote}-fork`,
        push: true,
      }))
      .onReviewWatchFinished(async (_context, payload) => {
        seenFinished.push(payload);
      });
    const harness = new TestHarness({
      granted_capabilities: [CAPABILITIES.reviewMutate],
    });

    const runPromise = harness.run(extension);
    const init = await harness.initialize({
      name: "sdk-ext",
      version: "1.0.0",
      source: "workspace",
    });

    expect(init.supported_hook_events).toEqual([
      HOOKS.reviewWatchFinished,
      HOOKS.reviewWatchPostRound,
      HOOKS.reviewWatchPrePush,
      HOOKS.reviewWatchPreRound,
    ]);

    await expect(
      harness.dispatchHook(
        "hook-watch-round-1",
        {
          name: HOOKS.reviewWatchPreRound,
          event: HOOKS.reviewWatchPreRound,
          mutable: true,
          required: false,
          priority: 500,
          timeout_ms: 5000,
        },
        {
          run_id: "run-watch",
          provider: "coderabbit",
          pr: "123",
          workflow: "demo",
          round: 1,
          head_sha: "head-1",
          status: "current_reviewed",
          nitpicks: true,
          runtime_overrides: { auto_commit: true },
          continue: true,
        }
      )
    ).resolves.toEqual({
      patch: {
        runtime_overrides: {
          auto_commit: true,
          model: "gpt-5.5",
        },
        continue: true,
      },
    });

    await expect(
      harness.dispatchHook(
        "hook-watch-push-1",
        {
          name: HOOKS.reviewWatchPrePush,
          event: HOOKS.reviewWatchPrePush,
          mutable: true,
          required: false,
          priority: 500,
          timeout_ms: 5000,
        },
        {
          run_id: "run-watch",
          provider: "coderabbit",
          pr: "123",
          workflow: "demo",
          round: 1,
          head_sha: "head-2",
          remote: "origin",
          branch: "feature",
          push: true,
        }
      )
    ).resolves.toEqual({
      patch: {
        remote: "origin-fork",
        push: true,
      },
    });

    await harness.dispatchHook(
      "hook-watch-post-round-1",
      {
        name: HOOKS.reviewWatchPostRound,
        event: HOOKS.reviewWatchPostRound,
        mutable: false,
        required: false,
        priority: 500,
        timeout_ms: 5000,
      },
      {
        run_id: "run-watch",
        provider: "coderabbit",
        pr: "123",
        workflow: "demo",
        round: 1,
        child_run_id: "child-1",
        status: "completed",
      }
    );
    await harness.dispatchHook(
      "hook-watch-finished-1",
      {
        name: HOOKS.reviewWatchFinished,
        event: HOOKS.reviewWatchFinished,
        mutable: false,
        required: false,
        priority: 500,
        timeout_ms: 5000,
      },
      {
        run_id: "run-watch",
        child_run_id: "child-1",
        provider: "coderabbit",
        pr: "123",
        workflow: "demo",
        status: "completed",
        terminal_reason: "review watch clean",
      }
    );
    expect(seenFinished).toHaveLength(1);

    await harness.shutdown({
      reason: "run_completed",
      deadline_ms: 1000,
    });
    await expect(runPromise).resolves.toBeUndefined();
  });

  it("dispatches fetch_reviews to a registered review provider", async () => {
    const extension = new Extension("sdk-ext", "1.0.0").registerReviewProvider("sdk-review", {
      async fetchReviews(context, request) {
        expect(context.provider).toBe("sdk-review");
        expect(request).toEqual({
          pr: "123",
          include_nitpicks: true,
        });
        return [
          {
            title: "issue",
            file: "README.md",
            body: "from provider",
            provider_ref: "thread-1",
          },
        ];
      },
    });
    const harness = new TestHarness({
      granted_capabilities: [CAPABILITIES.providersRegister],
    });

    const runPromise = harness.run(extension);
    const response = await harness.initialize({
      name: "sdk-ext",
      version: "1.0.0",
      source: "workspace",
    });
    expect(response.accepted_capabilities).toEqual([CAPABILITIES.providersRegister]);
    expect(response.registered_review_providers).toEqual(["sdk-review"]);

    await expect(
      harness.call("fetch_reviews", {
        provider: "sdk-review",
        pr: "123",
        include_nitpicks: true,
      })
    ).resolves.toEqual([
      {
        title: "issue",
        file: "README.md",
        body: "from provider",
        provider_ref: "thread-1",
      },
    ]);

    await harness.shutdown({
      reason: "run_completed",
      deadline_ms: 1000,
    });
    await expect(runPromise).resolves.toBeUndefined();
  });

  it("dispatches resolve_issues to a registered review provider", async () => {
    const seen: unknown[] = [];
    const extension = new Extension("sdk-ext", "1.0.0")
      .withCapabilities(CAPABILITIES.providersRegister)
      .registerReviewProvider("sdk-review", {
        async resolveIssues(context, request) {
          expect(context.provider).toBe("sdk-review");
          seen.push(request);
        },
      });
    const harness = new TestHarness({
      granted_capabilities: [CAPABILITIES.providersRegister],
    });

    const runPromise = harness.run(extension);
    await harness.initialize({
      name: "sdk-ext",
      version: "1.0.0",
      source: "workspace",
    });

    await expect(
      harness.call("resolve_issues", {
        provider: "sdk-review",
        pr: "123",
        issues: [{ file_path: "issue_001.md", provider_ref: "thread-1" }],
      })
    ).resolves.toEqual({});
    expect(seen).toEqual([
      {
        pr: "123",
        issues: [{ file_path: "issue_001.md", provider_ref: "thread-1" }],
      },
    ]);

    await harness.shutdown({
      reason: "run_completed",
      deadline_ms: 1000,
    });
    await expect(runPromise).resolves.toBeUndefined();
  });

  it("rejects initialize when a registered review provider was not granted providers.register", async () => {
    const extension = new Extension("sdk-ext", "1.0.0").registerReviewProvider("sdk-review", {});
    const harness = new TestHarness();

    const runPromise = harness.run(extension);
    await expect(
      harness.initialize({
        name: "sdk-ext",
        version: "1.0.0",
        source: "workspace",
      })
    ).rejects.toMatchObject<RPCError>({
      code: -32001,
      data: {
        method: "initialize",
        required: [CAPABILITIES.providersRegister],
        granted: [],
      },
    });
    await expect(runPromise).rejects.toMatchObject<RPCError>({
      code: -32001,
      data: {
        method: "initialize",
        required: [CAPABILITIES.providersRegister],
        granted: [],
      },
    });
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

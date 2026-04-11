import { describe, expect, it } from "vitest";

import { HOOKS } from "../src/types.js";
import {
  isMutableHook,
  registerMutableHook,
  registerObserverHook,
  requestContext,
  type HookRegistrationSurface,
  type RawHookHandler,
} from "../src/handlers.js";

class TestSurface implements HookRegistrationSurface {
  readonly handlers = new Map<string, RawHookHandler>();

  handle(hook: string, handler: RawHookHandler): this {
    this.handlers.set(hook, handler);
    return this;
  }
}

describe("handlers helpers", () => {
  it("marks observer hooks as non-mutable", () => {
    expect(isMutableHook(HOOKS.promptPostBuild)).toBe(true);
    expect(isMutableHook(HOOKS.runPostShutdown)).toBe(false);
    expect(isMutableHook(HOOKS.artifactPostWrite)).toBe(false);
  });

  it("wraps mutable and observer handlers on the registration surface", async () => {
    const surface = new TestSurface();
    const mutablePayloads: string[] = [];
    const observerPayloads: string[] = [];

    registerMutableHook(
      surface,
      HOOKS.promptPostBuild,
      async (_context, payload: { prompt_text: string }) => {
        mutablePayloads.push(payload.prompt_text);
        return { prompt_text: `${payload.prompt_text}\npatched` };
      }
    );
    registerObserverHook(
      surface,
      HOOKS.runPostShutdown,
      async (_context, payload: { run_id: string }) => {
        observerPayloads.push(payload.run_id);
      }
    );

    const context = requestContext(
      {
        invocation_id: "hook-1",
        hook: {
          name: HOOKS.promptPostBuild,
          event: HOOKS.promptPostBuild,
          mutable: true,
          required: false,
          priority: 500,
          timeout_ms: 5000,
        },
        payload: { prompt_text: "hello" },
      },
      { tasks: {} } as never
    );

    const mutableResult = await surface.handlers.get(HOOKS.promptPostBuild)?.(context, {
      prompt_text: "hello",
    });
    const observerResult = await surface.handlers.get(HOOKS.runPostShutdown)?.(context, {
      run_id: "run-1",
    });

    expect(mutablePayloads).toEqual(["hello"]);
    expect(mutableResult).toEqual({ prompt_text: "hello\npatched" });
    expect(observerPayloads).toEqual(["run-1"]);
    expect(observerResult).toBeUndefined();
  });

  it("builds the hook request context from the incoming request", () => {
    const host = { tasks: {} } as never;
    const context = requestContext(
      {
        invocation_id: "hook-2",
        hook: {
          name: HOOKS.jobPreExecute,
          event: HOOKS.jobPreExecute,
          mutable: true,
          required: true,
          priority: 100,
          timeout_ms: 2000,
        },
        payload: {},
      },
      host
    );

    expect(context).toEqual({
      invocation_id: "hook-2",
      hook: {
        name: HOOKS.jobPreExecute,
        event: HOOKS.jobPreExecute,
        mutable: true,
        required: true,
        priority: 100,
        timeout_ms: 2000,
      },
      host,
    });
  });
});

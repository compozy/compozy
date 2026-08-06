import { describe, expect, it, vi } from "vitest";

import { loopDetailByName } from "../../mocks/fixtures";
import { loopEditorLogic } from "../loop-editor-store";
import type { ValidateLoopResult } from "../../types";

function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (reason?: unknown) => void;
  const promise = new Promise<T>((resolvePromise, rejectPromise) => {
    resolve = resolvePromise;
    reject = rejectPromise;
  });
  return { promise, reject, resolve };
}

describe("loopEditorLogic", () => {
  // Invariant: positions, publish, and validation advance independently; scope
  // replacement and stale settlements cannot mutate the current draft. Owning
  // layer: the loop editor store logic. Canonical suite: this file.
  it("accepts only the latest validation settlement", async () => {
    const store = loopEditorLogic.createStore(undefined);
    const definition = loopDetailByName.get("implement-tasks")!.definition;
    const first = deferred<ValidateLoopResult>();
    const second = deferred<ValidateLoopResult>();
    const valid: ValidateLoopResult = { valid: true, errors: [] };

    store.trigger.draftInitialized({ definition, edges: [], nodes: [] });
    store.trigger.validationRequested({
      execute: () => first.promise,
      notifyFailure: false,
    });
    store.trigger.validationRequested({
      execute: () => second.promise,
      notifyFailure: false,
    });

    expect(store.getSnapshot().context.pendingValidationGeneration).toBe(2);
    expect(store.getSnapshot().context.validationGeneration).toBe(2);

    first.resolve(valid);
    await vi.waitFor(() => {
      expect(store.getSnapshot().context.lint.validated).toBe(false);
      expect(store.getSnapshot().context.pendingValidationGeneration).toBe(2);
    });

    second.resolve(valid);
    await vi.waitFor(() => {
      expect(store.getSnapshot().context.lint.validated).toBe(true);
      expect(store.getSnapshot().context.pendingValidationGeneration).toBeNull();
    });
  });

  it("runs request regions in parallel and fences replaced draft work", async () => {
    const store = loopEditorLogic.createStore(undefined);
    const loop = loopDetailByName.get("implement-tasks")!;
    const positions = deferred<unknown>();
    const publish = deferred<typeof loop>();
    const validation = deferred<{ valid: true; errors: [] }>();
    let publishCompleted = 0;
    store.on("publishCompleted", () => {
      publishCompleted += 1;
    });

    store.trigger.draftInitialized({
      definition: loop.definition,
      edges: [],
      nodes: [],
    });
    store.trigger.layoutApplied({ nodes: [] });
    store.trigger.positionsSaveRequested({ execute: () => positions.promise });
    store.trigger.publishRequested({ execute: () => publish.promise });
    store.trigger.validationRequested({
      execute: () => validation.promise,
      notifyFailure: false,
    });

    expect(store.getSnapshot().context.pendingPositionsGeneration).not.toBeNull();
    expect(store.getSnapshot().context.pendingPublishGeneration).not.toBeNull();
    expect(store.getSnapshot().context.pendingValidationGeneration).not.toBeNull();

    positions.resolve(undefined);
    await vi.waitFor(() => {
      expect(store.getSnapshot().context.pendingPositionsGeneration).toBeNull();
      expect(store.getSnapshot().context.pendingPublishGeneration).not.toBeNull();
      expect(store.getSnapshot().context.pendingValidationGeneration).not.toBeNull();
    });

    store.trigger.draftInitialized({
      definition: loop.definition,
      edges: [],
      nodes: [],
    });
    expect(store.getSnapshot().context.pendingPositionsGeneration).toBeNull();
    expect(store.getSnapshot().context.pendingPublishGeneration).toBeNull();
    expect(store.getSnapshot().context.pendingValidationGeneration).toBeNull();

    publish.resolve(loop);
    validation.resolve({ valid: true, errors: [] });
    await vi.waitFor(() => {
      expect(store.getSnapshot().context.lint.validated).toBe(false);
      expect(publishCompleted).toBe(0);
    });
  });

  it("Should settle a stale publish without completing the current editor flow", async () => {
    const store = loopEditorLogic.createStore(undefined);
    const loop = loopDetailByName.get("implement-tasks")!;
    const publish = deferred<typeof loop>();
    const published = vi.fn();
    store.on("publishCompleted", published);
    store.trigger.draftInitialized({ definition: loop.definition, edges: [], nodes: [] });
    store.trigger.publishRequested({ execute: () => publish.promise });

    store.trigger.connectionCreated({ edges: [] });
    publish.resolve({ ...loop, version: loop.version + 1 });

    await vi.waitFor(() => {
      expect(store.getSnapshot().context.pendingPublishGeneration).toBeNull();
      expect(store.getSnapshot().context.baseDefinition?.meta.version).toBe(loop.version + 1);
    });
    expect(store.getSnapshot().context.isDirty).toBe(true);
    expect(published).not.toHaveBeenCalled();
  });

  it("Should complete the current editor flow after a matching publish", async () => {
    const store = loopEditorLogic.createStore(undefined);
    const loop = loopDetailByName.get("implement-tasks")!;
    const published = vi.fn();
    store.on("publishCompleted", published);
    store.trigger.draftInitialized({ definition: loop.definition, edges: [], nodes: [] });
    store.trigger.publishRequested({
      execute: async () => ({ ...loop, version: loop.version + 1 }),
    });

    await vi.waitFor(() => {
      expect(store.getSnapshot().context.pendingPublishGeneration).toBeNull();
      expect(published).toHaveBeenCalledOnce();
    });
  });

  it("Should surface a publish failure even when the draft changed after submission", async () => {
    const store = loopEditorLogic.createStore(undefined);
    const loop = loopDetailByName.get("implement-tasks")!;
    const publish = deferred<typeof loop>();
    store.trigger.draftInitialized({ definition: loop.definition, edges: [], nodes: [] });
    store.trigger.publishRequested({ execute: () => publish.promise });

    store.trigger.connectionCreated({ edges: [] });
    publish.reject(new Error("publish unavailable"));

    await vi.waitFor(() => {
      expect(store.getSnapshot().context.pendingPublishGeneration).toBeNull();
      expect(store.getSnapshot().context.publishError).toBe("publish unavailable");
    });
  });
});

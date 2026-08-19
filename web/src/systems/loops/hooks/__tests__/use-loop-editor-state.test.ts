import { afterEach, describe, expect, it, vi } from "vitest";

import { LoopValidationError } from "../../adapters/loops-api";
import type { EditorEdge, EditorNode } from "../../lib/codec";
import { loopPaletteItems, type PaletteItem } from "../../lib/loop-palette";
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

function editorNode(id: string, x: number): EditorNode {
  return {
    id,
    type: "loopNode",
    position: { x, y: 40 },
    data: {
      raw: { id, class: "action", kind: "transform" },
      nodeClass: "action",
      kind: "transform",
      hasError: false,
    },
  };
}

function editorEdge(source: string, target: string, index: number): EditorEdge {
  return {
    id: `edge-${index}`,
    source,
    target,
    data: { raw: { from: source, to: target } },
  };
}

function paletteItem(kind: string): PaletteItem {
  const item = loopPaletteItems().find(candidate => candidate.kindLabel === kind);
  if (!item) throw new Error(`Palette item ${kind} not found`);
  return item;
}

function loopDefinitionFixture() {
  const detail = loopDetailByName.get("implement-tasks");
  if (!detail) throw new Error("implement-tasks fixture not found");
  return detail.definition;
}

afterEach(() => {
  vi.useRealTimers();
});

describe("loopEditorLogic", () => {
  // Invariant: positions, publish, and validation advance independently; scope
  // replacement and stale settlements cannot mutate the current draft. Owning
  // layer: the loop editor store logic. Canonical suite: this file.
  it("accepts only the latest validation settlement", async () => {
    const store = loopEditorLogic.createStore(undefined);
    const definition = loopDefinitionFixture();
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

  it("Should preserve an edited draft when the same source is observed again", () => {
    const store = loopEditorLogic.createStore(undefined);
    const definition = loopDefinitionFixture();
    store.trigger.draftInitialized({
      definition,
      edges: [],
      nodes: [],
      sourceKey: "workspace-a:loop-a:workspace",
    });
    store.trigger.connectionCreated({ edges: [] });

    store.trigger.draftInitialized({
      definition,
      edges: [],
      nodes: [],
      sourceKey: "workspace-a:loop-a:workspace",
    });

    expect(store.getSnapshot().context.isDirty).toBe(true);
    expect(store.getSnapshot().context.initializedSourceKey).toBe("workspace-a:loop-a:workspace");
  });

  it("Should debounce automatic validation to the latest store event", () => {
    vi.useFakeTimers();
    const store = loopEditorLogic.createStore(undefined);
    const definition = loopDefinitionFixture();
    const first = vi.fn();
    const latest = vi.fn();
    store.trigger.draftInitialized({ definition, edges: [], nodes: [] });
    const revision = store.getSnapshot().context.structuralRevision;

    store.trigger.automaticValidationRequested({ enabled: true, execute: first, revision });
    store.trigger.automaticValidationRequested({ enabled: true, execute: latest, revision });
    vi.advanceTimersByTime(399);
    expect(first).not.toHaveBeenCalled();
    expect(latest).not.toHaveBeenCalled();
    vi.advanceTimersByTime(1);

    expect(first).not.toHaveBeenCalled();
    expect(latest).toHaveBeenCalledOnce();
  });

  it("Should cancel a queued automatic validation when its retained editor becomes inactive", () => {
    vi.useFakeTimers();
    const store = loopEditorLogic.createStore(undefined);
    const definition = loopDetailByName.get("implement-tasks")!.definition;
    const validate = vi.fn();
    store.trigger.draftInitialized({ definition, edges: [], nodes: [] });
    const revision = store.getSnapshot().context.structuralRevision;

    store.trigger.automaticValidationRequested({ enabled: true, execute: validate, revision });
    vi.advanceTimersByTime(200);
    store.trigger.automaticValidationRequested({ enabled: false, execute: validate, revision });
    vi.advanceTimersByTime(400);

    expect(validate).not.toHaveBeenCalled();
  });

  it("Should notify once for each annotations-error transition", () => {
    const store = loopEditorLogic.createStore(undefined);
    const notify = vi.fn();

    store.trigger.annotationsStatusObserved({ failed: true, notify });
    store.trigger.annotationsStatusObserved({ failed: true, notify });
    expect(notify).toHaveBeenCalledOnce();

    store.trigger.annotationsStatusObserved({ failed: false, notify });
    store.trigger.annotationsStatusObserved({ failed: true, notify });
    expect(notify).toHaveBeenCalledTimes(2);
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
      expect(store.getSnapshot().context.publishFailureKind).toBe("transport");
    });
  });

  it("Should keep publish-rejected issues on the strip when the draft moved on", async () => {
    const store = loopEditorLogic.createStore(undefined);
    const loop = loopDetailByName.get("implement-tasks")!;
    const publish = deferred<typeof loop>();
    store.trigger.draftInitialized({ definition: loop.definition, edges: [], nodes: [] });
    store.trigger.publishRequested({ execute: () => publish.promise });
    store.trigger.connectionCreated({ edges: [] });
    publish.reject(
      new LoopValidationError("Publish rejected", {
        valid: false,
        errors: [
          {
            node_id: "implement",
            code: "fan_out_unbounded",
            message: "too wide",
            severity: "error",
          },
        ],
      })
    );

    await vi.waitFor(() => {
      expect(store.getSnapshot().context.pendingPublishGeneration).toBeNull();
      expect(store.getSnapshot().context.publishRejectedDockStale).toBe(true);
      expect(store.getSnapshot().context.publishFailureKind).toBe("rejected");
      expect(store.getSnapshot().context.publishRejectedIssues).toHaveLength(1);
      expect(store.getSnapshot().context.lint.validated).toBe(false);
    });
  });

  it("Should mark an explicitly positioned add dirty for both definition and annotations", () => {
    const store = loopEditorLogic.createStore(undefined);
    const definition = loopDefinitionFixture();
    store.trigger.draftInitialized({
      definition,
      edges: [],
      nodes: [editorNode("prepare", 40)],
    });
    const before = store.getSnapshot().context;

    store.trigger.nodeAdded({ item: paletteItem("transform"), position: { x: 320, y: 180 } });

    const after = store.getSnapshot().context;
    expect(after.nodes.at(-1)).toMatchObject({
      id: "transform",
      position: { x: 320, y: 180 },
    });
    expect(after.isDirty).toBe(true);
    expect(after.positionsDirty).toBe(true);
    expect(after.positionsGeneration).toBe(before.positionsGeneration + 1);
    expect(after.structuralRevision).toBe(before.structuralRevision + 1);
  });

  it("Should add a node and its connection in one structural transition", () => {
    const store = loopEditorLogic.createStore(undefined);
    const definition = loopDefinitionFixture();
    store.trigger.draftInitialized({
      definition,
      edges: [],
      nodes: [editorNode("prepare", 40)],
    });
    const before = store.getSnapshot().context;

    store.trigger.nodeAddedWithEdge({
      item: paletteItem("transform"),
      position: { x: 320, y: 180 },
      source: "prepare",
    });

    const after = store.getSnapshot().context;
    expect(after.nodes).toHaveLength(2);
    expect(after.edges).toHaveLength(1);
    expect(after.edges[0]).toMatchObject({
      source: "prepare",
      target: "transform",
      data: { raw: { from: "prepare", to: "transform" } },
    });
    expect(after.structuralRevision).toBe(before.structuralRevision + 1);
    expect(after.validationGeneration).toBe(before.validationGeneration + 1);
  });

  it("Should prune every incident edge when nodes are deleted", () => {
    const store = loopEditorLogic.createStore(undefined);
    const definition = loopDefinitionFixture();
    store.trigger.draftInitialized({
      definition,
      nodes: [editorNode("prepare", 40), editorNode("apply", 240), editorNode("finish", 440)],
      edges: [editorEdge("prepare", "apply", 0), editorEdge("apply", "finish", 1)],
    });
    const before = store.getSnapshot().context;

    store.trigger.nodesDeleted({ nodeIds: ["apply"] });

    const after = store.getSnapshot().context;
    expect(after.nodes.map(node => node.id)).toEqual(["prepare", "finish"]);
    expect(after.edges).toEqual([]);
    expect(after.structuralRevision).toBe(before.structuralRevision + 1);
    expect(after.validationGeneration).toBe(before.validationGeneration + 1);
  });
});

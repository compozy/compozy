import { notifyUser } from "@/lib/user-feedback";

import { LoopValidationError } from "../adapters/loops-api";
import { editorDefinitionFromLoop } from "../lib/loop-editor-definition";
import { graphToDefinition } from "../lib/codec";
import { applyLintToNodes, buildLintState } from "../lib/loop-editor-lint";

import type {
  LoopEditorEnqueue,
  LoopEditorEvents,
  LoopEditorState,
} from "./loop-editor-store-contracts";

function clearPublishPending(current: LoopEditorState, generation: number): LoopEditorState {
  return current.pendingPublishGeneration === generation
    ? { ...current, pendingPublishGeneration: null }
    : current;
}

export const loopEditorPublishTransitions = {
  publishFailed: (current: LoopEditorState, event: LoopEditorEvents["publishFailed"]) => {
    if (
      current.scopeGeneration !== event.scopeGeneration ||
      current.pendingPublishGeneration !== event.generation
    ) {
      return;
    }
    const next = clearPublishPending(current, event.generation);
    return {
      ...next,
      publishError: event.error,
      publishFailureKind: "transport" as const,
      publishRejectedIssues: [],
      publishRejectedDockStale: false,
    };
  },
  publishRejected: (current: LoopEditorState, event: LoopEditorEvents["publishRejected"]) => {
    if (
      current.scopeGeneration !== event.scopeGeneration ||
      current.pendingPublishGeneration !== event.generation
    ) {
      return;
    }
    const next = clearPublishPending(current, event.generation);
    const lint = buildLintState(event.result);
    const publishError = `Publish rejected — ${lint.errorCount} issue${lint.errorCount === 1 ? "" : "s"} to resolve.`;
    // The strip lists what the daemon returned, even when the draft moved on and the dock's
    // lint state is intentionally left alone.
    const publishRejectedIssues = lint.issues;
    if (current.structuralRevision !== event.revision) {
      return {
        ...next,
        publishError,
        publishFailureKind: "rejected" as const,
        publishRejectedIssues,
        publishRejectedDockStale: true,
      };
    }
    return {
      ...next,
      lint,
      nodes: applyLintToNodes(current.nodes, lint.byNode),
      publishError,
      publishFailureKind: "rejected" as const,
      publishRejectedIssues,
      publishRejectedDockStale: false,
    };
  },
  publishRequested: (
    current: LoopEditorState,
    event: LoopEditorEvents["publishRequested"],
    enqueue: LoopEditorEnqueue
  ) => {
    if (!current.baseDefinition || current.pendingPublishGeneration !== null) return;
    const definition = graphToDefinition(current.baseDefinition, current.nodes, current.edges);
    const expectedVersion = current.baseDefinition.meta.version ?? null;
    const generation = current.publishGeneration + 1;
    const revision = current.structuralRevision;
    const scopeGeneration = current.scopeGeneration;
    enqueue.effect(async ({ trigger }) => {
      try {
        const loop = await event.execute(definition, expectedVersion);
        trigger.publishSucceeded({ generation, loop, revision, scopeGeneration });
      } catch (error) {
        if (error instanceof LoopValidationError) {
          trigger.publishRejected({
            generation,
            result: error.result,
            revision,
            scopeGeneration,
          });
          return;
        }
        trigger.publishFailed({
          error: error instanceof Error ? error.message : "Failed to publish loop",
          generation,
          revision,
          scopeGeneration,
        });
      }
    });
    return {
      ...current,
      pendingPublishGeneration: generation,
      publishError: null,
      publishFailureKind: null,
      publishRejectedIssues: [],
      publishRejectedDockStale: false,
      publishGeneration: generation,
      validationGeneration: current.validationGeneration + 1,
    };
  },
  publishSucceeded: (
    current: LoopEditorState,
    event: LoopEditorEvents["publishSucceeded"],
    enqueue: LoopEditorEnqueue
  ) => {
    if (
      current.scopeGeneration !== event.scopeGeneration ||
      current.pendingPublishGeneration !== event.generation
    ) {
      return;
    }
    const next = clearPublishPending(current, event.generation);
    const baseDefinition = editorDefinitionFromLoop(event.loop);
    enqueue.effect(() =>
      notifyUser({
        message: `Published ${event.loop.name} v${event.loop.version}`,
        tone: "success",
      })
    );
    if (current.structuralRevision !== event.revision) {
      return { ...next, baseDefinition, isDirty: true };
    }
    const lint = buildLintState({ valid: true, errors: [] });
    enqueue.emit.publishCompleted({ loop: event.loop });
    return {
      ...next,
      baseDefinition,
      isDirty: false,
      lint,
      nodes: applyLintToNodes(current.nodes, lint.byNode),
    };
  },
};

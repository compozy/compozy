import { notifyUser } from "@/lib/user-feedback";

import { graphToDefinition } from "../lib/codec";
import { applyLintToNodes, buildLintState } from "../lib/loop-editor-lint";

import type {
  LoopEditorEnqueue,
  LoopEditorEvents,
  LoopEditorState,
} from "./loop-editor-store-contracts";

function clearValidationPending(current: LoopEditorState, generation: number): LoopEditorState {
  return current.pendingValidationGeneration === generation
    ? { ...current, pendingValidationGeneration: null }
    : current;
}

export const loopEditorValidationTransitions = {
  validationRequested: (
    current: LoopEditorState,
    event: LoopEditorEvents["validationRequested"],
    enqueue: LoopEditorEnqueue
  ) => {
    if (!current.baseDefinition) return;
    const definition = graphToDefinition(current.baseDefinition, current.nodes, current.edges);
    const generation = current.validationGeneration + 1;
    const scopeGeneration = current.scopeGeneration;
    enqueue.effect(async ({ trigger }) => {
      try {
        trigger.validationSucceeded({
          generation,
          result: await event.execute(definition),
          scopeGeneration,
        });
      } catch {
        trigger.validationUnavailable({
          generation,
          notifyFailure: event.notifyFailure,
          scopeGeneration,
        });
      }
    });
    return {
      ...current,
      pendingValidationGeneration: generation,
      validationGeneration: generation,
    };
  },
  validationSucceeded: (
    current: LoopEditorState,
    event: LoopEditorEvents["validationSucceeded"]
  ) => {
    if (current.scopeGeneration !== event.scopeGeneration) return;
    const next = clearValidationPending(current, event.generation);
    if (event.generation !== current.validationGeneration) {
      return next === current ? undefined : next;
    }
    const lint = buildLintState(event.result);
    return {
      ...next,
      lint,
      nodes: applyLintToNodes(current.nodes, lint.byNode),
      validateFailed: false,
    };
  },
  validationUnavailable: (
    current: LoopEditorState,
    event: LoopEditorEvents["validationUnavailable"],
    enqueue: LoopEditorEnqueue
  ) => {
    if (current.scopeGeneration !== event.scopeGeneration) return;
    const next = clearValidationPending(current, event.generation);
    if (event.generation !== current.validationGeneration) {
      return next === current ? undefined : next;
    }
    if (event.notifyFailure) {
      enqueue.effect(() =>
        notifyUser({
          message: "Couldn't validate — CompozyOS isn't reachable. Try again.",
          tone: "error",
        })
      );
    }
    return {
      ...next,
      validateFailed: true,
      lint: current.lint.validated ? { ...current.lint, validated: false } : current.lint,
    };
  },
};

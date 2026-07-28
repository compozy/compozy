import { notifyUser } from "@/lib/user-feedback";

import type {
  LoopEditorEnqueue,
  LoopEditorEvents,
  LoopEditorState,
} from "./loop-editor-store-contracts";

export const loopEditorPositionTransitions = {
  positionsSaveFailed: (
    current: LoopEditorState,
    event: LoopEditorEvents["positionsSaveFailed"],
    enqueue: LoopEditorEnqueue
  ) => {
    if (
      current.scopeGeneration !== event.scopeGeneration ||
      current.pendingPositionsGeneration !== event.generation
    ) {
      return;
    }
    enqueue.effect(() =>
      notifyUser({ message: "Could not save node positions. Try again.", tone: "error" })
    );
    return { ...current, pendingPositionsGeneration: null };
  },
  positionsSaveRequested: (
    current: LoopEditorState,
    event: LoopEditorEvents["positionsSaveRequested"],
    enqueue: LoopEditorEnqueue
  ) => {
    if (!current.positionsDirty || current.pendingPositionsGeneration !== null) return;
    const generation = current.positionsGeneration;
    const scopeGeneration = current.scopeGeneration;
    const annotations = current.nodes.map(node => ({
      node_id: node.id,
      x: Math.round(node.position.x),
      y: Math.round(node.position.y),
    }));
    enqueue.effect(async ({ trigger }) => {
      try {
        await event.execute(annotations);
        trigger.positionsSaveSucceeded({ generation, scopeGeneration });
      } catch {
        trigger.positionsSaveFailed({ generation, scopeGeneration });
      }
    });
    return { ...current, pendingPositionsGeneration: generation };
  },
  positionsSaveSucceeded: (
    current: LoopEditorState,
    event: LoopEditorEvents["positionsSaveSucceeded"]
  ) => {
    if (
      current.scopeGeneration !== event.scopeGeneration ||
      current.pendingPositionsGeneration !== event.generation
    ) {
      return;
    }
    return {
      ...current,
      pendingPositionsGeneration: null,
      positionsDirty:
        current.positionsGeneration === event.generation ? false : current.positionsDirty,
    };
  },
};

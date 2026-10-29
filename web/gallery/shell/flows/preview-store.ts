/**
 * The canvas's preview queue (`preview-queue.ts`) as a small external store, provided by
 * `PreviewQueue` and read by `usePreview`.
 */
import { createContext } from "react";

import {
  schedulePreviews,
  settlePreview,
  type PreviewPlan,
  type PreviewView,
} from "#gallery/shell/flows/preview-queue";

const settleDelayMs = 200;

export function createPreviewQueue() {
  let plan: PreviewPlan = [];
  // Nothing starts before the viewport first settles on the laid-out flow.
  let view: PreviewView | null = null;
  let moving = true;
  let settling: number | undefined;
  const sources = new Map<string, string>();
  const listeners = new Set<() => void>();

  const commit = (next: PreviewPlan) => {
    plan = next;
    for (const listener of listeners) listener();
  };
  const replan = () => {
    if (view && !moving) commit(schedulePreviews(plan, view, sources));
  };

  return {
    subscribe(listener: () => void) {
      listeners.add(listener);
      return () => listeners.delete(listener);
    },
    mounted(id: string, src: string) {
      return plan.some(entry => entry.id === id && entry.src === src);
    },
    register(id: string, src: string) {
      sources.set(id, src);
      replan();
      return () => {
        if (sources.get(id) === src) sources.delete(id);
        replan();
      };
    },
    moveStart() {
      moving = true;
      window.clearTimeout(settling);
    },
    settle(next: PreviewView) {
      view = next;
      window.clearTimeout(settling);
      settling = window.setTimeout(() => {
        moving = false;
        replan();
      }, settleDelayMs);
    },
    settled(id: string, src: string) {
      commit(settlePreview(plan, id, src));
      replan();
    },
  };
}

type PreviewQueueStore = ReturnType<typeof createPreviewQueue>;

export const PreviewQueueContext = createContext<PreviewQueueStore | null>(null);

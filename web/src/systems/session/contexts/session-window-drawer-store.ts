import { createStoreLogic } from "@xstate/store";

import type { SubagentView } from "../components/subagents/types";

/**
 * What the side chat panel shows. `new` is an empty side chat that exists only once its first
 * question is sent; `opening` is that question on its way, with the composer submission id as
 * the idempotency key so Retry reaches the same side chat (US-001.EC-5). `pending` is the default
 * side chat the toggle asked for before the first list arrived: the list decides it.
 */
export type SessionSideView =
  | { kind: "pending" }
  | { kind: "new" }
  | { kind: "side"; id: string }
  | {
      kind: "opening";
      message: string;
      key: string;
      error: string | null;
      /** The question was typed in the panel: a refusal puts the panel back on its draft. */
      fromPanel?: boolean;
    };

export interface SessionSidePanelState {
  open: boolean;
  /** Kept while hidden, so the toggle or ⌘⌥S brings back the same side chat (US-007.AC-2). */
  view: SessionSideView | null;
  lastViewedId: string | null;
  /** The unsent "New side chat" question: kept across hide and switching until it is sent. */
  draft: string;
}

/**
 * The session window's one drawer (`_uiux.md` Component plan): a subagent transcript or the side
 * chat panel, never both. Exclusivity lives in these transitions only — opening one closes the
 * other — so neither drawer coordinates the other.
 */
export const sessionWindowDrawerLogic = createStoreLogic({
  context: {
    selected: null as { subagent: SubagentView; workspaceId: string } | null,
    side: { open: false, view: null, lastViewedId: null, draft: "" } as SessionSidePanelState,
    width: 480,
    returnFocus: null as HTMLElement | null,
  },
  on: {
    opened: (
      context,
      event: { subagent: SubagentView; workspaceId: string; trigger: HTMLElement | null }
    ) => ({
      ...context,
      selected: { subagent: event.subagent, workspaceId: event.workspaceId },
      side: { ...context.side, open: false },
      returnFocus: context.selected ? context.returnFocus : event.trigger,
    }),
    closed: context => ({ ...context, selected: null }),
    resized: (context, event: { width: number }) => ({
      ...context,
      width: Math.max(360, Math.min(800, event.width)),
    }),
    /** Shows the side panel on `view`, or on the view it kept while hidden. */
    sideShown: (context, event: { view?: SessionSideView }) => {
      const view = event.view ?? context.side.view ?? { kind: "new" };
      return {
        ...context,
        selected: null,
        side: {
          ...context.side,
          open: true,
          view,
          lastViewedId: view.kind === "side" ? view.id : context.side.lastViewedId,
        },
      };
    },
    sideHidden: context => ({ ...context, side: { ...context.side, open: false } }),
    /** A closed side chat leaves the panel on the next one, or hidden with nothing kept. */
    sideClosed: (context, event: { id: string; nextId: string | null }) => {
      if (context.side.view?.kind !== "side" || context.side.view.id !== event.id) return context;
      return {
        ...context,
        side:
          event.nextId === null
            ? { ...context.side, open: false, view: null, lastViewedId: null }
            : {
                ...context.side,
                view: { kind: "side", id: event.nextId },
                lastViewedId: event.nextId,
              },
      };
    },
    sideDraftChanged: (context, event: { draft: string }) => ({
      ...context,
      side: { ...context.side, draft: event.draft },
    }),
    sideRequested: (context, event: { message: string; key: string; fromPanel?: boolean }) => ({
      ...context,
      selected: null,
      side: {
        ...context.side,
        open: true,
        view: {
          kind: "opening",
          message: event.message,
          key: event.key,
          error: null,
          fromPanel: event.fromPanel,
        },
      },
    }),
    /** The side chat exists: a question sent from the panel's draft leaves that draft empty. */
    sideSettled: (context, event: { key: string; id: string }) => {
      const view = context.side.view;
      if (!openingView(view, event.key)) return context;
      return {
        ...context,
        side: {
          ...context.side,
          view: { kind: "side", id: event.id },
          lastViewedId: event.id,
          draft: view.fromPanel ? "" : context.side.draft,
        },
      };
    },
    sideFailed: (context, event: { key: string; error: string }) => {
      const view = context.side.view;
      if (!openingView(view, event.key)) return context;
      return { ...context, side: { ...context.side, view: { ...view, error: event.error } } };
    },
    /**
     * A refusal created nothing. A question typed in the panel goes back to its draft there,
     * leaving the panel as shown or hidden as it is now (a subagent or a hide may have come since);
     * one typed in the main composer stays in that composer, so the panel lets go of it.
     */
    sideRefused: (context, event: { key: string }) => {
      const view = context.side.view;
      if (!openingView(view, event.key)) return context;
      return {
        ...context,
        side: view.fromPanel
          ? { ...context.side, view: { kind: "new" } }
          : { ...context.side, open: false, view: null },
      };
    },
    /**
     * A successful side list (open side chats, newest first) settles the view: a pending default
     * picks the last viewed or newest one, else a new side chat; a side chat closed or deleted
     * elsewhere moves to the newest remaining one, or hides the panel when none is left
     * (US-008.EC-1). An explicit new draft and a question on its way are left alone.
     */
    sidesListed: (context, event: { ids: readonly string[] }) => {
      const { view, lastViewedId } = context.side;
      if (view?.kind === "pending") {
        const id = lastViewedId && event.ids.includes(lastViewedId) ? lastViewedId : event.ids[0];
        const next: SessionSideView = id ? { kind: "side", id } : { kind: "new" };
        return {
          ...context,
          side: { ...context.side, view: next, lastViewedId: id ?? lastViewedId },
        };
      }
      if (view?.kind !== "side" || event.ids.includes(view.id)) return context;
      const nextId = event.ids[0] ?? null;
      return {
        ...context,
        side:
          nextId === null
            ? { ...context.side, open: false, view: null, lastViewedId: null }
            : { ...context.side, view: { kind: "side", id: nextId }, lastViewedId: nextId },
      };
    },
  },
});

function openingView(
  view: SessionSideView | null,
  key: string
): view is Extract<SessionSideView, { kind: "opening" }> {
  return view?.kind === "opening" && view.key === key;
}

export type SessionWindowDrawerStore = ReturnType<typeof sessionWindowDrawerLogic.createStore>;
export type SessionWindowDrawerContext = ReturnType<
  SessionWindowDrawerStore["getSnapshot"]
>["context"];

/** Which drawer the window shows: at most one. */
export function sessionWindowDrawerKind(
  context: SessionWindowDrawerContext
): "subagent" | "side" | "none" {
  if (context.selected !== null) return "subagent";
  return context.side.open ? "side" : "none";
}

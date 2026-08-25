import { createContext, use } from "react";
import { createStoreLogic } from "@xstate/store";
import type { SessionWorkGroupAnchor } from "../session-timeline.logic";

export type TimelineExpansion = "work-group" | "turn" | "changed-files";

function toggled(ids: ReadonlySet<string>, id: string): Set<string> {
  const next = new Set(ids);
  if (next.has(id)) next.delete(id);
  else next.add(id);
  return next;
}

export const timelineRowLogic = createStoreLogic({
  context: (_input: undefined) => ({
    expandedWorkGroups: new Set<string>(),
    workGroupAnchors: new Map<string, SessionWorkGroupAnchor>(),
    expandedTurns: new Set<string>(),
    expandedChangedFiles: new Set<string>(),
  }),
  on: {
    workGroupExpansionToggled: (context, event: { id: string }) => ({
      ...context,
      expandedWorkGroups: toggled(context.expandedWorkGroups, event.id),
    }),
    workGroupAnchorObserved: (context, event: SessionWorkGroupAnchor) => {
      const previous = context.workGroupAnchors.get(event.groupId);
      const sameAnchor =
        previous !== undefined &&
        previous.turnId === event.turnId &&
        previous.anchorToolCallId === event.anchorToolCallId;
      if (sameAnchor) return context;
      const workGroupAnchors = new Map(context.workGroupAnchors);
      workGroupAnchors.set(event.groupId, event);
      return { ...context, workGroupAnchors };
    },
    turnExpansionToggled: (context, event: { id: string }) => ({
      ...context,
      expandedTurns: toggled(context.expandedTurns, event.id),
    }),
    changedFilesExpansionToggled: (context, event: { id: string }) => ({
      ...context,
      expandedChangedFiles: toggled(context.expandedChangedFiles, event.id),
    }),
  },
});

export type TimelineRowStore = ReturnType<typeof timelineRowLogic.createStore>;

// Context provides only the stable, per-message store handle. Each row selects
// its own primitive expansion state so other rows ignore unrelated transitions.
export const TimelineRowContext = createContext<TimelineRowStore | null>(null);

export function useTimelineRowContext(): TimelineRowStore {
  const context = use(TimelineRowContext);
  if (!context) {
    throw new Error("Timeline row views must render within a TimelineRowContext provider");
  }
  return context;
}

export function toggleTimelineExpansion(
  store: TimelineRowStore,
  expansion: TimelineExpansion,
  id: string
): void {
  switch (expansion) {
    case "work-group":
      store.trigger.workGroupExpansionToggled({ id });
      break;
    case "turn":
      store.trigger.turnExpansionToggled({ id });
      break;
    case "changed-files":
      store.trigger.changedFilesExpansionToggled({ id });
      break;
  }
}

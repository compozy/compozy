import { createContext, type ReactNode } from "react";
import { createStoreLogic } from "@xstate/store";

import type { SubagentView } from "../components/subagents/types";
import type { SubagentOpenOptions } from "../components/subagents/subagent-card";

export const subagentDrawerLogic = createStoreLogic({
  context: {
    selected: null as { subagent: SubagentView; workspaceId: string } | null,
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
      returnFocus: context.selected ? context.returnFocus : event.trigger,
    }),
    closed: context => ({ ...context, selected: null }),
    resized: (context, event: { width: number }) => ({
      ...context,
      width: Math.max(360, Math.min(800, event.width)),
    }),
  },
});

export type SubagentDrawerStore = ReturnType<typeof subagentDrawerLogic.createStore>;
export const SubagentDrawerContext = createContext<{
  store: SubagentDrawerStore;
  open: (subagent: SubagentView, options: SubagentOpenOptions, workspaceId: string) => void;
  /** The subagent's own transcript, for surfaces that reveal it in place. */
  content: (subagent: SubagentView, workspaceId: string) => ReactNode;
} | null>(null);

import { createContext, type ReactNode } from "react";

import type { SubagentView } from "../components/subagents/types";
import type { SubagentOpenOptions } from "../components/subagents/subagent-card";
import type { SessionWindowDrawerStore } from "./session-window-drawer-store";

export const SubagentDrawerContext = createContext<{
  /** The window's one drawer store; the subagent drawer reads its `selected`. */
  store: SessionWindowDrawerStore;
  open: (subagent: SubagentView, options: SubagentOpenOptions, workspaceId: string) => void;
  /** The subagent's own transcript, for surfaces that reveal it in place. */
  content: (subagent: SubagentView, workspaceId: string) => ReactNode;
} | null>(null);

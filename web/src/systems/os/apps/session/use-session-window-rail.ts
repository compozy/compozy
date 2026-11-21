import { createContext, use } from "react";

import type {
  SessionWindowRailTransport,
  SessionWindowSidebarModel,
} from "./use-session-window-sidebar";

export interface SessionWindowRailValue {
  /** The rail's view-model, shared with the session pane's actions that open sessions. */
  sidebar: SessionWindowSidebarModel;
  /** The pane reports its session's stream; the rail outlives the pane, so it cannot read it. */
  reportTransport: (transport: SessionWindowRailTransport | null) => void;
}

export const SessionWindowRailContext = createContext<SessionWindowRailValue | null>(null);

/** The window's sessions rail, which the per-session pane mounts inside. */
export function useSessionWindowRail(): SessionWindowRailValue {
  const rail = use(SessionWindowRailContext);
  if (!rail) throw new Error("useSessionWindowRail must be used inside SessionWindowRail");
  return rail;
}

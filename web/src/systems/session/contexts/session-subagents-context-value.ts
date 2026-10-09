import { createContext } from "react";

import type { SubagentOpenOptions } from "../components/subagents/subagent-card";
import type { SubagentView } from "../components/subagents/types";

/**
 * Opens another session from inside a transcript: the subagent card's drill-in
 * (S1) and the "Subagent of" divider's `Open parent` (S4). Provided by the
 * session window, which alone knows which window "this" one is.
 */
export interface SubagentNavigationTarget {
  sessionId: string;
  /** Empty when unknown; the window then opens through the attention jump. */
  agentName: string;
  workspaceId: string;
}

export interface SubagentNavigation {
  /** This window unless `newWindow` (⌘/Ctrl): then a split beside it. */
  openSession: (target: SubagentNavigationTarget, options: { newWindow: boolean }) => void;
  /** Opens the inspector, where the full roster lives (S10); the banner's `and N more`. */
  showSubagents?: () => void;
}

export const SubagentNavigationContext = createContext<SubagentNavigation | null>(null);

/**
 * Which session's roster the transcript, banner and status line read, and how
 * a card drills in. Stable across roster updates: consumers select only the
 * rows they show from the query cache, so a progress tick re-renders the card
 * it changed, never every message (m19).
 */
export interface SessionSubagentsContextValue {
  workspaceId: string;
  sessionId: string;
  /** Drill-in; absent where no window can open the child. */
  onOpen?: (subagent: SubagentView, options: SubagentOpenOptions) => void;
}

const NO_SUBAGENTS: SessionSubagentsContextValue = { workspaceId: "", sessionId: "" };

export const SessionSubagentsContext = createContext<SessionSubagentsContextValue>(NO_SUBAGENTS);

/**
 * A delegated subagent session's origin for the divider before its first
 * message (S4). Provided by the session window; absent for other sessions.
 */
export interface SubagentOriginContextValue {
  parent: { id: string; title: string } | null;
  onOpenParent?: (parentSessionId: string) => void;
}

export const SubagentOriginContext = createContext<SubagentOriginContextValue | null>(null);

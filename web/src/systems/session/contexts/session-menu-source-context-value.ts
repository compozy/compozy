import { createContext, type ReactNode } from "react";

import type { SessionMenuLinkedWork } from "../lib/session-menu-linked-work";
import type { SessionRowLayoutMenu } from "../lib/session-menu-tree";
import type { SessionPayload } from "../types";

/** What an open session menu reads beyond the row: its full linked work and folder layout. */
export interface SessionMenuSourceData {
  /** Null while the session has nothing linked. */
  linkedWork: Omit<SessionMenuLinkedWork, "onLink" | "onShowAll"> | null;
  /** The session's folder moves, read for hosts without a folder layout of their own. */
  layout?: SessionRowLayoutMenu;
}

export interface SessionMenuSourceProps {
  session: SessionPayload;
  /** Read the session's folder layout: the host has none of its own (a board card). */
  folders: boolean;
  children: (data: SessionMenuSourceData) => ReactNode;
}

/**
 * Reads an open session menu's data. A host that owns queries provides it (`SessionDeriveHost`);
 * the menu mounts it only while open, so a closed row reads nothing. Absent (null), the menu
 * keeps to what the row carries: the catalog summary's lead and no folder layout.
 */
export const SessionMenuSourceContext = createContext<
  ((props: SessionMenuSourceProps) => ReactNode) | null
>(null);

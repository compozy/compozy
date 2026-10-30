import { createContext } from "react";

/**
 * Where a session window's session view is shown: the window body (`window`), or the drawer
 * its Kanban board opens over itself (`board-drawer`, #1052). The drawer shows the same
 * session view; the board already lists the sessions, so the drawer drops the sessions rail,
 * its toggle and the List | Kanban switch, and leaves catalog freshness to the board.
 */
export type SessionWindowPresentation = "window" | "board-drawer";

export const SessionWindowPresentationContext = createContext<SessionWindowPresentation>("window");

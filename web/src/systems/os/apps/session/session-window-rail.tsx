import { useEffect, useLayoutEffect, useRef, type ReactNode } from "react";

import {
  type SessionPayload,
  SessionSidebar,
  SessionSidebarLayout,
  useSessionWindowPresentation,
} from "@/systems/session";

import { SessionWindowRowDialogs } from "./session-window-row-dialogs";
import { SessionWindowSkeleton } from "./session-window-skeleton";
import {
  type SessionWindowSidebarModel,
  useSessionWindowSidebar,
} from "./use-session-window-sidebar";

/** The list's scroll container inside the rail (`SessionList` marks it). */
const LIST_SCROLL_SELECTOR = "[data-session-list-scroll]";

/**
 * Where each window's rail list was scrolled. The window's session pane is keyed by the session
 * it shows, so a row pick remounts the rail with it; the list resumes from here instead of the
 * top (#1923). Memory only: a reload starts at the top, as before.
 */
const railScrollTops = new Map<string, number>();

/** The sessions rail beside the session pane; over the board the board is the list, so no rail. */
export function SessionWindowRailLayout({
  sidebar,
  children,
}: {
  sidebar: ReactNode;
  children: ReactNode;
}) {
  if (useSessionWindowPresentation() === "board-drawer") {
    return <div className="flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden">{children}</div>;
  }
  return <SessionSidebarLayout sidebar={sidebar}>{children}</SessionSidebarLayout>;
}

/**
 * The window's sessions rail. Its list keeps its scroll position across the session switches
 * that remount it, so the row just picked stays where the operator clicked it, marked current.
 */
export function SessionWindowSidebarRail({
  windowId,
  sidebar,
  currentSessionId,
  revealedSession,
}: {
  windowId: string;
  sidebar: SessionWindowSidebarModel;
  currentSessionId: string;
  /** The session on screen; a subagent session is revealed under its parent row. */
  revealedSession?: SessionPayload | null;
}) {
  const hostRef = useRef<HTMLDivElement>(null);

  // Before paint, so the list never shows at the top first. Once per window, when the list
  // itself is in place: a rail that mounts while its catalog still loads shows placeholder rows
  // first, so the restore waits for the list to replace them. After that the operator's own
  // scrolling, and any later reload of the list, is never moved again.
  const restoredForRef = useRef<string | null>(null);
  const listLoading = Boolean(sidebar.view.catalog?.loading);
  useLayoutEffect(() => {
    if (restoredForRef.current === windowId) return;
    const list = hostRef.current?.querySelector<HTMLElement>(LIST_SCROLL_SELECTOR);
    if (!list) return;
    restoredForRef.current = windowId;
    const top = railScrollTops.get(windowId);
    if (top) list.scrollTop = top;
  }, [windowId, listLoading]);

  // Scroll does not bubble; the capture phase sees the list's scroll from this host.
  useEffect(() => {
    const host = hostRef.current;
    if (!host) return;
    const remember = (event: Event) => {
      const target = event.target;
      if (target instanceof HTMLElement && target.matches(LIST_SCROLL_SELECTOR)) {
        railScrollTops.set(windowId, target.scrollTop);
      }
    };
    host.addEventListener("scroll", remember, { capture: true, passive: true });
    return () => host.removeEventListener("scroll", remember, { capture: true });
  }, [windowId]);

  return (
    <div className="contents" ref={hostRef}>
      <SessionSidebar
        open={sidebar.open}
        sessions={sidebar.sessions}
        disconnected={sidebar.disconnected}
        collapsedThreadIds={sidebar.collapsedThreadIds}
        subagentThreadIds={sidebar.subagentThreadIds}
        view={sidebar.view}
        currentSessionId={currentSessionId}
        revealedSession={revealedSession}
        onThreadBranchChange={sidebar.onThreadBranchChange}
        onSelectSession={sidebar.onSelectSession}
        onNewSession={sidebar.onNewSession}
        sessionActions={sidebar.sessionActions}
      />
    </div>
  );
}

/**
 * The session window while the session it now names loads: the live rail, already marking that
 * session current at its kept scroll, beside the pane's placeholder. A row pick never trades the
 * rail for placeholder rows (#1923).
 */
export function SessionWindowLoadingFrame({
  windowId,
  sessionId,
}: {
  windowId: string;
  sessionId: string;
}) {
  const sidebar = useSessionWindowSidebar({ windowId, sessionId });
  return (
    <div className="flex min-h-0 min-w-0 flex-1 overflow-hidden">
      <SessionWindowRailLayout
        sidebar={
          <SessionWindowSidebarRail
            windowId={windowId}
            sidebar={sidebar}
            currentSessionId={sessionId}
          />
        }
      >
        <SessionWindowSkeleton />
      </SessionWindowRailLayout>
      <SessionWindowRowDialogs sidebar={sidebar} />
    </div>
  );
}

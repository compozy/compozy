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

/** The operator's own hand on the rail: a wheel, touch, pointer or key in it. */
const OPERATOR_GESTURES = ["wheel", "touchstart", "pointerdown", "keydown"] as const;

/** The rail's pending restore: the container it last resumed, until the operator takes over. */
interface RailRestore {
  windowId: string;
  list: HTMLElement | null;
  settled: boolean;
}

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

  // Before paint, so the list never shows at the top first. The list's container can appear after
  // the rail mounts (placeholder rows while the catalog loads) or be replaced (the folder layout
  // arriving after the catalog), so each new container resumes the saved scroll, until the
  // operator handles the list. From then on nothing moves it: their own scrolling stays, and a
  // list they reload (breadth, search) starts at the top.
  const restoreRef = useRef<RailRestore | null>(null);
  useLayoutEffect(() => {
    let restore = restoreRef.current;
    if (restore?.windowId !== windowId) {
      restore = { windowId, list: null, settled: false };
      restoreRef.current = restore;
    }
    if (restore.settled) return;
    const list = hostRef.current?.querySelector<HTMLElement>(LIST_SCROLL_SELECTOR) ?? null;
    if (list === null || list === restore.list) return;
    restore.list = list;
    const top = railScrollTops.get(windowId);
    if (top) list.scrollTop = top;
  });

  // Scroll does not bubble; the capture phase sees the list's scroll from this host. The
  // operator's first gesture in the rail ends the restore above.
  useEffect(() => {
    const host = hostRef.current;
    if (!host) return;
    const remember = (event: Event) => {
      const target = event.target;
      if (target instanceof HTMLElement && target.matches(LIST_SCROLL_SELECTOR)) {
        railScrollTops.set(windowId, target.scrollTop);
      }
    };
    const settle = () => {
      if (restoreRef.current?.windowId === windowId) restoreRef.current.settled = true;
    };
    host.addEventListener("scroll", remember, { capture: true, passive: true });
    for (const type of OPERATOR_GESTURES) {
      host.addEventListener(type, settle, { capture: true, passive: true });
    }
    return () => {
      host.removeEventListener("scroll", remember, { capture: true });
      for (const type of OPERATOR_GESTURES) {
        host.removeEventListener(type, settle, { capture: true });
      }
    };
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

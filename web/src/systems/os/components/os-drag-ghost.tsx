import { createPortal } from "react-dom";

import { cn } from "@/lib/utils";
import type { SessionPayload } from "@/systems/session";

import { useDesktop } from "../hooks/use-desktop";
import { tabSessionIds, useTabSessions } from "../hooks/use-os-tab-bar";
import { useOsTabDrag } from "../hooks/use-os-tab-drag";
import { getOsAppDescriptor } from "../lib/app-catalog";
import type { OsWindow } from "../lib/os-types";
import { leadWindowId, tabWindowIds } from "../lib/tab-model";
import { useOsWindowTitle } from "../hooks/use-os-window-title";
import { OsTabTiles } from "./os-tab";

type SessionOf = (win: OsWindow) => SessionPayload | undefined;

function GhostTitle({
  win,
  sessionOf,
}: {
  win: OsWindow;
  sessionOf: SessionOf;
}) {
  return <span className="truncate">{useOsWindowTitle(win, sessionOf(win))}</span>;
}

/**
 * The plate itself. It resolves its own windows' sessions: a tab dragged from another desktop in
 * the overview is not on the bar, so the bar's sessions would miss it (#1709).
 */
function OsDragGhostPlate({
  members,
  tabSource,
  dimmed,
  pointer,
}: {
  members: readonly [OsWindow, ...OsWindow[]];
  tabSource: boolean;
  dimmed: boolean;
  pointer: { x: number; y: number };
}) {
  const sessionById = useTabSessions(tabSessionIds(members));
  const sessionOf: SessionOf = win =>
    win.app === "session" ? sessionById.get(win.instanceKey ?? "") : undefined;
  const lead = members[0];
  return createPortal(
    <div
      aria-hidden="true"
      className={cn(
        "pointer-events-none fixed z-50 flex h-6.75 max-w-56 items-center gap-2 rounded-sm bg-elevated pr-3 pl-[3.5px] text-body font-medium text-fg shadow-pop",
        dimmed && "opacity-60"
      )}
      data-slot="os-drag-ghost"
      style={{ left: pointer.x + 10, top: pointer.y + 8 }}
    >
      <OsTabTiles members={members} sessionOf={sessionOf} />
      {tabSource ? (
        <GhostTitle win={lead} sessionOf={sessionOf} />
      ) : (
        <span className="truncate">{getOsAppDescriptor(lead.app).title}</span>
      )}
      {members.length > 1 ? (
        <span className="text-meta text-subtle tabular-nums">{members.length}</span>
      ) : null}
    </div>,
    document.body
  );
}

/**
 * What follows the pointer while a tab or window head is dragged: the tab plate, or the head's
 * app chip. It dims where a drop would do nothing.
 */
export function OsDragGhost() {
  const drag = useOsTabDrag();
  const windows = useDesktop(state => state.windows);
  const desktops = useDesktop(state => state.desktops);
  const focusOrder = useDesktop(state => state.client?.focusOrder);
  const session = drag?.session;
  if (drag === null || session == null || drag.pointer === null) return null;
  const { source } = session;
  let members: OsWindow[];
  if (source.kind === "tab") {
    const tab = desktops.flatMap(desktop => desktop.tabs).find(entry => entry.id === source.tabId);
    if (tab === undefined) return null;
    const lead = leadWindowId(tab, focusOrder ?? []);
    const ordered = [lead, ...tabWindowIds(tab).filter(id => id !== lead)];
    members = ordered.flatMap(id => (id !== null && windows[id] ? [windows[id]] : []));
  } else {
    members = windows[source.windowId] ? [windows[source.windowId]!] : [];
  }
  const [lead, ...rest] = members;
  if (lead === undefined) return null;
  return (
    <OsDragGhostPlate
      members={[lead, ...rest]}
      tabSource={source.kind === "tab"}
      dimmed={drag.target === null}
      pointer={drag.pointer}
    />
  );
}

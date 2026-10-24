import type { ReactNode } from "react";

import { cn } from "@/lib/utils";

import { useSessionSidebarState } from "../hooks/use-session-sidebar-state";
import { SESSION_SIDEBAR_PANE_FRAME, SESSION_SIDEBAR_SPLIT_FRAME } from "./session-sidebar-frame";

/**
 * The sessions rail and pane split as a window holds it before its code has loaded. It reads the
 * same rail state as the live layout (open, and the width this window saved, in pixels) and sets
 * the rail to exactly that width, so the split is where the live one will be from the first
 * paint, whatever the window measured when it mounted: the live layout resolves a pixel width
 * against the size its group has when it mounts, and a window that has not been laid out yet
 * would split half and half. It has no handle to drag and no panel library behind it, and the
 * pane keeps half the window as the live one does.
 */
export function SessionSidebarStaticLayout({
  sidebar,
  children,
}: {
  /** The rail: fills its width. */
  sidebar: ReactNode;
  /** The session pane right of the rail. */
  children: ReactNode;
}) {
  const { open, width } = useSessionSidebarState();

  return (
    <div
      className={cn("flex h-full w-full", SESSION_SIDEBAR_SPLIT_FRAME)}
      data-slot="session-sidebar-static-layout"
    >
      <div className="max-w-1/2 shrink-0 overflow-hidden" style={{ width: open ? width : 0 }}>
        {sidebar}
      </div>
      <div aria-hidden="true" className={cn("w-px shrink-0 bg-border", !open && "hidden")} />
      <div className={cn(SESSION_SIDEBAR_PANE_FRAME, "flex-1 overflow-hidden")}>{children}</div>
    </div>
  );
}

import type { ReactNode } from "react";

import { ResizableHandle, ResizablePanel, ResizablePanelGroup } from "@compozy/ui";

import { cn } from "@/lib/utils";

import {
  SESSION_SIDEBAR_PANEL_ID,
  useSessionSidebarLayout,
} from "../hooks/use-session-sidebar-layout";
import { SESSION_SIDEBAR_MAX_WIDTH } from "../lib/session-sidebar-width";
import { SESSION_SIDEBAR_PANE_FRAME, SESSION_SIDEBAR_SPLIT_FRAME } from "./session-sidebar-frame";

/**
 * Session window body: the sessions rail and the session pane split by a
 * resize handle. The rail's open state and width belong to this session
 * window; the width holds in pixels when the window resizes, within its bounds
 * and never past half the window.
 */
export function SessionSidebarLayout({
  sidebar,
  children,
}: {
  /** The rail: fills the resizable panel. */
  sidebar: ReactNode;
  /** The session pane right of the rail. */
  children: ReactNode;
}) {
  const layout = useSessionSidebarLayout();

  return (
    <ResizablePanelGroup
      elementRef={layout.groupRef}
      orientation="horizontal"
      disabled={!layout.open}
      className={cn(
        SESSION_SIDEBAR_SPLIT_FRAME,
        "data-toggling:*:data-panel:transition-[flex-grow] data-toggling:*:data-panel:duration-shell-slow motion-reduce:*:data-panel:transition-none"
      )}
      onLayoutChanged={layout.onLayoutChanged}
      onDoubleClick={layout.onDoubleClick}
    >
      <ResizablePanel
        id={SESSION_SIDEBAR_PANEL_ID}
        panelRef={layout.panelRef}
        collapsible
        collapsedSize={0}
        defaultSize={layout.mountSize}
        minSize={layout.railMinSize}
        maxSize={SESSION_SIDEBAR_MAX_WIDTH}
        groupResizeBehavior="preserve-pixel-size"
        style={{ overflow: "hidden" }}
      >
        {sidebar}
      </ResizablePanel>
      {/* The library's own reset targets the mount size, so the group resets to the default. */}
      <ResizableHandle
        aria-label="Resize sessions sidebar"
        className={cn(!layout.open && "hidden")}
        disableDoubleClick
        elementRef={layout.handleRef}
      />
      <ResizablePanel
        minSize="50%"
        className={SESSION_SIDEBAR_PANE_FRAME}
        style={{ overflow: "hidden" }}
      >
        {children}
      </ResizablePanel>
    </ResizablePanelGroup>
  );
}

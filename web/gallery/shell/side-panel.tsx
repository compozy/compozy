/**
 * The main area beside an optional right panel. The two sit in a `ResizablePanelGroup`: drag (or
 * arrow keys on) the handle to resize the panel, and the width is remembered per panel kind. The
 * main panel keeps its place whether or not the side panel shows, so the canvas iframe is never
 * remounted by opening it.
 */
import type { ReactNode } from "react";

import { ResizableHandle, ResizablePanel, ResizablePanelGroup } from "@compozy/ui";

function savedWidth(key: string) {
  const width = Number(localStorage.getItem(key));
  return Number.isFinite(width) && width > 0 ? `${width}px` : "340px";
}

export function WithSidePanel({
  id,
  label,
  side,
  children,
}: {
  /** Layout key: one remembered width per kind of panel. */
  id: string;
  /** Accessible name of the resize handle. */
  label: string;
  side: ReactNode;
  children: ReactNode;
}) {
  const key = `gallery-panel-width:${id}`;
  return (
    <ResizablePanelGroup className="min-h-0 min-w-0 flex-1" orientation="horizontal">
      <ResizablePanel className="flex h-full min-w-0" id="main" minSize="40%">
        {children}
      </ResizablePanel>
      {side ? (
        <>
          <ResizableHandle aria-label={label} className="mx-1" withHandle />
          <ResizablePanel
            className="flex h-full min-w-0"
            defaultSize={savedWidth(key)}
            groupResizeBehavior="preserve-pixel-size"
            id="side"
            maxSize="50%"
            minSize="280px"
            onResize={size => localStorage.setItem(key, String(size.inPixels))}
          >
            {side}
          </ResizablePanel>
        </>
      ) : null}
    </ResizablePanelGroup>
  );
}

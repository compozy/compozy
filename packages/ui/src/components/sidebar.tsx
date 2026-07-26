"use client";

import { PanelLeftIcon } from "lucide-react";
import * as React from "react";

import { cn } from "../lib/utils";
import {
  SIDEBAR_COLLAPSE_BREAKPOINT_DEFAULT,
  SIDEBAR_PANEL_WIDTH_DEFAULT,
  SIDEBAR_PANEL_WIDTH_MD,
  SIDEBAR_PANEL_WIDTH_MD_BREAKPOINT,
  SIDEBAR_RAIL_WIDTH,
  useSidebarState,
  useSidebarViewport,
} from "./hooks/use-sidebar-state";
import type { SidebarViewport } from "./hooks/use-sidebar-state";

export interface SidebarProps extends Omit<React.ComponentProps<"aside">, "onChange"> {
  rail?: React.ReactNode;
  header?: React.ReactNode;
  nav: React.ReactNode;
  footer?: React.ReactNode;
  collapsed?: boolean;
  defaultCollapsed?: boolean;
  onCollapse?: (next: boolean) => void;
  /** Explicit panel width override. When omitted the panel resolves from the viewport ladder. */
  panelWidth?: number;
  /** Width below which the panel becomes the drawer (replaces the legacy 768 px breakpoint). */
  collapseBreakpoint?: number;
  /** Width below which the panel shrinks from 244 px (default) to 220 px (md). */
  mdBreakpoint?: number;
  collapseLabel?: string;
}

function Sidebar({
  rail,
  header,
  nav,
  footer,
  collapsed: collapsedProp,
  defaultCollapsed = false,
  onCollapse,
  panelWidth: panelWidthOverride,
  collapseBreakpoint = SIDEBAR_COLLAPSE_BREAKPOINT_DEFAULT,
  mdBreakpoint = SIDEBAR_PANEL_WIDTH_MD_BREAKPOINT,
  collapseLabel = "Toggle sidebar",
  className,
  ...props
}: SidebarProps) {
  const {
    closeDrawer,
    effectivelyCollapsed,
    handleToggle,
    isDrawer,
    panelRef,
    panelVisible,
    resolvedPanelWidth,
    viewport,
  } = useSidebarState({
    collapsed: collapsedProp,
    defaultCollapsed,
    onCollapse,
    panelWidth: panelWidthOverride,
    collapseBreakpoint,
    mdBreakpoint,
  });

  return (
    <aside
      data-slot="sidebar"
      data-state={effectivelyCollapsed ? "collapsed" : "expanded"}
      data-narrow={isDrawer ? "true" : "false"}
      data-viewport={viewport}
      className={cn("relative flex h-full shrink-0 border-r border-line bg-rail", className)}
      {...props}
    >
      {isDrawer && panelVisible ? (
        <button
          type="button"
          aria-label="Close sidebar navigation"
          onClick={closeDrawer}
          className="absolute inset-y-0 z-40 bg-overlay-scrim"
          style={{ left: SIDEBAR_RAIL_WIDTH, width: "100vw" }}
        />
      ) : null}
      <div
        data-slot="sidebar-rail"
        className="relative z-50 flex shrink-0 flex-col items-center gap-1.5 border-r border-line bg-rail py-3"
        style={{ width: SIDEBAR_RAIL_WIDTH }}
      >
        {rail ? (
          <div className="flex flex-1 flex-col items-center gap-1.5">{rail}</div>
        ) : (
          <div className="flex-1" />
        )}
        <button
          type="button"
          data-slot="sidebar-collapse-trigger"
          aria-label={
            isDrawer
              ? panelVisible
                ? "Close sidebar navigation"
                : "Open sidebar navigation"
              : collapseLabel
          }
          aria-expanded={panelVisible}
          onClick={handleToggle}
          className="inline-flex size-7 items-center justify-center rounded-md text-muted transition-colors hover:bg-hover hover:text-fg focus-visible:shadow-focus-ring focus-visible:outline-none"
        >
          <PanelLeftIcon aria-hidden="true" className="size-3" />
        </button>
      </div>
      <div
        ref={panelRef}
        data-slot="sidebar-panel"
        style={{
          width: panelVisible ? resolvedPanelWidth : 0,
          ...(isDrawer ? { left: SIDEBAR_RAIL_WIDTH } : {}),
        }}
        className={cn(
          "flex min-h-0 flex-col overflow-hidden bg-sidebar",
          panelVisible ? "visible pointer-events-auto" : "pointer-events-none invisible",
          isDrawer && "absolute inset-y-0 z-50 border-r border-line"
        )}
        aria-hidden={effectivelyCollapsed}
      >
        <div
          className="flex h-full min-h-0 flex-col"
          style={{ width: resolvedPanelWidth, flexShrink: 0 }}
        >
          {header ? (
            <div
              data-slot="sidebar-header"
              className="flex min-h-12 shrink-0 items-center gap-2 border-b border-line px-2"
            >
              {header}
            </div>
          ) : null}
          <div data-slot="sidebar-nav" className="min-h-0 flex-1 overflow-y-auto">
            {nav}
          </div>
          {footer ? (
            <div data-slot="sidebar-footer" className="shrink-0 border-t border-line px-2 py-2.5">
              {footer}
            </div>
          ) : null}
        </div>
      </div>
    </aside>
  );
}

function SidebarSectionLabel({ className, ...props }: React.ComponentProps<"div">) {
  return (
    <div
      {...props}
      data-slot="sidebar-section-label"
      className={cn("eyebrow flex items-center px-2 pt-3 pb-1.5 text-muted", className)}
    />
  );
}

export {
  Sidebar,
  SIDEBAR_COLLAPSE_BREAKPOINT_DEFAULT,
  SIDEBAR_PANEL_WIDTH_DEFAULT,
  SIDEBAR_PANEL_WIDTH_MD,
  SIDEBAR_PANEL_WIDTH_MD_BREAKPOINT,
  SIDEBAR_RAIL_WIDTH,
  SidebarSectionLabel,
  useSidebarViewport,
};
export type { SidebarViewport };

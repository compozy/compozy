import * as React from "react";

import { useInitialState } from "../use-initial-state";

export const SIDEBAR_RAIL_WIDTH = 56;
export const SIDEBAR_PANEL_WIDTH_DEFAULT = 244;
export const SIDEBAR_PANEL_WIDTH_MD = 220;
export const SIDEBAR_PANEL_WIDTH_MD_BREAKPOINT = 1100;
export const SIDEBAR_COLLAPSE_BREAKPOINT_DEFAULT = 880;

export type SidebarViewport = "default" | "md" | "drawer";

export interface ViewportThresholds {
  drawer: number;
  md: number;
}

interface UseSidebarStateOptions {
  collapsed?: boolean;
  defaultCollapsed: boolean;
  onCollapse?: (next: boolean) => void;
  panelWidth?: number;
  collapseBreakpoint: number;
  mdBreakpoint: number;
}

/**
 * Returns the active viewport tier — `"default"`
 * above 1100 px, `"md"` between the drawer threshold and 1100 px, `"drawer"`
 * at or below the drawer threshold. The previous boolean
 * `useNarrowViewport(breakpoint)` collapsed the ladder into one query and
 * lost the 220 px tier.
 */
export function useSidebarViewport(
  thresholds: ViewportThresholds,
  onViewportChange?: (viewport: SidebarViewport) => void
): SidebarViewport {
  const { drawer, md } = thresholds;
  const [viewport, setViewport] = React.useState<SidebarViewport>("default");
  const notifyViewportChange = React.useEffectEvent((nextViewport: SidebarViewport) => {
    onViewportChange?.(nextViewport);
  });
  React.useEffect(() => {
    if (typeof window === "undefined" || typeof window.matchMedia !== "function") return;
    const drawerQuery = window.matchMedia(`(max-width: ${Math.max(0, drawer - 1)}px)`);
    const mdQuery = window.matchMedia(`(max-width: ${Math.max(0, md - 1)}px)`);
    function evaluate() {
      const nextViewport: SidebarViewport = drawerQuery.matches
        ? "drawer"
        : mdQuery.matches
          ? "md"
          : "default";
      notifyViewportChange(nextViewport);
      setViewport(nextViewport);
    }
    evaluate();
    drawerQuery.addEventListener("change", evaluate);
    mdQuery.addEventListener("change", evaluate);
    return () => {
      drawerQuery.removeEventListener("change", evaluate);
      mdQuery.removeEventListener("change", evaluate);
    };
  }, [drawer, md]);
  return viewport;
}

export function useSidebarState({
  collapsed: collapsedProp,
  defaultCollapsed,
  onCollapse,
  panelWidth: panelWidthOverride,
  collapseBreakpoint,
  mdBreakpoint,
}: UseSidebarStateOptions) {
  const isControlled = collapsedProp !== undefined;
  const [uncontrolled, setUncontrolled] = useInitialState(defaultCollapsed);
  const [mobileOpen, setMobileOpen] = React.useState(false);
  const panelRef = React.useRef<HTMLDivElement | null>(null);
  const userCollapsed = isControlled ? Boolean(collapsedProp) : uncontrolled;
  const viewport = useSidebarViewport(
    { drawer: collapseBreakpoint, md: mdBreakpoint },
    nextViewport => {
      if (nextViewport !== "drawer") setMobileOpen(false);
    }
  );
  const isDrawer = viewport === "drawer";
  const panelVisible = isDrawer ? mobileOpen : !userCollapsed;
  const effectivelyCollapsed = !panelVisible;
  const resolvedPanelWidth = resolvePanelWidth(viewport, panelWidthOverride);

  const setCollapsed = (next: boolean) => {
    if (!isControlled) setUncontrolled(next);
    onCollapse?.(next);
  };

  const handleToggle = () => {
    if (isDrawer) {
      setMobileOpen(current => !current);
      return;
    }
    setCollapsed(!userCollapsed);
  };

  const closeDrawer = () => {
    setMobileOpen(false);
  };

  React.useEffect(() => {
    if (!isDrawer || !mobileOpen || typeof window === "undefined") return;

    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        setMobileOpen(false);
      }
    };

    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [isDrawer, mobileOpen]);

  React.useEffect(() => {
    const panel = panelRef.current;
    if (!panel) return;

    panel.toggleAttribute("inert", effectivelyCollapsed);
    if ("inert" in panel) {
      panel.inert = effectivelyCollapsed;
    }

    return () => {
      panel.removeAttribute("inert");
      if ("inert" in panel) {
        panel.inert = false;
      }
    };
  }, [effectivelyCollapsed]);

  return {
    closeDrawer,
    effectivelyCollapsed,
    handleToggle,
    isDrawer,
    panelRef,
    panelVisible,
    resolvedPanelWidth,
    viewport,
  };
}

function resolvePanelWidth(viewport: SidebarViewport, override?: number): number {
  if (override !== undefined) return override;
  if (viewport === "md") return SIDEBAR_PANEL_WIDTH_MD;
  return SIDEBAR_PANEL_WIDTH_DEFAULT;
}

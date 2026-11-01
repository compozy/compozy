import * as React from "react";

export const SIDEBAR_PANEL_WIDTH_MD_BREAKPOINT = 1100;
export const SIDEBAR_COLLAPSE_BREAKPOINT_DEFAULT = 880;

export type SidebarViewport = "default" | "md" | "drawer";

export interface ViewportThresholds {
  drawer: number;
  md: number;
}

/**
 * Returns the active viewport tier — `"default"`
 * above 1100 px, `"md"` between the drawer threshold and 1100 px, `"drawer"`
 * at or below the drawer threshold.
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

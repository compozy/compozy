import type { LayoutDesktop } from "../lib/window-manager-types";
import { DesktopPager, type DesktopPagerOverflowRequest } from "./desktop-pager";

export interface DesktopPagerSurfaceProps {
  activeDesktopId: string | null;
  desktops: readonly LayoutDesktop[];
  compact: boolean;
  canSwitchDesktop: boolean;
  onSelectDesktop: (desktopId: string) => void;
  onOpenOverview: (request: DesktopPagerOverflowRequest) => void;
}

/** Presentational adapter for the Dock-owned daemon desktop pager. */
export function DesktopPagerSurface({
  activeDesktopId,
  desktops,
  compact,
  canSwitchDesktop,
  onSelectDesktop,
  onOpenOverview,
}: DesktopPagerSurfaceProps) {
  if (!activeDesktopId) return null;

  return (
    <DesktopPager
      desktops={desktops.map(desktop => ({ id: desktop.id, name: desktop.name }))}
      activeDesktopId={activeDesktopId}
      compact={compact}
      canSwitchDesktop={canSwitchDesktop}
      onSelectDesktop={onSelectDesktop}
      onOpenOverview={onOpenOverview}
    />
  );
}

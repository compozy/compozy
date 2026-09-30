import type { LayoutDesktop } from "../lib/window-manager-types";
import { DesktopPager, type DesktopPagerOverflowRequest } from "./desktop-pager";

export interface DesktopPagerSurfaceProps {
  activeDesktopId: string | null;
  desktops: readonly LayoutDesktop[];
  /** Desktops holding a window that needs you (marked while off-screen). */
  needsYouDesktopIds: ReadonlySet<string>;
  canSwitchDesktop: boolean;
  onSelectDesktop: (desktopId: string) => void;
  onOpenOverview: (request: DesktopPagerOverflowRequest) => void;
}

/** Presentational adapter for the topbar-owned daemon desktop pager. */
export function DesktopPagerSurface({
  activeDesktopId,
  desktops,
  needsYouDesktopIds,
  canSwitchDesktop,
  onSelectDesktop,
  onOpenOverview,
}: DesktopPagerSurfaceProps) {
  if (!activeDesktopId) return null;

  return (
    <DesktopPager
      desktops={desktops.map(desktop => ({
        id: desktop.id,
        name: desktop.name,
        needsYou: needsYouDesktopIds.has(desktop.id),
      }))}
      activeDesktopId={activeDesktopId}
      canSwitchDesktop={canSwitchDesktop}
      onSelectDesktop={onSelectDesktop}
      onOpenOverview={onOpenOverview}
    />
  );
}

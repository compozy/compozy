import { useState } from "react";

import { useDesktopOverflowDragHover } from "../hooks/use-desktop-overflow-drag-hover";
import { useOsShell } from "../hooks/use-os-shell";
import { useOsTabDragDesktopTarget, useOsTabDragSourceDesktop } from "../hooks/use-os-tab-drag";
import {
  desktopBarControls,
  desktopOverflowRequest,
  type DesktopIndicatorItem,
  type DesktopOverflowRequest,
} from "../lib/desktop-indicator-controls";
import type { LayoutDesktop } from "../lib/window-manager-types";
import { windowManagerStore } from "../stores/window-manager-store";
import { DesktopIndicators } from "./desktop-indicators";

export interface DesktopIndicatorsSurfaceProps {
  activeDesktopId: string | null;
  desktops: readonly LayoutDesktop[];
  /** Desktops holding a window that needs you (marked while off-screen). */
  needsYouDesktopIds: ReadonlySet<string>;
  canSwitchDesktop: boolean;
  onSelectDesktop: (desktopId: string) => void;
  onOpenOverview: (request: DesktopOverflowRequest) => void;
}

/**
 * The top bar's desktop indicators, wired to the daemon commands and the tab drag. Rename and
 * Close desktop finish on the overview card, the one place that owns the rename field and the
 * destination-and-confirm flow (US-015.EC-2).
 */
export function DesktopIndicatorsSurface({
  activeDesktopId,
  desktops,
  needsYouDesktopIds,
  canSwitchDesktop,
  onSelectDesktop,
  onOpenOverview,
}: DesktopIndicatorsSurfaceProps) {
  const { manager } = useOsShell();
  const [nav, setNav] = useState<HTMLElement | null>(null);
  const dragSourceDesktopId = useOsTabDragSourceDesktop();
  const dropTargetDesktopId = useOsTabDragDesktopTarget();
  const items: DesktopIndicatorItem[] = desktops.map(desktop => ({
    id: desktop.id,
    name: desktop.name,
    needsYou: needsYouDesktopIds.has(desktop.id),
  }));
  // A desktop hidden behind the overflow takes a dragged tab on its overview card (US-016.EC-2).
  const overflowHover = useDesktopOverflowDragHover(nav, direction => {
    if (activeDesktopId === null) return;
    const request = desktopOverflowRequest(
      desktopBarControls(items, activeDesktopId),
      direction,
      activeDesktopId
    );
    if (request !== null) onOpenOverview(request);
  });

  if (!activeDesktopId) return null;

  return (
    <DesktopIndicators
      ref={setNav}
      activeDesktopId={activeDesktopId}
      canSwitchDesktop={canSwitchDesktop}
      desktops={items}
      dragSourceDesktopId={dragSourceDesktopId}
      dropTargetDesktopId={dropTargetDesktopId}
      overflowHover={overflowHover}
      onCloseDesktop={desktopId =>
        windowManagerStore.trigger.overviewDesktopActionRequested({
          request: { desktopId, action: "close" },
        })
      }
      onCreateDesktop={() => manager.createDesktop()}
      onOpenOverview={onOpenOverview}
      onRenameDesktop={desktopId =>
        windowManagerStore.trigger.overviewDesktopActionRequested({
          request: { desktopId, action: "rename" },
        })
      }
      onReorderDesktop={(desktopId, order) => manager.reorderDesktop(desktopId, order)}
      onSelectDesktop={onSelectDesktop}
    />
  );
}

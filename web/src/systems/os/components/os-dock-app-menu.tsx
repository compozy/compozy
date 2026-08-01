import {
  ContextMenu,
  ContextMenuContent,
  ContextMenuItem,
  ContextMenuSeparator,
  ContextMenuTrigger,
} from "@compozy/ui";
import type * as React from "react";

import { useDesktop } from "../hooks/use-desktop";
import { useOsShell } from "../hooks/use-os-shell";
import { openContextMenuFromKeyboard } from "../lib/context-menu-keyboard";
import { mruWindowInstance, windowInstancesFor } from "../lib/window-instance-lookup";
import type { OsAppId } from "../lib/os-types";

export interface OsDockAppMenuProps {
  appId: OsAppId;
  children: React.ReactNode;
}

/**
 * The launch-surface destination chooser (ADR-005): plain activation stays
 * focus-first, while the right-click menu makes windows, tabs, and instances
 * explicit choices. A disabled option always says why (US-006 AC-3).
 */
export function OsDockAppMenu({ appId, children }: OsDockAppMenuProps) {
  const { manager, coordinator } = useOsShell();
  const hasFocusedWindow = useDesktop(state => state.focusedId !== null);
  const hasInstance = useDesktop(
    state => windowInstancesFor(state.windows, { app: appId }).length > 0
  );

  const openWindow = () => {
    void coordinator.userOpen({ app: appId, forceNewInstance: true });
  };
  const openAsTab = () => {
    const focusedId = manager.getState().focusedId;
    if (focusedId === null) return;
    void coordinator.userOpen({
      app: appId,
      stackTargetWindowId: focusedId,
      forceNewInstance: true,
    });
  };
  const goToTab = () => {
    const state = manager.getState();
    const target = mruWindowInstance(state.windows, state.client?.focusOrder ?? [], {
      app: appId,
    });
    if (target !== null) void coordinator.userActivateWindow(target.id);
  };

  return (
    <ContextMenu>
      <ContextMenuTrigger
        render={
          <div className="contents" onKeyDown={openContextMenuFromKeyboard}>
            {children}
          </div>
        }
      />
      <ContextMenuContent data-testid={`os-dock-app-menu-${appId}`}>
        <ContextMenuItem onClick={openWindow}>Open in new window</ContextMenuItem>
        <ContextMenuItem disabled={!hasFocusedWindow} onClick={openAsTab}>
          {hasFocusedWindow ? "Open as tab in focused window" : "Open as tab (no window focused)"}
        </ContextMenuItem>
        {hasInstance ? (
          <>
            <ContextMenuSeparator />
            <ContextMenuItem onClick={goToTab}>Go to tab</ContextMenuItem>
          </>
        ) : null}
      </ContextMenuContent>
    </ContextMenu>
  );
}

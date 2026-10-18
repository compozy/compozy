import {
  ContextMenu,
  ContextMenuContent,
  ContextMenuItem,
  ContextMenuSeparator,
  ContextMenuShortcut,
  ContextMenuTrigger,
} from "@compozy/ui";
import type * as React from "react";

import { useDesktop } from "../hooks/use-desktop";
import { useOsShell } from "../hooks/use-os-shell";
import { openContextMenuFromKeyboard } from "../lib/context-menu-keyboard";
import type { LaunchPlacement } from "../lib/launch-placement";
import { mruWindowInstance, windowInstancesFor } from "../lib/window-instance-lookup";
import type { OsAppId } from "../lib/os-types";

export interface OsDockAppMenuProps {
  appId: OsAppId;
  /** Opens a new instance at the chosen destination (the dock owns per-app launch rules). */
  onLaunch: (placement: LaunchPlacement) => void;
  children: React.ReactNode;
}

/**
 * The launch-surface destination chooser (ADR-005, shell-rail P6): plain
 * activation stays focus-first, while the right-click menu makes every
 * destination an explicit new instance. The modifier hints teach the click
 * grammar the rail also accepts (⌥ split, ⇧ new desktop).
 */
export function OsDockAppMenu({ appId, onLaunch, children }: OsDockAppMenuProps) {
  const { manager, coordinator } = useOsShell();
  const hasInstance = useDesktop(
    state => windowInstancesFor(state.windows, { app: appId }).length > 0
  );

  const goToTab = () => {
    const state = manager.getState();
    const target = mruWindowInstance(state.windows, state.client?.focusOrder ?? [], {
      app: appId,
    });
    if (target !== null) void coordinator.userActivateWindow(target.id);
  };
  const item = (placement: LaunchPlacement) => ({
    "data-testid": `os-dock-app-menu-${appId}-${placement}`,
    onClick: () => onLaunch(placement),
  });

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
        <ContextMenuItem {...item("tab")}>Open in new tab</ContextMenuItem>
        <ContextMenuItem {...item("split")}>
          Open in split
          <ContextMenuShortcut>⌥ click</ContextMenuShortcut>
        </ContextMenuItem>
        <ContextMenuItem {...item("window")}>Open in new window</ContextMenuItem>
        <ContextMenuItem {...item("desktop")}>
          Open in new desktop
          <ContextMenuShortcut>⇧ click</ContextMenuShortcut>
        </ContextMenuItem>
        {hasInstance ? (
          <>
            <ContextMenuSeparator />
            <ContextMenuItem data-testid={`os-dock-app-menu-${appId}-go-to-tab`} onClick={goToTab}>
              Go to tab
            </ContextMenuItem>
          </>
        ) : null}
      </ContextMenuContent>
    </ContextMenu>
  );
}

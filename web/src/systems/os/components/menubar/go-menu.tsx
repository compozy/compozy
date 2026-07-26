import { Fragment } from "react";

import {
  MenubarContent,
  MenubarItem,
  MenubarMenu,
  MenubarSeparator,
  MenubarShortcut,
  MenubarTrigger,
} from "@agh/ui";

import { dockApps } from "../../lib/app-registry";
import type { OsAppId } from "../../lib/os-types";

export interface GoMenuProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** Opening an app window needs a live window-manager client. */
  canOpenApps: boolean;
  onOpenPalette: () => void;
  onOpenApp: (app: OsAppId) => void;
  onOpenWorkspaces: () => void;
}

/**
 * Navigation menu. Groups and order mirror the dock strip exactly, so the two
 * ways to reach a surface teach the same map (recognition over recall).
 */
export function GoMenu({
  open,
  onOpenChange,
  canOpenApps,
  onOpenPalette,
  onOpenApp,
  onOpenWorkspaces,
}: GoMenuProps) {
  return (
    <MenubarMenu open={open} onOpenChange={onOpenChange}>
      <MenubarTrigger>Go</MenubarTrigger>
      <MenubarContent align="start" data-testid="os-menu-go">
        <MenubarItem data-testid="os-menu-palette" onClick={onOpenPalette}>
          Command palette…
          <MenubarShortcut>⌘K</MenubarShortcut>
        </MenubarItem>
        {dockApps().map(group => (
          <Fragment key={group[0].id}>
            <MenubarSeparator />
            {group.map(app => (
              <MenubarItem
                key={app.id}
                data-testid={`os-menu-app-${app.id}`}
                disabled={!canOpenApps}
                onClick={() => onOpenApp(app.id)}
              >
                <app.icon className="size-3.5 text-muted" />
                {app.title}
              </MenubarItem>
            ))}
          </Fragment>
        ))}
        <MenubarSeparator />
        <MenubarItem data-testid="os-menu-workspaces-overview" onClick={onOpenWorkspaces}>
          Workspaces…
        </MenubarItem>
      </MenubarContent>
    </MenubarMenu>
  );
}

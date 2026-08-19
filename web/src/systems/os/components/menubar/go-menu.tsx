import { Fragment } from "react";

import { MenubarContent, MenubarMenu, MenubarSeparator, MenubarTrigger } from "@compozy/ui";

import { dockApps } from "../../lib/app-registry";
import { MenubarCommandItem } from "./menubar-command-item";

export interface GoMenuProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** Runs a registry command through the one dispatch seam. */
  onRun: (commandId: string) => void;
}

/**
 * Navigation menu. Groups and order mirror the dock strip exactly, so the two
 * ways to reach a surface teach the same map (recognition over recall). The
 * app rows themselves are `app.open.*` registry commands, so their availability
 * tracks the live client instead of a locally-computed flag.
 */
export function GoMenu({ open, onOpenChange, onRun }: GoMenuProps) {
  return (
    <MenubarMenu open={open} onOpenChange={onOpenChange}>
      <MenubarTrigger>Go</MenubarTrigger>
      <MenubarContent align="start" data-testid="os-menu-go">
        <MenubarCommandItem commandId="palette.open" onRun={onRun} />
        {dockApps().map(group => (
          <Fragment key={group[0].id}>
            <MenubarSeparator />
            {group.map(app => (
              <MenubarCommandItem commandId={`app.open.${app.id}`} key={app.id} onRun={onRun} />
            ))}
          </Fragment>
        ))}
        <MenubarSeparator />
        <MenubarCommandItem commandId="workspace.picker" onRun={onRun} />
      </MenubarContent>
    </MenubarMenu>
  );
}

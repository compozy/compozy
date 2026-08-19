import {
  MenubarContent,
  MenubarItem,
  MenubarMenu,
  MenubarSeparator,
  MenubarTrigger,
} from "@compozy/ui";

import { MenubarCommandItem } from "./menubar-command-item";

export interface SessionMenuProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** Runs a registry command through the one dispatch seam. */
  onRun: (commandId: string) => void;
  /** Agent creation is a dialog the shell owns, not a registry command. */
  onNewAgent: () => void;
}

/** Session menu: create work, or reach the global sessions catalog. */
export function SessionMenu({ open, onOpenChange, onRun, onNewAgent }: SessionMenuProps) {
  return (
    <MenubarMenu open={open} onOpenChange={onOpenChange}>
      <MenubarTrigger>Session</MenubarTrigger>
      <MenubarContent align="start" data-testid="os-menu-session">
        <MenubarCommandItem commandId="session.new" onRun={onRun} />
        <MenubarItem data-testid="os-menu-new-agent" onClick={onNewAgent}>
          New agent…
        </MenubarItem>
        <MenubarSeparator />
        <MenubarCommandItem commandId="shell.sessions.toggle" onRun={onRun} />
      </MenubarContent>
    </MenubarMenu>
  );
}

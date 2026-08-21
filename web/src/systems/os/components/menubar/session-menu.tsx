import { MenubarContent, MenubarItem, MenubarMenu, MenubarTrigger } from "@compozy/ui";

import { usePaletteRegistry } from "../../hooks/use-palette-registry";
import { MenubarCommandGroups } from "./menubar-command-groups";
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
  const registry = usePaletteRegistry();
  return (
    <MenubarMenu open={open} onOpenChange={onOpenChange}>
      <MenubarTrigger>Session</MenubarTrigger>
      <MenubarContent align="start" data-testid="os-menu-session">
        <MenubarCommandGroups
          groups={[
            {
              id: "create",
              content: (
                <>
                  <MenubarCommandItem commandId="session.new" onRun={onRun} />
                  <MenubarItem data-testid="os-menu-new-agent" onClick={onNewAgent}>
                    New agent…
                  </MenubarItem>
                </>
              ),
            },
            {
              id: "catalog",
              content: registry.byId.has("shell.sessions.toggle") ? (
                <MenubarCommandItem commandId="shell.sessions.toggle" onRun={onRun} />
              ) : null,
            },
          ]}
        />
      </MenubarContent>
    </MenubarMenu>
  );
}

import { MenubarContent, MenubarItem, MenubarMenu } from "@compozy/ui";

import { usePaletteRegistry } from "../../hooks/use-palette-registry";
import { MenubarCommandGroups } from "./menubar-command-groups";
import { MenubarCommandItem } from "./menubar-command-item";

export interface CompozyMenuProps {
  /** The CompozyOS mark, already built as a `MenubarTrigger` by the chrome. */
  trigger: React.ReactNode;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** Runs a registry command through the one dispatch seam. */
  onRun: (commandId: string) => void;
  /** The About dialog is shell chrome, not a registry command. */
  onAbout: () => void;
}

/** The system menu on the CompozyOS mark: identity plus the settings surfaces. */
export function CompozyMenu({ trigger, open, onOpenChange, onRun, onAbout }: CompozyMenuProps) {
  const registry = usePaletteRegistry();
  const has = (commandId: string) => registry.byId.has(commandId);
  return (
    <MenubarMenu open={open} onOpenChange={onOpenChange}>
      {trigger}
      <MenubarContent align="start" data-testid="os-menu-compozy">
        <MenubarCommandGroups
          groups={[
            {
              id: "about",
              content: (
                <MenubarItem data-testid="os-menu-about" onClick={onAbout}>
                  About CompozyOS…
                </MenubarItem>
              ),
            },
            {
              id: "settings",
              content: ["settings.general", "settings.appearance", "settings.layouts"].some(has) ? (
                <>
                  <MenubarCommandItem commandId="settings.general" onRun={onRun} />
                  <MenubarCommandItem commandId="settings.appearance" onRun={onRun} />
                  <MenubarCommandItem commandId="settings.layouts" onRun={onRun} />
                </>
              ) : null,
            },
          ]}
        />
      </MenubarContent>
    </MenubarMenu>
  );
}

import { MenubarContent, MenubarItem, MenubarMenu, MenubarSeparator } from "@compozy/ui";

export interface CompozyMenuProps {
  /** The CompozyOS mark, already built as a `MenubarTrigger` by the chrome. */
  trigger: React.ReactNode;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** Opening a settings surface needs a live window-manager client. */
  canOpenApps: boolean;
  onAbout: () => void;
  onSettings: () => void;
  onAppearance: () => void;
  onLayouts: () => void;
}

/** The system menu on the CompozyOS mark: identity plus the settings surfaces. */
export function CompozyMenu({
  trigger,
  open,
  onOpenChange,
  canOpenApps,
  onAbout,
  onSettings,
  onAppearance,
  onLayouts,
}: CompozyMenuProps) {
  return (
    <MenubarMenu open={open} onOpenChange={onOpenChange}>
      {trigger}
      <MenubarContent align="start" data-testid="os-menu-compozy">
        <MenubarItem data-testid="os-menu-about" onClick={onAbout}>
          About CompozyOS…
        </MenubarItem>
        <MenubarSeparator />
        <MenubarItem data-testid="os-menu-settings" disabled={!canOpenApps} onClick={onSettings}>
          Settings…
        </MenubarItem>
        <MenubarItem
          data-testid="os-menu-appearance"
          disabled={!canOpenApps}
          onClick={onAppearance}
        >
          Appearance…
        </MenubarItem>
        <MenubarItem data-testid="os-menu-layouts" disabled={!canOpenApps} onClick={onLayouts}>
          Layouts…
        </MenubarItem>
      </MenubarContent>
    </MenubarMenu>
  );
}

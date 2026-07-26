import { MenubarContent, MenubarItem, MenubarMenu, MenubarSeparator } from "@agh/ui";

export interface AghMenuProps {
  /** The AGH mark, already built as a `MenubarTrigger` by the chrome. */
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

/** The system menu on the AGH mark: identity plus the settings surfaces. */
export function AghMenu({
  trigger,
  open,
  onOpenChange,
  canOpenApps,
  onAbout,
  onSettings,
  onAppearance,
  onLayouts,
}: AghMenuProps) {
  return (
    <MenubarMenu open={open} onOpenChange={onOpenChange}>
      {trigger}
      <MenubarContent align="start" data-testid="os-menu-agh">
        <MenubarItem data-testid="os-menu-about" onClick={onAbout}>
          About AGH…
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

import {
  MenubarContent,
  MenubarItem,
  MenubarMenu,
  MenubarSeparator,
  MenubarShortcut,
  MenubarTrigger,
} from "@agh/ui";

export interface SessionMenuProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onNewSession: () => void;
  onNewAgent: () => void;
  onOpenSessions: () => void;
}

/** Session menu: create work, or reach the global sessions catalog. */
export function SessionMenu({
  open,
  onOpenChange,
  onNewSession,
  onNewAgent,
  onOpenSessions,
}: SessionMenuProps) {
  return (
    <MenubarMenu open={open} onOpenChange={onOpenChange}>
      <MenubarTrigger>Session</MenubarTrigger>
      <MenubarContent align="start" data-testid="os-menu-session">
        <MenubarItem data-testid="os-menu-new-session" onClick={onNewSession}>
          New session
          <MenubarShortcut>⌘N</MenubarShortcut>
        </MenubarItem>
        <MenubarItem data-testid="os-menu-new-agent" onClick={onNewAgent}>
          New agent…
        </MenubarItem>
        <MenubarSeparator />
        <MenubarItem data-testid="os-menu-sessions" onClick={onOpenSessions}>
          Sessions…
        </MenubarItem>
      </MenubarContent>
    </MenubarMenu>
  );
}

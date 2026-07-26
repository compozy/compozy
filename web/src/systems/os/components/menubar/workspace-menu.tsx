import { Check } from "lucide-react";

import { MenubarContent, MenubarItem, MenubarMenu, MenubarSeparator } from "@agh/ui";

import type { WorkspacePayload } from "@/systems/workspace";

export interface WorkspaceMenuProps {
  /** The workspace chip, already built as a `MenubarTrigger` by the chrome. */
  trigger: React.ReactNode;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  workspaces: WorkspacePayload[];
  activeWorkspaceId: string | undefined;
  monogram: (name: string) => string;
  onSelectWorkspace: (workspaceId: string) => void;
  onOpenWorkspaces: () => void;
  onAddWorkspace: () => void;
}

/** Workspace switcher: the bound set, then the overview and creation paths. */
export function WorkspaceMenu({
  trigger,
  open,
  onOpenChange,
  workspaces,
  activeWorkspaceId,
  monogram,
  onSelectWorkspace,
  onOpenWorkspaces,
  onAddWorkspace,
}: WorkspaceMenuProps) {
  return (
    <MenubarMenu open={open} onOpenChange={onOpenChange}>
      {trigger}
      <MenubarContent align="start" data-testid="os-workspace-menu">
        {workspaces.map(workspace => (
          <MenubarItem
            key={workspace.id}
            data-testid={`os-workspace-option-${workspace.id}`}
            onClick={() => onSelectWorkspace(workspace.id)}
          >
            <span className="grid size-4 place-items-center rounded-xs border border-line-strong bg-elevated font-mono text-micro font-semibold">
              {monogram(workspace.name)}
            </span>
            {workspace.name}
            {workspace.id === activeWorkspaceId ? (
              <Check className="ml-auto size-3 text-accent" />
            ) : null}
          </MenubarItem>
        ))}
        <MenubarSeparator />
        <MenubarItem data-testid="os-workspace-overview" onClick={onOpenWorkspaces}>
          Workspaces overview…
        </MenubarItem>
        <MenubarItem data-testid="os-workspace-add" onClick={onAddWorkspace}>
          Add workspace…
        </MenubarItem>
      </MenubarContent>
    </MenubarMenu>
  );
}

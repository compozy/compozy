import { Check, Info } from "lucide-react";

import {
  Alert,
  AlertDescription,
  MenubarContent,
  MenubarItem,
  MenubarMenu,
  MenubarSeparator,
} from "@compozy/ui";

import { GLOBAL_SCOPE_COPY, type WorkspacePayload } from "@/systems/workspace";

export interface WorkspaceMenuProps {
  /** The workspace chip, already built as a `MenubarTrigger` by the chrome. */
  trigger: React.ReactNode;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  workspaces: WorkspacePayload[];
  activeWorkspaceId: string | undefined;
  globalScopeOn?: boolean;
  deletionNotice?: string | null;
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
  globalScopeOn = false,
  deletionNotice = null,
  monogram,
  onSelectWorkspace,
  onOpenWorkspaces,
  onAddWorkspace,
}: WorkspaceMenuProps) {
  const notice = deletionNotice ?? (globalScopeOn ? GLOBAL_SCOPE_COPY.menuNotice : null);

  return (
    <MenubarMenu open={open} onOpenChange={onOpenChange}>
      {trigger}
      <MenubarContent align="start" data-testid="os-workspace-menu">
        {notice ? (
          <Alert
            className="mx-1.5 mb-1 px-2.5 py-2"
            data-testid="os-workspace-global-notice"
            role="note"
            variant="info"
          >
            <Info aria-hidden="true" className="size-3.5" />
            <AlertDescription className="text-form-hint">{notice}</AlertDescription>
          </Alert>
        ) : null}
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
            {!globalScopeOn && workspace.id === activeWorkspaceId ? (
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

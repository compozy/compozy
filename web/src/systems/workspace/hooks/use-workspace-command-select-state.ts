import { useRef } from "react";
import { useSelector, useStore } from "@xstate/store-react";

import { partitionProjectWorkspaces } from "../lib/project-workspaces";
import { groupWorkspaceTree, type WorktreeListingByWorkspace } from "../lib/workspace-tree";
import { workspaceCommandSelectLogic } from "../stores/workspace-command-select-store";

interface WorkspaceOption {
  id: string;
  name: string;
  root_dir?: string | null;
}

interface UseWorkspaceCommandSelectStateOptions {
  userHomeDir?: string;
  workspaces: ReadonlyArray<WorkspaceOption> | undefined;
  value: string | null;
  onChange: (id: string) => void;
  onAddWorkspace?: () => void;
  open?: boolean;
  onOpenChange?: (open: boolean) => void;
  worktreesByWorkspace?: WorktreeListingByWorkspace;
}

export function useWorkspaceCommandSelectState({
  userHomeDir,
  workspaces,
  value,
  onChange,
  onAddWorkspace,
  open: controlledOpen,
  onOpenChange,
  worktreesByWorkspace,
}: UseWorkspaceCommandSelectStateOptions) {
  const store = useStore(workspaceCommandSelectLogic);
  const interaction = useSelector(store, snapshot => snapshot.context);
  const inputRef = useRef<HTMLInputElement | null>(null);
  const open = controlledOpen ?? interaction.open;
  const { projectWorkspaces } = partitionProjectWorkspaces(workspaces, userHomeDir);
  const selected = projectWorkspaces.find(workspace => workspace.id === value) ?? null;
  const tree = worktreesByWorkspace
    ? groupWorkspaceTree(projectWorkspaces, worktreesByWorkspace, userHomeDir)
    : null;
  const nodeByWorkspaceId = new Map(tree?.map(node => [node.workspace.id, node]) ?? []);
  const visibleSubmenu = open ? interaction.submenu : null;

  const handleCommandKeyDown = (event: React.KeyboardEvent) => {
    if (event.key !== "ArrowRight") return;
    const workspace = projectWorkspaces.find(item => item.name === interaction.activeCommandValue);
    if (!workspace || !nodeByWorkspaceId.get(workspace.id)?.gitBacked) return;
    event.preventDefault();
    store.trigger.submenuKeyboardOpened({ workspaceId: workspace.id });
  };

  return {
    activeCommandValue: interaction.activeCommandValue,
    focusedSubmenuWorkspaceId:
      visibleSubmenu?.focus === "pending" ? visibleSubmenu.workspaceId : null,
    inputRef,
    nodeByWorkspaceId,
    open,
    pointerSelectionArmed: () => store.trigger.pointerSelectionArmed(),
    projectWorkspaces,
    selected,
    submenuWorkspaceId: visibleSubmenu?.workspaceId ?? null,
    addWorkspace: () =>
      store.trigger.addWorkspaceSelected({ add: () => onAddWorkspace?.(), setOpen: onOpenChange }),
    changeOpen: (next: boolean) => store.trigger.openChanged({ open: next, setOpen: onOpenChange }),
    changeSubmenuOpen: (workspaceId: string, next: boolean) =>
      store.trigger.submenuOpenChanged({ open: next, workspaceId }),
    changeCommandValue: (next: string) => store.trigger.commandValueChanged({ value: next }),
    close: () => store.trigger.openChanged({ open: false, setOpen: onOpenChange }),
    handleCommandKeyDown,
    selectWorkspace: (workspaceId: string, gitBacked: boolean) =>
      store.trigger.workspaceSelected({
        gitBacked,
        select: () => onChange(workspaceId),
        setOpen: onOpenChange,
        workspaceId,
      }),
  };
}

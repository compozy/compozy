import { ArrowDown, ArrowUp, Folder, FolderOutput } from "lucide-react";
import type { ReactNode } from "react";

import {
  ContextMenuItem,
  ContextMenuSub,
  ContextMenuSubContent,
  ContextMenuSubTrigger,
  cn,
} from "@compozy/ui";

import type { SessionFolderDialogState } from "../../hooks/use-session-list-folder-layout";
import {
  projectDropEdge,
  projectFolderDropEdge,
  useProjectFoldersDnd,
} from "../../hooks/use-project-folders-dnd";
import { useSessionSidebarState } from "../../hooks/use-session-sidebar-state";
import type { WorkspaceLayout } from "../../hooks/use-workspace-layout";
import { shiftId } from "../../lib/session-folders";
import type { SessionListGroupSpec } from "../../lib/session-list-groups";
import {
  projectGroupContainerIds,
  projectGroupTreeGroups,
  projectGroupWorkspaceId,
  type ProjectGroupTree,
} from "../../lib/workspace-folders";
import type { WorkspaceFolderPayload } from "../../types";
import { BRANCH_NODE, BRANCH_NODE_HEADER_ARM } from "./session-list-branch-rail";
import type { SessionListGroupArrange } from "./session-list-group-header";
import { SessionListFolder } from "./session-list-folder";

export interface SessionListProjectFoldersProps {
  projectFolders: { layout: WorkspaceLayout; tree: ProjectGroupTree };
  onFolderDialog: (state: SessionFolderDialogState<WorkspaceFolderPayload>) => void;
  /** Renders one project group at its on-screen rank with its part in the folders. */
  renderGroup: (
    group: SessionListGroupSpec,
    path: readonly number[],
    arrange: SessionListGroupArrange | undefined
  ) => ReactNode;
  emptyMessage: string;
  testIdPrefix: string;
}

/**
 * Project groups of All projects arranged in one profile's project folders
 * (issue #1044): folders on top, in their order, each hanging its projects on
 * the tree rail; then the root. Folders and projects drag to reorder, projects
 * drag into a folder through its header and out through the root, and every
 * drag has a menu alternative on the header it starts from.
 */
export function SessionListProjectFolders({
  projectFolders: { layout, tree },
  onFolderDialog,
  renderGroup,
  emptyMessage,
  testIdPrefix,
}: SessionListProjectFoldersProps) {
  const dnd = useProjectFoldersDnd(layout, tree);
  const { collapsedFolderIds, toggleFolder } = useSessionSidebarState();
  const collapsed = new Set(collapsedFolderIds);
  const ranks = new Map(projectGroupTreeGroups(tree).map((group, index) => [group.key, index]));
  const draggingFoldered = dnd.dragging?.kind === "project" && dnd.dragging.folderId !== null;

  const arrangeFor = (
    group: SessionListGroupSpec,
    folderId: string | null
  ): SessionListGroupArrange | undefined => {
    const workspaceId = projectGroupWorkspaceId(group);
    if (workspaceId === null) return undefined;
    const container = projectGroupContainerIds(tree, folderId);
    const shift = (delta: -1 | 1) => {
      const next = shiftId(container, workspaceId, delta);
      return next ? () => layout.reorderWorkspaces(folderId, next) : undefined;
    };
    return {
      drag: dnd.projectHeader(workspaceId, folderId),
      dropEdge: projectDropEdge(dnd.indicator, workspaceId),
      dragging: dnd.dragging?.kind === "project" && dnd.dragging.workspaceId === workspaceId,
      menuItems: (
        <ProjectFolderMenuItems
          folders={tree.folders.map(node => node.folder)}
          folderId={folderId}
          onMove={target => layout.moveWorkspace(workspaceId, target)}
          onMoveUp={shift(-1)}
          onMoveDown={shift(1)}
          testId={`${testIdPrefix}-group-${group.key}-arrange`}
        />
      ),
    };
  };
  const render = (group: SessionListGroupSpec, folderId: string | null) =>
    renderGroup(group, [ranks.get(group.key) ?? 0], arrangeFor(group, folderId));

  return (
    <div className="flex flex-col gap-1">
      {tree.folders.map(({ folder, groups }, index) => (
        <SessionListFolder
          key={folder.id}
          folder={folder}
          count={groups.length}
          holds="projects"
          expanded={!collapsed.has(folder.id)}
          onToggle={() => toggleFolder(folder.id)}
          onRename={() => onFolderDialog({ kind: "rename", folder })}
          onDelete={() => onFolderDialog({ kind: "delete", folder })}
          onMoveUp={index > 0 ? () => layout.moveFolder(folder.id, -1) : undefined}
          onMoveDown={
            index < tree.folders.length - 1 ? () => layout.moveFolder(folder.id, 1) : undefined
          }
          drag={dnd.folderHeader(folder.id)}
          dropEdge={projectFolderDropEdge(dnd.indicator, folder.id)}
          dragging={dnd.dragging?.kind === "folder" && dnd.dragging.id === folder.id}
          testIdPrefix={`${testIdPrefix}-project`}
        >
          {groups.length === 0 ? (
            <p className="px-2 py-1 text-micro text-subtle">Drag a project here.</p>
          ) : (
            groups.map(group => (
              <div
                key={group.key}
                className={cn(BRANCH_NODE, BRANCH_NODE_HEADER_ARM)}
                data-slot="project-folder-member"
              >
                {render(group, folder.id)}
              </div>
            ))
          )}
        </SessionListFolder>
      ))}
      <div
        {...dnd.rootZone()}
        data-testid={`${testIdPrefix}-project-root`}
        data-drop={dnd.indicator?.kind === "root" ? "into" : undefined}
        className={cn(
          "flex flex-col gap-1 rounded-md transition-colors",
          dnd.indicator?.kind === "root" && "bg-accent-tint ring-1 ring-accent ring-inset"
        )}
      >
        {tree.root.map(group => render(group, null))}
        {tree.root.length === 0 && draggingFoldered ? (
          <p className="px-2 py-2 text-micro text-subtle">
            Drop here to take it out of the folder.
          </p>
        ) : null}
        {tree.root.length === 0 && tree.folders.length === 0 ? (
          <p className="px-3 py-8 text-center text-small-body text-muted">{emptyMessage}</p>
        ) : null}
      </div>
    </div>
  );
}

/** A project header's folder verbs: move into another folder, out of its own, or within it. */
function ProjectFolderMenuItems({
  folders,
  folderId,
  onMove,
  onMoveUp,
  onMoveDown,
  testId,
}: {
  folders: readonly WorkspaceFolderPayload[];
  folderId: string | null;
  onMove: (folderId: string | null) => void;
  onMoveUp?: () => void;
  onMoveDown?: () => void;
  testId: string;
}) {
  const targets = folders.filter(folder => folder.id !== folderId);
  return (
    <>
      {targets.length > 0 ? (
        <ContextMenuSub>
          <ContextMenuSubTrigger data-testid={`${testId}-move-to`}>
            <Folder aria-hidden="true" className="size-3" />
            Move to folder
          </ContextMenuSubTrigger>
          <ContextMenuSubContent className="min-w-44">
            {targets.map(folder => (
              <ContextMenuItem
                key={folder.id}
                data-testid={`${testId}-move-to-${folder.id}`}
                onClick={() => onMove(folder.id)}
              >
                {folder.name}
              </ContextMenuItem>
            ))}
          </ContextMenuSubContent>
        </ContextMenuSub>
      ) : null}
      {folderId !== null ? (
        <ContextMenuItem data-testid={`${testId}-move-out`} onClick={() => onMove(null)}>
          <FolderOutput aria-hidden="true" className="size-3" />
          Move out of folder
        </ContextMenuItem>
      ) : null}
      <ContextMenuItem
        data-testid={`${testId}-up`}
        disabled={onMoveUp === undefined}
        onClick={onMoveUp}
      >
        <ArrowUp aria-hidden="true" className="size-3" />
        Move project up
      </ContextMenuItem>
      <ContextMenuItem
        data-testid={`${testId}-down`}
        disabled={onMoveDown === undefined}
        onClick={onMoveDown}
      >
        <ArrowDown aria-hidden="true" className="size-3" />
        Move project down
      </ContextMenuItem>
    </>
  );
}

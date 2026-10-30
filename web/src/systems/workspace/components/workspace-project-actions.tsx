import type { ComponentProps, ReactElement, ReactNode } from "react";
import { Archive, ArchiveRestore, MoreHorizontal, Trash2 } from "lucide-react";
import {
  Button,
  ConfirmDialog,
  ContextMenu,
  ContextMenuContent,
  ContextMenuItem,
  ContextMenuSeparator,
  ContextMenuTrigger,
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@compozy/ui";
import { WorkspaceApiError } from "@/systems/workspace/adapters/workspace-api";
import {
  useWorkspaceProjectAction,
  type WorkspaceProjectAction,
  type WorkspaceProjectActionFlow,
} from "@/systems/workspace/hooks/use-workspace-project-action";
import type { WorkspacePayload } from "@/systems/workspace/types";

type ProjectAction = WorkspaceProjectAction;

/** The confirm every project action passes through, with the daemon's refusal. */
function WorkspaceProjectActionDialog({
  workspace,
  flow,
  finalFocus,
}: {
  workspace: WorkspacePayload;
  flow: WorkspaceProjectActionFlow;
  /** Where focus lands on close when the element that opened the confirm has unmounted. */
  finalFocus?: ComponentProps<typeof DropdownMenuContent>["finalFocus"];
}) {
  const { action, archived, pending, failure } = flow;
  const copy = projectActionCopy(action, archived);
  return (
    <ConfirmDialog
      open={action !== null}
      onOpenChange={open => {
        if (!open) flow.dismiss();
      }}
      title={`${copy.verb} ${workspace.name}?`}
      description={copy.description}
      confirmLabel={`${copy.verb} project`}
      cancelLabel="Cancel"
      tone={copy.tone}
      contentProps={finalFocus ? { finalFocus } : undefined}
      isPending={pending}
      onConfirm={flow.submit}
      // A project that left the catalog can no longer be acted on; its failure stays readable.
      confirmButtonProps={flow.listed ? undefined : { disabled: true }}
      error={
        failure instanceof WorkspaceApiError && failure.code === "workspace_busy"
          ? "Stop active work before archiving this project."
          : failure?.message
      }
      // An empty body still draws the dialog's body band; it renders only with blockers.
      body={
        flow.blockers.length > 0 ? <WorkspaceArchiveBlockers blockers={flow.blockers} /> : undefined
      }
      footNote={
        action === "remove" && !archived ? (
          <Button
            variant="ghost"
            disabled={pending || !flow.listed}
            onClick={() => flow.choose("archive")}
          >
            Archive instead
          </Button>
        ) : undefined
      }
    />
  );
}

/**
 * A project's actions: a context menu on `children` (right-click; the child
 * owns Shift+F10 by dispatching `contextmenu`), or a `…` button without one.
 */
export function WorkspaceProjectActions({
  workspace,
  children,
}: {
  workspace: WorkspacePayload;
  children?: ReactElement;
}) {
  const flow = useWorkspaceProjectAction(workspace);
  if (children) {
    return (
      <WorkspaceProjectContextMenu workspace={workspace}>{children}</WorkspaceProjectContextMenu>
    );
  }
  return (
    <>
      <DropdownMenu>
        <DropdownMenuTrigger
          render={<Button variant="ghost" size="icon" aria-label={`Manage ${workspace.name}`} />}
        >
          <MoreHorizontal />
        </DropdownMenuTrigger>
        <DropdownMenuContent>
          <WorkspaceActionItems workspace={workspace} choose={flow.choose} kind="dropdown" />
        </DropdownMenuContent>
      </DropdownMenu>
      <WorkspaceProjectActionDialog workspace={workspace} flow={flow} />
    </>
  );
}

/**
 * Right-click (and Shift+F10, which the child dispatches as `contextmenu`) on
 * `children` opens the project's actions. Without a project yet the trigger
 * stays mounted but inert, so the child never remounts when the project loads.
 */
export function WorkspaceProjectContextMenu({
  workspace,
  leadingItems,
  children,
}: {
  workspace: WorkspacePayload | undefined;
  /** Host items shown before the project's own actions; they keep the menu available without one. */
  leadingItems?: ReactNode;
  children: ReactElement;
}) {
  const flow = useWorkspaceProjectAction(workspace);
  const enabled = Boolean(workspace) || Boolean(leadingItems);
  return (
    <>
      <ContextMenu disabled={!enabled}>
        <ContextMenuTrigger render={children} />
        {enabled ? (
          <ContextMenuContent>
            {leadingItems}
            {leadingItems && workspace ? <ContextMenuSeparator /> : null}
            {workspace ? (
              <WorkspaceActionItems workspace={workspace} choose={flow.choose} kind="context" />
            ) : null}
          </ContextMenuContent>
        ) : null}
      </ContextMenu>
      {workspace ? <WorkspaceProjectActionDialog workspace={workspace} flow={flow} /> : null}
    </>
  );
}

/** Where a relayed project context click landed, in viewport coordinates. */
export interface WorkspaceProjectMenuPoint {
  x: number;
  y: number;
}

/**
 * A project's actions at a point, for a surface that cannot nest a context
 * menu of its own (a menubar menu): the host relays the click and keeps this
 * mounted, so the confirm dialog outlives the menu. A menu item that already
 * chose an action (the menubar submenu's Archive project) mounts it with
 * `initialAction` and no point, opening the confirm directly.
 */
export function WorkspaceProjectPointMenu({
  workspace,
  listed,
  point,
  initialAction = null,
  finalFocus,
  onClose,
}: {
  workspace: WorkspacePayload;
  /** The catalog still lists the project; an unlisted one closes its menu and confirm. */
  listed: boolean;
  /** The open menu's position; `null` while closed. */
  point: WorkspaceProjectMenuPoint | null;
  initialAction?: ProjectAction | null;
  finalFocus?: ComponentProps<typeof DropdownMenuContent>["finalFocus"];
  onClose: () => void;
}) {
  const flow = useWorkspaceProjectAction(workspace, listed, initialAction);
  return (
    <>
      <DropdownMenu
        open={point !== null && listed}
        onOpenChange={open => {
          if (!open) onClose();
        }}
      >
        <DropdownMenuContent
          anchor={point ? pointAnchor(point) : undefined}
          finalFocus={finalFocus}
          side="bottom"
          align="start"
          sideOffset={0}
          className="w-auto"
          data-testid={`workspace-project-menu-${workspace.id}`}
        >
          <WorkspaceActionItems workspace={workspace} choose={flow.choose} kind="dropdown" />
        </DropdownMenuContent>
      </DropdownMenu>
      <WorkspaceProjectActionDialog workspace={workspace} flow={flow} finalFocus={finalFocus} />
    </>
  );
}

function pointAnchor({ x, y }: WorkspaceProjectMenuPoint) {
  return { getBoundingClientRect: () => DOMRect.fromRect({ x, y, width: 0, height: 0 }) };
}

const PROJECT_MENU_KITS = {
  context: { Item: ContextMenuItem },
  dropdown: { Item: DropdownMenuItem },
} as const;

function WorkspaceActionItems({
  workspace,
  choose,
  kind,
}: {
  workspace: WorkspacePayload;
  choose: (action: ProjectAction) => void;
  kind: keyof typeof PROJECT_MENU_KITS;
}) {
  const { Item } = PROJECT_MENU_KITS[kind];
  const archived = Boolean(workspace.archived_at);
  const ArchiveIcon = archived ? ArchiveRestore : Archive;
  return (
    <>
      <Item onClick={() => choose("archive")}>
        <ArchiveIcon />
        {archived ? "Unarchive project" : "Archive project"}
      </Item>
      <Item variant="destructive" onClick={() => choose("remove")}>
        <Trash2 />
        Remove project
      </Item>
    </>
  );
}

function WorkspaceArchiveBlockers({ blockers }: { blockers: WorkspaceApiError["blockers"] }) {
  return (
    <div role="alert">
      <p>Stop these sessions and Loops, or disable these automations, then try again.</p>
      <ul className="mt-2 space-y-1">
        {blockers.map(blocker => (
          <li key={`${blocker.kind}:${blocker.id}`}>
            {blocker.kind}: {blocker.name || blocker.id}
          </li>
        ))}
      </ul>
    </div>
  );
}

function projectActionCopy(action: ProjectAction | null, archived: boolean) {
  if (action === "remove")
    return {
      verb: "Remove",
      tone: "danger" as const,
      description:
        "Stopped session history is deleted and window bindings are removed. Adding this folder again creates a new project ID. Files on disk are untouched.",
    };
  if (archived)
    return {
      verb: "Unarchive",
      tone: "neutral" as const,
      description: "Restore this project to navigation with the same ID, history and layouts.",
    };
  return {
    verb: "Archive",
    tone: "neutral" as const,
    description:
      "Hide this project from navigation and All projects. Sessions, history, layouts, worktrees and configuration are kept.",
  };
}

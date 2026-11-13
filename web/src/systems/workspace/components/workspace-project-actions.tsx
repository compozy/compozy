import { use, useEffect, type ComponentProps, type ReactElement } from "react";
import { MoreHorizontal } from "@compozy/ui/icons";
import {
  Button,
  ConfirmDialog,
  ContextMenu,
  ContextMenuContent,
  ContextMenuTrigger,
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuTrigger,
  MenuTreeItems,
  type MenuTreeHost,
} from "@compozy/ui";
import { WorkspaceApiError } from "@/systems/workspace/adapters/workspace-api";
import { ProjectMenuHostContext } from "@/systems/workspace/contexts/project-menu-host-context";
import {
  useWorkspaceProjectAction,
  type WorkspaceProjectAction,
  type WorkspaceProjectActionFlow,
} from "@/systems/workspace/hooks/use-workspace-project-action";
import {
  hasProjectMenuVerbs,
  mergeProjectMenuVerbs,
  projectMenuTree,
  type ProjectMenuVerbs,
} from "@/systems/workspace/lib/project-menu";
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
 * A project's menu: a context menu on `children` (right-click; the child
 * owns Shift+F10 by dispatching `contextmenu`), or a `…` button without one.
 */
export function WorkspaceProjectActions({
  workspace,
  onLeave,
  children,
}: {
  workspace: WorkspacePayload;
  /** After a verb that opens something elsewhere (a session, Files, a folder dialog). */
  onLeave?: () => void;
  children?: ReactElement;
}) {
  const flow = useWorkspaceProjectAction(workspace);
  if (children) {
    return (
      <WorkspaceProjectContextMenu workspace={workspace} onLeave={onLeave}>
        {children}
      </WorkspaceProjectContextMenu>
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
          <ProjectMenuItems
            workspace={workspace}
            choose={flow.choose}
            host="dropdown"
            onLeave={onLeave}
          />
        </DropdownMenuContent>
      </DropdownMenu>
      <WorkspaceProjectActionDialog workspace={workspace} flow={flow} />
    </>
  );
}

/**
 * Right-click (and Shift+F10, which the child dispatches as `contextmenu`) on
 * `children` opens the project's menu. Without a project yet the trigger
 * stays mounted but inert, so the child never remounts when the project loads.
 */
export function WorkspaceProjectContextMenu({
  workspace,
  verbs,
  testId,
  onLeave,
  children,
}: {
  workspace: WorkspacePayload | undefined;
  /** The surface's own verbs, over the shell's; they keep the menu available without a project. */
  verbs?: ProjectMenuVerbs;
  /** Test id prefix of the menu's rows. */
  testId?: string;
  /** After a verb that opens something elsewhere (a session, Files, a folder dialog). */
  onLeave?: () => void;
  children: ReactElement;
}) {
  const flow = useWorkspaceProjectAction(workspace);
  const enabled = Boolean(workspace) || (verbs !== undefined && hasProjectMenuVerbs(verbs));
  return (
    <>
      <ContextMenu disabled={!enabled}>
        <ContextMenuTrigger render={children} />
        {enabled ? (
          <ContextMenuContent>
            <ProjectMenuItems
              workspace={workspace}
              verbs={verbs}
              choose={flow.choose}
              host="context"
              testId={testId}
              onLeave={onLeave}
            />
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
          <ProjectMenuItems workspace={workspace} choose={flow.choose} host="dropdown" />
        </DropdownMenuContent>
      </DropdownMenu>
      <WorkspaceProjectActionDialog workspace={workspace} flow={flow} finalFocus={finalFocus} />
    </>
  );
}

function pointAnchor({ x, y }: WorkspaceProjectMenuPoint) {
  return { getBoundingClientRect: () => DOMRect.fromRect({ x, y, width: 0, height: 0 }) };
}

/**
 * The project menu's rows, mounted only while the menu is open: the surface's verbs over the
 * shell's, then the archive/remove flow. Opening tells the shell to read what its verbs need.
 */
function ProjectMenuItems({
  workspace,
  verbs,
  choose,
  host,
  testId,
  onLeave,
}: {
  workspace: WorkspacePayload | undefined;
  verbs?: ProjectMenuVerbs;
  choose: (action: ProjectAction) => void;
  host: MenuTreeHost;
  testId?: string;
  onLeave?: () => void;
}) {
  const shell = use(ProjectMenuHostContext);
  const menuOpened = shell?.menuOpened;
  useEffect(() => {
    menuOpened?.();
  }, [menuOpened]);
  const merged = mergeProjectMenuVerbs(
    workspace && shell ? shell.verbsFor(workspace) : undefined,
    verbs
  );
  return (
    <MenuTreeItems
      host={host}
      nodes={projectMenuTree({
        workspace,
        verbs: onLeave ? leavingVerbs(merged, onLeave) : merged,
        choose,
        testId,
      })}
    />
  );
}

/** The verbs that open something elsewhere, followed by `leave`. */
function leavingVerbs(verbs: ProjectMenuVerbs, leave: () => void): ProjectMenuVerbs {
  const then = (verb?: () => void) =>
    verb
      ? () => {
          verb();
          leave();
        }
      : undefined;
  return {
    ...verbs,
    newSession: then(verbs.newSession),
    newFolder: then(verbs.newFolder),
    addAlwaysOn: then(verbs.addAlwaysOn),
    openFiles: then(verbs.openFiles),
  };
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
      tone: "destructive" as const,
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

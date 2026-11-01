import type { ReactNode } from "react";

import { StatusDot, cn } from "@compozy/ui";

import { openContextMenuFromKeyboard } from "@/systems/os";
import { ProfileOwnerTag } from "@/systems/profiles";
import {
  CreateDestinationPicker,
  WorkspaceBadge,
  workspaceWorkingLabel,
} from "@/systems/workspace";

import {
  useSessionListGroupFolderCreate,
  type SessionGroupFolderAxis,
} from "../../hooks/use-session-list-group-folder-create";
import type { SessionDragHandlers, SessionDropPosition } from "../../hooks/use-session-list-dnd";
import type { SessionCreateOpenOptions } from "../../lib/session-create-target";
import {
  sessionListGroupName,
  sessionListGroupTarget,
  type SessionListGroupSpec,
} from "../../lib/session-list-groups";
import { StaysOnCreateMenu } from "../stays-on/stays-on-create-menu";
import { StaysOnCollapsedSignal } from "../stays-on/stays-on-signals";
import { SessionFolderDialogs } from "./session-folder-dialogs";
import { SessionListCreateMenu } from "./session-list-create-menu";
import { SessionListGroupProjectMenu } from "./session-list-group-project-menu";
import { SessionListGroupToggle } from "./session-list-group-toggle";

/** Collapsed header signal: needs-you outranks working; an open group shows its rows instead. */
function GroupSignal({ group }: { group: SessionListGroupSpec }) {
  const scope = group.kind === "project" ? sessionListGroupTarget(group).workspaceId : null;
  return (
    <>
      {scope ? <StaysOnCollapsedSignal workspaceId={scope} /> : null}
      <SessionGroupSignal group={group} />
    </>
  );
}

function SessionGroupSignal({ group }: { group: SessionListGroupSpec }) {
  if (group.needsYou > 0) {
    return (
      <StatusDot
        tone="brand"
        size="sm"
        label={`${group.needsYou} need${group.needsYou === 1 ? "s" : ""} you`}
        data-slot="session-group-needs-you"
      />
    );
  }
  if (group.working > 0) {
    return (
      <StatusDot
        tone="success"
        size="sm"
        label={workspaceWorkingLabel(group.working)}
        data-slot="session-group-working"
      />
    );
  }
  return null;
}

/** A group's identity: the project badge, or the profile owner tag. */
export function SessionListGroupIdentity({ group }: { group: SessionListGroupSpec }) {
  if (group.badge) {
    return (
      <WorkspaceBadge
        identity={group.badge}
        variant="chrome"
        className={cn("font-medium", group.badge.kind !== "unknown" && "text-fg")}
      />
    );
  }
  if (group.owner) return <ProfileOwnerTag owner={group.owner} />;
  return null;
}

export interface SessionListGroupHeaderProps {
  group: SessionListGroupSpec;
  collapsed: boolean;
  onToggle: () => void;
  onNewSession: (options?: SessionCreateOpenOptions) => void;
  testIdPrefix: string;
  /** Drag source and reorder target among project folders (issue #1044). */
  arrange?: SessionListGroupArrange;
}

/** How a project header takes part in project folders: drag, drop edge, and menu items. */
export interface SessionListGroupArrange {
  drag: SessionDragHandlers;
  dropEdge: SessionDropPosition | null;
  dragging: boolean;
  /** Folder verbs shown first in the header's context menu. */
  menuItems?: ReactNode;
}

function groupSessionOptions(target: ReturnType<typeof sessionListGroupTarget>) {
  return {
    ...(target.workspaceId ? { workspaceId: target.workspaceId } : {}),
    ...(target.profile ? { profile: target.profile } : {}),
  };
}

function groupStaysOn(
  group: SessionListGroupSpec,
  workspaceId: string | null | undefined,
  name: string
) {
  return group.kind === "project" && workspaceId ? { place: workspaceId, name } : undefined;
}

function GroupDropEdge({ edge }: { edge: SessionDropPosition }) {
  return (
    <span
      aria-hidden="true"
      className={cn(
        "pointer-events-none absolute inset-x-1 z-10 h-0.5 rounded-full bg-brand",
        edge === "before" ? "-top-px" : "-bottom-px"
      )}
    />
  );
}

/** Where a new folder lands, with a picker for each axis the group leaves open. */
function GroupFolderDestination({
  folder,
  name,
  testId,
}: {
  folder: ReturnType<typeof useSessionListGroupFolderCreate>;
  name: string;
  testId: string;
}) {
  if (!folder.project && !folder.profile) return undefined;
  return (
    <p
      className="flex flex-wrap items-baseline gap-x-1 text-small-body text-muted-foreground"
      data-testid={`${testId}-destination`}
    >
      In
      {folder.project ? (
        <FolderAxisPicker axis="Project" testId={`${testId}-project`} {...folder.project} />
      ) : (
        <span className="font-medium text-fg">{name}</span>
      )}
      {folder.profile ? (
        <>
          <span aria-hidden="true">·</span>
          <FolderAxisPicker axis="Profile" testId={`${testId}-profile`} {...folder.profile} />
        </>
      ) : null}
    </p>
  );
}

/**
 * One group's header row: the collapse toggle (identity, collapsed signal,
 * count) and, after it, the group's own `+` — New session or New folder in
 * the project and profile the group stands for. The `+` is a sibling of the
 * toggle, never inside it, so each keeps its own target and name.
 */
export function SessionListGroupHeader({
  group,
  collapsed,
  onToggle,
  onNewSession,
  testIdPrefix,
  arrange,
}: SessionListGroupHeaderProps) {
  const target = sessionListGroupTarget(group);
  const folder = useSessionListGroupFolderCreate(target);
  const testId = `${testIdPrefix}-group-${group.key}`;
  const name = sessionListGroupName(group);
  const staysOn = groupStaysOn(group, target.workspaceId, name);
  const createMenu = {
    variant: "quiet",
    size: "icon-xs",
    className: "text-subtle",
    label: { session: `New session in ${name}`, menu: `New session or folder in ${name}` },
    onNewSession: () => onNewSession(groupSessionOptions(target)),
    onNewFolder: folder.openDialog,
    testId,
  } as const;
  return (
    <>
      <SessionListGroupProjectMenu group={group} leadingItems={arrange?.menuItems}>
        <div
          {...arrange?.drag}
          className={cn(
            "relative flex min-h-7 min-w-0 items-center gap-1 rounded-sm",
            arrange?.dragging && "opacity-50"
          )}
          data-testid={`${testId}-header`}
          data-drop={arrange?.dropEdge ?? undefined}
          data-dragging={arrange?.dragging || undefined}
          onKeyDown={openContextMenuFromKeyboard}
        >
          {arrange?.dropEdge ? <GroupDropEdge edge={arrange.dropEdge} /> : null}
          <SessionListGroupToggle
            collapsed={collapsed}
            count={group.total}
            identity={<SessionListGroupIdentity group={group} />}
            onToggle={onToggle}
            signal={<GroupSignal group={group} />}
            testId={testId}
          />
          {staysOn ? (
            <StaysOnCreateMenu {...createMenu} staysOn={staysOn} />
          ) : (
            <SessionListCreateMenu {...createMenu} />
          )}
        </div>
      </SessionListGroupProjectMenu>
      <SessionFolderDialogs
        state={folder.open ? { kind: "create" } : null}
        onClose={folder.close}
        onCreate={folder.create}
        onRename={() => Promise.resolve()}
        onDelete={() => Promise.resolve()}
        destination={
          <GroupFolderDestination folder={folder} name={name} testId={`${testId}-folder`} />
        }
      />
    </>
  );
}

function FolderAxisPicker({
  axis,
  testId,
  options,
  value,
  onChange,
}: SessionGroupFolderAxis & { axis: string; testId: string }) {
  return (
    <CreateDestinationPicker
      axis={axis}
      options={options}
      value={value}
      onChange={onChange}
      testId={testId}
    />
  );
}

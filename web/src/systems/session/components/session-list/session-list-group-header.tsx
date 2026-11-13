import { useState } from "react";

import { StatusDot, cn } from "@compozy/ui";

import { openContextMenuFromKeyboard } from "@/systems/os";
import { ProfileOwnerTag } from "@/systems/profiles";
import { WorkspaceBadge, type ProjectMenuMove } from "@/systems/workspace";

import { useSessionListGroupFolderCreate } from "../../hooks/use-session-list-group-folder-create";
import type { SessionDragHandlers, SessionDropPosition } from "../../hooks/use-session-list-dnd";
import type { SessionCreateOpenOptions } from "../../lib/session-create-target";
import {
  sessionListGroupName,
  sessionListGroupSignalLabel,
  sessionListGroupTarget,
  type SessionListGroupSpec,
} from "../../lib/session-list-groups";
import { AlwaysOnCreateMenu } from "../always-on/always-on-create-menu";
import { AlwaysOnCollapsedSignal } from "../always-on/always-on-signals";
import { SessionListCreateMenu } from "./session-list-create-menu";
import { SessionListGroupFolderDialog } from "./session-list-group-folder-dialog";
import { SessionListGroupProjectMenu } from "./session-list-group-project-menu";
import { SessionListGroupToggle } from "./session-list-group-toggle";

/** Collapsed header signal: needs-you outranks working; an open group shows its rows instead. */
export function SessionListGroupSignal({ group }: { group: SessionListGroupSpec }) {
  const scope = group.kind === "project" ? sessionListGroupTarget(group).workspaceId : null;
  return (
    <>
      {scope ? <AlwaysOnCollapsedSignal workspaceId={scope} /> : null}
      <SessionGroupSignal group={group} />
    </>
  );
}

function SessionGroupSignal({ group }: { group: SessionListGroupSpec }) {
  const label = sessionListGroupSignalLabel(group);
  if (label === null) return null;
  if (group.needsYou > 0) {
    return <StatusDot tone="attn" size="sm" label={label} data-slot="session-group-needs-you" />;
  }
  return <StatusDot tone="success" size="sm" label={label} data-slot="session-group-working" />;
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

/** How a project header takes part in project folders: drag, drop edge, and its Move ▸ verbs. */
export interface SessionListGroupArrange {
  drag: SessionDragHandlers;
  dropEdge: SessionDropPosition | null;
  dragging: boolean;
  /** The header menu's Move ▸: folder choices and manual order. */
  move?: ProjectMenuMove;
}

function groupSessionOptions(target: ReturnType<typeof sessionListGroupTarget>) {
  return {
    ...(target.workspaceId ? { workspaceId: target.workspaceId } : {}),
    ...(target.profile ? { profile: target.profile } : {}),
  };
}

function groupAlwaysOn(
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
  const alwaysOn = groupAlwaysOn(group, target.workspaceId, name);
  const [addingAlwaysOn, setAddingAlwaysOn] = useState(false);
  const newSession = () => onNewSession(groupSessionOptions(target));
  const createMenu = {
    variant: "quiet",
    size: "icon-xs",
    className: "text-subtle",
    label: { session: `New session in ${name}`, menu: `New session or folder in ${name}` },
    onNewSession: newSession,
    onNewFolder: folder.openDialog,
    testId,
  } as const;
  return (
    <>
      <SessionListGroupProjectMenu
        group={group}
        // The `+`'s verbs, so right-click starts what the `+` starts (the picker opens over it).
        verbs={{
          newSession,
          newFolder: folder.openDialog,
          addAlwaysOn: alwaysOn ? () => setAddingAlwaysOn(true) : undefined,
          move: arrange?.move,
        }}
        testId={`${testId}-menu`}
      >
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
            signal={<SessionListGroupSignal group={group} />}
            testId={testId}
          />
          {alwaysOn ? (
            <AlwaysOnCreateMenu
              {...createMenu}
              alwaysOn={alwaysOn}
              adding={addingAlwaysOn}
              onAddingChange={setAddingAlwaysOn}
            />
          ) : (
            <SessionListCreateMenu {...createMenu} />
          )}
        </div>
      </SessionListGroupProjectMenu>
      <SessionListGroupFolderDialog folder={folder} name={name} testId={`${testId}-folder`} />
    </>
  );
}

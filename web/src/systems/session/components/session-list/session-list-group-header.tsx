import { useState } from "react";

import { StatusDot, cn } from "@compozy/ui";

import { isRemoteComputerId } from "@/lib/computer-scope";
import { ComputerMark, ComputerTag } from "@/systems/computers";
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

/**
 * Collapsed header signal: needs-you outranks working; an open group shows its rows
 * instead. The sidebar header passes `states={false}`: its leading column carries
 * needs-you and working (#1926), so the right keeps the always-on signal alone.
 */
export function SessionListGroupSignal({
  group,
  states = true,
}: {
  group: SessionListGroupSpec;
  states?: boolean;
}) {
  const scope = group.kind === "project" ? sessionListGroupTarget(group).workspaceId : null;
  return (
    <>
      {scope ? <AlwaysOnCollapsedSignal computerId={group.computerId} workspaceId={scope} /> : null}
      {states ? <SessionGroupSignal group={group} /> : null}
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

/** A group's identity: the project badge (with optional Together computer name), or the profile owner tag. */
export function SessionListGroupIdentity({ group }: { group: SessionListGroupSpec }) {
  if (group.badge) {
    const badge = (
      <WorkspaceBadge
        identity={group.badge}
        variant="chrome"
        className={cn("font-medium", group.badge.kind !== "unknown" && "text-fg")}
      />
    );
    if (group.computerName) {
      return (
        <span className="flex min-w-0 items-center gap-1.5">
          {badge}
          <ComputerTag bare name={group.computerName} />
        </span>
      );
    }
    return badge;
  }
  if (group.owner) {
    const tag = <ProfileOwnerTag owner={group.owner} />;
    if (group.computerName) {
      return (
        <span className="flex min-w-0 items-center gap-1.5">
          {tag}
          <ComputerTag bare name={group.computerName} />
        </span>
      );
    }
    return tag;
  }
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
  /** Suppresses create/folder controls (offline or lacking `sessions:operate`). */
  readOnly?: boolean;
}

/** How a project header takes part in project folders: drag, drop edge, and its Move ▸ verbs. */
export interface SessionListGroupArrange {
  drag: SessionDragHandlers;
  dropEdge: SessionDropPosition | null;
  dragging: boolean;
  /** The header menu's Move ▸: folder choices and manual order. */
  move?: ProjectMenuMove;
}

function groupSessionOptions(
  target: ReturnType<typeof sessionListGroupTarget>,
  group: Pick<SessionListGroupSpec, "computerId" | "computerName">
): SessionCreateOpenOptions {
  const hasComputer = isRemoteComputerId(target.computerId) || group.computerName !== undefined;
  return {
    ...(target.workspaceId ? { workspaceId: target.workspaceId } : {}),
    ...(target.profile ? { profile: target.profile } : {}),
    ...(target.computerId ? { computerId: target.computerId } : {}),
    ...(hasComputer ? { fixedComputer: true } : {}),
  };
}

function groupAlwaysOn(
  group: SessionListGroupSpec,
  workspaceId: string | null | undefined,
  name: string
) {
  if (isRemoteComputerId(group.computerId)) return undefined;
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

function GroupHeaderToggle({
  group,
  collapsed,
  onToggle,
  testId,
}: Pick<SessionListGroupHeaderProps, "group" | "collapsed" | "onToggle"> & {
  testId: string;
}) {
  const mark =
    group.computerName && group.computerPlatform && group.computerIconKind ? (
      <ComputerMark iconKind={group.computerIconKind} platform={group.computerPlatform} />
    ) : undefined;
  return (
    <SessionListGroupToggle
      collapsed={collapsed}
      count={group.total}
      identity={<SessionListGroupIdentity group={group} />}
      mark={mark}
      onToggle={onToggle}
      signal={<SessionListGroupSignal group={group} states={false} />}
      leading={{ needsYou: group.needsYou, working: group.working, slot: "session-group" }}
      testId={testId}
    />
  );
}

function groupHeaderDragProps(arrange: SessionListGroupArrange | undefined, offline?: boolean) {
  return {
    ...arrange?.drag,
    className: cn(
      "relative flex min-h-7 min-w-0 items-center gap-1 rounded-sm",
      arrange?.dragging && "opacity-50",
      offline && "opacity-70"
    ),
    "data-drop": arrange?.dropEdge ?? undefined,
    "data-dragging": arrange?.dragging || undefined,
  };
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
  readOnly = false,
}: SessionListGroupHeaderProps) {
  const target = sessionListGroupTarget(group);
  const folder = useSessionListGroupFolderCreate(target);
  const testId = `${testIdPrefix}-group-${group.key}`;
  const name = sessionListGroupName(group);
  const where = group.computerName ? `${name} on ${group.computerName}` : name;
  const muted = readOnly || group.offline === true;
  const alwaysOn = muted ? undefined : groupAlwaysOn(group, target.workspaceId, name);
  const [addingAlwaysOn, setAddingAlwaysOn] = useState(false);
  const newSession = () => onNewSession(groupSessionOptions(target, group));
  const createMenu = {
    variant: "quiet",
    size: "icon-xs",
    className: "text-subtle",
    label: { session: `New session in ${where}`, menu: `New session or folder in ${where}` },
    onNewSession: newSession,
    onNewFolder: folder.openDialog,
    testId,
  } as const;
  return (
    <>
      <SessionListGroupProjectMenu
        group={group}
        // The `+`'s verbs, so right-click starts what the `+` starts (the picker opens over it).
        verbs={
          muted
            ? undefined
            : {
                newSession,
                newFolder: folder.openDialog,
                addAlwaysOn: alwaysOn ? () => setAddingAlwaysOn(true) : undefined,
                move: arrange?.move,
              }
        }
        testId={`${testId}-menu`}
      >
        <div
          {...groupHeaderDragProps(arrange, group.offline)}
          data-testid={`${testId}-header`}
          onKeyDown={openContextMenuFromKeyboard}
        >
          {arrange?.dropEdge ? <GroupDropEdge edge={arrange.dropEdge} /> : null}
          <GroupHeaderToggle
            group={group}
            collapsed={collapsed}
            onToggle={onToggle}
            testId={testId}
          />
          {muted ? null : alwaysOn ? (
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
      {muted ? null : (
        <SessionListGroupFolderDialog folder={folder} name={where} testId={`${testId}-folder`} />
      )}
    </>
  );
}

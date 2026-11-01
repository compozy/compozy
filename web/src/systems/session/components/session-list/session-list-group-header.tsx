import { ChevronRight } from "@compozy/ui/icons";
import type { ReactNode } from "react";

import { Icon, StatusDot, cn } from "@compozy/ui";

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
import { SessionFolderDialogs } from "./session-folder-dialogs";
import { SessionListCreateMenu } from "./session-list-create-menu";
import { SessionListGroupProjectMenu } from "./session-list-group-project-menu";

/** Collapsed header signal: needs-you outranks working; an open group shows its rows instead. */
export function SessionListGroupSignal({ group }: { group: SessionListGroupSpec }) {
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
          {arrange?.dropEdge ? (
            <span
              aria-hidden="true"
              className={cn(
                "pointer-events-none absolute inset-x-1 z-10 h-0.5 rounded-full bg-brand",
                arrange.dropEdge === "before" ? "-top-px" : "-bottom-px"
              )}
            />
          ) : null}
          <button
            type="button"
            aria-expanded={!collapsed}
            data-testid={testId}
            className="flex min-h-7 min-w-0 flex-1 items-center gap-2 rounded-sm px-2 py-1 text-left transition-colors hover:bg-surface-2 focus-visible:shadow-focus-ring focus-visible:outline-none"
            onClick={onToggle}
          >
            <Icon
              as={ChevronRight}
              size="sm"
              className={cn("shrink-0 text-faint transition-transform", !collapsed && "rotate-90")}
            />
            <span className="flex min-w-0 flex-1 items-center">
              <SessionListGroupIdentity group={group} />
            </span>
            {collapsed ? <SessionListGroupSignal group={group} /> : null}
            <span className="font-mono text-micro text-faint">{group.total}</span>
          </button>
          <SessionListCreateMenu
            variant="quiet"
            size="icon-xs"
            className="text-subtle"
            label={{ session: `New session in ${name}`, menu: `New session or folder in ${name}` }}
            onNewSession={() =>
              onNewSession({
                ...(target.workspaceId ? { workspaceId: target.workspaceId } : {}),
                ...(target.profile ? { profile: target.profile } : {}),
              })
            }
            onNewFolder={folder.openDialog}
            testId={testId}
          />
        </div>
      </SessionListGroupProjectMenu>
      <SessionFolderDialogs
        state={folder.open ? { kind: "create" } : null}
        onClose={folder.close}
        onCreate={folder.create}
        onRename={() => Promise.resolve()}
        onDelete={() => Promise.resolve()}
        destination={
          folder.project || folder.profile ? (
            <p
              className="flex flex-wrap items-baseline gap-x-1 text-small-body text-muted-foreground"
              data-testid={`${testId}-folder-destination`}
            >
              In
              {folder.project ? (
                <FolderAxisPicker
                  axis="Project"
                  testId={`${testId}-folder-project`}
                  {...folder.project}
                />
              ) : (
                <span className="font-medium text-fg">{name}</span>
              )}
              {folder.profile ? (
                <>
                  <span aria-hidden="true">·</span>
                  <FolderAxisPicker
                    axis="Profile"
                    testId={`${testId}-folder-profile`}
                    {...folder.profile}
                  />
                </>
              ) : null}
            </p>
          ) : undefined
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

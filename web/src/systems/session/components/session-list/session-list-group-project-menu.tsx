import type * as React from "react";

import { WorkspaceProjectContextMenu, useWorkspaces } from "@/systems/workspace";

import type { SessionListGroupSpec } from "../../lib/session-list-groups";

/**
 * A project group's header takes its project's context menu, the same actions
 * and confirms as every other project surface; the header owns Shift+F10.
 * Profile groups, the User group, and a project the catalog does not list keep
 * an inert trigger, so the header never remounts when the catalog answers.
 */
export function SessionListGroupProjectMenu({
  group,
  children,
}: {
  group: Pick<SessionListGroupSpec, "kind" | "scope">;
  /** The header element; it becomes the right-click and Shift+F10 target. */
  children: React.ReactElement<React.HTMLAttributes<HTMLElement>>;
}) {
  const workspaces = useWorkspaces();
  const { scope } = group;
  const workspace =
    group.kind === "project" && scope.kind === "project"
      ? workspaces.data?.find(candidate => candidate.id === scope.workspaceId)
      : undefined;
  return (
    <WorkspaceProjectContextMenu workspace={workspace}>{children}</WorkspaceProjectContextMenu>
  );
}

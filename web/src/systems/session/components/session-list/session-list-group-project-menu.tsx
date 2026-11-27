import type * as React from "react";

import { isRemoteComputerId } from "@/lib/computer-scope";
import { useOpenFilesApp } from "@/systems/os";
import {
  WorkspaceProjectContextMenu,
  useWorkspaces,
  type ProjectMenuVerbs,
} from "@/systems/workspace";

import type { SessionListGroupSpec } from "../../lib/session-list-groups";

/**
 * A project group's header takes the project menu every project surface opens:
 * New ▸, Open files, Move ▸, Archive, Remove. The header owns Shift+F10. A host
 * that holds the `+` passes its verbs; the shell supplies the rest. Profile groups,
 * the User group, remote project headers, and a project the catalog does not list
 * omit local-only project actions (Open files, Archive, Remove) while keeping
 * applicable surface verbs, so the header never remounts when the catalog answers.
 */
export function SessionListGroupProjectMenu({
  group,
  verbs,
  testId,
  children,
}: {
  group: Pick<SessionListGroupSpec, "kind" | "scope" | "computerId">;
  /** The header's own verbs (its `+` and project folders); they win over the shell's. */
  verbs?: ProjectMenuVerbs;
  /** Test id prefix of the menu's rows. */
  testId?: string;
  /** The header element; it becomes the right-click and Shift+F10 target. */
  children: React.ReactElement<React.HTMLAttributes<HTMLElement>>;
}) {
  const remote = isRemoteComputerId(group.computerId);
  const workspaces = useWorkspaces({ enabled: !remote });
  const openFiles = useOpenFilesApp();
  const { scope } = group;
  const workspace =
    !remote && group.kind === "project" && scope.kind === "project"
      ? workspaces.data?.find(candidate => candidate.id === scope.workspaceId)
      : undefined;
  // Remote headers keep only the surface's own verbs (New session, New folder, Move).
  // Without a local project behind the header only its place among project folders remains.
  const own: ProjectMenuVerbs | undefined = remote
    ? verbs
    : workspace
      ? { ...verbs, openFiles: openFiles ? () => openFiles(workspace.id) : undefined }
      : verbs?.move
        ? { move: verbs.move }
        : undefined;
  return (
    <WorkspaceProjectContextMenu workspace={workspace} verbs={own} testId={testId}>
      {children}
    </WorkspaceProjectContextMenu>
  );
}

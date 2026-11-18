import type * as React from "react";

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
 * the User group, and a project the catalog does not list keep an inert trigger
 * (the User group keeps only its Move ▸ in project folders), so the header never
 * remounts when the catalog answers.
 */
export function SessionListGroupProjectMenu({
  group,
  verbs,
  testId,
  children,
}: {
  group: Pick<SessionListGroupSpec, "kind" | "scope">;
  /** The header's own verbs (its `+` and project folders); they win over the shell's. */
  verbs?: ProjectMenuVerbs;
  /** Test id prefix of the menu's rows. */
  testId?: string;
  /** The header element; it becomes the right-click and Shift+F10 target. */
  children: React.ReactElement<React.HTMLAttributes<HTMLElement>>;
}) {
  const workspaces = useWorkspaces();
  const openFiles = useOpenFilesApp();
  const { scope } = group;
  const workspace =
    group.kind === "project" && scope.kind === "project"
      ? workspaces.data?.find(candidate => candidate.id === scope.workspaceId)
      : undefined;
  // Without a project behind the header only its place among project folders remains.
  const own: ProjectMenuVerbs | undefined = workspace
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

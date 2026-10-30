import { useState } from "react";

import type { WorkspacePayload, WorkspaceProjectMenuPoint } from "@/systems/workspace";

/**
 * The project menu a workspace-menu row relays. It stays mounted after the
 * menu closes so the confirm dialog it opened survives, and it reads the
 * project's current payload while the catalog still lists it. Once the
 * project leaves the catalog it keeps the last payload only so a running
 * mutation's confirm can settle. Each relay is a new request, so a confirm
 * chosen from the menubar submenu opens even after an earlier relay of the
 * same project.
 */
export function useProjectPointMenu(workspaces: readonly WorkspacePayload[]) {
  const [target, setTarget] = useState<{
    workspace: WorkspacePayload;
    point: WorkspaceProjectMenuPoint | null;
    /** The submenu chose Archive project: the confirm opens without a menu. */
    archive: boolean;
    request: number;
  } | null>(null);
  const live = target
    ? workspaces.find(candidate => candidate.id === target.workspace.id)
    : undefined;
  // A project that leaves the catalog closes its menu for good: a later return
  // must not reopen it at the old pointer. The payload stays for a running confirm.
  if (target?.point && !live) setTarget({ ...target, point: null });
  const relay = (
    workspace: WorkspacePayload,
    point: WorkspaceProjectMenuPoint | null,
    archive: boolean
  ) => setTarget(current => ({ workspace, point, archive, request: (current?.request ?? 0) + 1 }));
  return {
    workspace: live ?? target?.workspace ?? null,
    /** Whether the catalog still lists the project; an unlisted one offers no actions. */
    listed: live !== undefined,
    point: target?.point ?? null,
    /** Mount key: a new relay starts a fresh flow. */
    key: target ? `${target.workspace.id}:${target.request}` : null,
    initialAction: target?.archive ? ("archive" as const) : null,
    open: (next: WorkspacePayload, point: WorkspaceProjectMenuPoint) => relay(next, point, false),
    archive: (next: WorkspacePayload) => relay(next, null, true),
    close: () => setTarget(current => (current ? { ...current, point: null } : null)),
  };
}

import { useState } from "react";

import { WorkspaceApiError } from "../adapters/workspace-api";
import type { WorkspacePayload } from "../types";
import { useDeleteWorkspace, useSetWorkspaceArchived } from "./use-workspaces";

export type WorkspaceProjectAction = "archive" | "remove";

/**
 * One project's archive/remove flow: the chosen action, its confirm, and the
 * refusal it reports. Every project menu, wherever it opens, chooses here. A
 * surface whose project is not known yet holds the flow idle, and a project
 * that leaves the catalog (archived or removed elsewhere) drops an open
 * confirm unless this flow's own mutation is still running. A failure stays on
 * screen while the surface still holds the project's payload to show it.
 */
export function useWorkspaceProjectAction(
  workspace: WorkspacePayload | undefined,
  listedInCatalog?: boolean,
  /** An action chosen before the flow mounted (a menu item that closed with its menu). */
  initialAction: WorkspaceProjectAction | null = null
) {
  const listed = listedInCatalog ?? Boolean(workspace);
  const [action, setAction] = useState<WorkspaceProjectAction | null>(initialAction);
  const archive = useSetWorkspaceArchived();
  const remove = useDeleteWorkspace();
  const pending = archive.isPending || remove.isPending;
  const failure = action === "remove" ? remove.error : archive.error;
  const archived = Boolean(workspace?.archived_at);
  if (!listed && action !== null && !pending && (!failure || !workspace)) setAction(null);
  return {
    action,
    listed,
    archived,
    pending,
    failure,
    blockers: failure instanceof WorkspaceApiError ? failure.blockers : [],
    choose: (next: WorkspaceProjectAction) => {
      archive.reset();
      remove.reset();
      setAction(next);
    },
    dismiss: () => {
      if (!pending) setAction(null);
    },
    submit: () => {
      if (!workspace || !listed) return;
      const onSuccess = () => setAction(null);
      if (action === "remove") remove.mutate(workspace.id, { onSuccess });
      else archive.mutate({ id: workspace.id, archived: !archived }, { onSuccess });
    },
  };
}

export type WorkspaceProjectActionFlow = ReturnType<typeof useWorkspaceProjectAction>;

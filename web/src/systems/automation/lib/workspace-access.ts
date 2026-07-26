import type { AutomationJob, AutomationTrigger } from "../types";

type WorkspaceBoundAutomation = Pick<AutomationJob | AutomationTrigger, "scope" | "workspace_id">;

export function automationMatchesActiveWorkspace(
  item: WorkspaceBoundAutomation,
  activeWorkspaceId: string | null | undefined
): boolean {
  return (
    item.scope === "global" ||
    (typeof activeWorkspaceId === "string" &&
      activeWorkspaceId !== "" &&
      item.workspace_id === activeWorkspaceId)
  );
}

export function automationWorkspaceAccessError(
  kind: "job" | "trigger",
  item: WorkspaceBoundAutomation | null | undefined,
  activeWorkspaceId: string | null | undefined,
  workspaceLoading: boolean
): Error | null {
  if (workspaceLoading || !item || automationMatchesActiveWorkspace(item, activeWorkspaceId)) {
    return null;
  }
  return new Error(`This workspace-scoped ${kind} belongs to another workspace.`);
}

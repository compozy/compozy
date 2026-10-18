import type { AutomationJob, AutomationTrigger } from "../types";

type WorkspaceBoundAutomation = Pick<AutomationJob | AutomationTrigger, "scope" | "workspace_id">;

/** Editing follows the definition's owner even when it is inspected from Global. */
export function automationEditorWorkspaceId(
  item: WorkspaceBoundAutomation | undefined,
  activeWorkspaceId: string | null | undefined
) {
  return item?.scope === "workspace" ? item.workspace_id : activeWorkspaceId;
}

export function automationMatchesActiveWorkspace(
  item: WorkspaceBoundAutomation,
  activeWorkspaceId: string | null | undefined
): boolean {
  // null is the resolved Global lens; undefined is still an unresolved project.
  return (
    activeWorkspaceId === null ||
    item.scope === "global" ||
    (typeof activeWorkspaceId === "string" &&
      activeWorkspaceId !== "" &&
      item.workspace_id === activeWorkspaceId)
  );
}

export const AUTOMATION_ELSEWHERE_MESSAGE =
  "This automation belongs to another project. Switch to it to open this page.";

/** A definition owned by another project than the selected one; null when it is readable here. */
export function automationWorkspaceAccessError(
  item: WorkspaceBoundAutomation | null | undefined,
  activeWorkspaceId: string | null | undefined,
  workspaceLoading: boolean
): Error | null {
  if (workspaceLoading || !item || automationMatchesActiveWorkspace(item, activeWorkspaceId)) {
    return null;
  }
  return new Error(AUTOMATION_ELSEWHERE_MESSAGE);
}

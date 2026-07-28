import type { WorkspaceCommandSelectOption } from "../components/workspace-command-select";
import type { WorkspacePayload } from "../types";

export function toWorkspaceCommandSelectOptions(
  workspaces: ReadonlyArray<Pick<WorkspacePayload, "id" | "name" | "root_dir">>
): WorkspaceCommandSelectOption[] {
  return workspaces.map(workspace => ({
    id: workspace.id,
    name: workspace.name,
    root_dir: workspace.root_dir,
  }));
}

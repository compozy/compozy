import type { DataScope } from "@/systems/workspace";

import type { AutomationSuggestionsSource } from "../types";

/**
 * Suggestions fill an empty, unfiltered listing of one profile: a project's own,
 * or every project's under All projects. User has none (they are per project).
 */
export function automationSuggestionsSource(
  dataScope: DataScope | null,
  eligible: boolean
): AutomationSuggestionsSource | null {
  if (!eligible || dataScope === null || dataScope.kind === "user") return null;
  return dataScope.kind === "all"
    ? { kind: "all", workspaceIDs: [] }
    : { kind: "workspace", workspaceID: dataScope.workspaceId };
}

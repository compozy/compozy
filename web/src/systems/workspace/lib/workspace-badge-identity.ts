import { ownerDataScope } from "./destination";
import { workspaceMonogram } from "./active-workspace";
import { DESTINATION_COPY } from "./workspace-scope-copy";

export interface WorkspaceBadgeIdentity {
  /** `unknown`: an item whose project could not be established (never shown as User). */
  kind: "project" | "user" | "unknown";
  name: string;
  monogram: string;
}

/**
 * The project an item belongs to, from its own `workspace_id` (absent or empty
 * is User) and the registered catalog. An unregistered id shows the raw id.
 */
export function workspaceBadgeIdentity(
  workspaceId: string | null | undefined,
  names: ReadonlyMap<string, string>
): WorkspaceBadgeIdentity {
  const scope = ownerDataScope(workspaceId);
  if (scope.kind === "user") {
    return { kind: "user", name: DESTINATION_COPY.user, monogram: DESTINATION_COPY.userMonogram };
  }
  const name = names.get(scope.workspaceId) ?? scope.workspaceId;
  return { kind: "project", name, monogram: workspaceMonogram(name) };
}

/** The badge of an item whose project could not be established: muted `?`, never "User". */
export function unknownProjectIdentity(): WorkspaceBadgeIdentity {
  return {
    kind: "unknown",
    name: DESTINATION_COPY.unknownProject,
    monogram: DESTINATION_COPY.unknownProjectMonogram,
  };
}

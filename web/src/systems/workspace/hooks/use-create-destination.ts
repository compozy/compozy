import { useActiveWorkspace } from "./use-active-workspace";
import type { DataScope } from "../lib/destination";
import { DESTINATION_COPY } from "../lib/workspace-scope-copy";

export interface CreateDestination {
  /** Where new work lands; `null` while resolution is unsettled. */
  dataScope: DataScope | null;
  /** True while scope resolution is unsettled — withhold submits that bind a destination. */
  pending: boolean;
  workspaceId: string | null;
  workspaceName: string | null;
  destinationLabel: string;
  runtimeWorkspaceId: string | null;
  /** Session root, or the operator home outside a project, for `kind="session"` statements. */
  sessionRoot: string;
}

function labelFor(dataScope: DataScope | null, workspaceName: string): string {
  if (dataScope?.kind === "all") return DESTINATION_COPY.allProjects;
  if (dataScope?.kind === "user") return DESTINATION_COPY.user;
  return workspaceName;
}

/** Derived create/install landing from the menubar-owned destination. */
export function useCreateDestination(): CreateDestination {
  const active = useActiveWorkspace();
  const project = active.dataScope?.kind === "project";
  const workspaceName = active.activeWorkspace?.name ?? "";

  return {
    dataScope: active.dataScope,
    pending: active.pending,
    workspaceId: project ? active.activeWorkspaceId : null,
    workspaceName: project ? workspaceName || null : null,
    destinationLabel: labelFor(active.dataScope, workspaceName),
    runtimeWorkspaceId: active.runtimeWorkspaceId,
    sessionRoot: project ? (active.activeWorkspace?.root_dir ?? "") : (active.userHomeDir ?? "~"),
  };
}

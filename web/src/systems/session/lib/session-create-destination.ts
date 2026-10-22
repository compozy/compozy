import {
  destinationLabel,
  type DataScope,
  type DestinationKind,
  type WorkspacePayload,
} from "@/systems/workspace";

import { sessionCreateBinding } from "./session-create-binding";

interface ResolveSessionCreateDestinationOptions {
  activeWorkspace: WorkspacePayload | undefined;
  projectWorkspaceId?: string | null;
  /** The menubar destination; omitted follows `activeWorkspace`. */
  dataScope?: DataScope | null;
  userHomeDir?: string;
}

export function resolveSessionCreateDestination({
  activeWorkspace,
  projectWorkspaceId,
  dataScope,
  userHomeDir,
}: ResolveSessionCreateDestinationOptions) {
  const projectScoped =
    dataScope === undefined ? activeWorkspace !== undefined : dataScope?.kind === "project";
  const scope: DestinationKind = projectScoped ? "project" : "user";
  const runtimeWorkspaceId = activeWorkspace?.id ?? null;
  const binding = sessionCreateBinding({
    scope,
    projectWorkspaceId: projectScoped ? (projectWorkspaceId ?? runtimeWorkspaceId) : null,
  });
  return {
    binding,
    destinationLabel: destinationLabel(scope, activeWorkspace?.name),
    environmentWorkspaceId: projectScoped ? (projectWorkspaceId ?? runtimeWorkspaceId ?? "") : "",
    runtimeWorkspaceId,
    scope,
    sessionRoot: projectScoped ? (activeWorkspace?.root_dir ?? "") : (userHomeDir ?? "~"),
  };
}

import {
  GLOBAL_SCOPE_COPY,
  destinationLabel,
  type WorkspacePayload,
  type WorkspaceScopeMode,
} from "@/systems/workspace";

import { sessionCreateBinding } from "./session-create-binding";

interface ResolveSessionCreateDestinationOptions {
  activeWorkspace: WorkspacePayload | undefined;
  homeWorkspaceId?: string;
  projectWorkspaceId?: string | null;
  requestedScope?: WorkspaceScopeMode;
  userHomeDir?: string;
}

export function resolveSessionCreateDestination({
  activeWorkspace,
  homeWorkspaceId,
  projectWorkspaceId,
  requestedScope,
  userHomeDir,
}: ResolveSessionCreateDestinationOptions) {
  const scope: WorkspaceScopeMode = requestedScope ?? (activeWorkspace ? "workspace" : "global");
  const runtimeWorkspaceId = activeWorkspace?.id ?? null;
  const binding = sessionCreateBinding({
    scope,
    projectWorkspaceId:
      scope === "workspace"
        ? (projectWorkspaceId ?? runtimeWorkspaceId)
        : (projectWorkspaceId ?? null),
    homeWorkspaceId:
      homeWorkspaceId ?? (scope === "global" ? (runtimeWorkspaceId ?? undefined) : undefined),
    userHomeDir,
  });
  return {
    binding,
    destinationLabel: destinationLabel(
      scope,
      scope === "global" ? GLOBAL_SCOPE_COPY.chipLabel : activeWorkspace?.name
    ),
    environmentWorkspaceId:
      scope === "workspace" ? (projectWorkspaceId ?? runtimeWorkspaceId ?? "") : "",
    runtimeWorkspaceId,
    scope,
    sessionRoot: scope === "global" ? (userHomeDir ?? "~") : (activeWorkspace?.root_dir ?? ""),
  };
}

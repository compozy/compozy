"use client";

import { useQuery } from "@tanstack/react-query";

import { useProfileReadScope } from "@/systems/profiles";
import { terminalCatalogQuery, terminalScope, terminalsRunning } from "@/systems/terminal";
import { useActiveWorkspace } from "@/systems/workspace";

/**
 * Whether any terminal in this project is alive.
 *
 * The dock's live mark is catalog truth, not "a Terminal window is open".
 * An empty query or a project that is not ready is idle — never a guess
 * from window state.
 */
export function useTerminalDockRunning(): boolean {
  const workspace = useActiveWorkspace();
  const profile = useProfileReadScope();
  const workspaceId = workspace.runtimeWorkspaceId ?? "";
  const catalogScope = terminalScope(workspaceId, profile.destination, profile.aggregate);
  const catalog = useQuery({
    ...terminalCatalogQuery(catalogScope),
    enabled: workspaceId !== "",
  });
  if (workspaceId === "" || catalog.data === undefined) return false;
  return terminalsRunning(catalog.data);
}

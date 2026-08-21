import { useSelectedWorkspaceId, useWorkspaceScopeMode } from "@/systems/workspace";

import type { ProfileLens } from "../types";

/**
 * Which slot the remembered choice is read from and written to.
 *
 * The lens follows the shell: inside a project it is that project's slot, and
 * across projects it is the Global slot, which keeps its own memory (ADR-003).
 * A project that has not resolved yet falls back to Global rather than guessing
 * a workspace id.
 */
export function useProfileLens(): ProfileLens {
  const scope = useWorkspaceScopeMode();
  const workspaceId = useSelectedWorkspaceId();
  if (scope === "workspace" && workspaceId !== null && workspaceId !== "") {
    return { scope: "workspace", workspaceId };
  }
  return { scope: "global" };
}

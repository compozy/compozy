import { useAgents } from "@/systems/agent";
import type { WorkspaceSetupCollection, WorkspaceSetupDefaultsModel } from "@/systems/workspace";

/** Loads the agent catalog that seed the workspace setup dialog. */
export function useWorkspaceSetupDefaults(): WorkspaceSetupDefaultsModel {
  const agentsQuery = useAgents();

  return {
    agents: workspaceSetupCollection(
      agentsQuery.data,
      agentsQuery.isLoading,
      agentsQuery.error,
      "Could not load agents."
    ),
  };
}

function workspaceSetupCollection<T>(
  entries: T[] | undefined,
  isLoading: boolean,
  error: unknown,
  fallbackMessage: string
): WorkspaceSetupCollection<T> {
  if (isLoading) return { state: "loading" };
  if (error) {
    return {
      state: "error",
      message:
        error instanceof Error && error.message.trim() !== "" ? error.message : fallbackMessage,
    };
  }
  return { state: "ready", entries: entries ?? [] };
}

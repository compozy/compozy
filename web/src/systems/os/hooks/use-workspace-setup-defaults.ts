import { useAgents } from "@/systems/agent";
import { useSettingsSandboxes } from "@/systems/settings";
import type { WorkspaceSetupCollection, WorkspaceSetupDefaultsModel } from "@/systems/workspace";

/** Loads the two catalogs that seed the workspace setup dialog. */
export function useWorkspaceSetupDefaults(): WorkspaceSetupDefaultsModel {
  const agentsQuery = useAgents();
  const sandboxesQuery = useSettingsSandboxes();

  return {
    agents: workspaceSetupCollection(
      agentsQuery.data,
      agentsQuery.isLoading,
      agentsQuery.error,
      "Could not load agents."
    ),
    sandboxes: workspaceSetupCollection(
      sandboxesQuery.data?.sandboxes.map(entry => ({
        name: entry.name,
        backend: entry.profile.backend,
      })),
      sandboxesQuery.isLoading,
      sandboxesQuery.error,
      "Could not load sandbox profiles."
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

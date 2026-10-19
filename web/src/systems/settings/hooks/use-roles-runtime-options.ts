import {
  type SettingsRuntimeCatalog,
  useSettingsRuntimeCatalog,
} from "./use-settings-runtime-catalog";
import { type AgentPayload, useAgents } from "@/systems/agent";

export interface RolesRuntimeOptions extends SettingsRuntimeCatalog {
  agents: AgentPayload[];
}

/**
 * One catalog read for the whole Roles page. Every runtime selector on the page
 * — each role route plus each fallback entry — is fed from this single query
 * through props; no selector fetches on its own.
 */
export function useRolesRuntimeOptions(): RolesRuntimeOptions {
  const catalog = useSettingsRuntimeCatalog();
  const agentsQuery = useAgents();
  return {
    ...catalog,
    loading: catalog.loading || agentsQuery.isLoading,
    agents: agentsQuery.data ?? [],
  };
}

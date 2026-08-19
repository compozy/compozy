import { createContext, useContext, useEffect } from "react";

import { useAgents } from "@/systems/agent";
import { useRuntimeModelCatalog } from "@/systems/model-catalog";
import type { RuntimeProviderOption } from "@/systems/runtime";
import { useSessions } from "@/systems/session";
import { useSkills } from "@/systems/skill";
import { useVaultSecrets } from "@/systems/vault";
import { useWorkspace, useWorkspaces, useWorktrees } from "@/systems/workspace";

import type {
  LoopEntityCatalog,
  LoopEntityOption,
  LoopInputCatalogNeeds,
  LoopInputCatalogs,
} from "../lib/loop-input-catalogs";
import { useLoops } from "./use-loops";

const EMPTY_ENTITY_CATALOG: LoopEntityCatalog = {
  options: [],
  loading: false,
  error: null,
};

const EMPTY_CATALOGS: LoopInputCatalogs = {
  agents: [],
  agentLoading: false,
  agentError: null,
  entities: {
    skill: EMPTY_ENTITY_CATALOG,
    loop: EMPTY_ENTITY_CATALOG,
    worktree: EMPTY_ENTITY_CATALOG,
    session: EMPTY_ENTITY_CATALOG,
    workspace: EMPTY_ENTITY_CATALOG,
    secret: EMPTY_ENTITY_CATALOG,
  },
  worktrees: [],
  runtimeProviders: [],
  runtimeModels: [],
  runtimeLoading: false,
  runtimeError: null,
  refreshRuntime: () => undefined,
  refreshingRuntime: false,
};

export const LoopInputCatalogContext = createContext<LoopInputCatalogs>(EMPTY_CATALOGS);

function errorMessage(error: unknown, fallback: string): string | null {
  if (!error) return null;
  if (error instanceof Error && error.message.trim() !== "") return error.message;
  return fallback;
}

function catalog(
  options: readonly LoopEntityOption[],
  loading: boolean,
  error: unknown,
  fallback: string
): LoopEntityCatalog {
  return { options, loading, error: errorMessage(error, fallback) };
}

export function useLoopInputCatalogValue(
  workspaceId: string,
  needs: LoopInputCatalogNeeds
): LoopInputCatalogs {
  const agentsQuery = useAgents(workspaceId, { enabled: needs.entities.has("agent") });
  const skillsQuery = useSkills(workspaceId, needs.entities.has("skill"));
  const loopsQuery = useLoops(
    workspaceId,
    { limit: 100, sort: "name" },
    needs.entities.has("loop")
  );

  useEffect(() => {
    if (loopsQuery.hasNextPage && !loopsQuery.isFetchingNextPage) {
      void loopsQuery.fetchNextPage();
    }
  }, [loopsQuery.hasNextPage, loopsQuery.isFetchingNextPage, loopsQuery.fetchNextPage]);
  const worktreesQuery = useWorktrees(workspaceId, {
    enabled: needs.entities.has("worktree"),
  });
  const sessionsQuery = useSessions(workspaceId, {
    enabled: needs.entities.has("session"),
    filters: { limit: 100 },
    loadAll: true,
  });
  const workspacesQuery = useWorkspaces({ enabled: needs.entities.has("workspace") });
  const secretsQuery = useVaultSecrets({}, { enabled: needs.entities.has("secret") });
  const workspaceQuery = useWorkspace(workspaceId, { enabled: needs.runtime });

  const runtimeProviders: RuntimeProviderOption[] = (workspaceQuery.data?.providers ?? []).map(
    provider => ({
      id: provider.name,
      name: provider.display_name || provider.name,
      harness: provider.harness,
      runtime_provider: provider.runtime_provider,
    })
  );
  const runtimeCatalog = useRuntimeModelCatalog(runtimeProviders, {
    enabled: needs.runtime && runtimeProviders.length > 0,
  });
  const worktrees = worktreesQuery.data?.worktrees ?? [];

  return {
    agents: agentsQuery.data ?? [],
    agentLoading: agentsQuery.isLoading,
    agentError: errorMessage(agentsQuery.error, "Unable to load agents."),
    entities: {
      skill: catalog(
        (skillsQuery.data ?? []).map(skill => ({
          value: skill.name,
          label: skill.name,
          detail: skill.source,
        })),
        skillsQuery.isLoading,
        skillsQuery.error,
        "Unable to load skills."
      ),
      loop: catalog(
        loopsQuery.loops.map(loop => ({ value: loop.name, label: loop.name })),
        loopsQuery.isLoading,
        loopsQuery.error,
        "Unable to load Loops."
      ),
      worktree: catalog(
        worktrees.map(worktree => ({
          value: worktree.id,
          label: worktree.name,
          detail: worktree.branch,
        })),
        worktreesQuery.isLoading,
        worktreesQuery.error,
        "Unable to load worktrees."
      ),
      session: catalog(
        (sessionsQuery.data ?? []).map(session => ({
          value: session.id,
          label: session.id,
          detail: session.agent_name,
        })),
        sessionsQuery.isLoading,
        sessionsQuery.error,
        "Unable to load sessions."
      ),
      workspace: catalog(
        (workspacesQuery.data ?? []).map(workspace => ({
          value: workspace.id,
          label: workspace.name,
          detail: workspace.root_dir,
        })),
        workspacesQuery.isLoading,
        workspacesQuery.error,
        "Unable to load workspaces."
      ),
      secret: catalog(
        (secretsQuery.data ?? []).map(secret => ({
          value: secret.ref,
          label: secret.ref,
          detail: secret.namespace,
        })),
        secretsQuery.isLoading,
        secretsQuery.error,
        "Unable to load secret references."
      ),
    },
    worktrees,
    runtimeProviders,
    runtimeModels: runtimeCatalog.models,
    runtimeLoading: workspaceQuery.isLoading || runtimeCatalog.loading,
    runtimeError:
      errorMessage(workspaceQuery.error, "Unable to load runtime providers.") ??
      runtimeCatalog.error,
    refreshRuntime: runtimeCatalog.refresh,
    refreshingRuntime: runtimeCatalog.refreshing,
  };
}

export function useLoopInputCatalogs(): LoopInputCatalogs {
  return useContext(LoopInputCatalogContext);
}

import { useNavigate } from "@tanstack/react-router";

import type { AgentPayload } from "../types";
import { describeError, useAgentRuntimeSave } from "./use-agent-runtime-save";
import { useAgentWindowScope } from "./use-agent-window-scope";
import { type RuntimeCatalogProvider, useRuntimeModelCatalog } from "@/systems/model-catalog";
import { type RuntimeSpeed } from "@/lib/api-contract";
import type {
  RuntimeModelOption,
  RuntimeProviderOption,
  RuntimeSelectorValue,
} from "@/systems/runtime";
import { settingsProviderToOption, useSettingsProviders } from "@/systems/settings";
import { useWorkspace, workspaceProviderToOption } from "@/systems/workspace";

export interface UseAgentRuntimeEditorOptions {
  agent: AgentPayload | undefined;
}

export interface UseAgentRuntimeEditorResult {
  value: RuntimeSelectorValue;
  speed: RuntimeSpeed;
  /** The agent sets its own provider, model, Reasoning, Fast or options instead of the default. */
  overridden: boolean;
  onChange: (next: RuntimeSelectorValue, normalizedSpeed?: RuntimeSpeed) => void;
  onSpeedChange: (next: RuntimeSpeed) => void;
  /** Drops the agent's own runtime so it follows the default again. */
  onUseDefault: () => void;
  providerOptions: RuntimeProviderOption[];
  providersLoading: boolean;
  providerSourceError: string | null;
  onRetryProviderSource: () => void;
  runtimeModels: RuntimeModelOption[];
  modelCatalogLoading: boolean;
  modelCatalogRefreshing: boolean;
  modelCatalogError: string | null;
  onRefreshCatalog: () => void;
  onOpenProviderSettings: () => void;
  /** A return to the default is saving; the selector waits for it. */
  isPending: boolean;
  error: string | null;
  conflictMessage: string | null;
  onDismissError: () => void;
}

function useAgentRuntimeProviders(agent: AgentPayload | undefined, workspaceId: string | null) {
  const settingsProviders = useSettingsProviders();
  const workspaceDetail = useWorkspace(workspaceId ?? "", {
    enabled: workspaceId !== null,
  });
  const useWorkspaceProviders = agent?.origin === "workspace";
  const globalProviders = settingsProviders.data?.providers.map(settingsProviderToOption) ?? [];
  const workspaceProviders = (workspaceDetail.data?.providers ?? []).map(workspaceProviderToOption);
  const providerOptions = useWorkspaceProviders ? workspaceProviders : globalProviders;
  const providersLoading = useWorkspaceProviders
    ? workspaceId !== null && workspaceDetail.isLoading
    : settingsProviders.isLoading;
  const providerSourceError = useWorkspaceProviders
    ? workspaceDetail.error
      ? describeError("Couldn't load workspace providers", workspaceDetail.error)
      : null
    : settingsProviders.error
      ? describeError("Couldn't load providers", settingsProviders.error)
      : null;

  return {
    providerOptions,
    providersLoading,
    providerSourceError,
    onRetryProviderSource: () => {
      if (useWorkspaceProviders) {
        void workspaceDetail.refetch();
        return;
      }
      void settingsProviders.refetch();
    },
  };
}

/** Immediate Provider · Model · Reasoning editing for the agent's Model row. */
export function useAgentRuntimeEditor({
  agent,
}: UseAgentRuntimeEditorOptions): UseAgentRuntimeEditorResult {
  const navigate = useNavigate();
  const { projectId } = useAgentWindowScope();
  const runtime = useAgentRuntimeSave(agent);
  const providers = useAgentRuntimeProviders(agent, projectId);
  const catalogProviders: RuntimeCatalogProvider[] = providers.providerOptions.map(option => ({
    id: option.id,
    needsAuth: option.needs_auth,
  }));
  const catalog = useRuntimeModelCatalog(catalogProviders, { enabled: Boolean(agent) });

  return {
    ...runtime,
    ...providers,
    runtimeModels: catalog.models as RuntimeModelOption[],
    modelCatalogLoading: catalog.loading,
    modelCatalogRefreshing: catalog.refreshing,
    modelCatalogError: catalog.error,
    onRefreshCatalog: () => {
      void catalog.refresh();
    },
    onOpenProviderSettings: () => {
      void navigate({ to: "/settings/providers" });
    },
  };
}

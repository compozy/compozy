import { useState } from "react";
import { useNavigate } from "@tanstack/react-router";
import { useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";

import { useRuntimeModelCatalog, type RuntimeCatalogProvider } from "@/systems/model-catalog";
import type {
  RuntimeModelOption,
  RuntimeProviderOption,
  RuntimeSelectorValue,
} from "@/systems/runtime";
import { settingsProviderToOption, useSettingsProviders } from "@/systems/settings";
import type { SessionProviderOption } from "@/systems/workspace";
import { useWorkspace } from "@/systems/workspace";

import { isAgentDigestConflict } from "../adapters/agent-api";
import { resolveAgentRuntimeValue } from "../lib/agent-effective-runtime";
import { buildSettingsDraftFromAgent, buildUpdateAgentParams } from "../lib/agent-settings-draft";
import { agentKeys } from "../lib/query-keys";
import type { AgentPayload } from "../types";
import { useUpdateAgent } from "./use-agents";

function describeError(fallback: string, error: unknown): string {
  if (error instanceof Error && error.message.trim().length > 0) {
    return error.message;
  }
  return fallback;
}

function workspaceProviderToOption(provider: SessionProviderOption): RuntimeProviderOption {
  const displayName = provider.display_name?.trim();
  const harness = provider.harness?.trim();
  const runtimeProvider = provider.runtime_provider?.trim();
  return {
    id: provider.name,
    name: displayName || provider.name,
    ...(harness ? { harness } : {}),
    runtime_provider: runtimeProvider || provider.name,
  };
}

export interface UseAgentRuntimeEditorOptions {
  agent: AgentPayload | undefined;
  workspaceId: string | null;
}

export interface UseAgentRuntimeEditorResult {
  value: RuntimeSelectorValue;
  onChange: (next: RuntimeSelectorValue) => void;
  providerOptions: RuntimeProviderOption[];
  providersLoading: boolean;
  providerSourceError: string | null;
  onRetryProviderSource: () => void;
  runtimeModels: RuntimeModelOption[];
  modelCatalogLoading: boolean;
  modelCatalogLoaded: boolean;
  modelCatalogRefreshing: boolean;
  modelCatalogError: string | null;
  onRefreshCatalog: () => void;
  onOpenProviderSettings: () => void;
  isPending: boolean;
  error: string | null;
  conflictMessage: string | null;
  onDismissError: () => void;
}

/**
 * Immediate Provider · Model · Reasoning mutation for the agent detail header.
 * Keeps the closed trigger on server truth; pending disables without collapsing.
 */
export function useAgentRuntimeEditor({
  agent,
  workspaceId,
}: UseAgentRuntimeEditorOptions): UseAgentRuntimeEditorResult {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const updateAgent = useUpdateAgent();
  const settingsProviders = useSettingsProviders();
  const workspaceDetail = useWorkspace(workspaceId ?? "", {
    enabled: workspaceId !== null,
  });
  const [error, setError] = useState<string | null>(null);
  const [conflictMessage, setConflictMessage] = useState<string | null>(null);

  const useWorkspaceProviders = agent?.origin === "workspace";
  const globalProviders = settingsProviders.data?.providers.map(settingsProviderToOption) ?? [];
  const workspaceProviders = (workspaceDetail.data?.providers ?? []).map(workspaceProviderToOption);
  const providerOptions = useWorkspaceProviders ? workspaceProviders : globalProviders;
  const providersLoading = useWorkspaceProviders
    ? workspaceId !== null && workspaceDetail.isLoading
    : settingsProviders.isLoading || settingsProviders.isFetching;
  const providerSourceError = useWorkspaceProviders
    ? workspaceDetail.error
      ? describeError("Couldn't load workspace providers", workspaceDetail.error)
      : null
    : settingsProviders.error
      ? describeError("Couldn't load providers", settingsProviders.error)
      : null;

  const catalogProviders: RuntimeCatalogProvider[] = providerOptions.map(option => ({
    id: option.id,
    needsAuth: option.needs_auth,
  }));
  const catalog = useRuntimeModelCatalog(catalogProviders, { enabled: Boolean(agent) });

  const value: RuntimeSelectorValue = resolveAgentRuntimeValue(agent);

  const onChange = (next: RuntimeSelectorValue) => {
    if (!agent || updateAgent.isPending) return;
    const draft = {
      ...buildSettingsDraftFromAgent(agent),
      provider: next.provider,
      model: next.model ?? "",
      reasoningEffort: next.reasoning_effort,
    };
    const params = buildUpdateAgentParams(draft, workspaceId);
    if (!params) {
      setError("Runtime selection is incomplete.");
      return;
    }

    setError(null);
    setConflictMessage(null);
    const cacheWorkspace = workspaceId;
    const detailKey = agentKeys.detail(agent.name, cacheWorkspace);
    updateAgent.mutate(
      { name: agent.name, params, cacheWorkspace },
      {
        onSuccess: () => {
          toast.success("Runtime updated");
        },
        onError: mutationError => {
          if (isAgentDigestConflict(mutationError)) {
            // Refetch the exact view key only. Do not write the rejected choice
            // into cache — mutation onSuccess never ran, and invalidate+refetch
            // restores server truth under the same key useAgent reads.
            void queryClient
              .invalidateQueries({ queryKey: detailKey })
              .then(() => queryClient.refetchQueries({ queryKey: detailKey }))
              .finally(() => {
                setConflictMessage(
                  "This agent changed elsewhere. Server truth was refreshed — choose the runtime again."
                );
              });
            return;
          }
          setError(describeError("Couldn't update runtime", mutationError));
        },
      }
    );
  };

  return {
    value,
    onChange,
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
    runtimeModels: catalog.models as RuntimeModelOption[],
    modelCatalogLoading: catalog.loading,
    modelCatalogLoaded: catalog.loaded,
    modelCatalogRefreshing: catalog.refreshing,
    modelCatalogError: catalog.error,
    onRefreshCatalog: () => {
      void catalog.refresh();
    },
    onOpenProviderSettings: () => {
      void navigate({ to: "/settings/providers" });
    },
    isPending: updateAgent.isPending,
    error,
    conflictMessage,
    onDismissError: () => {
      setError(null);
      setConflictMessage(null);
    },
  };
}

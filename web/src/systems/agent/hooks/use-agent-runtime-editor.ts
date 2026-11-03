import { useRef, useState } from "react";
import { useNavigate } from "@tanstack/react-router";
import { useQueryClient } from "@tanstack/react-query";
import { toast } from "@compozy/ui";

import { isAgentDigestConflict } from "../adapters/agent-api";
import { hasAgentRuntimeOverride, resolveAgentRuntimeValue } from "../lib/agent-effective-runtime";
import {
  buildSettingsDraftFromAgent,
  buildUpdateAgentParams,
  type AgentSettingsDraft,
} from "../lib/agent-settings-draft";
import { agentKeys } from "../lib/query-keys";
import type { AgentPayload } from "../types";
import { useAgentWindowScope } from "./use-agent-window-scope";
import { useUpdateAgent } from "./use-agents";
import { type RuntimeCatalogProvider, useRuntimeModelCatalog } from "@/systems/model-catalog";
import { type RuntimeSpeed } from "@/lib/api-contract";
import type {
  RuntimeModelOption,
  RuntimeProviderOption,
  RuntimeSelectorValue,
} from "@/systems/runtime";
import { settingsProviderToOption, useSettingsProviders } from "@/systems/settings";
import { useWorkspace, workspaceProviderToOption } from "@/systems/workspace";
import { useProfileReadScope } from "@/systems/profiles";

function describeError(fallback: string, error: unknown): string {
  if (error instanceof Error && error.message.trim().length > 0) {
    return error.message;
  }
  return fallback;
}

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
  isPending: boolean;
  error: string | null;
  conflictMessage: string | null;
  onDismissError: () => void;
}

const DEFAULT_RUNTIME: Partial<AgentSettingsDraft> = {
  provider: "",
  model: "",
  reasoningEffort: "",
  speed: "",
  acpOptions: [],
};

/**
 * Immediate Provider · Model · Reasoning mutation for the agent's Model row.
 * Keeps the closed trigger on server truth; pending disables without collapsing.
 */
export function useAgentRuntimeEditor({
  agent,
}: UseAgentRuntimeEditorOptions): UseAgentRuntimeEditorResult {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const { destination } = useProfileReadScope();
  // The window's scope: saves go to its project, the cache write to the key `useAgent` reads.
  const { workspace: cacheWorkspace, projectId: workspaceId } = useAgentWindowScope();
  const updateAgent = useUpdateAgent();
  const settingsProviders = useSettingsProviders();
  const workspaceDetail = useWorkspace(workspaceId ?? "", {
    enabled: workspaceId !== null,
  });
  const [error, setError] = useState<string | null>(null);
  const [conflictMessage, setConflictMessage] = useState<string | null>(null);
  // A choice made while a save is in flight waits here and applies on the saved definition.
  const inFlight = useRef(false);
  const queued = useRef<Partial<AgentSettingsDraft> | null>(null);

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

  const catalogProviders: RuntimeCatalogProvider[] = providerOptions.map(option => ({
    id: option.id,
    needsAuth: option.needs_auth,
  }));
  const catalog = useRuntimeModelCatalog(catalogProviders, { enabled: Boolean(agent) });

  const value: RuntimeSelectorValue = resolveAgentRuntimeValue(agent);

  const speed = agent?.effective_runtime?.speed ?? agent?.speed ?? "normal";

  const save = (patch: Partial<AgentSettingsDraft>, base: AgentPayload | undefined = agent) => {
    if (!base) return;
    if (inFlight.current) {
      queued.current = { ...queued.current, ...patch };
      return;
    }
    const draft = { ...buildSettingsDraftFromAgent(base), ...patch };
    const params = buildUpdateAgentParams(draft, workspaceId);
    if (!params) {
      setError("This agent's setup needs fixing before it can change.");
      return;
    }

    setError(null);
    setConflictMessage(null);
    const detailKey = agentKeys.detail(base.name, cacheWorkspace, destination);
    inFlight.current = true;
    updateAgent.mutate(
      { name: base.name, params, cacheWorkspace, profile: destination },
      {
        onSuccess: saved => {
          inFlight.current = false;
          const next = queued.current;
          queued.current = null;
          if (next) {
            save(next, saved);
            return;
          }
          toast.success("Model updated");
        },
        onError: mutationError => {
          // The waiting choice was made on a definition that didn't save; drop it.
          inFlight.current = false;
          queued.current = null;
          if (isAgentDigestConflict(mutationError)) {
            // Refetch the exact view key only. Do not write the rejected choice
            // into cache — mutation onSuccess never ran, and invalidate+refetch
            // restores server truth under the same key useAgent reads.
            void queryClient
              .invalidateQueries({ queryKey: detailKey })
              .then(() => queryClient.refetchQueries({ queryKey: detailKey }))
              .finally(() => {
                setConflictMessage("This agent changed somewhere else. Pick the model again.");
              });
            return;
          }
          setError(describeError("Couldn't update the model", mutationError));
        },
      }
    );
  };

  const onChange = (next: RuntimeSelectorValue, normalizedSpeed?: RuntimeSpeed) =>
    save({
      provider: next.provider,
      model: next.model ?? "",
      reasoningEffort: next.reasoning_effort,
      acpOptions: next.acp_options ?? [],
      ...(normalizedSpeed ? { speed: normalizedSpeed } : {}),
    });

  return {
    value,
    speed,
    overridden: agent ? hasAgentRuntimeOverride(buildSettingsDraftFromAgent(agent)) : false,
    onChange,
    onSpeedChange: next => save({ speed: next }),
    onUseDefault: () => save(DEFAULT_RUNTIME),
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

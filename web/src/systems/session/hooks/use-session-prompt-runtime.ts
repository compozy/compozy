import { useEffect, useRef } from "react";
import { useSelector } from "@xstate/store-react";
import { toast } from "sonner";

import { isReasoningEffort, type RuntimeSpeed } from "@/lib/api-contract";
import { resolveAgentRuntimeValue, useAgents } from "@/systems/agent";
import {
  providerNeedsAuth,
  useRuntimeModelCatalog,
  type RuntimeCatalogProvider,
} from "@/systems/model-catalog";
import { useProviders, type ProviderSummary } from "@/systems/providers";
import type { RuntimeProviderOption, RuntimeSelectorValue } from "@/systems/runtime";
import { useWorkspace, type SessionProviderOption } from "@/systems/workspace";

import type { SessionPromptRuntimeSnapshot } from "../contexts/session-prompt-runtime-context-value";
import type { SessionPromptRuntimeStore } from "../stores/session-prompt-runtime-store";
import type { SessionRuntimeEffective } from "../types";
import { useSetSessionRuntime } from "./use-session-runtime-selection";

function runtimeValueFromEffective(
  effective: SessionRuntimeEffective | undefined
): RuntimeSelectorValue {
  return {
    provider: effective?.provider?.trim() ?? "",
    model: effective?.model?.trim() ?? "",
    reasoning_effort: effective?.reasoning_effort ?? "",
  };
}

function runtimeSpeedFromEffective(effective: SessionRuntimeEffective | undefined): RuntimeSpeed {
  return effective?.speed ?? "normal";
}

function snapshotFromSelection(
  value: RuntimeSelectorValue,
  speed: RuntimeSpeed
): SessionPromptRuntimeSnapshot | null {
  const provider = value.provider.trim();
  if (provider.length === 0) return null;

  const model = value.model.trim();
  return {
    provider,
    ...(model.length > 0 ? { model } : {}),
    ...(isReasoningEffort(value.reasoning_effort)
      ? { reasoning_effort: value.reasoning_effort }
      : {}),
    ...(speed === "fast" ? { speed } : {}),
  };
}

function runtimeProviderOptions(
  providers: SessionProviderOption[] | undefined,
  globalProviders: ProviderSummary[] | undefined
): RuntimeProviderOption[] {
  const needsAuthByProvider = new Map(
    (globalProviders ?? []).map(provider => [
      provider.name,
      providerNeedsAuth(provider.auth_status?.state),
    ])
  );
  return (providers ?? []).map(provider => ({
    id: provider.name,
    name: provider.display_name?.trim() || provider.name,
    ...(provider.harness?.trim() ? { harness: provider.harness.trim() } : {}),
    runtime_provider: provider.runtime_provider?.trim() || provider.name,
    needs_auth: needsAuthByProvider.get(provider.name) ?? false,
  }));
}

/**
 * Reads server-owned runtime availability from Query and keeps prompt intent in
 * the injected interaction store. A prompt snapshots that intent only at its
 * dispatch boundary, so catalog refetches never overwrite a user choice.
 */
export function useSessionPromptRuntime(store: SessionPromptRuntimeStore) {
  const input = useSelector(store, snapshot => snapshot.context.input);
  const selectedValue = useSelector(store, snapshot => snapshot.context.selectedValue);
  const selectedSpeed = useSelector(store, snapshot => snapshot.context.selectedSpeed);
  const runtimeSelection = useSetSessionRuntime(input.workspaceId);
  const runtimeSelectionController = useRef<AbortController | null>(null);
  const workspace = useWorkspace(input.workspaceId, { enabled: input.canPrompt });
  const globalProviders = useProviders();
  const agents = useAgents(input.workspaceId, { enabled: input.canPrompt });
  const agent = agents.data?.find(candidate => candidate.name === input.agentName);
  const agentRuntime = resolveAgentRuntimeValue(agent);
  const fallbackValue = runtimeValueFromEffective(input.effectiveRuntime);
  const defaultValue = fallbackValue.provider.length > 0 ? fallbackValue : agentRuntime;
  const defaultSpeed = runtimeSpeedFromEffective(input.effectiveRuntime);
  const value = selectedValue ?? defaultValue;
  const speed = selectedSpeed ?? defaultSpeed;

  useEffect(() => {
    const fallback = runtimeValueFromEffective(input.effectiveRuntime);
    store.trigger.defaultRuntimeResolved({
      speed: runtimeSpeedFromEffective(input.effectiveRuntime),
      value: fallback.provider.length > 0 ? fallback : agentRuntime,
    });
  }, [agentRuntime, input.effectiveRuntime, store]);
  useEffect(
    () => () => {
      runtimeSelectionController.current?.abort();
      runtimeSelectionController.current = null;
    },
    []
  );
  const providers = runtimeProviderOptions(
    workspace.data?.providers,
    globalProviders.data?.providers
  );
  const catalogProviders: RuntimeCatalogProvider[] = providers.map(provider => ({
    id: provider.id,
    needsAuth: provider.needs_auth,
  }));
  const availabilityError = workspace.error
    ? describeWorkspaceProvidersError(workspace.error)
    : globalProviders.error
      ? describeGlobalProvidersError(globalProviders.error)
      : null;
  const catalog = useRuntimeModelCatalog(catalogProviders, {
    enabled:
      input.canPrompt &&
      !globalProviders.isLoading &&
      availabilityError === null &&
      providers.length > 0,
  });
  const canSelectRuntime =
    input.canPrompt &&
    !runtimeSelection.isPending &&
    !workspace.isLoading &&
    !globalProviders.isLoading &&
    availabilityError === null &&
    providers.length > 0;

  const persistRuntimeSelection = (nextValue: RuntimeSelectorValue, nextSpeed: RuntimeSpeed) => {
    runtimeSelectionController.current?.abort();
    runtimeSelectionController.current = null;

    const runtime = snapshotFromSelection(nextValue, nextSpeed);
    if (!runtime) {
      store.trigger.runtimePersistenceFailed({
        sessionId: input.sessionId,
        workspaceId: input.workspaceId,
      });
      return;
    }

    const controller = new AbortController();
    runtimeSelectionController.current = controller;
    runtimeSelection.mutate(
      {
        id: input.sessionId,
        request: {
          expected_revision: input.selectionRevision,
          runtime,
        },
        signal: controller.signal,
      },
      {
        onSuccess: session => {
          if (controller.signal.aborted) return;
          store.trigger.runtimePersisted({
            revision: session.runtime.selection_revision,
            runtime: session.runtime.selected ?? undefined,
            sessionId: input.sessionId,
            workspaceId: input.workspaceId,
          });
        },
        onError: error => {
          if (controller.signal.aborted) return;
          store.trigger.runtimePersistenceFailed({
            sessionId: input.sessionId,
            workspaceId: input.workspaceId,
          });
          toast.error(
            error instanceof Error && error.message.trim().length > 0
              ? error.message
              : "Couldn't save the session runtime."
          );
        },
        onSettled: () => {
          if (runtimeSelectionController.current === controller) {
            runtimeSelectionController.current = null;
          }
        },
      }
    );
  };

  return {
    availabilityError,
    canSelectRuntime,
    catalog: {
      error: catalog.error,
      loaded: catalog.loaded,
      loading: catalog.loading,
      models: catalog.models,
      providers,
      refresh: catalog.refresh,
      refreshError: catalog.refreshError,
      refreshing: catalog.refreshing,
      stale: catalog.stale,
    },
    getRuntimeSnapshot: () => snapshotFromSelection(value, speed),
    onRetryProviders: () => {
      void workspace.refetch();
      void globalProviders.refetch();
    },
    onRuntimeChange: (next: RuntimeSelectorValue) => {
      store.trigger.runtimeSelected({ value: next });
      persistRuntimeSelection(next, speed);
    },
    onSpeedChange: (next: RuntimeSpeed) => {
      store.trigger.speedSelected({ speed: next });
      persistRuntimeSelection(value, next);
    },
    speed,
    value,
  };
}

export function getSessionPromptRuntimeSnapshot(
  store: SessionPromptRuntimeStore
): SessionPromptRuntimeSnapshot | null {
  const context = store.getSnapshot().context;
  const value = context.selectedValue ?? context.defaultValue;
  const speed = context.selectedSpeed ?? context.defaultSpeed;
  return snapshotFromSelection(value, speed);
}

function describeWorkspaceProvidersError(error: unknown): string {
  if (error instanceof Error && error.message.trim().length > 0) return error.message;
  return "Couldn't load workspace providers. Retry to choose a runtime.";
}

function describeGlobalProvidersError(error: unknown): string {
  if (error instanceof Error && error.message.trim().length > 0) return error.message;
  return "Couldn't load provider authentication. Retry to choose a runtime.";
}

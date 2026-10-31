import { use, useEffect, useRef, useState } from "react";
import { useSelector } from "@xstate/store-react";
import { toast } from "sonner";

import { isReasoningEffort, type RuntimeSpeed } from "@/lib/api-contract";

import type { SessionPromptRuntimeSnapshot } from "../contexts/session-prompt-runtime-context-value";
import {
  SessionPromptRuntimeActivityContext,
  SessionPromptRuntimeCapabilitiesContext,
  SessionPromptRuntimePersistenceContext,
} from "../contexts/session-prompt-runtime-context-value";
import type { SessionPromptRuntimeStore } from "../stores/session-prompt-runtime-store";
import type { SessionRuntimeSelection, SessionRuntimePayload } from "../types";
import {
  currentRuntimeApplyNotice,
  runtimeApplyTargetSettled,
  type RuntimeApplyTarget,
} from "../lib/runtime-apply-notice";
import { useSetSessionRuntime } from "./use-session-runtime-selection";
import {
  normalizeRuntimeSpeed,
  resolveAgentRuntimeValue,
  useAgents,
  type AgentPayload,
} from "@/systems/agent";
import {
  providerNeedsAuth,
  type RuntimeCatalogProvider,
  useRuntimeModelCatalog,
} from "@/systems/model-catalog";
import { type ProviderSummary, useProviders } from "@/systems/providers";
import {
  normalizeRuntimeACPSelections,
  type RuntimeProviderOption,
  type RuntimeSelectorValue,
  type RuntimeModelOption,
} from "@/systems/runtime";
import {
  type SessionProviderOption,
  useWorkspace,
  workspaceProviderToOption,
} from "@/systems/workspace";

/** Convert the requested selection to picker intent without inventing optional ACP values. */
function runtimeValueFromSelection(
  selection: SessionRuntimeSelection | undefined
): RuntimeSelectorValue {
  const acpOptions = normalizeRuntimeACPSelections(selection?.acp_options);
  return {
    provider: selection?.provider?.trim() ?? "",
    model: selection?.model?.trim() ?? "",
    reasoning_effort: selection?.reasoning_effort ?? "",
    ...(acpOptions ? { acp_options: acpOptions } : {}),
  };
}

/** Use normal speed when the requested selection omits speed. */
function runtimeSpeedFromSelection(selection: SessionRuntimeSelection | undefined): RuntimeSpeed {
  return selection?.speed ?? "normal";
}

/** Freeze only valid explicit selector fields for the prompt submission boundary. */
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
    ...(value.acp_options ? { acp_options: value.acp_options } : {}),
  };
}

/** Combine workspace provider availability with current authentication status. */
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
    ...workspaceProviderToOption(provider),
    needs_auth: needsAuthByProvider.get(provider.name) ?? false,
  }));
}

/**
 * Reads server-owned runtime availability from Query and keeps prompt intent in
 * the injected interaction store. A prompt snapshots that intent only at its
 * dispatch boundary, so catalog refetches never overwrite a user choice.
 */
export function useSessionPromptRuntime(store: SessionPromptRuntimeStore) {
  const runtimeCapabilities = use(SessionPromptRuntimeCapabilitiesContext);
  const liveBadge = use(SessionPromptRuntimeActivityContext);
  const persistDraft = use(SessionPromptRuntimePersistenceContext);
  const input = useSelector(store, snapshot => snapshot.context.input);
  const selectedValue = useSelector(store, snapshot => snapshot.context.selectedValue);
  const selectedSpeed = useSelector(store, snapshot => snapshot.context.selectedSpeed);
  const runtimeSelection = useSetSessionRuntime(input.workspaceId);
  const runtimeSelectionController = useRef<AbortController | null>(null);
  const queuedSelection = useRef<{ value: RuntimeSelectorValue; speed: RuntimeSpeed } | null>(null);
  const [applyTarget, setApplyTarget] = useState<RuntimeApplyTarget | null>(null);
  const liveSession = { badge: liveBadge, runtime: runtimeCapabilities };
  const applyNotice = applyTarget ? currentRuntimeApplyNotice(applyTarget, liveSession) : null;
  if (applyTarget && runtimeApplyTargetSettled(applyTarget, liveSession)) setApplyTarget(null);
  const workspace = useWorkspace(input.workspaceId, { enabled: input.canPrompt });
  const globalProviders = useProviders();
  const agents = useAgents(input.workspaceId, { enabled: input.canPrompt });
  const agent = agents.data?.find(candidate => candidate.name === input.agentName);
  const baseline = input.requestedRuntime ?? input.effectiveRuntime;
  const { defaultValue, defaultSpeed } = promptRuntimeDefaults(agent, baseline);
  const value = selectedValue ?? defaultValue;
  const speed = selectedSpeed ?? defaultSpeed;

  useEffect(() => {
    // Synchronize server input changes into the external store used by prompt snapshots.
    const defaults = promptRuntimeDefaults(agent, baseline);
    store.trigger.defaultRuntimeResolved({
      speed: defaults.defaultSpeed,
      value: defaults.defaultValue,
    });
  }, [agent, baseline, store]);
  useEffect(
    () => () => {
      runtimeSelectionController.current?.abort();
      runtimeSelectionController.current = null;
      queuedSelection.current = null;
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
    if (persistDraft) {
      const runtime = snapshotFromSelection(nextValue, nextSpeed);
      if (runtime) persistDraft(runtime);
      return;
    }
    if (runtimeSelectionController.current) {
      queuedSelection.current = { value: nextValue, speed: nextSpeed };
      return;
    }

    const runtime = snapshotFromSelection(nextValue, nextSpeed);
    if (!runtime) {
      store.trigger.runtimePersistenceFailed({
        sessionId: input.sessionId,
        workspaceId: input.workspaceId,
      });
      return;
    }

    setApplyTarget(null);
    const controller = new AbortController();
    runtimeSelectionController.current = controller;
    runtimeSelection.mutate(
      {
        id: input.sessionId,
        request: {
          expected_revision: store.getSnapshot().context.input.selectionRevision,
          runtime,
        },
        signal: controller.signal,
      },
      {
        onSuccess: session => {
          if (controller.signal.aborted) return;
          const liveApply = providers.find(
            provider => provider.id === session.runtime.selected?.provider
          )?.runtime_live_apply;
          setApplyTarget(queuedSelection.current ? null : { saved: session, liveApply });
          store.trigger.runtimePersisted({
            revision: session.runtime.selection_revision,
            runtime: session.runtime.selected ?? undefined,
            sessionId: input.sessionId,
            workspaceId: input.workspaceId,
          });
          const queued = queuedSelection.current;
          if (queued) {
            store.trigger.runtimeSelected({ value: queued.value });
            store.trigger.speedSelected({ speed: queued.speed });
          }
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
            const queued = queuedSelection.current;
            queuedSelection.current = null;
            if (queued && !controller.signal.aborted) {
              persistRuntimeSelection(queued.value, queued.speed);
            }
          }
        },
      }
    );
  };

  return {
    applyNotice,
    availabilityError,
    canSelectRuntime,
    catalog: {
      error: catalog.error,
      loaded: catalog.loaded,
      loading: catalog.loading,
      models: sessionModelCapabilities(catalog.models, runtimeCapabilities),
      providers,
      refresh: catalog.refresh,
      refreshError: catalog.refreshError,
      providerIssues: catalog.providerIssues,
      refreshing: catalog.refreshing,
      stale: catalog.stale,
    },
    getRuntimeSnapshot: () => snapshotFromSelection(value, speed),
    onRetryProviders: () => {
      void workspace.refetch();
      void globalProviders.refetch();
    },
    onRuntimeChange: (next: RuntimeSelectorValue, normalizedSpeed?: RuntimeSpeed) => {
      const nextSpeed = normalizedSpeed ?? speed;
      store.trigger.runtimeSelected({ value: next });
      if (normalizedSpeed && normalizedSpeed !== speed) {
        store.trigger.speedSelected({ speed: normalizedSpeed });
      }
      persistRuntimeSelection(next, nextSpeed);
    },
    onSpeedChange: (next: RuntimeSpeed) => {
      store.trigger.speedSelected({ speed: next });
      persistRuntimeSelection(value, next);
    },
    speed,
    value,
  };
}

/** Resolve agent defaults only until a session has binding intent of its own. */
function promptRuntimeDefaults(
  agent: AgentPayload | undefined,
  selection: SessionRuntimeSelection | undefined
) {
  const fallback = runtimeValueFromSelection(selection);
  return {
    defaultValue: fallback.provider.length > 0 ? fallback : resolveAgentRuntimeValue(agent),
    defaultSpeed: selection
      ? runtimeSpeedFromSelection(selection)
      : normalizeRuntimeSpeed(agent?.effective_runtime?.speed ?? agent?.speed) || "normal",
  };
}

/** Overlay acknowledged options only on their effective model, respecting provider policy and matrices. */
function sessionModelCapabilities(
  models: RuntimeModelOption[],
  runtime: SessionRuntimePayload | undefined
): RuntimeModelOption[] {
  const options = runtime?.acp_caps?.config_options;
  const effective = runtime?.effective;
  if (!options || !effective?.model) return models;
  const reasoning = options.find(
    option =>
      option.kind === "select" &&
      (option.category === "thought_level" ||
        option.id === "effort" ||
        option.id === "reasoning_effort")
  );
  // ACP config_options is a complete snapshot: removing thought_level revokes stale levels.
  const effortSet = new Set<string>();
  for (const value of reasoning?.values ?? []) {
    if (value.value !== "default" && isReasoningEffort(value.value)) effortSet.add(value.value);
  }
  const efforts = [...effortSet];
  return models.map<RuntimeModelOption>(model => {
    // Catalog eligibility enforces the provider apply strategy and explicit matrices.
    // Live options belong only to the effective model, never pending prompt intent.
    if (
      model.provider !== effective.provider ||
      model.id !== effective.model ||
      model.reasoning_apply === "none" ||
      (model.reasoning_apply !== "acp_option" && model.efforts.length === 0) ||
      model.configurations
    )
      return model;
    return {
      ...model,
      efforts,
      default_effort: effortSet.has(reasoning?.current_value_id ?? "")
        ? reasoning?.current_value_id
        : "",
      reasoning_source: "acp",
      reasoning_known: true,
    };
  });
}

/** Read the latest selected intent, falling back to server defaults, at dispatch time. */
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

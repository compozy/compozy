import { useEffect } from "react";
import { useNavigate } from "@tanstack/react-router";
import { useSelector, useStore } from "@xstate/store-react";

import type { EntityMode } from "@compozy/ui";

import {
  networkParticipationDraftFromValues,
  networkParticipationValidationMessage,
  serializeNetworkParticipation,
  type NetworkParticipationDraft,
} from "@/lib/network-participation";
import { resolveAgentRuntimeValue, useAgents, type AgentPayload } from "@/systems/agent";
import {
  deriveActiveSessionOptions,
  useRuntimeModelCatalog,
  type ProviderModelPayload,
  type RuntimeCatalogProvider,
} from "@/systems/model-catalog";
import type {
  RuntimeModelOption,
  RuntimeProviderOption,
  RuntimeSelectorValue,
} from "@/systems/runtime";
import type {
  SessionProviderOption,
  WorkspaceCommandSelectOption,
  WorkspacePayload,
} from "@/systems/workspace";
import { toWorkspaceCommandSelectOptions, useWorkspace, useWorkspaces } from "@/systems/workspace";

import {
  describeWorkspaceError,
  normalizeEffort,
  resolveSessionWorkspacePath,
  resolveSelectedProvider,
  type SessionCreateDialogDraft,
} from "../lib/session-create-draft";
import {
  MODEL_CATALOG_PENDING,
  validateSessionModelSelection,
} from "../lib/session-model-selection";
import { sessionCreateStoreLogic, type SessionCreateStore } from "../stores/session-create-store";
import { useCreateSession } from "./use-session-actions";

interface SessionCreateDialogContext {
  agents: AgentPayload[] | undefined;
  activeWorkspace: WorkspacePayload | undefined;
}

export interface SessionCreateDialogState {
  open: boolean;
  mode: EntityMode;
  agents: AgentPayload[];
  workspace: WorkspacePayload | undefined;
  workspaces: WorkspaceCommandSelectOption[];
  workspaceId: string | null;
  sessionName: string;
  workspacePath: string;
  providersLoading: boolean;
  providersError: string | null;
  hasProviderOptions: boolean;
  selectedAgentName: string;
  runtimeValue: RuntimeSelectorValue;
  runtimeProviders: RuntimeProviderOption[];
  runtimeModels: RuntimeModelOption[];
  catalogStale: boolean;
  catalogLoading: boolean;
  catalogLoaded: boolean;
  catalogError: string | null;
  catalogRefreshing: boolean;
  catalogRefreshError: string | null;
  isSubmitting: boolean;
  submitError: string | null;
  pendingAgentName: string | null;
  pendingWorkspaceId: string | null;
  promptValue: string;
}

export interface SessionCreateDialogApi extends SessionCreateDialogState {
  onOpenChange: (open: boolean) => void;
  onModeChange: (mode: EntityMode) => void;
  onAgentChange: (agentName: string) => void;
  onWorkspaceChange: (workspaceId: string) => void;
  onSessionNameChange: (next: string) => void;
  onWorkspacePathChange: (next: string) => void;
  onPromptChange: (next: string) => void;
  onRuntimeChange: (next: RuntimeSelectorValue) => void;
  onNetworkParticipationChange: (next: NetworkParticipationDraft) => void;
  networkParticipation: NetworkParticipationDraft;
  refreshCatalog: () => void;
  openProviderSettings: () => void;
  submit: () => void;
}

export interface SessionCreateDialogController {
  store: SessionCreateStore;
}

export function useSessionCreateDialogController(): SessionCreateDialogController {
  return { store: useStore(sessionCreateStoreLogic) };
}

export function useSessionCreateDialogViewModel(
  { agents, activeWorkspace }: SessionCreateDialogContext,
  store: SessionCreateStore
): SessionCreateDialogApi {
  const navigate = useNavigate();
  const createSession = useCreateSession();
  const flow = useSelector(store, snapshot => snapshot.context);
  const { draft } = flow;

  const workspaceId = draft.workspaceId || (activeWorkspace?.id ?? "");
  const workspaceAgentsQuery = useAgents(workspaceId, { enabled: workspaceId.length > 0 });
  const workspacesQuery = useWorkspaces();
  const workspaceOptions = toWorkspaceCommandSelectOptions(workspacesQuery.data ?? []);
  const {
    data: workspaceDetail,
    isLoading: workspaceDetailLoading,
    error: workspaceDetailError,
  } = useWorkspace(workspaceId, { enabled: workspaceId.length > 0 });
  const targetWorkspace =
    (workspacesQuery.data ?? []).find(candidate => candidate.id === workspaceId) ??
    (activeWorkspace?.id === workspaceId ? activeWorkspace : undefined);
  const providerOptions: SessionProviderOption[] = workspaceDetail?.providers ?? [];

  const agentList =
    workspaceAgentsQuery.data ?? (workspaceId === activeWorkspace?.id ? (agents ?? []) : []);
  const selectedAgent =
    agentList.find(agent => agent.name === draft.agentName) ??
    (flow.open ? agentList[0] : undefined);
  const selectedAgentName = selectedAgent?.name ?? draft.agentName;
  const selectedProvider = resolveSelectedProvider(
    selectedAgentName,
    draft.providerOverride,
    selectedAgent,
    providerOptions
  );
  const runtimeProviders: RuntimeProviderOption[] = providerOptions.map(option => ({
    id: option.name,
    name: option.display_name?.trim() || option.name,
    ...(option.harness?.trim() ? { harness: option.harness.trim() } : {}),
    runtime_provider: option.runtime_provider?.trim() || option.name,
  }));
  const catalogProviders: RuntimeCatalogProvider[] = runtimeProviders.map(entry => ({
    id: entry.id,
  }));
  const catalog = useRuntimeModelCatalog(catalogProviders, { enabled: flow.open });
  const runtimeModels = catalog.models;
  const catalogModels: ProviderModelPayload[] = catalog.payloadsByProvider[selectedProvider] ?? [];

  const trimmedSelectedModel = draft.modelOverride.trim();
  const agentRuntime = resolveAgentRuntimeValue(selectedAgent);
  const trimmedAgentProvider = agentRuntime.provider.trim();
  const trimmedAgentModel =
    trimmedAgentProvider === selectedProvider ? agentRuntime.model.trim() : "";
  const trimmedAgentReasoning =
    trimmedAgentProvider === selectedProvider ? agentRuntime.reasoning_effort : "";
  const effectiveSelectedModel = trimmedSelectedModel || trimmedAgentModel;
  const reasoningSupported = deriveActiveSessionOptions({
    catalog: catalogModels,
    selectedModel: effectiveSelectedModel.length > 0 ? effectiveSelectedModel : null,
  }).reasoningSupported;
  const selectedReasoning = reasoningSupported
    ? draft.providerOverride.length > 0
      ? draft.reasoningEffort
      : trimmedAgentReasoning
    : "";
  const runtimeValue: RuntimeSelectorValue = {
    provider: selectedProvider,
    model: effectiveSelectedModel,
    reasoning_effort: selectedReasoning,
  };
  const modelSelection = validateSessionModelSelection({
    provider: selectedProvider,
    model: effectiveSelectedModel,
    models: runtimeModels,
    catalogLoading: catalog.loading,
    catalogLoaded: catalog.loaded,
    catalogError: catalog.error,
  });

  useEffect(() => {
    if (!flow.navigationTarget) return;
    const { attempt, session } = flow.navigationTarget;
    store.trigger.navigationRequested({
      attempt,
      execute: async () => {
        await navigate({
          to: "/agents/$name/sessions/$id",
          params: { name: session.agent_name, id: session.id },
        });
      },
    });
  }, [flow.navigationTarget, navigate, store]);

  const submit = () => {
    if (!targetWorkspace || flow.isSubmitting) return;
    const agentName = selectedAgentName.trim();
    const provider = selectedProvider.trim();
    const prompt = draft.prompt.trim();
    if (agentName.length === 0) return;
    if (prompt.length === 0) {
      store.trigger.validationFailed({
        message: "Write the first message before starting the session.",
      });
      return;
    }
    if (provider.length === 0) {
      store.trigger.validationFailed({
        message: "Choose a provider configured for this workspace.",
      });
      return;
    }
    if (!modelSelection.valid) {
      store.trigger.validationFailed({ message: modelSelection.error ?? MODEL_CATALOG_PENDING });
      return;
    }
    const networkParticipation = networkParticipationDraftFromValues(
      draft.networkParticipationMode,
      draft.networkChannelId,
      draft.networkChannelStrategy
    );
    const participationError = networkParticipationValidationMessage(networkParticipation, [
      "named",
    ]);
    if (participationError) {
      store.trigger.validationFailed({ message: participationError });
      return;
    }
    const workspacePathResolution = resolveSessionWorkspacePath(
      targetWorkspace.root_dir,
      draft.workspacePath
    );
    if ("error" in workspacePathResolution) {
      store.trigger.validationFailed({ message: workspacePathResolution.error });
      return;
    }

    const hasRuntimeOverride = draft.providerOverride.trim().length > 0;
    const modelOverride = hasRuntimeOverride ? draft.modelOverride.trim() : "";
    const reasoningEffort =
      hasRuntimeOverride && selectedReasoning !== "" ? selectedReasoning : undefined;
    const sessionName = draft.sessionName.trim();
    const workspacePath = workspacePathResolution.workspacePath;

    store.trigger.submissionRequested({
      agentName,
      workspaceId: targetWorkspace.id,
      execute: () =>
        createSession.mutateAsync({
          agent_name: agentName,
          prompt,
          ...(sessionName.length > 0 ? { name: sessionName } : {}),
          ...(workspacePath.length > 0
            ? { workspace_path: workspacePath }
            : { workspace: targetWorkspace.id }),
          ...(hasRuntimeOverride ? { provider } : {}),
          ...(modelOverride.length > 0 ? { model: modelOverride } : {}),
          ...(reasoningEffort ? { reasoning_effort: reasoningEffort } : {}),
          network_participation: serializeNetworkParticipation(networkParticipation),
        }),
    });
  };

  const effectiveProvider = resolveAgentRuntimeValue(selectedAgent).provider.trim();
  const providerUnavailable =
    selectedAgentName.trim().length > 0 &&
    draft.providerOverride.trim().length === 0 &&
    effectiveProvider.length > 0 &&
    !providerOptions.some(option => option.name === effectiveProvider);
  const providersError = workspaceDetailError
    ? describeWorkspaceError(workspaceDetailError)
    : providerUnavailable
      ? `Provider "${effectiveProvider}" is not available in this workspace. Choose a configured provider.`
      : null;

  return {
    open: flow.open,
    mode: flow.mode,
    agents: agentList,
    workspace: targetWorkspace,
    workspaces: workspaceOptions,
    workspaceId: workspaceId.length > 0 ? workspaceId : null,
    sessionName: draft.sessionName,
    workspacePath: draft.workspacePath,
    providersLoading: workspaceId.length > 0 && workspaceDetailLoading,
    providersError,
    hasProviderOptions: providerOptions.length > 0,
    selectedAgentName,
    runtimeValue,
    runtimeProviders,
    runtimeModels,
    catalogStale: catalog.stale,
    catalogLoading: catalog.loading,
    catalogLoaded: catalog.loaded,
    catalogError: catalog.error,
    catalogRefreshing: catalog.refreshing,
    catalogRefreshError: catalog.refreshError,
    isSubmitting: flow.isSubmitting,
    submitError: flow.submitError,
    pendingAgentName: flow.pendingAgentName,
    pendingWorkspaceId: flow.pendingWorkspaceId,
    promptValue: draft.prompt,
    onOpenChange: open => store.trigger.dialogOpenChanged({ open }),
    onModeChange: mode => store.trigger.modeSelected({ mode }),
    onAgentChange: agentName => store.trigger.agentSelected({ agentName, workspaceId }),
    onWorkspaceChange: nextWorkspaceId =>
      store.trigger.workspaceSelected({ workspaceId: nextWorkspaceId }),
    onSessionNameChange: sessionName => store.trigger.sessionNameChanged({ sessionName }),
    onWorkspacePathChange: workspacePath => store.trigger.workspacePathChanged({ workspacePath }),
    onPromptChange: prompt => store.trigger.promptChanged({ prompt }),
    onRuntimeChange: next =>
      store.trigger.runtimeSelected({
        providerOverride: next.provider,
        modelOverride: next.model,
        reasoningEffort: normalizeEffort(next.reasoning_effort),
      }),
    onNetworkParticipationChange: next =>
      store.trigger.networkParticipationSelected({
        networkParticipationMode: next.mode,
        networkChannelId: next.channelId,
        networkChannelStrategy: next.channelStrategy,
      }),
    networkParticipation: networkParticipationDraftFromValues(
      draft.networkParticipationMode,
      draft.networkChannelId,
      draft.networkChannelStrategy
    ),
    refreshCatalog: catalog.refresh,
    openProviderSettings: () => {
      store.trigger.providerSettingsRequested();
      void navigate({ to: "/settings/providers" });
    },
    submit,
  };
}

export type { SessionCreateDialogDraft };

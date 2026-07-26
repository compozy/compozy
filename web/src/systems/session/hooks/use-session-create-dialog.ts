import { useEffect, useRef, useState } from "react";
import { useNavigate } from "@tanstack/react-router";
import { toast } from "sonner";

import type { EntityMode } from "@agh/ui";

import { resolveAgentRuntimeValue, useAgents, type AgentPayload } from "@/systems/agent";
import {
  deriveActiveSessionOptions,
  useRuntimeModelCatalog,
  type ProviderModelPayload,
  type RuntimeCatalogProvider,
} from "@/systems/model-catalog";
import {
  networkParticipationDraftFromValues,
  networkParticipationValidationMessage,
  serializeNetworkParticipation,
  type NetworkParticipationDraft,
} from "@/systems/network";
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
  ADVANCED_DEFAULTS,
  applySessionAgentSelection,
  describeError,
  describeWorkspaceError,
  EMPTY_SESSION_CREATE_DRAFT,
  normalizeEffort,
  resolveSessionWorkspacePath,
  resolveSelectedProvider,
  RUNTIME_OVERRIDE_DEFAULTS,
  type SessionCreateDialogDraft,
} from "../lib/session-create-draft";
import {
  MODEL_CATALOG_PENDING,
  validateSessionModelSelection,
} from "../lib/session-model-selection";
import type { SessionPayload } from "../types";
import { useCreateSession } from "./use-session-actions";

interface SessionCreateDialogContext {
  agents: AgentPayload[] | undefined;
  activeWorkspace: WorkspacePayload | undefined;
}

interface SessionNavigationTarget {
  agentName: string;
  sessionId: string;
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
  openForAgent: (agentName: string) => void;
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
  submit: () => Promise<void>;
}

export function useSessionCreateDialog({
  agents,
  activeWorkspace,
}: SessionCreateDialogContext): SessionCreateDialogApi {
  const navigate = useNavigate();
  const createSession = useCreateSession();

  const [open, setOpenState] = useState(false);
  const [mode, setMode] = useState<EntityMode>("simple");
  const [draft, setDraft] = useState<SessionCreateDialogDraft>(EMPTY_SESSION_CREATE_DRAFT);

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

  const [submitError, setSubmitError] = useState<string | null>(null);
  const [pendingAgentName, setPendingAgentName] = useState<string | null>(null);
  const [pendingWorkspaceId, setPendingWorkspaceId] = useState<string | null>(null);
  const [navigationTarget, setNavigationTarget] = useState<SessionNavigationTarget | null>(null);
  const navigatedTarget = useRef<SessionNavigationTarget | null>(null);
  const submitInFlight = useRef(false);

  useEffect(() => {
    if (!navigationTarget || navigatedTarget.current === navigationTarget) return;
    navigatedTarget.current = navigationTarget;
    void navigate({
      to: "/agents/$name/sessions/$id",
      params: { name: navigationTarget.agentName, id: navigationTarget.sessionId },
    });
  }, [navigate, navigationTarget]);

  // The shell's active-workspace list only seeds this dialog for its initial workspace.
  // A dialog-local workspace selection owns a distinct query identity and must never reuse
  // an agent population fetched for a different workspace.
  const agentList =
    workspaceAgentsQuery.data ?? (workspaceId === activeWorkspace?.id ? (agents ?? []) : []);
  const selectedAgent = agentList.find(agent => agent.name === draft.agentName);
  const selectedProvider = resolveSelectedProvider(
    draft.agentName,
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
  const catalog = useRuntimeModelCatalog(catalogProviders, { enabled: open });
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

  const openForAgent = (agentName: string) => {
    if (!activeWorkspace) {
      toast.error("Select an active workspace before starting a session.");
      return;
    }
    const matched = agentList.find(agent => agent.name === agentName) ?? agentList[0];
    const nextAgentName = matched?.name ?? agentName;
    setDraft(current => applySessionAgentSelection(current, nextAgentName, activeWorkspace.id));
    setMode("simple");
    setSubmitError(null);
    setOpenState(true);
  };

  const onModeChange = (next: EntityMode) => {
    setMode(next);
    if (next === "advanced") return;
    setSubmitError(null);
    setDraft(current => ({ ...current, ...ADVANCED_DEFAULTS }));
  };

  const onWorkspaceChange = (nextWorkspaceId: string) => {
    setSubmitError(null);
    setDraft(current => ({
      ...current,
      workspaceId: nextWorkspaceId,
      agentName: "",
      prompt: "",
      ...RUNTIME_OVERRIDE_DEFAULTS,
      ...ADVANCED_DEFAULTS,
    }));
  };

  const onSessionNameChange = (sessionName: string) => {
    setDraft(current => ({ ...current, sessionName }));
  };

  const onWorkspacePathChange = (workspacePath: string) => {
    setDraft(current => ({ ...current, workspacePath }));
  };

  const handleOpenChange = (next: boolean) => {
    setOpenState(next);
    if (!next) setSubmitError(null);
  };

  const onAgentChange = (agentName: string) => {
    setSubmitError(null);
    setDraft(current => ({
      ...applySessionAgentSelection(current, agentName, workspaceId),
      sessionName: current.sessionName,
    }));
  };

  const onPromptChange = (prompt: string) => {
    setSubmitError(null);
    setDraft(current => ({ ...current, prompt }));
  };

  const onNetworkParticipationChange = (next: NetworkParticipationDraft) => {
    setSubmitError(null);
    setDraft(current => ({
      ...current,
      networkParticipationMode: next.mode,
      networkChannelId: next.channelId,
      networkChannelStrategy: next.channelStrategy,
    }));
  };

  const onRuntimeChange = (next: RuntimeSelectorValue) => {
    setSubmitError(null);
    setDraft(current => ({
      ...current,
      providerOverride: next.provider,
      modelOverride: next.model,
      reasoningEffort: normalizeEffort(next.reasoning_effort),
    }));
  };

  const openProviderSettings = () => {
    setOpenState(false);
    void navigate({ to: "/settings/providers" });
  };

  const submit = async () => {
    if (submitInFlight.current || !targetWorkspace) return;
    const agentName = draft.agentName.trim();
    const provider = selectedProvider.trim();
    const prompt = draft.prompt.trim();
    if (agentName.length === 0) return;
    if (prompt.length === 0) {
      setSubmitError("Write the first message before starting the session.");
      return;
    }
    if (provider.length === 0) {
      setSubmitError("Choose a provider configured for this workspace.");
      return;
    }
    if (!modelSelection.valid) {
      setSubmitError(modelSelection.error ?? MODEL_CATALOG_PENDING);
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
      setSubmitError(participationError);
      return;
    }

    setSubmitError(null);
    setPendingAgentName(agentName);
    setPendingWorkspaceId(targetWorkspace.id);

    const hasRuntimeOverride = draft.providerOverride.trim().length > 0;
    const modelOverride = hasRuntimeOverride ? draft.modelOverride.trim() : "";
    const reasoningEffort =
      hasRuntimeOverride && selectedReasoning !== "" ? selectedReasoning : undefined;
    const sessionName = draft.sessionName.trim();
    const workspacePathResolution = resolveSessionWorkspacePath(
      targetWorkspace.root_dir,
      draft.workspacePath
    );
    if ("error" in workspacePathResolution) {
      setSubmitError(workspacePathResolution.error);
      return;
    }
    const workspacePath = workspacePathResolution.workspacePath;

    submitInFlight.current = true;
    let session: SessionPayload;
    try {
      session = await createSession.mutateAsync({
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
      });
    } catch (error) {
      submitInFlight.current = false;
      const message = describeError("Failed to create session.", error);
      setSubmitError(message);
      toast.error(message);
      setPendingAgentName(null);
      setPendingWorkspaceId(null);
      return;
    }

    submitInFlight.current = false;
    setDraft(EMPTY_SESSION_CREATE_DRAFT);
    setMode("simple");
    setOpenState(false);
    setPendingAgentName(null);
    setPendingWorkspaceId(null);
    setNavigationTarget({ agentName: session.agent_name, sessionId: session.id });
  };

  const effectiveProvider = resolveAgentRuntimeValue(selectedAgent).provider.trim();
  const providerUnavailable =
    draft.agentName.trim().length > 0 &&
    draft.providerOverride.trim().length === 0 &&
    effectiveProvider.length > 0 &&
    !providerOptions.some(option => option.name === effectiveProvider);
  const providersError = workspaceDetailError
    ? describeWorkspaceError(workspaceDetailError)
    : providerUnavailable
      ? `Provider "${effectiveProvider}" is not available in this workspace. Choose a configured provider.`
      : null;

  return {
    open,
    mode,
    agents: agentList,
    workspace: targetWorkspace,
    workspaces: workspaceOptions,
    workspaceId: workspaceId.length > 0 ? workspaceId : null,
    sessionName: draft.sessionName,
    workspacePath: draft.workspacePath,
    providersLoading: workspaceId.length > 0 && workspaceDetailLoading,
    providersError,
    hasProviderOptions: providerOptions.length > 0,
    selectedAgentName: draft.agentName,
    runtimeValue,
    runtimeProviders,
    runtimeModels,
    catalogStale: catalog.stale,
    catalogLoading: catalog.loading,
    catalogLoaded: catalog.loaded,
    catalogError: catalog.error,
    catalogRefreshing: catalog.refreshing,
    catalogRefreshError: catalog.refreshError,
    isSubmitting: createSession.isPending,
    submitError,
    pendingAgentName,
    pendingWorkspaceId,
    promptValue: draft.prompt,
    openForAgent,
    onOpenChange: handleOpenChange,
    onModeChange,
    onAgentChange,
    onWorkspaceChange,
    onSessionNameChange,
    onWorkspacePathChange,
    onPromptChange,
    onRuntimeChange,
    onNetworkParticipationChange,
    networkParticipation: networkParticipationDraftFromValues(
      draft.networkParticipationMode,
      draft.networkChannelId,
      draft.networkChannelStrategy
    ),
    refreshCatalog: catalog.refresh,
    openProviderSettings,
    submit,
  };
}

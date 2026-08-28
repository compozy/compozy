import { useEffect } from "react";
import { useNavigate } from "@tanstack/react-router";
import { useSelector, useStore } from "@xstate/store-react";

import { useCreateAgent, useDuplicateAgent } from "./use-agents";
import { agentCreateDialogLogic } from "./agent-create-dialog-logic";
import {
  buildCreateAgentParams,
  buildDraftFromAgentPayload,
  buildDuplicateAgentParams,
  createDefaultAgentCreateDraft,
  updateAgentCreateScope,
  validateAgentCreateDraft,
  type AgentCreateDialogDraft,
} from "../lib/agent-create-draft";
import type { AgentPayload } from "../types";
import { type RuntimeCatalogProvider, useRuntimeModelCatalog } from "@/systems/model-catalog";
import type { RuntimeModelOption, RuntimeProviderOption } from "@/systems/runtime";
import { settingsProviderToOption, useSettingsProviders } from "@/systems/settings";
import {
  workspaceProviderToOption,
  type SessionProviderOption,
  type WorkspacePayload,
} from "@/systems/workspace";

interface AgentCreateDialogContext {
  activeWorkspace: WorkspacePayload | undefined;
  workspaceProviders: SessionProviderOption[];
  workspaceProvidersError: string | null;
  workspaceProvidersLoading: boolean;
}

export interface AgentCreateDialogState {
  open: boolean;
  draft: AgentCreateDialogDraft;
  providerOptions: RuntimeProviderOption[];
  providersLoading: boolean;
  providersError: string | null;
  runtimeModels: RuntimeModelOption[];
  modelCatalogLoading: boolean;
  modelCatalogRefreshing: boolean;
  modelCatalogError: string | null;
  submitError: string | null;
  isSubmitting: boolean;
  hasActiveWorkspace: boolean;
  workspaceId: string | null;
  workspaceName: string | null;
  mode: "create" | "duplicate";
  duplicateSourceName: string | null;
}

export interface AgentCreateDialogApi extends AgentCreateDialogState {
  openDialog: () => void;
  openForDuplicate: (agent: AgentPayload) => void;
  onDraftChange: (draft: AgentCreateDialogDraft) => void;
  onOpenChange: (open: boolean) => void;
  onRefreshCatalog: () => void;
  onOpenProviderSettings: () => void;
  onSubmit: () => void;
}

function describeError(fallback: string, error: unknown): string {
  if (error instanceof Error && error.message.trim().length > 0) {
    return error.message;
  }
  return fallback;
}

export function useAgentCreateDialog({
  activeWorkspace,
  workspaceProviders,
  workspaceProvidersError,
  workspaceProvidersLoading,
}: AgentCreateDialogContext): AgentCreateDialogApi {
  const navigate = useNavigate();
  const createAgent = useCreateAgent();
  const duplicateAgent = useDuplicateAgent();
  const settingsProviders = useSettingsProviders();
  const dialogStore = useStore(agentCreateDialogLogic, {
    initialDraft: createDefaultAgentCreateDraft(Boolean(activeWorkspace)),
  });
  const flow = useSelector(dialogStore, snapshot => snapshot.context);
  const storedDraft = flow.draft;

  const globalProviderEntries = settingsProviders.data?.providers;
  const globalProviders: RuntimeProviderOption[] =
    globalProviderEntries?.map(settingsProviderToOption) ?? [];

  const workspaceProviderOptions: RuntimeProviderOption[] =
    workspaceProviders.map(workspaceProviderToOption);

  // The menubar owns scope: the draft never freezes it, both flip directions included.
  const scopedDraft: AgentCreateDialogDraft = updateAgentCreateScope(
    storedDraft,
    activeWorkspace ? "workspace" : "global"
  );
  const sourceProviders =
    scopedDraft.scope === "workspace" ? workspaceProviders : (globalProviderEntries ?? []);
  const draft: AgentCreateDialogDraft =
    scopedDraft.provider.length > 0 &&
    !sourceProviders.some(provider => provider.name === scopedDraft.provider)
      ? { ...scopedDraft, provider: "", model: "", reasoningEffort: "" as const }
      : scopedDraft;
  const providerOptions: RuntimeProviderOption[] =
    draft.scope === "workspace" ? workspaceProviderOptions : globalProviders;

  const providersLoading =
    draft.scope === "workspace"
      ? workspaceProvidersLoading
      : settingsProviders.isLoading || settingsProviders.isFetching;
  const providersError =
    draft.scope === "workspace"
      ? workspaceProvidersError
      : settingsProviders.error
        ? describeError("Unable to load global provider settings.", settingsProviders.error)
        : null;

  const catalogProviders: RuntimeCatalogProvider[] = providerOptions.map(option => ({
    id: option.id,
    needsAuth: option.needs_auth,
  }));
  const open =
    flow.status === "editing" || flow.status === "failed" || flow.status === "submitting";
  const catalog = useRuntimeModelCatalog(catalogProviders, { enabled: open });
  const runtimeModels = catalog.models;

  const validationContext = {
    hasActiveWorkspace: Boolean(activeWorkspace),
    providerOptions,
    providersError,
    providersLoading,
  };

  const defaultDraft = () => createDefaultAgentCreateDraft(Boolean(activeWorkspace));
  const navigationTarget = flow.status === "navigation-pending" ? flow : null;

  useEffect(() => {
    if (!navigationTarget) return;
    const { agentName, attempt } = navigationTarget;
    dialogStore.trigger.navigationRequested({
      attempt,
      draft: createDefaultAgentCreateDraft(Boolean(activeWorkspace)),
      execute: async () => {
        await navigate({
          to: "/agents/$name",
          params: { name: agentName },
        });
      },
    });
  }, [activeWorkspace, dialogStore, navigate, navigationTarget]);

  const openDialog = () => {
    dialogStore.trigger.createOpened({ draft: defaultDraft() });
  };

  const openForDuplicate = (agent: AgentPayload) => {
    dialogStore.trigger.duplicateOpened({
      draft: buildDraftFromAgentPayload(agent, Boolean(activeWorkspace)),
      source: agent,
    });
  };

  const onOpenChange = (next: boolean) => {
    if (next) {
      if (flow.status === "closed") {
        openDialog();
      }
      return;
    }
    dialogStore.trigger.dialogDismissed({ draft: defaultDraft() });
  };

  const onDraftChange = (nextDraft: AgentCreateDialogDraft) => {
    dialogStore.trigger.draftChanged({ draft: nextDraft });
  };

  const onRefreshCatalog = catalog.refresh;

  const onOpenProviderSettings = () => {
    dialogStore.trigger.dialogDismissed({ draft: defaultDraft() });
    void navigate({ to: "/settings/providers" });
  };

  const onSubmit = () => {
    if (flow.status !== "editing" && flow.status !== "failed") {
      return;
    }
    let execute: () => Promise<AgentPayload>;
    let failureMessage: string;
    if (flow.kind === "duplicate") {
      const request = buildDuplicateAgentParams(
        flow.source,
        draft,
        activeWorkspace?.id,
        validationContext
      );
      if (!request) {
        const validation = validateAgentCreateDraft(draft, validationContext);
        const message =
          Object.values(validation.fields).find(field => field && field.length > 0) ??
          "Fix the highlighted fields before duplicating an agent.";
        dialogStore.trigger.submissionRejected({ error: message });
        return;
      }
      execute = () =>
        duplicateAgent.mutateAsync({
          sourceName: flow.source.name,
          params: request,
        });
      failureMessage = "Failed to duplicate agent.";
    } else {
      const request = buildCreateAgentParams(draft, activeWorkspace?.id, validationContext);
      if (!request) {
        const validation = validateAgentCreateDraft(draft, validationContext);
        const message =
          Object.values(validation.fields).find(field => field && field.length > 0) ??
          "Fix the highlighted fields before creating an agent.";
        dialogStore.trigger.submissionRejected({ error: message });
        return;
      }
      execute = () => createAgent.mutateAsync(request);
      failureMessage = "Failed to create agent.";
    }

    dialogStore.trigger.submissionRequested({
      execute,
      failureMessage,
    });
  };

  return {
    open,
    draft,
    providerOptions,
    providersLoading,
    providersError,
    runtimeModels,
    modelCatalogLoading: catalog.loading,
    modelCatalogRefreshing: catalog.refreshing,
    modelCatalogError: catalog.error,
    submitError: flow.status === "failed" ? flow.error : null,
    isSubmitting: flow.status === "submitting",
    hasActiveWorkspace: Boolean(activeWorkspace),
    workspaceId: activeWorkspace?.id ?? null,
    workspaceName: activeWorkspace?.name ?? null,
    mode: flow.status === "closed" ? "create" : flow.kind,
    duplicateSourceName:
      flow.status !== "closed" && flow.kind === "duplicate" ? flow.source.name : null,
    openDialog,
    openForDuplicate,
    onDraftChange,
    onOpenChange,
    onRefreshCatalog,
    onOpenProviderSettings,
    onSubmit,
  };
}

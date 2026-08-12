import { useSelector } from "@xstate/store-react";
import { useNavigate } from "@tanstack/react-router";

import { useStoreBinding } from "@/hooks/use-store-binding";

import {
  createAgentSettingsEditorLogic,
  shouldAdoptAgentSettingsSource,
  type AgentSettingsEditorState,
} from "../stores/agent-settings-editor-store";
import {
  isAgentSettingsDraftDirty,
  validateAgentSettingsDraft,
  type AgentSettingsDraft,
} from "../lib/agent-settings-draft";
import type { AgentSettingsSection } from "../lib/agent-settings-search";
import { useAgent, useUpdateAgent } from "./use-agents";
import { useAgentDeleteFlow } from "./use-agent-delete-flow";
import { useUnsavedGuard } from "./use-unsaved-guard";
import {
  providerNeedsAuth,
  type RuntimeCatalogProvider,
  useRuntimeModelCatalog,
} from "@/systems/model-catalog";
import type { RuntimeModelOption, RuntimeProviderOption } from "@/systems/runtime";
import { type SettingsProviderEntry, useSettingsProviders } from "@/systems/settings";
import { type SessionProviderOption, useActiveWorkspace, useWorkspace } from "@/systems/workspace";

function settingsProviderToOption(provider: SettingsProviderEntry): RuntimeProviderOption {
  const displayName = provider.settings.display_name?.trim();
  const harness = provider.settings.harness?.trim();
  const runtimeProvider = provider.settings.runtime_provider?.trim();
  return {
    id: provider.name,
    name: displayName || provider.name,
    ...(harness ? { harness } : {}),
    ...(runtimeProvider ? { runtime_provider: runtimeProvider } : {}),
    needs_auth: providerNeedsAuth(provider.auth_status?.state),
  };
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

export interface UseAgentSettingsPageOptions {
  name: string;
  section: AgentSettingsSection;
}

export function useAgentSettingsPage({ name, section }: UseAgentSettingsPageOptions) {
  const navigate = useNavigate();
  const { activeWorkspaceId, activeWorkspace } = useActiveWorkspace();
  const agentQuery = useAgent(name, activeWorkspaceId);
  const updateAgent = useUpdateAgent();
  const settingsProviders = useSettingsProviders();
  const workspaceDetail = useWorkspace(activeWorkspaceId ?? "", {
    enabled: activeWorkspaceId !== null,
  });

  const resourceKey = JSON.stringify([activeWorkspaceId, name]);
  const { store } = useStoreBinding(
    resourceKey,
    () =>
      createAgentSettingsEditorLogic().createStore({
        resourceKey,
        agent: agentQuery.data,
      }),
    () =>
      createAgentSettingsEditorLogic().createStore({
        resourceKey,
        agent: agentQuery.data,
      }),
    (current, nextResourceKey) =>
      current.key !== nextResourceKey ||
      shouldAdoptAgentSettingsSource(current.store.getSnapshot().context, agentQuery.data)
  );
  const editor = useSelector(store, snapshot => snapshot.context);

  const draft = editor.draft;
  const dirty = isEditorDirty(editor);
  const validation = draft ? validateAgentSettingsDraft(draft) : null;
  const canSave = Boolean(validation?.canSave);
  const mutationDenied = editor.phase === "denied";
  const editorError = "error" in editor ? editor.error : null;
  const saveBlocked = dirty && (!canSave || mutationDenied);
  const fieldErrorCount = validation ? Object.values(validation.fields).filter(Boolean).length : 0;

  const guard = useUnsavedGuard({ dirty, entityName: name });
  const deleteFlow = useAgentDeleteFlow({ agent: agentQuery.data, workspaceId: activeWorkspaceId });

  const useWorkspaceProviders = agentQuery.data?.origin === "workspace";
  const globalProviders = settingsProviders.data?.providers.map(settingsProviderToOption) ?? [];
  const workspaceProviders = (workspaceDetail.data?.providers ?? []).map(workspaceProviderToOption);
  const providerOptions = useWorkspaceProviders ? workspaceProviders : globalProviders;
  const providersLoading = useWorkspaceProviders
    ? activeWorkspaceId !== null && workspaceDetail.isLoading
    : settingsProviders.isLoading || settingsProviders.isFetching;

  const catalogProviders: RuntimeCatalogProvider[] = providerOptions.map(option => ({
    id: option.id,
    needsAuth: option.needs_auth,
  }));
  const catalog = useRuntimeModelCatalog(catalogProviders, { enabled: Boolean(agentQuery.data) });

  const saveBlockedCaption = mutationDenied
    ? "Editing is not permitted for this agent."
    : saveBlocked && fieldErrorCount > 0
      ? `Fix ${fieldErrorCount} field${fieldErrorCount === 1 ? "" : "s"} before saving`
      : undefined;

  return {
    agent: agentQuery.data,
    agentLoading: agentQuery.isLoading,
    agentError: (agentQuery.error as Error | null) ?? null,
    draft,
    setDraft: (next: AgentSettingsDraft) => store.trigger.draftReplaced({ draft: next }),
    patchDraft: (patch: Partial<AgentSettingsDraft>) => store.trigger.draftPatched({ patch }),
    dirty,
    validation,
    canSave,
    saveBlocked,
    saveBlockedCaption,
    section,
    setSection: (next: AgentSettingsSection) => {
      void navigate({
        to: "/agents/$name/settings",
        params: { name },
        search: { section: next },
        replace: true,
      });
    },
    onSave: () =>
      store.trigger.saveRequested({
        name,
        save: updateAgent.mutateAsync,
        workspaceId: activeWorkspaceId,
      }),
    onDiscard: () => store.trigger.discardRequested(),
    onReloadAndRetry: () =>
      store.trigger.reloadRequested({ reload: async () => (await agentQuery.refetch()).data }),
    onBackToDetail: () => void navigate({ to: "/agents/$name", params: { name } }),
    onOpenProviderSettings: () => void navigate({ to: "/settings/providers" }),
    phase: editor.phase,
    error: editorError,
    providerOptions,
    providersLoading,
    runtimeModels: catalog.models as RuntimeModelOption[],
    modelCatalogLoading: catalog.loading,
    modelCatalogRefreshing: catalog.refreshing,
    modelCatalogError: catalog.error,
    onRefreshCatalog: catalog.refresh,
    workspaceName: activeWorkspace?.name ?? null,
    deleteFlow,
    unsavedGuardDialog: guard.confirmDialog,
  };
}

function isEditorDirty(editor: AgentSettingsEditorState): boolean {
  return Boolean(
    editor.baseline && editor.draft && isAgentSettingsDraftDirty(editor.draft, editor.baseline)
  );
}

import { useState } from "react";
import { useNavigate } from "@tanstack/react-router";
import { toast } from "sonner";

import {
  providerNeedsAuth,
  useRuntimeModelCatalog,
  type RuntimeCatalogProvider,
} from "@/systems/model-catalog";
import type { RuntimeModelOption, RuntimeProviderOption } from "@/systems/runtime";
import { useSettingsProviders, type SettingsProviderEntry } from "@/systems/settings";
import type { SessionProviderOption } from "@/systems/workspace";
import { useActiveWorkspace, useWorkspace } from "@/systems/workspace";

import { isAgentDigestConflict } from "../adapters/agent-api";
import {
  buildSettingsDraftFromAgent,
  buildUpdateAgentParams,
  isAgentSettingsDraftDirty,
  validateAgentSettingsDraft,
  type AgentSettingsDraft,
} from "../lib/agent-settings-draft";
import type { AgentSettingsSection } from "../lib/agent-settings-search";
import type { AgentPayload } from "../types";
import { useAgent, useUpdateAgent } from "./use-agents";
import { useAgentDeleteFlow } from "./use-agent-delete-flow";
import { useUnsavedGuard } from "./use-unsaved-guard";

function describeError(fallback: string, error: unknown): string {
  if (error instanceof Error && error.message.trim().length > 0) {
    return error.message;
  }
  return fallback;
}

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

  const agent = agentQuery.data;
  const agentKey = agent?.definition_digest ?? "pending";
  const [editorState, setEditorState] = useState<{
    baseline: AgentPayload | null;
    draft: AgentSettingsDraft | null;
    key: string;
  }>({ baseline: null, draft: null, key: agentKey });
  const baselineAgent =
    editorState.key === agentKey ? (editorState.baseline ?? agent ?? null) : (agent ?? null);
  const draft =
    editorState.key === agentKey
      ? (editorState.draft ?? (agent ? buildSettingsDraftFromAgent(agent) : null))
      : agent
        ? buildSettingsDraftFromAgent(agent)
        : null;
  const [conflictBanner, setConflictBanner] = useState<string | null>(null);
  const [mutationDenied, setMutationDenied] = useState(false);
  const [saveError, setSaveError] = useState<string | null>(null);

  const draftSafe = draft;
  const dirty = Boolean(
    baselineAgent && draftSafe && isAgentSettingsDraftDirty(draftSafe, baselineAgent)
  );
  const validation = draftSafe ? validateAgentSettingsDraft(draftSafe) : null;
  const canSave = Boolean(validation?.canSave);
  const saveBlocked = dirty && (!canSave || mutationDenied);
  const fieldErrorCount = validation ? Object.values(validation.fields).filter(Boolean).length : 0;

  const guard = useUnsavedGuard({ dirty, entityName: name });
  const deleteFlow = useAgentDeleteFlow({ agent, workspaceId: activeWorkspaceId });

  const useWorkspaceProviders = agent?.origin === "workspace";
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
  const catalog = useRuntimeModelCatalog(catalogProviders, { enabled: Boolean(agent) });

  const setDraft = (next: AgentSettingsDraft) => {
    setEditorState({ baseline: baselineAgent, draft: next, key: agentKey });
    setSaveError(null);
    setConflictBanner(null);
  };

  const patchDraft = (patch: Partial<AgentSettingsDraft>) => {
    setEditorState({
      baseline: baselineAgent,
      draft: draftSafe ? { ...draftSafe, ...patch } : null,
      key: agentKey,
    });
    setSaveError(null);
    setConflictBanner(null);
  };

  const onDiscard = () => {
    if (!baselineAgent) return;
    setEditorState({
      baseline: baselineAgent,
      draft: buildSettingsDraftFromAgent(baselineAgent),
      key: baselineAgent.definition_digest,
    });
    setSaveError(null);
    setConflictBanner(null);
  };

  const onReloadAndRetry = async () => {
    setConflictBanner(null);
    setSaveError(null);
    const result = await agentQuery.refetch();
    if (result.data) {
      setEditorState({
        baseline: result.data,
        draft: buildSettingsDraftFromAgent(result.data),
        key: result.data.definition_digest,
      });
    }
  };

  const onSave = () => {
    if (!draftSafe || !agent || saveBlocked || updateAgent.isPending) return;
    const params = buildUpdateAgentParams(draftSafe, activeWorkspaceId);
    if (!params) return;

    setSaveError(null);
    setConflictBanner(null);
    setMutationDenied(false);

    updateAgent.mutate(
      { name: agent.name, params, cacheWorkspace: activeWorkspaceId },
      {
        onSuccess: updated => {
          setEditorState({
            baseline: updated,
            draft: buildSettingsDraftFromAgent(updated),
            key: updated.definition_digest,
          });
          toast.success("Changes saved");
        },
        onError: error => {
          if (isAgentDigestConflict(error)) {
            setConflictBanner(
              "This agent changed elsewhere. Reload the latest definition, then retry your edits."
            );
            return;
          }
          const status =
            error && typeof error === "object" && "status" in error
              ? Number((error as { status?: number }).status)
              : NaN;
          if (status === 403) {
            setMutationDenied(true);
            return;
          }
          setSaveError(describeError("Couldn't save agent", error));
        },
      }
    );
  };

  const setSection = (next: AgentSettingsSection) => {
    void navigate({
      to: "/agents/$name/settings",
      params: { name },
      search: { section: next },
      replace: true,
    });
  };

  const onBackToDetail = () => {
    void navigate({
      to: "/agents/$name",
      params: { name },
    });
  };

  const onOpenProviderSettings = () => {
    void navigate({ to: "/settings/providers" });
  };

  const saveBlockedCaption = mutationDenied
    ? "Editing is not permitted for this agent."
    : saveBlocked && fieldErrorCount > 0
      ? `Fix ${fieldErrorCount} field${fieldErrorCount === 1 ? "" : "s"} before saving`
      : undefined;

  return {
    agent,
    agentLoading: agentQuery.isLoading,
    agentError: (agentQuery.error as Error | null) ?? null,
    draft: draftSafe,
    setDraft,
    patchDraft,
    dirty,
    validation,
    canSave,
    saveBlocked,
    saveBlockedCaption,
    section,
    setSection,
    onSave,
    onDiscard,
    onReloadAndRetry,
    onBackToDetail,
    onOpenProviderSettings,
    isSaving: updateAgent.isPending,
    saveError,
    conflictBanner,
    mutationDenied,
    fieldsDisabled: updateAgent.isPending,
    fieldsReadOnly: mutationDenied,
    providerOptions,
    providersLoading,
    runtimeModels: catalog.models as RuntimeModelOption[],
    modelCatalogLoading: catalog.loading,
    modelCatalogLoaded: catalog.loaded,
    modelCatalogRefreshing: catalog.refreshing,
    modelCatalogError: catalog.error,
    onRefreshCatalog: catalog.refresh,
    workspaceName: activeWorkspace?.name ?? null,
    deleteFlow,
    unsavedGuardDialog: guard.confirmDialog,
  };
}

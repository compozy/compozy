import { useState, type SetStateAction } from "react";

import { useSettingsPage } from "./use-settings-page";
import {
  SettingsApiError,
  useReloadSettings,
  useSettingsApplyRecords,
  useSettingsGeneral,
  useSettingsUpdate,
  useUpdateSettingsGeneral,
  type SettingsGeneralSection,
  type SettingsUpdateGeneralRequest,
} from "@/systems/settings";
import { useActiveWorkspace } from "@/systems/workspace";

type GeneralConfig = SettingsGeneralSection["config"];

function applyResultLabel(result: {
  active_generation: number;
  next_action: string;
  restart_required?: boolean;
  skipped?: boolean;
  skipped_reason?: string;
}) {
  if (result.skipped) {
    return result.skipped_reason ?? "No config changes detected";
  }
  if (result.restart_required || result.next_action === "restart-daemon") {
    return "Saved · restart required to apply";
  }
  if (result.next_action === "new-session") {
    return "Saved · new sessions use this config";
  }
  if (result.next_action === "retry") {
    return "Saved · reload required";
  }
  return `Saved · active generation ${result.active_generation}`;
}

export function useSettingsGeneralPage() {
  const query = useSettingsGeneral();
  const update = useSettingsUpdate();
  const applyRecords = useSettingsApplyRecords({ limit: 8 });
  const mutation = useUpdateSettingsGeneral();
  const reload = useReloadSettings();
  const page = useSettingsPage({ currentSlug: "general" });
  const { activeWorkspaceId } = useActiveWorkspace();

  const envelope = query.data ?? null;
  const workspaceContextKey = activeWorkspaceId ?? "__none__";

  const [draftState, setDraftState] = useState<{
    draft: GeneralConfig | null;
    workspaceKey: string;
  }>({ draft: null, workspaceKey: workspaceContextKey });
  const draft =
    draftState.workspaceKey === workspaceContextKey
      ? (draftState.draft ?? envelope?.config ?? null)
      : (envelope?.config ?? null);
  const setDraft = (update: SetStateAction<GeneralConfig | null>) => {
    setDraftState(current => {
      const currentDraft =
        current.workspaceKey === workspaceContextKey
          ? (current.draft ?? envelope?.config ?? null)
          : (envelope?.config ?? null);
      return {
        draft: typeof update === "function" ? update(currentDraft) : update,
        workspaceKey: workspaceContextKey,
      };
    });
  };
  const [lastAppliedState, setLastAppliedState] = useState<{
    label: string | null;
    workspaceKey: string;
  }>({ label: null, workspaceKey: workspaceContextKey });
  const lastAppliedLabel =
    lastAppliedState.workspaceKey === workspaceContextKey ? lastAppliedState.label : null;
  const setLastAppliedLabel = (label: string | null) => {
    setLastAppliedState({ label, workspaceKey: workspaceContextKey });
  };

  const isDirty =
    envelope && draft ? JSON.stringify(envelope.config) !== JSON.stringify(draft) : false;

  const handleReset = () => {
    if (envelope) {
      setDraft(envelope.config);
    }
  };

  const handleSave = () => {
    if (!draft) return;
    const body: SettingsUpdateGeneralRequest = { config: draft };
    mutation.mutate(body, {
      onSuccess: result => {
        setLastAppliedLabel(applyResultLabel(result));
      },
    });
  };

  const handleReload = () => {
    reload.mutate(undefined, {
      onSuccess: result => {
        setLastAppliedLabel(applyResultLabel(result));
      },
    });
  };

  const saveError =
    mutation.error instanceof SettingsApiError
      ? mutation.error.message
      : mutation.error instanceof Error
        ? mutation.error.message
        : null;

  const handleRetry = () => {
    void query.refetch();
  };

  return {
    isLoading: query.isLoading,
    error: query.error,
    envelope,
    draft,
    setDraft,
    isDirty,
    handleReset,
    handleSave,
    isSaving: mutation.isPending,
    saveError,
    warnings: mutation.data?.warnings,
    lastAppliedLabel,
    handleRetry,
    restart: page.restart,
    update,
    applyRecords,
    handleReload,
    isReloading: reload.isPending,
    reloadError:
      reload.error instanceof SettingsApiError
        ? reload.error.message
        : reload.error instanceof Error
          ? reload.error.message
          : null,
    reloadResult: reload.data ?? null,
  };
}

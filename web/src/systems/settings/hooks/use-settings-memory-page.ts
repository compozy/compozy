import { useState, type SetStateAction } from "react";

import { useSettingsPage } from "./use-settings-page";
import { useTriggerMemoryDream } from "@/systems/knowledge";
import {
  SettingsApiError,
  useSettingsMemory,
  useUpdateSettingsMemory,
  type SettingsMemorySection,
  type SettingsUpdateMemoryRequest,
} from "@/systems/settings";

type MemoryConfig = SettingsMemorySection["config"];

export function useSettingsMemoryPage() {
  const query = useSettingsMemory();
  const mutation = useUpdateSettingsMemory();
  const triggerDream = useTriggerMemoryDream();
  const page = useSettingsPage({ currentSlug: "memory" });

  const envelope = query.data ?? null;

  const [draftOverride, setDraftOverride] = useState<MemoryConfig | null>();
  const [lastAppliedLabel, setLastAppliedLabel] = useState<string | null>(null);
  const [actionMessage, setActionMessage] = useState<string | null>(null);
  const draft = draftOverride === undefined ? (envelope?.config ?? null) : draftOverride;
  const setDraft = (update: SetStateAction<MemoryConfig | null>) => {
    setDraftOverride(current => {
      const resolved = current === undefined ? (envelope?.config ?? null) : current;
      return typeof update === "function" ? update(resolved) : update;
    });
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
    const body: SettingsUpdateMemoryRequest = { config: draft };
    mutation.mutate(body, {
      onSuccess: result => {
        setLastAppliedLabel(
          result.restart_required
            ? "Saved · restart required to apply"
            : "Saved · applied immediately"
        );
      },
    });
  };

  const handleTriggerDream = () => {
    setActionMessage(null);
    triggerDream.mutate(
      {},
      {
        onSuccess: response => {
          setActionMessage(
            response.triggered ? "Dream triggered" : response.reason || "Dream not triggered"
          );
        },
        onError: error => {
          setActionMessage(
            error instanceof Error ? error.message : "Failed to trigger memory dream"
          );
        },
      }
    );
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
    handleTriggerDream,
    isTriggeringDream: triggerDream.isPending,
    actionMessage,
    handleRetry,
    restart: page.restart,
  };
}

import { useState, type SetStateAction } from "react";

import { useSettingsPage } from "./use-settings-page";
import {
  SettingsApiError,
  useSettingsAutomation,
  useUpdateSettingsAutomation,
  type SettingsAutomationSection,
  type SettingsUpdateAutomationRequest,
} from "@/systems/settings";

type AutomationConfig = SettingsAutomationSection["config"];

export function useSettingsAutomationPage() {
  const query = useSettingsAutomation();
  const mutation = useUpdateSettingsAutomation();
  const page = useSettingsPage({ currentSlug: "automation" });

  const envelope = query.data ?? null;

  const [draftOverride, setDraftOverride] = useState<AutomationConfig | null>();
  const [lastAppliedLabel, setLastAppliedLabel] = useState<string | null>(null);
  const draft = draftOverride === undefined ? (envelope?.config ?? null) : draftOverride;
  const setDraft = (update: SetStateAction<AutomationConfig | null>) => {
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
    const body: SettingsUpdateAutomationRequest = { config: draft };
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
  };
}

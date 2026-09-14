import { useState } from "react";

import {
  marketplaceCatalogDraftInvalid,
  sameMarketplaceCatalogConfig,
  type SettingsMarketplaceCatalogConfig,
} from "../components/marketplace-catalog-draft";
import { useSettingsMarketplace, useUpdateSettingsMarketplace } from "./use-settings-marketplace";
import { useSettingsPage } from "./use-settings-page";

/**
 * Local draft over the catalog settings envelope: the page edits a copy, Save sends the whole
 * config through the section's apply semantics, Discard returns to the envelope.
 */
export function useSettingsMarketplacePage() {
  const query = useSettingsMarketplace();
  const mutation = useUpdateSettingsMarketplace();
  const page = useSettingsPage({ currentSlug: "marketplace" });
  const [draftOverride, setDraftOverride] = useState<SettingsMarketplaceCatalogConfig | null>(null);
  const [lastAppliedLabel, setLastAppliedLabel] = useState<string | null>(null);

  const envelope = query.data ?? null;
  const draft = draftOverride ?? envelope?.config ?? null;
  const isDirty =
    envelope !== null && draft !== null && !sameMarketplaceCatalogConfig(envelope.config, draft);

  return {
    isLoading: query.isPending,
    error: query.error,
    envelope,
    draft,
    setDraft: (next: SettingsMarketplaceCatalogConfig) => setDraftOverride(next),
    isDirty,
    isInvalid: draft !== null && marketplaceCatalogDraftInvalid(draft),
    isSaving: mutation.isPending,
    saveError: mutation.error instanceof Error ? mutation.error.message : null,
    warnings: mutation.data?.warnings,
    lastAppliedLabel,
    restart: page.restart,
    handleReset: () => setDraftOverride(null),
    handleRetry: () => void query.refetch(),
    handleSave: () => {
      if (draft === null || !isDirty) return;
      mutation.mutate(
        { config: draft },
        {
          onSuccess: result => {
            setDraftOverride(null);
            setLastAppliedLabel(
              result.restart_required
                ? "Saved · restart required to apply"
                : "Saved · applied immediately"
            );
          },
        }
      );
    },
  };
}

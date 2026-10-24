import { useState } from "react";

import { useProfileReadScope } from "@/systems/profiles";
import { useProviderUsage } from "@/systems/provider-usage";

import {
  PROVIDER_USAGE_COLLECTOR_IDS,
  sameProviderUsageConfig,
  type ProviderUsageSettingsConfig,
} from "../lib/provider-usage-settings";
import { useSettingsProviders } from "./use-settings-collections";
import { useSettingsPage } from "./use-settings-page";
import {
  useSettingsProviderUsage,
  useUpdateSettingsProviderUsage,
} from "./use-settings-provider-usage";

export interface ProviderUsageSettingsProvider {
  id: string;
  name: string;
}

/**
 * Configured providers with a usage collector, in collector order, plus any
 * the config already names (a provider turned off stays listed so it can be
 * turned back on). Names follow the usage rows — the daemon's catalog display
 * name — then the provider's own display name, then its ID.
 */
function usageProviders(
  configured: ReadonlyArray<{ name: string; settings?: { display_name?: string } }>,
  config: ProviderUsageSettingsConfig | null,
  catalogNames: ReadonlyMap<string, string>
): ProviderUsageSettingsProvider[] {
  const names = new Map(configured.map(entry => [entry.name, entry.settings?.display_name]));
  const named = new Set(Object.keys(config?.providers ?? {}));
  const providers: ProviderUsageSettingsProvider[] = [];
  for (const id of PROVIDER_USAGE_COLLECTOR_IDS) {
    if (!names.has(id) && !named.has(id)) continue;
    providers.push({ id, name: catalogNames.get(id) || names.get(id)?.trim() || id });
  }
  return providers;
}

/** Display names the usage rows show, by provider ID. */
function useUsageCatalogNames(profile: string): ReadonlyMap<string, string> {
  const { viewModel } = useProviderUsage(profile);
  return new Map(
    (viewModel?.rows ?? []).map(row => [row.providerId, row.target.provider_name.trim()])
  );
}

/**
 * Local draft over the provider usage section: the page edits a copy, Save
 * sends the whole config through the apply lifecycle, Discard returns to the
 * envelope. User scope only — the daemon rejects these keys in overlays.
 */
export function useSettingsProviderUsagePage() {
  const query = useSettingsProviderUsage();
  const providersQuery = useSettingsProviders();
  const mutation = useUpdateSettingsProviderUsage();
  // The shell's active profile: usage names come from it, and a consent
  // change collects for it.
  const profile = useProfileReadScope().destination;
  const catalogNames = useUsageCatalogNames(profile);
  const page = useSettingsPage({ currentSlug: "provider-usage" });
  const [draftOverride, setDraftOverride] = useState<ProviderUsageSettingsConfig | null>(null);
  const [lastAppliedLabel, setLastAppliedLabel] = useState<string | null>(null);

  const envelope = query.data ?? null;
  const draft = draftOverride ?? envelope?.config ?? null;
  const isDirty =
    envelope !== null && draft !== null && !sameProviderUsageConfig(envelope.config, draft);

  return {
    // One loading area: the section and the provider list its rows come from arrive together, so
    // the Providers group is never inserted into a page that has already rendered.
    isLoading: query.isPending || providersQuery.isPending,
    error: query.error,
    envelope,
    draft,
    providers: usageProviders(providersQuery.data?.providers ?? [], draft, catalogNames),
    setDraft: (next: ProviderUsageSettingsConfig) => setDraftOverride(next),
    isDirty,
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
        { body: { config: draft }, collectionProfile: profile },
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

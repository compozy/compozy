import { useState } from "react";

import { useSettingsPage } from "./use-settings-page";
import {
  SettingsApiError,
  useSettingsHooksExtensions,
  useUpdateSettingsHooksExtensions,
  type SettingsHooksExtensionsSection,
  type SettingsUpdateHooksExtensionsRequest,
} from "@/systems/settings";

type PolicyConfig = SettingsHooksExtensionsSection["config"];

function clonePolicy(config: PolicyConfig): PolicyConfig {
  return {
    marketplace: { ...config.marketplace },
    resources: {
      ...config.resources,
      allowed_kinds: [...(config.resources.allowed_kinds ?? [])],
      operator_write_rate_limit: { ...config.resources.operator_write_rate_limit },
      snapshot_rate_limit: { ...config.resources.snapshot_rate_limit },
    },
  };
}

function sameMarketplace(a: PolicyConfig, b: PolicyConfig): boolean {
  return (
    (a.marketplace.registry ?? "") === (b.marketplace.registry ?? "") &&
    (a.marketplace.base_url ?? "") === (b.marketplace.base_url ?? "") &&
    a.marketplace.allow_unverified === b.marketplace.allow_unverified
  );
}

function errorMessage(error: unknown): string | null {
  if (error instanceof SettingsApiError || error instanceof Error) return error.message;
  return null;
}

export function useSettingsExtensionsPage() {
  const query = useSettingsHooksExtensions();
  const mutation = useUpdateSettingsHooksExtensions();
  const page = useSettingsPage({ currentSlug: "extensions" });
  const envelope = query.data ?? null;
  const [draftOverride, setDraftOverride] = useState<PolicyConfig | undefined>();
  const draft = draftOverride ?? (envelope ? clonePolicy(envelope.config) : null);
  const isPolicyDirty = Boolean(envelope && draft && !sameMarketplace(envelope.config, draft));
  const handleSavePolicy = () => {
    if (!draft) return;
    const body: SettingsUpdateHooksExtensionsRequest = { config: draft };
    mutation.mutate(body);
  };
  return {
    canMutatePolicy: envelope?.transport_parity?.settings_http !== false,
    draft,
    envelope,
    error: query.error,
    handleResetPolicy: () => {
      if (envelope) setDraftOverride(clonePolicy(envelope.config));
      mutation.reset();
    },
    handleRetry: () => void query.refetch(),
    handleSavePolicy,
    isLoading: query.isLoading,
    isPolicyDirty,
    isSavingPolicy: mutation.isPending,
    policyWarnings: mutation.data?.warnings,
    restart: page.restart,
    savePolicyError: errorMessage(mutation.error),
    updatePolicyDraft: (updater: (current: PolicyConfig) => PolicyConfig) => {
      if (draft) setDraftOverride(updater(draft));
    },
  };
}

import type { EntityMode } from "@agh/ui";

import type { ProviderDraft, SettingsProviderEntry } from "../types";
import { ProviderAuthFields } from "./provider-edit-form-auth-fields";
import { ProviderGeneralFields } from "./provider-edit-form-general-fields";
import { ProviderRuntimeFields } from "./provider-edit-form-runtime-fields";

export type ProviderDraftChange = (updater: (draft: ProviderDraft) => ProviderDraft) => void;

interface ProviderEditFormProps {
  mode: "create" | "edit";
  tier: EntityMode;
  draft: ProviderDraft;
  entry: SettingsProviderEntry | null;
  onChange: ProviderDraftChange;
}

/**
 * Provider body grammar: basics, auth ownership, and — in Advanced — runtime
 * and models. Advanced appends; nothing required disappears when it closes.
 */
export function ProviderEditForm({ mode, tier, draft, entry, onChange }: ProviderEditFormProps) {
  return (
    <div className="flex flex-col">
      <ProviderGeneralFields draft={draft} mode={mode} onChange={onChange} />
      <ProviderAuthFields draft={draft} entry={entry} mode={mode} onChange={onChange} />
      {tier === "advanced" ? <ProviderRuntimeFields draft={draft} onChange={onChange} /> : null}
    </div>
  );
}

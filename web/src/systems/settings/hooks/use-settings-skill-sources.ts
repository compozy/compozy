import { useEffect, useState } from "react";

import { SettingsApiError, type SettingsErrorDetail } from "../adapters/settings-api-error";
import {
  addSkillSourceEntry,
  removeSkillSourceEntry,
  toggleSkillSourcePreset,
  validateSkillSourceEntry,
  type SkillSourceEntryError,
} from "../lib/skill-source-draft";
import {
  sameSourcesConfig,
  sameStringList,
  skillsSourcesConfig,
} from "../lib/settings-skills-save";
import { groupSkillSources, type SkillSourceGroups } from "../lib/skill-sources-view";
import type {
  SettingsSkillsSection,
  SettingsSkillSourceInherits,
  SettingsUpdateSkillsRequest,
  SettingsUpdateSkillsFilter,
} from "../types";
import { useUpdateSettingsSkills } from "./use-settings-mutations";
import type { SkillsDraftUpdate } from "./settings-skills-draft-logic";

type SkillsConfig = SettingsSkillsSection["config"];
export type SkillSourceKey = "sources" | "custom_sources";

const SOURCE_KEYS = ["sources", "custom_sources"] as const;
const NO_INHERITANCE: SettingsSkillSourceInherits = { sources: false, custom_sources: false };
const NOTHING_ARMED: Record<SkillSourceKey, boolean> = { sources: false, custom_sources: false };

export interface SkillSourceKeyPosture {
  key: SkillSourceKey;
  inherited: boolean;
  /** True once this workspace owns the key — already saved, or about to be. */
  armed: boolean;
}

export interface SettingsSkillSourcesModel {
  groups: SkillSourceGroups;
  enabledPresets: readonly string[];
  customEntries: readonly string[];
  /** Agent and repository-profile projections read source policy but never write it. */
  readOnly: boolean;
  readOnlyReason: "agent" | "repository-profile" | null;
  /** Per-key inheritance, at workspace scope only. */
  postures: SkillSourceKeyPosture[] | null;
  isDirty: boolean;
  isSaving: boolean;
  saveError: string | null;
  saveErrorCode: string | null;
  lastLabel: string | null;
  togglePreset: (slug: string, enabled: boolean) => void;
  addCustom: (path: string) => void;
  removeCustom: (path: string) => void;
  validateEntry: (entry: string) => SkillSourceEntryError | null;
  customize: (key: SkillSourceKey) => void;
  useInherited: (key: SkillSourceKey) => void;
  inheritPendingKey: SkillSourceKey | null;
  inheritError: string | null;
  save: () => void;
  reset: () => void;
}

interface SettingsSkillSourcesInput {
  envelope: SettingsSkillsSection | null;
  draft: SkillsConfig | null;
  filter: SettingsUpdateSkillsFilter;
  /** Identity of the scope being edited; arming resets when it changes. */
  scopeKey: string;
  isSaving: boolean;
  lastLabel: string | null;
  onDraftChange: (update: SkillsDraftUpdate) => void;
  onSaveRequested: (input: {
    baseline: SkillsConfig;
    execute: () => Promise<unknown>;
    label: string;
  }) => void;
}

interface ArmState {
  scopeKey: string;
  keys: Record<SkillSourceKey, boolean>;
}

const APPLIED_LABEL = "Saved · applied immediately";

function errorMessage(error: unknown): string | null {
  return error instanceof Error ? error.message : null;
}

function errorDetail(error: unknown): SettingsErrorDetail | undefined {
  return error instanceof SettingsApiError ? error.detail : undefined;
}

function pendingInheritKey(override: unknown): SkillSourceKey | null {
  if (override == null || typeof override !== "object") return null;
  for (const key of SOURCE_KEYS) {
    if (Reflect.get(override, key) === null) return key;
  }
  return null;
}

/**
 * Source policy for one scope.
 *
 * At user and profile scope a save writes the full config. At workspace scope it
 * writes the presence-aware override: a key this workspace never claimed stays
 * absent from the body, so the daemon leaves it inheriting. Returning a key to
 * inheritance is its own immediate action rather than a drafted state, because a
 * drafted "inherit" would have to display a parent value nobody has reported yet.
 */
export function useSettingsSkillSources(
  input: SettingsSkillSourcesInput
): SettingsSkillSourcesModel {
  const { envelope, draft, filter, scopeKey } = input;
  const workspaceScope = envelope?.scope === "workspace";
  const repositoryProfileScope =
    filter.scope === "profile" && typeof filter.workspace_id === "string";
  const readOnlyReason =
    envelope?.scope === "agent"
      ? ("agent" as const)
      : repositoryProfileScope
        ? ("repository-profile" as const)
        : null;
  const readOnly = readOnlyReason !== null;
  const sourcesMutation = useUpdateSettingsSkills();
  const inheritMutation = useUpdateSettingsSkills();
  const resetSourcesMutation = sourcesMutation.reset;
  const resetInheritMutation = inheritMutation.reset;
  const [armState, setArmState] = useState<ArmState>({ scopeKey, keys: NOTHING_ARMED });
  const armed = armState.scopeKey === scopeKey ? armState.keys : NOTHING_ARMED;

  useEffect(() => {
    resetSourcesMutation();
    resetInheritMutation();
  }, [scopeKey, resetSourcesMutation, resetInheritMutation]);

  const inherits = envelope?.inherits ?? NO_INHERITANCE;
  const baseline = envelope?.config ?? null;
  const isDirty = baseline !== null && draft !== null && !sameSourcesConfig(baseline, draft);

  const postures: SkillSourceKeyPosture[] | null =
    workspaceScope && baseline !== null && draft !== null
      ? SOURCE_KEYS.map(key => ({
          key,
          inherited: inherits[key],
          armed: !inherits[key] || armed[key] || !sameStringList(baseline[key], draft[key]),
        }))
      : null;

  const setArmed = (key: SkillSourceKey, value: boolean) => {
    setArmState(current => {
      const keys = current.scopeKey === scopeKey ? current.keys : NOTHING_ARMED;
      return { scopeKey, keys: { ...keys, [key]: value } };
    });
  };

  const editDraft = (key: SkillSourceKey, next: string[]) => {
    if (readOnly || draft === null) return;
    if (workspaceScope) setArmed(key, true);
    input.onDraftChange({ ...draft, [key]: next });
  };

  const saveFullConfig = () => {
    if (baseline === null || draft === null) return;
    const config = skillsSourcesConfig(baseline, draft);
    input.onSaveRequested({
      baseline: config,
      execute: () => sourcesMutation.mutateAsync({ body: { config }, filter }),
      label: APPLIED_LABEL,
    });
  };

  const saveWorkspaceOverride = () => {
    if (baseline === null || draft === null) return;
    const override: NonNullable<SettingsUpdateSkillsRequest["override"]> = {};
    for (const posture of postures ?? []) {
      if (posture.armed) override[posture.key] = [...draft[posture.key]];
    }
    if (Object.keys(override).length === 0) return;
    input.onSaveRequested({
      baseline: skillsSourcesConfig(baseline, draft),
      execute: () => sourcesMutation.mutateAsync({ body: { override }, filter }),
      label: APPLIED_LABEL,
    });
  };

  const useInherited = (key: SkillSourceKey) => {
    if (envelope === null || readOnly || !workspaceScope || inheritMutation.isPending) return;
    void inheritMutation
      .mutateAsync({ body: { override: { [key]: null } }, filter })
      .then(() => setArmed(key, false))
      .catch(() => {
        // The mutation's error state carries the failure; nothing was applied.
      });
  };

  const saveDetail = errorDetail(sourcesMutation.error);
  const measuredGroups = groupSkillSources(
    envelope?.sources ?? [],
    envelope?.runtime_available ?? false
  );
  const enabledPresets = new Set(draft?.sources ?? []);
  const customEntries = new Set(draft?.custom_sources ?? []);
  const groups: SkillSourceGroups = {
    presets: measuredGroups.presets.map(source =>
      source.alwaysOn ? source : { ...source, enabled: enabledPresets.has(source.slug) }
    ),
    custom: measuredGroups.custom.filter(
      source => source.path !== null && customEntries.has(source.path)
    ),
  };
  return {
    groups,
    enabledPresets: draft?.sources ?? [],
    customEntries: draft?.custom_sources ?? [],
    readOnly,
    readOnlyReason,
    postures,
    isDirty,
    isSaving: input.isSaving,
    saveError: saveDetail?.message ?? errorMessage(sourcesMutation.error),
    saveErrorCode: saveDetail?.code ?? null,
    lastLabel: input.lastLabel,
    togglePreset: (slug, enabled) =>
      editDraft("sources", toggleSkillSourcePreset(draft?.sources ?? [], slug, enabled)),
    addCustom: path =>
      editDraft("custom_sources", addSkillSourceEntry(draft?.custom_sources ?? [], path)),
    removeCustom: path =>
      editDraft("custom_sources", removeSkillSourceEntry(draft?.custom_sources ?? [], path)),
    validateEntry: entry =>
      validateSkillSourceEntry(entry, {
        customEntries: draft?.custom_sources ?? [],
        sources: envelope?.sources ?? [],
        workspaceScope,
      }),
    customize: key => {
      if (envelope === null || readOnly) return;
      setArmed(key, true);
    },
    useInherited,
    inheritPendingKey: inheritMutation.isPending
      ? pendingInheritKey(inheritMutation.variables?.body.override)
      : null,
    inheritError: errorMessage(inheritMutation.error),
    save: () => {
      if (envelope === null || draft === null || readOnly || !isDirty) return;
      if (workspaceScope) saveWorkspaceOverride();
      else saveFullConfig();
    },
    reset: () => {
      if (baseline === null || draft === null || readOnly) return;
      input.onDraftChange({
        ...draft,
        sources: [...baseline.sources],
        custom_sources: [...baseline.custom_sources],
      });
      setArmState({ scopeKey, keys: NOTHING_ARMED });
    },
  };
}

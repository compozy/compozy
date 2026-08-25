import { useSelector } from "@xstate/store-react";

import { useStoreBinding } from "@/hooks/use-store-binding";

import { useSettingsPage } from "./use-settings-page";

import { SettingsApiError } from "../adapters/settings-api";
import { skillsDisabledConfig, skillsPolicyConfig } from "../lib/settings-skills-save";
import type { SettingsSkillsSection } from "../types";
import { useSettingsSkills } from "./use-settings-sections";
import { useUpdateSettingsSkills } from "./use-settings-mutations";

import {
  settingsSkillsDraftLogic,
  shouldRebindSkillsDraft,
  type SkillsDraftUpdate,
} from "./settings-skills-draft-logic";
import { useSettingsSkillSources } from "./use-settings-skill-sources";
import { useSettingsSkillsScope } from "./use-settings-skills-scope";

type SkillsConfig = SettingsSkillsSection["config"];

export type { SkillsScopeSelection, SkillsScopeValue } from "./use-settings-skills-scope";

function sameDisabled(a: string[] | undefined, b: string[] | undefined): boolean {
  const left = a ?? [];
  const right = b ?? [];
  if (left.length !== right.length) return false;
  return left.every((value, index) => value === right[index]);
}

function samePolicy(a: SkillsConfig, b: SkillsConfig): boolean {
  if (a.enabled !== b.enabled) return false;
  if (a.poll_interval !== b.poll_interval) return false;
  if (a.marketplace.registry !== b.marketplace.registry) return false;
  if ((a.marketplace.base_url ?? "") !== (b.marketplace.base_url ?? "")) return false;
  if (
    JSON.stringify(a.allowed_marketplace_hooks ?? []) !==
    JSON.stringify(b.allowed_marketplace_hooks ?? [])
  ) {
    return false;
  }
  return (
    JSON.stringify(a.allowed_marketplace_mcp ?? []) ===
    JSON.stringify(b.allowed_marketplace_mcp ?? [])
  );
}

function errorMessage(error: unknown): string | null {
  if (error instanceof SettingsApiError) return error.message;
  if (error instanceof Error) return error.message;
  return null;
}

/**
 * Placeholder for the window before the first read resolves. The page renders a
 * spinner then, so nothing here reaches the screen — it only keeps hook order
 * stable while `envelope` is still null.
 */
const EMPTY_ENVELOPE: SettingsSkillsSection = {
  section: "skills",
  scope: "user",
  available_scopes: ["user"],
  runtime_available: false,
  discovered_count: 0,
  disabled_count: 0,
  config: {
    enabled: false,
    marketplace: { registry: "" },
    poll_interval: "",
    sources: [],
    custom_sources: [],
  },
  sources: [],
};

function envelopeScopeKey(envelope: SettingsSkillsSection): string {
  return [
    envelope.scope,
    envelope.profile ?? "",
    envelope.workspace_id ?? "",
    envelope.agent_name ?? "",
  ].join(":");
}

export function useSettingsSkillsPage() {
  const page = useSettingsPage({ currentSlug: "skills" });
  const scope = useSettingsSkillsScope();
  const disabledMutation = useUpdateSettingsSkills();
  const policyMutation = useUpdateSettingsSkills();

  const query = useSettingsSkills(scope.filter);
  const envelope = query.data ?? null;

  const envelopeKey = envelope ? envelopeScopeKey(envelope) : "pending";
  const baseline = envelope?.config ?? null;
  const { store: draftLogic } = useStoreBinding(
    envelopeKey,
    () => settingsSkillsDraftLogic.createStore({ baseline, key: envelopeKey }),
    previous =>
      settingsSkillsDraftLogic.createStore({
        baseline,
        key: envelopeKey,
        previous: previous.getSnapshot().context,
      }),
    current => shouldRebindSkillsDraft(current.store.getSnapshot().context, baseline, envelopeKey)
  );
  const draftFlow = useSelector(draftLogic, snapshot => snapshot.context);
  const draft = draftFlow.draft;
  const setDraft = (update: SkillsDraftUpdate) => {
    draftLogic.trigger.draftChanged({ update });
  };
  const labelFor = (kind: "disabled" | "policy" | "sources") =>
    draftFlow.key === envelopeKey ? draftFlow.labels[kind] : null;

  const isUserLayer = scope.selection.scope === "user";
  const isRepositoryProfile =
    scope.filter.scope === "profile" && typeof scope.filter.workspace_id === "string";
  const isDisabledDirty =
    envelope && draft && !isRepositoryProfile
      ? !sameDisabled(envelope.config.disabled_skills, draft.disabled_skills)
      : false;
  const isPolicyDirty =
    envelope && draft && isUserLayer ? !samePolicy(envelope.config, draft) : false;

  const handleResetDisabled = () => {
    if (!envelope || !draft || isRepositoryProfile) return;
    setDraft({ ...draft, disabled_skills: [...(envelope.config.disabled_skills ?? [])] });
  };

  const handleResetPolicy = () => {
    if (!envelope || !draft || !isUserLayer) return;
    setDraft(skillsPolicyConfig(draft, envelope.config));
  };

  const handleSaveDisabled = () => {
    if (!envelope || !draft || isRepositoryProfile) return;
    const config = skillsDisabledConfig(envelope.config, draft);
    draftLogic.trigger.saveRequested({
      baseline: config,
      execute: () => disabledMutation.mutateAsync({ body: { config }, filter: scope.filter }),
      kind: "disabled",
      label: "Saved · applied immediately",
    });
  };

  const handleSavePolicy = () => {
    if (!envelope || !draft || !isUserLayer) return;
    const config = skillsPolicyConfig(envelope.config, draft);
    draftLogic.trigger.saveRequested({
      baseline: config,
      execute: () => policyMutation.mutateAsync({ body: { config }, filter: scope.filter }),
      kind: "policy",
      label: "Saved · restart required to apply",
    });
  };

  const toggleDisabled = (name: string) => {
    if (!draft || isRepositoryProfile) return;
    const current = draft.disabled_skills ?? [];
    const next = current.includes(name)
      ? current.filter(entry => entry !== name)
      : [...current, name].sort();
    setDraft({ ...draft, disabled_skills: next });
  };

  const sources = useSettingsSkillSources({
    envelope: envelope ?? EMPTY_ENVELOPE,
    draft: draft ?? EMPTY_ENVELOPE.config,
    filter: scope.filter,
    scopeKey: scope.scopeKey,
    isSaving: draftFlow.pending.sources !== null,
    lastLabel: labelFor("sources"),
    onDraftChange: setDraft,
    onSaveRequested: request => draftLogic.trigger.saveRequested({ ...request, kind: "sources" }),
  });

  return {
    isLoading: query.isLoading || scope.isLoading,
    error: query.error ?? scope.error,
    envelope,
    draft,
    setDraft,
    sources,
    toggleDisabled,
    isDisabledDirty,
    isPolicyDirty,
    handleResetDisabled,
    handleResetPolicy,
    handleSaveDisabled,
    handleSavePolicy,
    isSavingDisabled: draftFlow.pending.disabled !== null,
    isSavingPolicy: draftFlow.pending.policy !== null,
    saveDisabledError: errorMessage(disabledMutation.error),
    savePolicyError: errorMessage(policyMutation.error),
    disabledWarnings: disabledMutation.data?.warnings,
    policyWarnings: policyMutation.data?.warnings,
    lastDisabledLabel: labelFor("disabled"),
    lastPolicyLabel: labelFor("policy"),
    handleRetry: () => {
      void query.refetch();
      scope.refetch();
    },
    restart: page.restart,
    availableScopes: envelope?.available_scopes ?? ["user"],
    selection: scope.selection,
    isRepositoryProfile,
    personalLabel: scope.personalLabel,
    actingProfile: scope.actingProfile,
    agents: scope.agents,
    workspaces: scope.workspaces,
    selectedAgent: scope.selectedAgent,
    selectedWorkspace: scope.selectedWorkspace,
    selectUser: scope.selectUser,
    selectWorkspaceScope: scope.selectWorkspaceScope,
    selectAgentScope: scope.selectAgentScope,
    selectAgent: scope.selectAgent,
    selectWorkspace: scope.selectWorkspace,
  };
}

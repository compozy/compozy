import type {
  SettingsApplyRecordsFilter,
  SettingsAttentionFilter,
  SettingsCmdPaletteFilter,
  SettingsHookListFilter,
  SettingsMCPServerListFilter,
  SettingsNotificationPresetFilter,
  SettingsPersonaFilter,
  SettingsSectionName,
  SettingsSectionSlug,
  SettingsSkillsFilter,
} from "../types";

function normalizeText(value?: string | null): string {
  if (value == null) {
    return "";
  }

  const trimmed = value.trim();
  return trimmed;
}

export type SettingsSectionKey = SettingsSectionName | SettingsSectionSlug;

export const settingsKeys = {
  all: ["settings"] as const,

  sections: () => [...settingsKeys.all, "section"] as const,
  section: (section: SettingsSectionKey) => [...settingsKeys.sections(), section] as const,
  personaSection: (filter: SettingsPersonaFilter) =>
    [
      ...settingsKeys.section("persona"),
      filter.scope ?? "",
      normalizeText(filter.workspace_id),
      normalizeText(filter.profile),
    ] as const,
  windowManagerLayoutsRoot: () => [...settingsKeys.section("window-manager"), "layouts"] as const,
  windowManagerLayoutProfiles: (workspaceId: string, profile: string) =>
    [
      ...settingsKeys.windowManagerLayoutsRoot(),
      "profiles",
      normalizeText(workspaceId),
      normalizeText(profile),
    ] as const,
  windowManagerLayoutReview: (
    workspaceId: string,
    profile: string,
    revision: number,
    fingerprint: string
  ) =>
    [
      ...settingsKeys.windowManagerLayoutsRoot(),
      "review",
      normalizeText(workspaceId),
      normalizeText(profile),
      revision,
      fingerprint,
    ] as const,
  skillsSection: (filter: SettingsSkillsFilter = {}) =>
    [
      ...settingsKeys.section("skills"),
      filter.scope ?? "",
      normalizeText(filter.workspace_id),
      normalizeText(filter.agent_name),
    ] as const,
  // Scope is part of the key: user, profile, and workspace answer differently,
  // so one entry serving all of them would show another scope's setting.
  cmdPaletteSection: (filter: SettingsCmdPaletteFilter = {}) =>
    [
      ...settingsKeys.section("cmd-palette"),
      filter.scope ?? "",
      normalizeText(filter.workspace_id),
      normalizeText(filter.profile),
    ] as const,
  attentionSection: (filter: SettingsAttentionFilter) =>
    [
      ...settingsKeys.section("attention"),
      filter.scope ?? "",
      normalizeText(filter.profile),
    ] as const,

  rolesStatus: () => [...settingsKeys.all, "roles-status"] as const,

  collections: () => [...settingsKeys.all, "collection"] as const,

  providersRoot: () => [...settingsKeys.collections(), "providers"] as const,
  providersList: () => [...settingsKeys.providersRoot(), "list"] as const,
  providerDetail: (name: string) => [...settingsKeys.providersRoot(), "detail", name] as const,

  sandboxesRoot: () => [...settingsKeys.collections(), "sandboxes"] as const,
  sandboxesList: () => [...settingsKeys.sandboxesRoot(), "list"] as const,
  sandboxDetail: (name: string) => [...settingsKeys.sandboxesRoot(), "detail", name] as const,

  hooksRoot: () => [...settingsKeys.collections(), "hooks"] as const,
  hooksList: (filter: SettingsHookListFilter = {}) =>
    [
      ...settingsKeys.hooksRoot(),
      "list",
      filter.scope ?? "",
      normalizeText(filter.workspace_id),
      normalizeText(filter.profile),
    ] as const,

  mcpRoot: () => [...settingsKeys.collections(), "mcp-servers"] as const,
  mcpLists: () => [...settingsKeys.mcpRoot(), "list"] as const,
  mcpList: (filter: SettingsMCPServerListFilter = {}) =>
    [
      ...settingsKeys.mcpLists(),
      filter.scope ?? "",
      normalizeText(filter.workspace_id),
      normalizeText(filter.profile),
    ] as const,

  notificationsRoot: () => [...settingsKeys.all, "notifications"] as const,
  notificationPresetsList: (filter: SettingsNotificationPresetFilter = {}) =>
    [
      ...settingsKeys.notificationsRoot(),
      "presets",
      filter.enabled ?? "",
      filter.built_in ?? "",
      normalizeText(filter.name),
      filter.limit ?? "",
    ] as const,

  restartRoot: () => [...settingsKeys.all, "restart"] as const,
  restartStatus: (operationId: string) => [...settingsKeys.restartRoot(), operationId] as const,

  applyRoot: () => [...settingsKeys.all, "apply"] as const,
  applyRecords: (filter: SettingsApplyRecordsFilter = {}) =>
    [
      ...settingsKeys.applyRoot(),
      "records",
      filter.status ?? "",
      normalizeText(filter.actor),
      filter.limit ?? "",
    ] as const,

  updateStatus: () => [...settingsKeys.all, "update"] as const,
};

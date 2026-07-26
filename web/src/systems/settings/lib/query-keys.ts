import type {
  SettingsApplyRecordsFilter,
  SettingsMCPServerListFilter,
  SettingsNotificationPresetFilter,
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
  windowManagerLayoutsRoot: () => [...settingsKeys.section("window-manager"), "layouts"] as const,
  windowManagerLayoutProfiles: (workspaceId: string) =>
    [...settingsKeys.windowManagerLayoutsRoot(), "profiles", normalizeText(workspaceId)] as const,
  windowManagerLayoutReview: (workspaceId: string, revision: number, fingerprint: string) =>
    [
      ...settingsKeys.windowManagerLayoutsRoot(),
      "review",
      normalizeText(workspaceId),
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

  rolesStatus: () => [...settingsKeys.all, "roles-status"] as const,

  collections: () => [...settingsKeys.all, "collection"] as const,

  providersRoot: () => [...settingsKeys.collections(), "providers"] as const,
  providersList: () => [...settingsKeys.providersRoot(), "list"] as const,
  providerDetail: (name: string) => [...settingsKeys.providersRoot(), "detail", name] as const,

  sandboxesRoot: () => [...settingsKeys.collections(), "sandboxes"] as const,
  sandboxesList: () => [...settingsKeys.sandboxesRoot(), "list"] as const,
  sandboxDetail: (name: string) => [...settingsKeys.sandboxesRoot(), "detail", name] as const,

  hooksRoot: () => [...settingsKeys.collections(), "hooks"] as const,
  hooksList: () => [...settingsKeys.hooksRoot(), "list"] as const,

  mcpRoot: () => [...settingsKeys.collections(), "mcp-servers"] as const,
  mcpLists: () => [...settingsKeys.mcpRoot(), "list"] as const,
  mcpList: (filter: SettingsMCPServerListFilter = {}) =>
    [...settingsKeys.mcpLists(), filter.scope ?? "", normalizeText(filter.workspace_id)] as const,

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

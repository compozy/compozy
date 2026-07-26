import type { LucideIcon } from "lucide-react";

import type { OperationQuery, OperationRequestBody, OperationResponse } from "@/lib/api-contract";

export type SettingsGeneralSection = OperationResponse<"getSettingsGeneral", 200>;
export type SettingsMemorySection = OperationResponse<"getSettingsMemory", 200>;
export type SettingsSkillsSection = OperationResponse<"getSettingsSkills", 200>;
export type SettingsAutomationSection = OperationResponse<"getSettingsAutomation", 200>;
export type SettingsNetworkSection = OperationResponse<"getSettingsNetwork", 200>;
export type SettingsObservabilitySection = OperationResponse<"getSettingsObservability", 200>;
export type SettingsHooksExtensionsSection = OperationResponse<"getSettingsHooksExtensions", 200>;
export type SettingsWindowManagerSection = OperationResponse<"getSettingsWindowManager", 200>;
export type SettingsHooksExtensionsHook = NonNullable<
  SettingsHooksExtensionsSection["hooks"]
>[number];
export type SettingsHooksExtensionsInstalled = NonNullable<
  SettingsHooksExtensionsSection["installed"]
>[number];

// Roles: editable settings section (getSettingsRoles) + read-only effective
// projection (listRoles). All derived from generated operations — never hand-rolled.
export type SettingsRolesSection = OperationResponse<"getSettingsRoles", 200>;
export type SettingsRolesConfig = SettingsRolesSection["config"];
export type RoleName = keyof SettingsRolesConfig;
export type RoleFallbackEntry = SettingsRolesConfig["dream"]["fallback_chain"][number];

export type RolesStatusResponse = OperationResponse<"listRoles", 200>;
export type RoleStatus = RolesStatusResponse["roles"][number];
export type RoleResolutionMode = RoleStatus["resolution_mode"];
export type RoleDiagnostic = RoleStatus["diagnostics"][number];
export type RoleFallbackStatus = RoleStatus["fallback_chain"][number];

export type SettingsNotificationPresetCollection = OperationResponse<
  "listNotificationPresets",
  200
>;
export type SettingsNotificationPresetEntry =
  SettingsNotificationPresetCollection["presets"][number];
export type SettingsNotificationPresetTarget = SettingsNotificationPresetEntry["targets"][number];
export type SettingsNotificationPresetFilter = NonNullable<
  OperationQuery<"listNotificationPresets">
>;
export type SettingsCreateNotificationPresetRequest =
  OperationRequestBody<"createNotificationPreset">;
export type SettingsUpdateNotificationPresetRequest =
  OperationRequestBody<"updateNotificationPreset">;

export type SettingsProviderCollection = OperationResponse<"listSettingsProviders", 200>;
export type SettingsProviderEntry = SettingsProviderCollection["providers"][number];
export type SettingsProviderDetail = OperationResponse<"getSettingsProvider", 200>["provider"];
export type SettingsProviderRequest = OperationRequestBody<"putSettingsProvider">;
export type SettingsProviderCredentialSlotRequest = NonNullable<
  NonNullable<SettingsProviderRequest["settings"]>["credential_slots"]
>[number];
export type SettingsProviderModelsRequest = NonNullable<
  NonNullable<SettingsProviderRequest["settings"]>["models"]
>;
export type SettingsProviderModelRequest = NonNullable<
  SettingsProviderModelsRequest["curated"]
>[number];

/** Who owns launch-time provider authentication (`internal/config/provider.go`). */
export type ProviderAuthMode = "native_cli" | "bound_secret" | "none";

export type ProviderCredentialSlotDraft = SettingsProviderCredentialSlotRequest & {
  /**
   * Stable row identity for the editor. Slot names are user-editable and may
   * repeat mid-typing, so they cannot key a list whose rows carry rotation
   * state. Never sent to the daemon.
   */
  key: string;
};

export type ProviderDraft = {
  name: string;
  command: string;
  display_name: string;
  model_default: string;
  curated_models: string;
  harness: string;
  runtime_provider: string;
  transport: string;
  base_url: string;
  auth_mode: ProviderAuthMode;
  env_policy: string;
  home_policy: string;
  auth_status_command: string;
  auth_login_command: string;
  /**
   * Declared credential bindings. Only `auth_mode = bound_secret` may carry
   * them — the daemon rejects slots under any other mode.
   */
  credential_slots: ProviderCredentialSlotDraft[];
  /** Write-only plaintext per slot, parallel to `credential_slots`. Never read back. */
  credential_secret_values: string[];
};

export type SettingsSandboxCollection = OperationResponse<"listSettingsSandboxes", 200>;
export type SettingsSandboxEntry = SettingsSandboxCollection["sandboxes"][number];
export type SettingsSandboxDetail = OperationResponse<"getSettingsSandbox", 200>["sandbox"];
export type SettingsSandboxRequest = OperationRequestBody<"putSettingsSandbox">;

export type SettingsHookCollection = OperationResponse<"listSettingsHooks", 200>;
export type SettingsHookEntry = SettingsHookCollection["hooks"][number];
export type SettingsHookRequest = OperationRequestBody<"putSettingsHook">;

export type SettingsMCPServerCollection = OperationResponse<"listSettingsMCPServers", 200>;
export type SettingsMCPServerEntry = SettingsMCPServerCollection["mcp_servers"][number];
export type SettingsMCPServerRequest = OperationRequestBody<"putSettingsMCPServer">;
export type SettingsMCPServerListFilter = NonNullable<OperationQuery<"listSettingsMCPServers">>;
export type SettingsMCPServerPutFilter = NonNullable<OperationQuery<"putSettingsMCPServer">>;
export type SettingsMCPServerDeleteFilter = NonNullable<OperationQuery<"deleteSettingsMCPServer">>;

// Daemon-mediated OAuth (ADR-016). The begin response returns the ONLY live PKCE
// authorization URL; the exchange/logout responses return the fresh auth-status shape.
export type SettingsMCPAuthFilter = NonNullable<OperationQuery<"beginSettingsMCPAuth">>;
export type SettingsMCPAuthBeginRequest = OperationRequestBody<"beginSettingsMCPAuth">;
export type SettingsMCPAuthBeginMode = SettingsMCPAuthBeginRequest["mode"];
export type SettingsMCPAuthBeginResponse = OperationResponse<"beginSettingsMCPAuth", 200>;
export type SettingsMCPAuthExchangeRequest = OperationRequestBody<"exchangeSettingsMCPAuth">;
export type SettingsMCPAuthStatusResponse = OperationResponse<"exchangeSettingsMCPAuth", 200>;

export type SettingsUpdateGeneralRequest = OperationRequestBody<"updateSettingsGeneral">;
export type SettingsUpdateMemoryRequest = OperationRequestBody<"updateSettingsMemory">;
export type SettingsUpdateRolesRequest = OperationRequestBody<"updateSettingsRoles">;
export type SettingsUpdateSkillsRequest = OperationRequestBody<"updateSettingsSkills">;
export type SettingsSkillsFilter = NonNullable<OperationQuery<"getSettingsSkills">>;
export type SettingsUpdateSkillsFilter = NonNullable<OperationQuery<"updateSettingsSkills">>;
export type SettingsUpdateAutomationRequest = OperationRequestBody<"updateSettingsAutomation">;
export type SettingsUpdateNetworkRequest = OperationRequestBody<"updateSettingsNetwork">;
export type SettingsUpdateObservabilityRequest =
  OperationRequestBody<"updateSettingsObservability">;
export type SettingsUpdateHooksExtensionsRequest =
  OperationRequestBody<"updateSettingsHooksExtensions">;
export type SettingsUpdateWindowManagerRequest =
  OperationRequestBody<"updateSettingsWindowManager">;

export type SettingsRestartResponse = OperationResponse<"triggerSettingsRestart", 202>;
export type SettingsRestartStatus = OperationResponse<"getSettingsRestartStatus", 200>;
export type SettingsUpdateStatus = OperationResponse<"getSettingsUpdate", 200>;
export type SettingsApplyResponse = OperationResponse<"reloadSettings", 200>;
export type ConfigApplyRecordsResponse = OperationResponse<"listSettingsApplyRecords", 200>;
export type ConfigApplyRecord = ConfigApplyRecordsResponse["entries"][number];
export type ConfigApplyRecordStatus = ConfigApplyRecord["status"];
export type ConfigApplyLifecycle = ConfigApplyRecord["lifecycle"];
export type SettingsApplyNextAction = ConfigApplyRecord["next_action"];
export type SettingsApplyRecordsFilter = NonNullable<OperationQuery<"listSettingsApplyRecords">>;

export type SettingsMutationResult =
  | OperationResponse<"updateSettingsGeneral", 200>
  | OperationResponse<"updateSettingsMemory", 200>
  | OperationResponse<"updateSettingsSkills", 200>
  | OperationResponse<"updateSettingsAutomation", 200>
  | OperationResponse<"updateSettingsNetwork", 200>
  | OperationResponse<"updateSettingsObservability", 200>
  | OperationResponse<"updateSettingsHooksExtensions", 200>
  | OperationResponse<"updateSettingsWindowManager", 200>
  | OperationResponse<"updateSettingsRoles", 200>
  | OperationResponse<"putSettingsProvider", 200>
  | OperationResponse<"deleteSettingsProvider", 200>
  | OperationResponse<"putSettingsMCPServer", 200>
  | OperationResponse<"deleteSettingsMCPServer", 200>
  | OperationResponse<"putSettingsSandbox", 200>
  | OperationResponse<"deleteSettingsSandbox", 200>
  | OperationResponse<"putSettingsHook", 200>
  | OperationResponse<"deleteSettingsHook", 200>;
export type SettingsScope = SettingsMutationResult["scope"];
export type SettingsWriteTarget = NonNullable<SettingsMutationResult["write_target"]>;
export type SettingsSectionName =
  | SettingsGeneralSection["section"]
  | SettingsMemorySection["section"]
  | SettingsRolesSection["section"]
  | SettingsSkillsSection["section"]
  | SettingsAutomationSection["section"]
  | SettingsNetworkSection["section"]
  | SettingsObservabilitySection["section"]
  | SettingsHooksExtensionsSection["section"]
  | SettingsWindowManagerSection["section"];
export type SettingsRestartStatusName = SettingsRestartResponse["status"];
export type SettingsSource = SettingsProviderEntry["source_metadata"]["effective_source"];
export type SettingsSourceKind = SettingsSource["kind"];
export type SettingsMCPServerTarget = NonNullable<SettingsMCPServerPutFilter["target"]>;

export type SettingsCollectionName = "providers" | "mcp-servers" | "sandboxes" | "hooks";

export type SettingsSectionGroup = "workspace" | "runtime" | "system";

export interface SettingsSectionDescriptor {
  slug: SettingsSectionSlug;
  label: string;
  icon: LucideIcon;
  /** Nav group (design system §03). */
  group: SettingsSectionGroup;
  /** Space-separated search keywords for the sidebar filter. */
  keywords: string;
}

export type SettingsSectionSlug =
  | "general"
  | "appearance"
  | "layouts"
  | "providers"
  | "sandboxes"
  | "memory"
  | "roles"
  | "skills"
  | "automation"
  | "network"
  | "observability"
  | "hooks"
  | "extensions";

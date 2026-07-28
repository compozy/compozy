import type {
  BridgeCheckStatus,
  BridgeDetailResponse,
  BridgeHealth,
  BridgeProvider,
  BridgeSecretBinding,
  BridgeVerifyCheck,
  BridgeVerifyResponse,
  BridgeWebhookRegistrationResponse,
} from "../types";

export type BridgeSetupState =
  | "complete"
  | "action-needed"
  | "unknown"
  | "warn"
  | "fail"
  | "skipped";

export type BridgeSetupChecklistItemId =
  | "provider"
  | "secrets"
  | "webhook"
  | "verified"
  | "enabled"
  | "healthy";

export interface BridgeSetupProfile {
  readonly platform: string;
  readonly dashboardUrl: string | null;
  readonly manifest: "slack" | null;
  readonly webhookApplicable: boolean;
  readonly webhookRegistration: "telegram" | null;
  readonly checkSecretSlots: Readonly<Record<string, readonly string[]>>;
}

export interface BridgeSetupChecklistItem {
  readonly id: BridgeSetupChecklistItemId;
  readonly state: BridgeSetupState;
  readonly remediation?: string;
}

export type BridgeSetupProfileIssue =
  | {
      readonly kind: "provider-mismatch";
      readonly expectedPlatform: string;
      readonly actualPlatform: string;
    }
  | {
      readonly kind: "missing-secret-slot";
      readonly check: string;
      readonly slot: string;
    };

export interface BridgeSetupFacts {
  readonly providerPresent: boolean;
  readonly requiredSecretSlots: readonly string[];
  readonly missingRequiredSecretSlots: readonly string[];
  readonly webhookPublicUrl: string | null;
  readonly verificationCurrent: boolean;
  readonly registrationCurrent: boolean;
  readonly enabled: boolean;
  readonly healthy: boolean;
}

export interface BridgeSetupProjectionInput {
  readonly provider: BridgeProvider | null;
  readonly bridge: BridgeDetailResponse["bridge"];
  readonly health: BridgeHealth | null;
  readonly bindings: readonly BridgeSecretBinding[];
  readonly verification: BridgeVerifyResponse | null;
  readonly registration: BridgeWebhookRegistrationResponse | null;
}

export interface BridgeSetupProjection {
  readonly profile: BridgeSetupProfile | null;
  readonly checklist: readonly BridgeSetupChecklistItem[];
  readonly checksBySecretSlot: Readonly<Record<string, readonly BridgeVerifyCheck[]>>;
  readonly globalChecks: readonly BridgeVerifyCheck[];
  readonly profileIssues: readonly BridgeSetupProfileIssue[];
  readonly facts: BridgeSetupFacts;
}

const BRIDGE_SETUP_PROFILES = {
  discord: {
    platform: "discord",
    dashboardUrl: "https://discord.com/developers/applications",
    manifest: null,
    webhookApplicable: true,
    webhookRegistration: null,
    checkSecretSlots: {
      "provider.identity": ["bot_token"],
      "webhook.public_key": ["public_key"],
    },
  },
  gchat: {
    platform: "gchat",
    dashboardUrl: "https://console.cloud.google.com/apis/api/chat.googleapis.com/overview",
    manifest: null,
    webhookApplicable: true,
    webhookRegistration: null,
    checkSecretSlots: {
      "provider.identity": ["credentials_json"],
    },
  },
  github: {
    platform: "github",
    dashboardUrl: "https://github.com/settings/apps",
    manifest: null,
    webhookApplicable: true,
    webhookRegistration: null,
    checkSecretSlots: {},
  },
  linear: {
    platform: "linear",
    dashboardUrl: "https://linear.app/settings/api/applications",
    manifest: null,
    webhookApplicable: true,
    webhookRegistration: null,
    checkSecretSlots: {},
  },
  slack: {
    platform: "slack",
    dashboardUrl: "https://api.slack.com/apps",
    manifest: "slack",
    webhookApplicable: true,
    webhookRegistration: null,
    checkSecretSlots: {
      "provider.identity": ["bot_token"],
      "scope.app_mentions:read": ["bot_token"],
      "scope.assistant:write": ["bot_token"],
      "scope.channels:history": ["bot_token"],
      "scope.chat:write": ["bot_token"],
      "scope.commands": ["bot_token"],
      "scope.groups:history": ["bot_token"],
      "scope.im:history": ["bot_token"],
      "scope.mpim:history": ["bot_token"],
      "scope.reactions:read": ["bot_token"],
      "scope.reactions:write": ["bot_token"],
      "webhook.signing_secret": ["signing_secret"],
    },
  },
  teams: {
    platform: "teams",
    dashboardUrl:
      "https://portal.azure.com/#view/Microsoft_AAD_RegisteredApps/ApplicationsListBlade",
    manifest: null,
    webhookApplicable: true,
    webhookRegistration: null,
    checkSecretSlots: {},
  },
  telegram: {
    platform: "telegram",
    dashboardUrl: "https://t.me/BotFather",
    manifest: null,
    webhookApplicable: true,
    webhookRegistration: "telegram",
    checkSecretSlots: {
      "provider.identity": ["bot_token"],
      "webhook.registration": ["webhook_secret"],
      "webhook.secret": ["webhook_secret"],
    },
  },
  whatsapp: {
    platform: "whatsapp",
    dashboardUrl: "https://developers.facebook.com/apps/",
    manifest: null,
    webhookApplicable: true,
    webhookRegistration: null,
    checkSecretSlots: {
      "provider.identity": ["access_token"],
      "webhook.app_secret": ["app_secret"],
      "webhook.verify_token": ["verify_token"],
    },
  },
} as const satisfies Readonly<Record<string, BridgeSetupProfile>>;

export function getBridgeSetupProfile(platform: string): BridgeSetupProfile | null {
  return isBuiltInPlatform(platform) ? BRIDGE_SETUP_PROFILES[platform] : null;
}

export function projectBridgeSetup(input: BridgeSetupProjectionInput): BridgeSetupProjection {
  const { bridge, health, provider, registration, verification } = input;
  const profile = getBridgeSetupProfile(bridge.platform);
  const providerMatches = isMatchingProvider(provider, bridge);
  const providerState =
    provider === null ? "action-needed" : providerMatches ? "complete" : "unknown";
  const providerIssues = getProviderIssues(provider, bridge.platform, providerMatches);
  const requiredSecretSlots: string[] = [];
  if (providerMatches) {
    for (const slot of provider.secret_slots ?? []) {
      if (slot.required !== false) requiredSecretSlots.push(slot.name);
    }
  }
  const boundSecretSlots = new Set<string>();
  for (const binding of input.bindings) {
    if (binding.bridge_instance_id === bridge.id) boundSecretSlots.add(binding.binding_name);
  }
  const missingRequiredSecretSlots = requiredSecretSlots.filter(
    slot => !boundSecretSlots.has(slot)
  );
  const secretsState: BridgeSetupState = !providerMatches
    ? "unknown"
    : missingRequiredSecretSlots.length === 0
      ? "complete"
      : "action-needed";
  const webhookPublicUrl = bridge.webhook_public_url ?? null;
  const verificationCurrent = verification?.bridge_instance_id === bridge.id;
  const registrationCurrent =
    profile?.webhookRegistration === "telegram" && registration?.bridge_instance_id === bridge.id;
  const { mappings, issues: mappingIssues } = validateProfileMappings(
    profile,
    providerMatches ? provider : null
  );
  const { checksBySecretSlot, globalChecks } = projectVerificationChecks(
    verificationCurrent ? verification.checks : [],
    mappings
  );

  return {
    profile,
    checklist: [
      makeChecklistItem("provider", providerState),
      makeChecklistItem("secrets", secretsState),
      projectWebhookItem(profile, webhookPublicUrl, registration, bridge.id),
      projectVerificationItem(verification, bridge.id),
      makeChecklistItem("enabled", bridge.enabled ? "complete" : "action-needed"),
      projectHealthItem(health, bridge.id),
    ],
    checksBySecretSlot,
    globalChecks,
    profileIssues: [...providerIssues, ...mappingIssues],
    facts: {
      providerPresent: providerMatches,
      requiredSecretSlots,
      missingRequiredSecretSlots,
      webhookPublicUrl,
      verificationCurrent,
      registrationCurrent,
      enabled: bridge.enabled,
      healthy: health?.bridge_instance_id === bridge.id && health.status === "ready",
    },
  };
}

function isBuiltInPlatform(platform: string): platform is keyof typeof BRIDGE_SETUP_PROFILES {
  return Object.hasOwn(BRIDGE_SETUP_PROFILES, platform);
}

function isMatchingProvider(
  provider: BridgeProvider | null,
  bridge: BridgeDetailResponse["bridge"]
): provider is BridgeProvider {
  return (
    provider !== null &&
    provider.platform === bridge.platform &&
    provider.extension_name === bridge.extension_name
  );
}

function getProviderIssues(
  provider: BridgeProvider | null,
  expectedPlatform: string,
  providerMatches: boolean
): BridgeSetupProfileIssue[] {
  if (provider === null || providerMatches) {
    return [];
  }
  return [{ kind: "provider-mismatch", expectedPlatform, actualPlatform: provider.platform }];
}

function validateProfileMappings(
  profile: BridgeSetupProfile | null,
  provider: BridgeProvider | null
): {
  mappings: Readonly<Record<string, readonly string[]>>;
  issues: BridgeSetupProfileIssue[];
} {
  if (profile === null || provider === null) {
    return { mappings: {}, issues: [] };
  }
  const providerSlots = new Set((provider.secret_slots ?? []).map(slot => slot.name));
  const mappings: Record<string, readonly string[]> = {};
  const issues: BridgeSetupProfileIssue[] = [];

  for (const [check, slots] of Object.entries(profile.checkSecretSlots)) {
    const missingSlots = slots.filter(slot => !providerSlots.has(slot));
    if (missingSlots.length > 0) {
      issues.push(
        ...missingSlots.map(slot => ({
          kind: "missing-secret-slot" as const,
          check,
          slot,
        }))
      );
      continue;
    }
    mappings[check] = slots;
  }
  return { mappings, issues };
}

function projectVerificationChecks(
  checks: readonly BridgeVerifyCheck[],
  mappings: Readonly<Record<string, readonly string[]>>
): {
  checksBySecretSlot: Readonly<Record<string, readonly BridgeVerifyCheck[]>>;
  globalChecks: readonly BridgeVerifyCheck[];
} {
  const checksBySecretSlot: Record<string, BridgeVerifyCheck[]> = {};
  const globalChecks: BridgeVerifyCheck[] = [];

  for (const check of checks) {
    const slots = mappings[check.check];
    if (slots === undefined) {
      globalChecks.push(check);
      continue;
    }
    for (const slot of slots) {
      (checksBySecretSlot[slot] ??= []).push(check);
    }
  }
  return { checksBySecretSlot, globalChecks };
}

function projectWebhookItem(
  profile: BridgeSetupProfile | null,
  webhookPublicUrl: string | null,
  registration: BridgeWebhookRegistrationResponse | null,
  bridgeID: string
): BridgeSetupChecklistItem {
  if (profile === null) {
    return makeChecklistItem("webhook", "unknown");
  }
  if (!profile.webhookApplicable) {
    return makeChecklistItem("webhook", "skipped");
  }
  if (webhookPublicUrl === null) {
    return makeChecklistItem("webhook", "action-needed");
  }
  if (profile.webhookRegistration !== "telegram") {
    return makeChecklistItem("webhook", "complete");
  }
  if (registration === null) {
    return makeChecklistItem("webhook", "action-needed");
  }
  if (registration.bridge_instance_id !== bridgeID) {
    return makeChecklistItem("webhook", "unknown");
  }
  return makeChecklistItem(
    "webhook",
    stateForCheckStatus(registration.status),
    registration.remediation
  );
}

function projectVerificationItem(
  verification: BridgeVerifyResponse | null,
  bridgeID: string
): BridgeSetupChecklistItem {
  if (verification === null) {
    return makeChecklistItem("verified", "action-needed");
  }
  if (verification.bridge_instance_id !== bridgeID || verification.checks.length === 0) {
    return makeChecklistItem("verified", "unknown");
  }

  const status = summarizeBridgeCheckStatus(verification.checks);
  if (status === null) {
    return makeChecklistItem("verified", "unknown");
  }
  const representative = verification.checks.find(check => check.status === status);
  return makeChecklistItem(
    "verified",
    stateForCheckStatus(status),
    status === "pass" ? undefined : representative?.remediation
  );
}

export function summarizeBridgeCheckStatus(
  checks: readonly BridgeVerifyCheck[]
): BridgeCheckStatus | null {
  for (const status of ["fail", "warn", "pass", "skipped"] as const) {
    if (checks.some(check => check.status === status)) {
      return status;
    }
  }
  return null;
}

function projectHealthItem(
  health: BridgeHealth | null,
  bridgeID: string
): BridgeSetupChecklistItem {
  if (health === null || health.bridge_instance_id !== bridgeID) {
    return makeChecklistItem("healthy", "unknown");
  }
  switch (health.status) {
    case "ready":
      return makeChecklistItem("healthy", "complete");
    case "starting":
    case "degraded":
      return makeChecklistItem("healthy", "warn");
    case "auth_required":
    case "error":
      return makeChecklistItem("healthy", "fail");
    case "disabled":
      return makeChecklistItem("healthy", "skipped");
  }
}

function stateForCheckStatus(status: BridgeCheckStatus): BridgeSetupState {
  switch (status) {
    case "pass":
      return "complete";
    case "warn":
    case "fail":
    case "skipped":
      return status;
  }
}

function makeChecklistItem(
  id: BridgeSetupChecklistItemId,
  state: BridgeSetupState,
  remediation?: string
): BridgeSetupChecklistItem {
  return remediation === undefined || remediation === ""
    ? { id, state }
    : { id, state, remediation };
}

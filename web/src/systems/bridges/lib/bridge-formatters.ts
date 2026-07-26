import type { PillTone } from "@agh/ui";

import type {
  BridgeDeliveryDefaults,
  BridgeDmPolicy,
  BridgeProgressConfig,
  BridgeProgressGrouping,
  BridgeProgressMode,
  BridgeProviderConfig,
  BridgeProvider,
  BridgeProviderConfigSchemaHint,
  BridgeProviderSecretSlot,
  BridgeRoute,
  BridgeRoutingPolicy,
  BridgeScope,
  BridgeStatus,
  BridgeTarget,
} from "@/systems/bridges/types";

function normalizeText(value: unknown): string | undefined {
  if (typeof value !== "string") {
    return undefined;
  }

  const normalized = value.trim();
  return normalized === "" ? undefined : normalized;
}

function normalizeBridgeProgressMode(value: unknown): BridgeProgressMode | undefined {
  switch (normalizeText(value)) {
    case "off":
      return "off";
    case "new":
      return "new";
    case "all":
      return "all";
    case "verbose":
      return "verbose";
    default:
      return undefined;
  }
}

function normalizeBridgeProgressGrouping(value: unknown): BridgeProgressGrouping | undefined {
  switch (normalizeText(value)) {
    case "accumulate":
      return "accumulate";
    case "separate":
      return "separate";
    default:
      return undefined;
  }
}

function normalizeBridgeProgressConfig(value: unknown): BridgeProgressConfig | undefined {
  if (!value || typeof value !== "object" || Array.isArray(value)) {
    return undefined;
  }

  const candidate = value as Record<string, unknown>;
  const grouping = normalizeBridgeProgressGrouping(candidate.grouping);
  const toolProgress = normalizeBridgeProgressMode(candidate.tool_progress);
  if (!grouping || !toolProgress) {
    return undefined;
  }

  const progress: BridgeProgressConfig = {
    grouping,
    tool_progress: toolProgress,
  };
  if (typeof candidate.reactions === "boolean") {
    progress.reactions = candidate.reactions;
  }
  if (typeof candidate.typing === "boolean") {
    progress.typing = candidate.typing;
  }
  return progress;
}

const COMMON_BRIDGE_DELIVERY_DEFAULT_KEYS = new Set([
  "group_id",
  "mode",
  "peer_id",
  "progress",
  "thread_id",
]);

function providerBridgeDeliveryDefaults(value: Record<string, unknown>): Record<string, string> {
  return Object.fromEntries(
    Object.entries(value).filter((entry): entry is [string, string] => {
      const [key, fieldValue] = entry;
      return !COMMON_BRIDGE_DELIVERY_DEFAULT_KEYS.has(key) && typeof fieldValue === "string";
    })
  );
}

export function buildBridgeProviderKey(
  provider: Pick<BridgeProvider, "extension_name" | "platform">
) {
  return `${provider.extension_name}::${provider.platform}`;
}

export function findBridgeProviderByKey(providers: BridgeProvider[], providerKey: string) {
  return providers.find(provider => buildBridgeProviderKey(provider) === providerKey);
}

export function isBridgeProviderSelectable(provider: BridgeProvider): boolean {
  return provider.enabled && provider.health !== "unhealthy";
}

const BRIDGE_STATUS_TONE = {
  ready: "success",
  auth_required: "info",
  error: "danger",
  starting: "info",
  degraded: "warning",
  disabled: "neutral",
} as const satisfies Record<BridgeStatus, PillTone>;

export function bridgeStatusTone(status: BridgeStatus): PillTone {
  return BRIDGE_STATUS_TONE[status] ?? "neutral";
}

export function bridgeStatusLabel(status: BridgeStatus): string {
  return status.replaceAll("_", " ");
}

export function bridgeScopeTone(scope: BridgeScope): PillTone {
  return scope === "workspace" ? "info" : "neutral";
}

export function normalizeBridgeDeliveryDefaults(value: unknown): BridgeDeliveryDefaults {
  if (!value || typeof value !== "object" || Array.isArray(value)) {
    return {};
  }

  const candidate = value as Record<string, unknown>;
  const mode = candidate.mode;

  const normalized: BridgeDeliveryDefaults = {
    ...providerBridgeDeliveryDefaults(candidate),
    group_id: normalizeText(candidate.group_id),
    mode: mode === "direct-send" || mode === "reply" ? mode : undefined,
    peer_id: normalizeText(candidate.peer_id),
    thread_id: normalizeText(candidate.thread_id),
  };
  const progress = normalizeBridgeProgressConfig(candidate.progress);
  if (progress) {
    normalized.progress = progress;
  }
  return normalized;
}

export function compactBridgeDeliveryDefaults(
  value: BridgeDeliveryDefaults
): BridgeDeliveryDefaults | undefined {
  const normalized: BridgeDeliveryDefaults = {
    ...providerBridgeDeliveryDefaults(value as Record<string, unknown>),
  };
  const groupId = normalizeText(value.group_id);
  const peerId = normalizeText(value.peer_id);
  const progress = normalizeBridgeProgressConfig(value.progress);
  const threadId = normalizeText(value.thread_id);

  if (groupId) normalized.group_id = groupId;
  if (value.mode) normalized.mode = value.mode;
  if (peerId) normalized.peer_id = peerId;
  if (progress) normalized.progress = progress;
  if (threadId) normalized.thread_id = threadId;

  if (Object.keys(normalized).length === 0) {
    return undefined;
  }

  return normalized;
}

export function describeBridgeRoutingPolicy(policy: BridgeRoutingPolicy): string {
  const parts: string[] = [];

  if (policy.include_peer) {
    parts.push("peer");
  }
  if (policy.include_group) {
    parts.push("group");
  }
  if (policy.include_thread) {
    parts.push("thread");
  }

  return parts.length > 0 ? parts.join(" + ") : "No routing dimensions enabled";
}

export function describeBridgeDeliveryDefaults(value: unknown): string {
  const defaults = normalizeBridgeDeliveryDefaults(value);
  const parts: string[] = [];

  if (defaults.mode) {
    parts.push(defaults.mode);
  }
  if (defaults.peer_id) {
    parts.push(`peer:${defaults.peer_id}`);
  }
  if (defaults.group_id) {
    parts.push(`group:${defaults.group_id}`);
  }
  if (defaults.thread_id) {
    parts.push(`thread:${defaults.thread_id}`);
  }
  if (defaults.progress) {
    parts.push(`progress:${defaults.progress.tool_progress}`);
    parts.push(`grouping:${defaults.progress.grouping}`);
    parts.push(`typing:${defaults.progress.typing ?? false}`);
    parts.push(`reactions:${defaults.progress.reactions ?? false}`);
  }

  return parts.length > 0 ? parts.join(" · ") : "No delivery defaults configured";
}

export function describeBridgeDmPolicy(value?: BridgeDmPolicy | null): string {
  switch (value) {
    case "open":
      return "Open direct messages";
    case "allowlist":
      return "Allowlisted direct messages only";
    case "pairing":
      return "Pairing required before direct messages";
    default:
      return "Provider default";
  }
}

export function formatBridgeProviderConfig(value: unknown): string {
  if (!value || typeof value !== "object" || Array.isArray(value)) {
    return "";
  }

  const providerConfig = value as BridgeProviderConfig;
  if (Object.keys(providerConfig).length === 0) {
    return "";
  }

  return JSON.stringify(providerConfig, null, 2);
}

export function fingerprintBridgeProviderConfig(
  value: BridgeProviderConfig | null | undefined
): string {
  return stableBridgeJSON(value ?? null);
}

/** Canonical key-sorted JSON serialization for bridge comparison and fingerprints. */
export function stableBridgeJSON(value: unknown): string {
  return JSON.stringify(sortJSONValue(value)) ?? "null";
}

function sortJSONValue(value: unknown): unknown {
  if (Array.isArray(value)) {
    return value.map(sortJSONValue);
  }
  if (value === null || typeof value !== "object") {
    return value;
  }

  return Object.fromEntries(
    Object.entries(value)
      .sort(([left], [right]) => left.localeCompare(right))
      .map(([key, entry]) => [key, sortJSONValue(entry)])
  );
}

export function describeBridgeProviderConfigSchema(
  value?: BridgeProviderConfigSchemaHint | null
): string {
  if (!value?.schema && !value?.version) {
    return "No structured config schema published";
  }

  if (value.schema && value.version) {
    return `${value.schema} · v${value.version}`;
  }

  return value.schema ?? `v${value.version}`;
}

export function describeBridgeSecretSlot(slot: BridgeProviderSecretSlot): string {
  const requirement = slot.required === false ? "Optional" : "Required";
  return slot.description ? `${requirement} · ${slot.description}` : requirement;
}

export function formatBridgeDateTime(value?: string | null): string {
  if (!value) {
    return "Unavailable";
  }

  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }

  return date.toLocaleString("en-US", {
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
    month: "short",
    year: "numeric",
  });
}

export function formatBridgeRelativeTime(value?: string | null): string {
  if (!value) {
    return "Never";
  }

  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }

  const diffMs = date.getTime() - Date.now();
  if (Math.abs(diffMs) < 1000 * 60) {
    return "Just now";
  }

  const diffMinutes = Math.floor(Math.abs(diffMs) / (1000 * 60));
  if (diffMinutes < 60) {
    return diffMs >= 0 ? `In ${diffMinutes}m` : `${diffMinutes}m ago`;
  }

  const diffHours = Math.floor(diffMinutes / 60);
  if (diffHours < 24) {
    return diffMs >= 0 ? `In ${diffHours}h` : `${diffHours}h ago`;
  }

  const diffDays = Math.floor(diffHours / 24);
  return diffMs >= 0 ? `In ${diffDays}d` : `${diffDays}d ago`;
}

export function describeBridgeRouteTarget(
  route: Pick<BridgeRoute, "group_id" | "peer_id" | "thread_id">
) {
  const parts: string[] = [];

  if (route.peer_id) {
    parts.push(`peer:${route.peer_id}`);
  }
  if (route.group_id) {
    parts.push(`group:${route.group_id}`);
  }
  if (route.thread_id) {
    parts.push(`thread:${route.thread_id}`);
  }

  return parts.length > 0 ? parts.join(" · ") : "default target";
}

export function describeBridgeTargetQualifier(target: Pick<BridgeTarget, "qualifier">): string {
  return normalizeText(target.qualifier) ?? "No qualifier";
}

export function describeBridgeTargetCapabilities(
  target: Pick<BridgeTarget, "capabilities">
): string {
  return target.capabilities.length > 0 ? target.capabilities.join(" + ") : "No capabilities";
}

export function bridgeTargetTypeLabel(target: Pick<BridgeTarget, "target_type">): string {
  return target.target_type.replaceAll("_", " ");
}

export function describeBridgeTestTarget(
  target: Pick<BridgeDeliveryDefaults, "group_id" | "mode" | "peer_id" | "thread_id">
): string {
  const parts: string[] = [];

  if (target.mode) {
    parts.push(target.mode);
  }
  if (target.peer_id) {
    parts.push(`peer:${target.peer_id}`);
  }
  if (target.group_id) {
    parts.push(`group:${target.group_id}`);
  }
  if (target.thread_id) {
    parts.push(`thread:${target.thread_id}`);
  }

  return parts.length > 0 ? parts.join(" · ") : "Bridge defaults";
}

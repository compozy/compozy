import {
  buildBridgeProviderKey,
  compactBridgeDeliveryDefaults,
  formatBridgeProviderConfig,
  isBridgeProviderSelectable,
  normalizeBridgeDeliveryDefaults,
} from "@/systems/bridges/lib/bridge-formatters";

import type {
  BridgeCreateDraft,
  BridgeDeliveryDefaults,
  BridgeDmPolicy,
  BridgeSecretBinding,
  BridgeProviderConfig,
  BridgeProvider,
  BridgeSummary,
  BridgeUpdateDraft,
  CreateBridgeRequest,
  PutBridgeSecretBindingRequest,
  BridgeTestDeliveryDraft,
  UpdateBridgeRequest,
} from "@/systems/bridges/types";

type BridgeUpdateDraftSource = Omit<
  Pick<
    BridgeSummary,
    "delivery_defaults" | "display_name" | "dm_policy" | "provider_config" | "routing_policy"
  >,
  "delivery_defaults"
> & {
  delivery_defaults?: BridgeDeliveryDefaults | null;
};

export const DEFAULT_BRIDGE_ROUTING_POLICY = {
  include_group: true,
  include_peer: true,
  include_thread: true,
} as const;

interface BridgeCreateRequestProviderRef {
  extension_name: string;
  platform: string;
}

interface BridgeTestDeliveryDraftSource {
  delivery_defaults?: BridgeDeliveryDefaults | null;
}

type BuildBridgeCreateRequestResult =
  | {
      data: CreateBridgeRequest;
      ok: true;
    }
  | {
      error: string;
      ok: false;
    };

type BuildBridgeUpdateRequestResult =
  | {
      data: UpdateBridgeRequest;
      ok: true;
    }
  | {
      error: string;
      ok: false;
    };

type BuildBridgeSecretBindingRequestResult =
  | {
      data: PutBridgeSecretBindingRequest;
      ok: true;
    }
  | {
      error: string;
      ok: false;
    };

const DM_POLICIES = new Set<BridgeDmPolicy>(["allowlist", "open", "pairing"]);
const BRIDGE_SECRET_PATH_SEGMENT = /^[A-Za-z0-9][A-Za-z0-9_.-]*$/;

export function createBridgeCreateDraft(
  providers: BridgeProvider[],
  activeWorkspaceId?: string | null
): BridgeCreateDraft {
  const preferredProvider = providers.find(isBridgeProviderSelectable) ?? providers[0] ?? null;

  return {
    deliveryDefaults: {},
    dmPolicy: "",
    displayName: preferredProvider?.display_name ?? "",
    // Setup instances are created disabled: the provider-declared credentials
    // are bound after the POST, so a bridge that started on acceptance would be
    // running without them.
    enabled: false,
    notificationSuppress: false,
    secretSlotValues: {},
    providerConfigText: "",
    routingPolicy: { ...DEFAULT_BRIDGE_ROUTING_POLICY },
    scope: activeWorkspaceId ? "workspace" : "global",
    selectedProviderKey: preferredProvider ? buildBridgeProviderKey(preferredProvider) : "",
  };
}

export function createBridgeTestDeliveryDraft(
  bridge?: BridgeTestDeliveryDraftSource
): BridgeTestDeliveryDraft {
  const defaults = normalizeBridgeDeliveryDefaults(bridge?.delivery_defaults);
  return {
    message: "",
    target: {
      group_id: defaults.group_id,
      mode: defaults.mode,
      peer_id: defaults.peer_id,
      thread_id: defaults.thread_id,
    },
  };
}

export function createBridgeUpdateDraft(bridge?: BridgeUpdateDraftSource): BridgeUpdateDraft {
  return {
    deliveryDefaults: normalizeBridgeDeliveryDefaults(bridge?.delivery_defaults),
    dmPolicy: bridge?.dm_policy ?? "",
    displayName: bridge?.display_name ?? "",
    providerConfigText: formatBridgeProviderConfig(bridge?.provider_config),
    routingPolicy: bridge?.routing_policy ?? { ...DEFAULT_BRIDGE_ROUTING_POLICY },
  };
}

export function parseBridgeDmPolicy(value: string): BridgeDmPolicy | undefined {
  if (DM_POLICIES.has(value as BridgeDmPolicy)) {
    return value as BridgeDmPolicy;
  }

  return undefined;
}

export function parseBridgeProviderConfig(value: string): {
  error?: string;
  value?: BridgeProviderConfig;
} {
  const normalized = value.trim();
  if (normalized === "") {
    return {};
  }

  try {
    const parsed = JSON.parse(normalized) as unknown;

    if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) {
      return {
        error: "Provider configuration must be a JSON object.",
      };
    }

    const providerConfig = parsed as BridgeProviderConfig;

    if (Object.keys(providerConfig).length === 0) {
      return {};
    }

    return { value: providerConfig };
  } catch {
    return {
      error: "Provider configuration must be valid JSON.",
    };
  }
}

export function buildBridgeCreateRequest(
  draft: BridgeCreateDraft,
  provider: BridgeCreateRequestProviderRef,
  activeWorkspaceId?: string | null
): BuildBridgeCreateRequestResult {
  const providerConfigResult = parseBridgeProviderConfig(draft.providerConfigText);
  if (providerConfigResult.error) {
    return {
      error: providerConfigResult.error,
      ok: false,
    };
  }

  const scope = draft.scope;

  return {
    data: {
      delivery_defaults: compactBridgeDeliveryDefaults(draft.deliveryDefaults),
      display_name: draft.displayName.trim(),
      dm_policy: parseBridgeDmPolicy(draft.dmPolicy),
      // Credentials are bound only after the bridge instance exists. Activation
      // is therefore an explicit lifecycle action from the detail surface.
      enabled: false,
      extension_name: provider.extension_name,
      notification_suppress: draft.notificationSuppress,
      platform: provider.platform,
      provider_config: providerConfigResult.value,
      routing_policy: draft.routingPolicy,
      scope,
      workspace_id: scope === "workspace" ? (activeWorkspaceId ?? undefined) : undefined,
    },
    ok: true,
  };
}

export function buildBridgeUpdateRequest(draft: BridgeUpdateDraft): BuildBridgeUpdateRequestResult {
  const providerConfigResult = parseBridgeProviderConfig(draft.providerConfigText);
  if (providerConfigResult.error) {
    return {
      error: providerConfigResult.error,
      ok: false,
    };
  }

  return {
    data: {
      delivery_defaults: compactBridgeDeliveryDefaults(draft.deliveryDefaults) ?? null,
      display_name: draft.displayName.trim(),
      dm_policy: parseBridgeDmPolicy(draft.dmPolicy),
      provider_config: providerConfigResult.value ?? null,
      routing_policy: draft.routingPolicy,
    },
    ok: true,
  };
}

export function bridgeSecretBindingVaultRef(
  binding?: Pick<BridgeSecretBinding, "secret_ref"> | null
): string {
  const secretRef = binding?.secret_ref?.trim();
  if (!secretRef?.startsWith("vault:bridges/")) {
    return "";
  }

  return secretRef;
}

export function buildBridgeSecretBindingRequest(
  bridgeId: string,
  bindingName: string,
  secretValue: string,
  kind: string
): BuildBridgeSecretBindingRequestResult {
  const normalizedBridgeId = bridgeId.trim();
  const normalizedBindingName = bindingName.trim();
  const normalizedKind = kind.trim();
  if (
    !BRIDGE_SECRET_PATH_SEGMENT.test(normalizedBridgeId) ||
    !BRIDGE_SECRET_PATH_SEGMENT.test(normalizedBindingName)
  ) {
    return {
      error: "Secret binding must use a bridge vault reference.",
      ok: false,
    };
  }
  if (!secretValue.trim()) {
    return {
      error: "Secret binding value is required.",
      ok: false,
    };
  }

  return {
    data: {
      kind: normalizedKind,
      secret_ref: `vault:bridges/${normalizedBridgeId}/${normalizedBindingName}`,
      secret_value: secretValue,
    },
    ok: true,
  };
}

import type { AccountApprovalPayload, AccountDevicePayload } from "@/systems/account";

import type {
  AccountDeviceFixture,
  AccountDeviceKind,
  PendingApprovalFixture,
  RemoteScope,
} from "../mocks/remote-computers-types";
import type { RemoteDevicePayload } from "../types";

const KNOWN_SCOPES: ReadonlySet<RemoteScope> = new Set([
  "sessions:operate",
  "terminal:operate",
  "files:read",
  "files:write",
  "tasks:operate",
  "settings:write",
  "remote:admin",
]);

function formatRelativeLabel(iso: string | null | undefined, emptyFallback = "never"): string {
  if (!iso) return emptyFallback;
  const parsed = Date.parse(iso);
  if (Number.isNaN(parsed)) return iso;
  const diffSeconds = Math.max(0, Math.floor((Date.now() - parsed) / 1000));
  if (diffSeconds < 60) return "just now";
  const minutes = Math.floor(diffSeconds / 60);
  if (minutes < 60) return `${minutes}m ago`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h ago`;
  return `${Math.floor(hours / 24)}d ago`;
}

function normalizeRemoteDeviceKind(kind: string | undefined, name: string): AccountDeviceKind {
  if (kind === "browser" || kind === "cli" || kind === "desktop") return kind;
  const lower = name.toLowerCase();
  if (
    lower.includes("browser") ||
    lower.includes("safari") ||
    lower.includes("chrome") ||
    lower.includes("firefox") ||
    lower.includes("iphone") ||
    lower.includes("ipad")
  ) {
    return "browser";
  }
  if (lower.includes("cli")) return "cli";
  return "desktop";
}

export function toRemoteDeviceView(
  device: RemoteDevicePayload,
  currentDeviceId?: string | null,
  accountDevicesById?: ReadonlyMap<string, AccountDevicePayload>
): AccountDeviceFixture {
  const scopesOnLocal = device.scopes.filter((scope): scope is RemoteScope =>
    KNOWN_SCOPES.has(scope as RemoteScope)
  );
  const accountDevice = accountDevicesById?.get(device.id);
  return {
    id: device.id,
    name: device.name,
    kind: normalizeRemoteDeviceKind(accountDevice?.kind, device.name),
    admin: device.admin,
    thisDevice: Boolean(currentDeviceId && device.id === currentDeviceId),
    lastSeenAt: device.last_seen_at ?? "",
    lastSeenLabel: formatRelativeLabel(device.last_seen_at),
    keyFingerprint: device.approvals?.[0]?.jkt ?? "",
    scopesOnLocal,
  };
}

export function extractLocalPendingApprovals(
  devices: readonly RemoteDevicePayload[],
  options: {
    accountApprovals?: readonly AccountApprovalPayload[];
    accountDevicesById?: ReadonlyMap<string, AccountDevicePayload>;
    localComputerId?: string | null;
    localComputerName?: string;
    currentDeviceIsAdmin?: boolean;
  } = {}
): PendingApprovalFixture[] {
  const byId = new Map<string, PendingApprovalFixture>();
  const localComputerId = options.localComputerId?.trim() || "";
  const localComputerName = options.localComputerName || "This computer";

  for (const device of devices) {
    const accountDevice = options.accountDevicesById?.get(device.id);
    for (const approval of device.approvals ?? []) {
      if (approval.state !== "pending") continue;
      const deviceName = approval.name || device.name;
      byId.set(approval.id, {
        id: approval.id,
        computerId: localComputerId || "local",
        computerName: localComputerName,
        deviceName,
        deviceKind: normalizeRemoteDeviceKind(accountDevice?.kind, deviceName),
        keyFingerprint: approval.jkt,
        createdAt: approval.created_at,
        createdLabel: formatRelativeLabel(approval.created_at),
        state: "pending",
        appliedResult: null,
        canDecide: true,
      });
    }
  }

  for (const approval of options.accountApprovals ?? []) {
    if (localComputerId && approval.computer_id !== localComputerId) continue;
    if (approval.state !== "pending" && approval.state !== "decided") continue;
    const isDelivering = approval.state === "decided" && approval.delivery?.state === "delivering";
    if (approval.state === "decided" && !isDelivering) continue;
    const existing = byId.get(approval.id);
    const deviceName = approval.device_name || existing?.deviceName || "Unknown device";
    byId.set(approval.id, {
      id: approval.id,
      computerId: approval.computer_id,
      computerName: localComputerName,
      deviceName,
      deviceKind: normalizeRemoteDeviceKind(approval.device_kind, deviceName),
      keyFingerprint: approval.jkt || existing?.keyFingerprint || "",
      createdAt: approval.created_at,
      createdLabel: formatRelativeLabel(approval.created_at),
      state: isDelivering ? "delivering" : "pending",
      appliedResult: null,
      canDecide: existing ? true : Boolean(options.currentDeviceIsAdmin),
    });
  }

  return Array.from(byId.values());
}

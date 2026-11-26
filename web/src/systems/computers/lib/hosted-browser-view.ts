import { hostedComputerNameFromHostname } from "@/lib/hosted-browser-auth";

import type { HostedPeerPayload } from "../adapters/hosted-browser-api";
import type {
  ComputerFixture,
  HostedPeerFixture,
  RemoteScope,
} from "../mocks/remote-computers-types";

const KNOWN_SCOPES: ReadonlySet<RemoteScope> = new Set([
  "sessions:operate",
  "terminal:operate",
  "files:read",
  "files:write",
  "tasks:operate",
  "settings:write",
  "remote:admin",
]);

export const DEFAULT_HOSTED_SCOPES: readonly RemoteScope[] = [
  "sessions:operate",
  "terminal:operate",
  "files:read",
  "files:write",
  "tasks:operate",
  "settings:write",
];

/** Normalizes wire scope strings into known `RemoteScope` entries. */
export function normalizeHostedScopes(scopes?: readonly string[] | null): RemoteScope[] {
  if (!scopes || scopes.length === 0) return [...DEFAULT_HOSTED_SCOPES];
  return scopes.filter((scope): scope is RemoteScope => KNOWN_SCOPES.has(scope as RemoteScope));
}

/** True when `scopes` includes the requested remote scope. */
export function hasHostedScope(scopes: readonly RemoteScope[], scope: RemoteScope): boolean {
  return scopes.includes(scope);
}

/** Strips scheme and trailing slash from a hosted URL so menus display `slug.compozy.run`. */
export function hostnameFromHostedUrl(rawUrl: string): string {
  const trimmed = rawUrl.trim();
  if (!trimmed) return "";
  try {
    return new URL(trimmed.includes("://") ? trimmed : `https://${trimmed}`).hostname.toLowerCase();
  } catch {
    return trimmed
      .replace(/^https?:\/\//i, "")
      .replace(/\/.*$/, "")
      .toLowerCase();
  }
}

/** Ensures a peer navigation target is a full `https://` origin URL. */
export function peerTargetUrl(hostedUrl: string): string {
  const trimmed = hostedUrl.trim();
  if (/^https?:\/\//i.test(trimmed)) return trimmed;
  return `https://${trimmed.replace(/^\/+/, "")}`;
}

/**
 * Projects `GET /remote/v1/browser/peers` plus the current hosted hostname into
 * the `HostedPeerFixture[]` list consumed by `HostedComputerMenu` and `PhoneComputerMenu`.
 */
export function toHostedPeerViews(
  currentHostname: string,
  peers: readonly HostedPeerPayload[],
  currentName?: string
): HostedPeerFixture[] {
  const normalizedCurrent = currentHostname.trim().toLowerCase();
  const seen = new Set<string>();
  const result: HostedPeerFixture[] = [];

  for (const peer of peers) {
    const peerHost = hostnameFromHostedUrl(peer.hosted_url);
    if (!peerHost) continue;
    seen.add(peerHost);
    result.push({
      id: peer.computer_id,
      name: peer.name || hostedComputerNameFromHostname(peerHost),
      platform: "linux_x86_64",
      iconKind: "server",
      hostname: peerHost,
      hostedUrl: peerTargetUrl(peer.hosted_url),
      online: true,
      current: peerHost === normalizedCurrent,
    });
  }

  if (normalizedCurrent && !seen.has(normalizedCurrent)) {
    result.unshift({
      id: "current-hosted",
      name: currentName || hostedComputerNameFromHostname(normalizedCurrent),
      platform: "linux_x86_64",
      iconKind: "server",
      hostname: normalizedCurrent,
      hostedUrl: `https://${normalizedCurrent}`,
      online: true,
      current: true,
    });
  }

  return result;
}

/** Builds a `ComputerFixture` view representing the hosted computer serving this tab. */
export function toHostedComputerFixture(
  hostname: string,
  peers: readonly HostedPeerFixture[],
  scopes: readonly RemoteScope[]
): ComputerFixture {
  const normalizedHost = hostname.trim().toLowerCase();
  const current = peers.find(peer => peer.current);
  const name = current?.name || hostedComputerNameFromHostname(normalizedHost);
  return {
    id: current?.id ?? "local",
    name,
    platform: current?.platform ?? "linux_x86_64",
    iconKind: current?.iconKind ?? "server",
    version: "",
    protocol: { min: 1, max: 1 },
    status: "connected",
    enabled: true,
    planState: "ok",
    originKind: "account",
    routeInUse: {
      id: "hosted-cloud-route",
      kind: "cloud",
      address: normalizedHost,
      label: "via hosted route",
      inUse: true,
      learned: false,
      lastError: null,
    },
    routes: [
      {
        id: "hosted-cloud-route",
        kind: "cloud",
        address: normalizedHost,
        label: "via hosted route",
        inUse: true,
        learned: false,
        lastError: null,
      },
    ],
    scopes: [...scopes],
    load: null,
    loadBalancingWeight: 50,
    diagnosis: null,
    providers: [],
    projects: [],
    addedAt: "",
    lastConnectedAt: "",
    lastConnectedLabel: "now",
    hostedUrl: normalizedHost,
  };
}

import type {
  ComputerFixture,
  ComputerIconKind,
  ComputerPlatform,
  ComputerRouteFixture,
  ComputerRouteKind,
  ComputerStatus,
  LoadBalancingWeight,
  LocalComputerFixture,
  RemoteScope,
} from "../mocks/remote-computers-types";
import type { ComputerPayload, ComputerRoutePayload, LocalComputerPayload } from "../types";

const KNOWN_PLATFORMS: ReadonlySet<ComputerPlatform> = new Set([
  "darwin_arm64",
  "darwin_x86_64",
  "linux_x86_64",
  "linux_arm64",
  "windows_x86_64",
]);

const KNOWN_STATUSES: ReadonlySet<ComputerStatus> = new Set([
  "connected",
  "connecting",
  "offline",
  "needs_attention",
  "needs_pro",
  "incompatible",
  "disabled",
]);

const KNOWN_SCOPES: ReadonlySet<RemoteScope> = new Set([
  "sessions:operate",
  "terminal:operate",
  "files:read",
  "files:write",
  "tasks:operate",
  "settings:write",
  "remote:admin",
]);

export function normalizeComputerPlatform(platform?: string | null): ComputerPlatform {
  if (platform && KNOWN_PLATFORMS.has(platform as ComputerPlatform)) {
    return platform as ComputerPlatform;
  }
  if (platform?.startsWith("darwin")) return "darwin_arm64";
  if (platform?.startsWith("windows")) return "windows_x86_64";
  return "linux_x86_64";
}

export function iconKindForPlatform(platform?: string | null): ComputerIconKind {
  const normalized = normalizeComputerPlatform(platform);
  if (normalized.startsWith("darwin")) return "desktop";
  if (normalized.startsWith("windows")) return "windows";
  return "server";
}

export function normalizeComputerStatus(status?: string | null): ComputerStatus {
  if (status && KNOWN_STATUSES.has(status as ComputerStatus)) {
    return status as ComputerStatus;
  }
  return "offline";
}

function normalizeRouteKind(kind: string): ComputerRouteKind {
  if (kind === "direct" || kind === "ssh" || kind === "cloud") return kind;
  return "direct";
}

function routeDisplayLabel(route: ComputerRoutePayload): string {
  if (route.kind === "direct") return `via LAN ${route.address}`;
  if (route.kind === "ssh") return `via SSH ${route.address}`;
  if (route.kind === "cloud") return "via hosted route";
  return route.address;
}

function normalizeWeight(weight: number): LoadBalancingWeight {
  if (weight >= 100) return 100;
  if (weight >= 50) return 50;
  if (weight >= 25) return 25;
  return 0;
}

function formatLastConnectedLabel(lastConnectedAt: string | null): string {
  if (!lastConnectedAt) return "never";
  const parsed = Date.parse(lastConnectedAt);
  if (Number.isNaN(parsed)) return lastConnectedAt;
  const diffSeconds = Math.max(0, Math.floor((Date.now() - parsed) / 1000));
  if (diffSeconds < 60) return "just now";
  const minutes = Math.floor(diffSeconds / 60);
  if (minutes < 60) return `${minutes}m ago`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h ago`;
  return `${Math.floor(hours / 24)}d ago`;
}

/**
 * Projects a live generated `Computer` DTO into the view model consumed by
 * `RunOnPicker`, `ComputerChip`, and multi-computer row badges.
 */
export function toComputerView(computer: ComputerPayload): ComputerFixture {
  const platform = normalizeComputerPlatform(computer.platform);
  const routes: ComputerRouteFixture[] = computer.routes.map(route => ({
    id: route.id,
    kind: normalizeRouteKind(route.kind),
    address: route.address,
    label: routeDisplayLabel(route),
    inUse: route.in_use,
    learned: route.learned,
    lastError: route.last_error,
  }));
  const routeInUse = computer.route_in_use
    ? (routes.find(r => r.id === computer.route_in_use?.id) ?? {
        id: computer.route_in_use.id,
        kind: normalizeRouteKind(computer.route_in_use.kind),
        address: computer.route_in_use.label,
        label: computer.route_in_use.label,
        inUse: true,
        learned: false,
        lastError: null,
      })
    : null;
  const scopes = computer.scopes.filter((scope): scope is RemoteScope =>
    KNOWN_SCOPES.has(scope as RemoteScope)
  );
  const sshRoute = routes.find(route => route.kind === "ssh");

  return {
    id: computer.id,
    name: computer.name,
    platform,
    iconKind: iconKindForPlatform(platform),
    version: computer.version,
    protocol: computer.protocol ?? { min: 1, max: 1 },
    status: normalizeComputerStatus(computer.status),
    enabled: computer.enabled,
    planState: computer.plan_state === "limited" ? "limited" : "ok",
    originKind: sshRoute ? "ssh" : "account",
    ...(sshRoute ? { sshTarget: sshRoute.address } : {}),
    routeInUse,
    routes,
    scopes,
    load: null,
    loadBalancingWeight: normalizeWeight(computer.load_balancing_weight),
    diagnosis: computer.diagnosis
      ? {
          failingStep:
            computer.diagnosis.step === "cloud" ||
            computer.diagnosis.step === "route" ||
            computer.diagnosis.step === "computer" ||
            computer.diagnosis.step === "sign_in"
              ? computer.diagnosis.step
              : "route",
          code: computer.diagnosis.code,
          message: computer.diagnosis.message,
          at: computer.diagnosis.at,
          steps: [],
        }
      : null,
    providers: [],
    projects: [],
    addedAt: computer.added_at,
    lastConnectedAt: computer.last_connected_at ?? "",
    lastConnectedLabel: formatLastConnectedLabel(computer.last_connected_at),
  };
}

/** Projects the local daemon info into the `LocalComputerFixture` shape expected by `RunOnPicker`. */
export function toLocalComputerView(
  local: LocalComputerPayload | undefined,
  options: { name?: string; platform?: ComputerPlatform } = {}
): LocalComputerFixture {
  const platform = options.platform ?? "darwin_arm64";
  return {
    id: "local",
    name: options.name ?? "This computer",
    platform,
    iconKind: platform.startsWith("darwin") ? "laptop" : iconKindForPlatform(platform),
    version: "",
    localRunAgents: local?.run_agents ?? true,
    activeLocalSessions: 0,
    remoteEnabled: false,
    hostedRouteEnabled: false,
    hostedRouteState: "off",
    hostedUrl: "",
    requireDeviceApproval: false,
    load: {
      cpuCount: 0,
      cpuUtil: 0,
      memTotalBytes: 0,
      memAvailableBytes: 0,
      sampledAt: "",
      sampledAgoLabel: "",
    },
    loadBalancingWeight: 50,
    projects: [],
  };
}

export interface RunOnOption {
  id: string;
  name: string;
  status: ComputerStatus;
  selectable: boolean;
  hint?: string;
}

/**
 * Pure option projection for `RunOnPicker` (`_uiux.md` U4, UT-080/UT-083).
 * `needs_pro` and `incompatible` computers are listed with a clear hint and marked non-selectable.
 * Offline computers stay selectable for drafting so Send can explain `<name> is offline`.
 */
export function runOnOptions(
  computers: readonly Pick<ComputerPayload, "id" | "name" | "status" | "enabled">[]
): RunOnOption[] {
  const options: RunOnOption[] = [];
  for (const computer of computers) {
    if (!computer.enabled) continue;
    const status = normalizeComputerStatus(computer.status);
    if (status === "needs_pro") {
      options.push({
        id: computer.id,
        name: computer.name,
        status,
        selectable: false,
        hint: "Needs Pro",
      });
      continue;
    }
    if (status === "incompatible") {
      options.push({
        id: computer.id,
        name: computer.name,
        status,
        selectable: false,
        hint: "Incompatible",
      });
      continue;
    }
    if (status === "offline") {
      options.push({
        id: computer.id,
        name: computer.name,
        status,
        selectable: true,
        hint: "Offline",
      });
      continue;
    }
    options.push({
      id: computer.id,
      name: computer.name,
      status,
      selectable: true,
    });
  }
  return options;
}

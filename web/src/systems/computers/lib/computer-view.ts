import { ComputersApiError } from "../adapters/computers-api";
import type {
  AccountDeviceFixture,
  AccountUnaddedComputerFixture,
  ComputerAttentionReasonFixture,
  ComputerDiagnosisFixture,
  ComputerDiagnosisStepFixture,
  ComputerFixture,
  ComputerIconKind,
  ComputerPlatform,
  ComputerRouteFixture,
  ComputerRouteKind,
  ComputerStatus,
  DiagnosisHop,
  HostedRouteState,
  LoadBalancingWeight,
  LocalComputerFixture,
  PendingApprovalFixture,
  RemoteScope,
  SshHostSuggestionFixture,
} from "../mocks/remote-computers-types";
import type {
  AvailableComputerPayload,
  ComputerPayload,
  ComputerRoutePayload,
  ComputerSSHHostPayload,
  LocalComputerPayload,
  RemoteDevicePayload,
  RemoteStatusPayload,
} from "../types";

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

const DIAGNOSIS_HOPS: readonly { hop: DiagnosisHop; label: string }[] = [
  { hop: "cloud", label: "Compozy Cloud" },
  { hop: "route", label: "Route" },
  { hop: "computer", label: "Computer" },
  { hop: "sign_in", label: "Sign-in" },
];

export function isComputersCloudUnreachableError(error: unknown): boolean {
  if (error instanceof ComputersApiError) {
    return error.code === "account_cloud_unreachable" || error.status === 503;
  }
  return false;
}

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

function isRouteIdentityMismatch(lastError: string | null | undefined): boolean {
  if (!lastError) return false;
  const lower = lastError.toLowerCase();
  return (
    lower.includes("route_identity_mismatch") ||
    lower.includes("different computer at this address") ||
    lower.includes("identity mismatch")
  );
}

function normalizeWeight(weight: number): LoadBalancingWeight {
  if (weight >= 100) return 100;
  if (weight >= 50) return 50;
  if (weight >= 25) return 25;
  return 0;
}

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

function buildDiagnosisView(computer: ComputerPayload): ComputerDiagnosisFixture | null {
  if (!computer.diagnosis) return null;
  const failingStep: DiagnosisHop =
    computer.diagnosis.step === "cloud" ||
    computer.diagnosis.step === "route" ||
    computer.diagnosis.step === "computer" ||
    computer.diagnosis.step === "sign_in"
      ? computer.diagnosis.step
      : "route";
  const failIdx = DIAGNOSIS_HOPS.findIndex(item => item.hop === failingStep);
  const relativeAt = formatRelativeLabel(computer.diagnosis.at, "");

  const steps: ComputerDiagnosisStepFixture[] = DIAGNOSIS_HOPS.map((item, idx) => {
    if (idx < failIdx) {
      return {
        step: item.hop,
        label: item.label,
        status: "done",
        message:
          item.hop === "cloud"
            ? "Account verified"
            : item.hop === "route"
              ? (computer.route_in_use?.label ?? "Route reachable")
              : `CompozyOS ${computer.version || "connected"}`,
        at: relativeAt || undefined,
      };
    }
    if (idx === failIdx) {
      return {
        step: item.hop,
        label: item.label,
        status: "failed",
        message: computer.diagnosis?.message,
        at: relativeAt || undefined,
      };
    }
    return {
      step: item.hop,
      label: item.label,
      status: "skipped",
      message: "Not checked",
    };
  });

  const whatToTry: string[] = [];
  if (failingStep === "route") {
    whatToTry.push(`Check that ${computer.name} is powered on and reachable on your network.`);
    whatToTry.push("Open the Routes tab to remove stale or mismatched learned addresses.");
  } else if (failingStep === "sign_in") {
    whatToTry.push(
      `Open CompozyOS on ${computer.name} or a device trusted for admin to approve this device.`
    );
  } else if (failingStep === "computer") {
    whatToTry.push(`Check that CompozyOS is running and up to date on ${computer.name}.`);
  } else if (failingStep === "cloud") {
    whatToTry.push("Check your internet connection and Compozy Cloud status.");
  }

  return {
    failingStep,
    code: computer.diagnosis.code,
    message: computer.diagnosis.message,
    at: computer.diagnosis.at,
    whatToTry,
    steps,
  };
}

function buildAttentionReason(
  computer: ComputerPayload
): ComputerAttentionReasonFixture | undefined {
  if (computer.diagnosis?.code === "linger_off") {
    const sshRoute = computer.routes.find(route => route.kind === "ssh");
    const userPart = sshRoute?.address.includes("@")
      ? sshRoute.address.slice(0, sshRoute.address.indexOf("@"))
      : "$USER";
    return {
      kind: "linger_off",
      summary: `Runs only while you're logged in on ${computer.name}`,
      detail:
        computer.diagnosis.message ||
        "Enable Linux user lingering so agents keep running after SSH closes.",
      command: `sudo loginctl enable-linger ${userPart}`,
      actionLabel: "Fix",
    };
  }
  return undefined;
}

/**
 * Projects a live generated `Computer` DTO into the view model consumed by
 * `ComputerRow`, `ComputerDetailsPanel`, `RunOnPicker`, and `ComputerChip`.
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
    identityMismatch: isRouteIdentityMismatch(route.last_error),
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
  const cloudRoute = routes.find(route => route.kind === "cloud");

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
    attentionReason: buildAttentionReason(computer),
    diagnosis: buildDiagnosisView(computer),
    providers: [],
    projects: [],
    addedAt: computer.added_at,
    lastConnectedAt: computer.last_connected_at ?? "",
    lastConnectedLabel: formatRelativeLabel(computer.last_connected_at),
    ...(cloudRoute ? { hostedUrl: cloudRoute.address } : {}),
  };
}

function normalizeHostedRouteState(state: string | undefined, enabled: boolean): HostedRouteState {
  if (!enabled) return "off";
  if (state === "ready" || state === "connected" || state === "active") return "ready";
  if (state === "starting" || state === "connecting" || state === "pending") return "starting";
  if (state === "unreachable" || state === "error" || state === "failed") return "unreachable";
  return "starting";
}

/** Projects the local daemon info + optional `/api/remote` status into `LocalComputerFixture`. */
export function toLocalComputerView(
  local: LocalComputerPayload | undefined,
  options: {
    name?: string;
    platform?: ComputerPlatform;
    remote?: RemoteStatusPayload;
    activeLocalSessions?: number;
  } = {}
): LocalComputerFixture {
  const platform = options.platform ?? "darwin_arm64";
  const remote = options.remote;
  const hostedEnabled = Boolean(remote?.cloud_route?.enabled);
  const hostedState = normalizeHostedRouteState(remote?.cloud_route?.state, hostedEnabled);

  return {
    id: "local",
    name: options.name ?? "This computer",
    platform,
    iconKind: platform.startsWith("darwin") ? "laptop" : iconKindForPlatform(platform),
    version: "",
    localRunAgents: local?.run_agents ?? true,
    activeLocalSessions: options.activeLocalSessions ?? 0,
    remoteEnabled: Boolean(remote?.enabled),
    hostedRouteEnabled: hostedEnabled,
    hostedRouteState: hostedState,
    hostedUrl: remote?.cloud_route?.hostname ?? "",
    requireDeviceApproval: Boolean(remote?.require_device_approval),
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

export function toAvailableComputerView(
  computer: AvailableComputerPayload
): AccountUnaddedComputerFixture {
  return {
    id: computer.id,
    name: computer.name,
    platform: "linux_x86_64",
    iconKind: "server",
    online: computer.online,
    hostedOnly: false,
    linkedVia: "device",
  };
}

export function toSSHHostSuggestionView(
  host: ComputerSSHHostPayload,
  addedComputers: readonly ComputerPayload[]
): SshHostSuggestionFixture {
  const targetHost = host.hostname || host.alias;
  const alreadyAdded = addedComputers.some(computer =>
    computer.routes.some(
      route =>
        route.kind === "ssh" &&
        (route.address === targetHost ||
          route.address.includes(`@${targetHost}`) ||
          route.address.startsWith(`${targetHost}:`) ||
          computer.name.toLowerCase() === host.alias.toLowerCase())
    )
  );
  return {
    alias: host.alias,
    hostname: host.hostname || host.alias,
    user: host.user,
    port: host.port || 22,
    source: host.source === "known_hosts" ? "known_hosts" : "ssh_config",
    alreadyAdded,
  };
}

export function toRemoteDeviceView(
  device: RemoteDevicePayload,
  currentDeviceId?: string | null
): AccountDeviceFixture {
  const scopesOnLocal = device.scopes.filter((scope): scope is RemoteScope =>
    KNOWN_SCOPES.has(scope as RemoteScope)
  );
  return {
    id: device.id,
    name: device.name,
    kind: "desktop",
    admin: device.admin,
    thisDevice: Boolean(currentDeviceId && device.id === currentDeviceId),
    lastSeenAt: device.last_seen_at ?? "",
    lastSeenLabel: formatRelativeLabel(device.last_seen_at),
    keyFingerprint: device.approvals?.[0]?.jkt ?? "",
    scopesOnLocal,
  };
}

export function extractLocalPendingApprovals(
  devices: readonly RemoteDevicePayload[]
): PendingApprovalFixture[] {
  const approvals: PendingApprovalFixture[] = [];
  for (const device of devices) {
    for (const approval of device.approvals ?? []) {
      if (approval.state !== "pending") continue;
      approvals.push({
        id: approval.id,
        computerId: "local",
        computerName: "This computer",
        deviceName: approval.name || device.name,
        deviceKind: "desktop",
        keyFingerprint: approval.jkt,
        createdAt: approval.created_at,
        createdLabel: formatRelativeLabel(approval.created_at),
        state: "pending",
        appliedResult: null,
        canDecide: true,
      });
    }
  }
  return approvals;
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

import type { RunOnTarget } from "../components/run-on-picker";
import {
  LOAD_BALANCING_WEIGHT_OPTIONS,
  type ComputerFixture,
  type ComputerLoadFixture,
  type LoadBalancingWeight,
  type LocalComputerFixture,
} from "../mocks/remote-computers-types";
import type { HostResourcesResponse } from "../types";

/** Resource samples older than 15 s count as unsampled (Business Rule 16, US-015.EC-1). */
export const MAX_RESOURCE_SAMPLE_AGE_MS = 15_000;

export const UNSAMPLED_LOCAL_LOAD: ComputerLoadFixture = {
  cpuCount: 0,
  cpuUtil: 0,
  memTotalBytes: 0,
  memAvailableBytes: 0,
  sampledAt: "",
  sampledAgoLabel: "",
  stale: true,
};

export function normalizeLoadBalancingWeight(weight: number): LoadBalancingWeight {
  if (weight >= 100) return 100;
  if (weight >= 50) return 50;
  if (weight >= 25) return 25;
  return 0;
}

export function loadBalancingWeightLabel(weight: LoadBalancingWeight): string {
  return LOAD_BALANCING_WEIGHT_OPTIONS.find(option => option.value === weight)?.label ?? "Normal";
}

function formatSampleAgeLabel(ageMs: number): string {
  const seconds = Math.max(0, Math.floor(ageMs / 1000));
  if (seconds < 2) return "just now";
  if (seconds < 60) return `${seconds}s ago`;
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes}m ago`;
  return `${Math.floor(minutes / 60)}h ago`;
}

/**
 * Projects a live `/api/host/resources` sample into `ComputerLoadFixture`.
 * Returns `null` when CPU or memory readings are missing or invalid so the UI renders "Unsampled".
 */
export function toHostLoadView(
  sample: HostResourcesResponse | null | undefined,
  nowMs = Date.now()
): ComputerLoadFixture | null {
  if (!sample) return null;
  if (
    sample.cpu_count <= 0 ||
    sample.cpu_util == null ||
    sample.mem_total == null ||
    sample.mem_available == null ||
    sample.mem_total <= 0 ||
    Number.isNaN(sample.cpu_util) ||
    sample.cpu_util < 0 ||
    sample.cpu_util > 1 ||
    sample.mem_available > sample.mem_total
  ) {
    return null;
  }
  const parsed = Date.parse(sample.sampled_at);
  if (Number.isNaN(parsed)) return null;
  const ageMs = Math.max(0, nowMs - parsed);
  return {
    cpuCount: sample.cpu_count,
    cpuUtil: sample.cpu_util,
    memTotalBytes: sample.mem_total,
    memAvailableBytes: sample.mem_available,
    sampledAt: sample.sampled_at,
    sampledAgoLabel: formatSampleAgeLabel(ageMs),
    stale: ageMs > MAX_RESOURCE_SAMPLE_AGE_MS,
  };
}

function isSelectableRemoteComputer(computer: ComputerFixture): boolean {
  return computer.enabled && computer.status !== "needs_pro" && computer.status !== "incompatible";
}

function isConnectedRemoteComputer(computer: ComputerFixture): boolean {
  return (
    computer.enabled && (computer.status === "connected" || computer.status === "needs_attention")
  );
}

/**
 * Resolves the effective Run on target for the session create dialog (`_uiux.md` U4, US-014, US-015).
 *
 * - Keeps `"auto"` only while load balancing is on and allowed.
 * - Never leaves `"local"` active when `localRunAgents` is false and a selectable remote exists.
 * - Falls back cleanly when a previously picked remote is removed, disabled, or becomes blocked.
 */
export function resolveEffectiveRunOnTarget(input: {
  requested: RunOnTarget;
  local: Pick<LocalComputerFixture, "localRunAgents">;
  computers: readonly ComputerFixture[];
  loadBalancingEnabled: boolean;
  allowAuto: boolean;
}): RunOnTarget {
  const { requested, local, computers, loadBalancingEnabled, allowAuto } = input;
  const canUseAuto = allowAuto && loadBalancingEnabled;

  if (requested === "auto" && canUseAuto) {
    return "auto";
  }

  if (requested !== "local" && requested !== "auto") {
    const match = computers.find(item => item.id === requested);
    if (match && isSelectableRemoteComputer(match)) {
      return match.id;
    }
  }

  if (requested === "local" && local.localRunAgents) {
    return "local";
  }

  const firstConnected = computers.find(isConnectedRemoteComputer);
  if (firstConnected) return firstConnected.id;

  if (canUseAuto) return "auto";

  if (local.localRunAgents) return "local";

  const firstSelectable = computers.find(isSelectableRemoteComputer);
  return firstSelectable ? firstSelectable.id : "local";
}

export interface AutoPlacementCandidateView {
  id: string;
  name: string;
  load: ComputerLoadFixture | null;
  weightLabel: string;
  winner: boolean;
}

/**
 * Builds the candidate rows shown under the `Auto` resolution panel in Session Create
 * (`remote-computers-05-run-on` parity), placing the daemon-selected winner first.
 */
export function buildAutoPlacementCandidates(
  winnerComputerId: string,
  local: LocalComputerFixture,
  computers: readonly ComputerFixture[]
): AutoPlacementCandidateView[] {
  const rows: Array<AutoPlacementCandidateView & { weight: number }> = [];

  if (local.localRunAgents && local.loadBalancingWeight > 0) {
    rows.push({
      id: "local",
      name: "This computer",
      load: local.load,
      weight: local.loadBalancingWeight,
      weightLabel: loadBalancingWeightLabel(local.loadBalancingWeight),
      winner: winnerComputerId === "local",
    });
  }

  for (const computer of computers) {
    if (!isConnectedRemoteComputer(computer) || computer.loadBalancingWeight <= 0) continue;
    rows.push({
      id: computer.id,
      name: computer.name,
      load: computer.load,
      weight: computer.loadBalancingWeight,
      weightLabel: loadBalancingWeightLabel(computer.loadBalancingWeight),
      winner: winnerComputerId === computer.id,
    });
  }

  rows.sort((a, b) => {
    if (a.winner !== b.winner) return a.winner ? -1 : 1;
    if (b.weight !== a.weight) return b.weight - a.weight;
    return a.name.localeCompare(b.name);
  });

  return rows.map(row => ({
    id: row.id,
    name: row.name,
    load: row.load,
    weightLabel: row.weightLabel,
    winner: row.winner,
  }));
}

/**
 * Plain-language reason sentence paired with `No computer can run this project.`
 * when `POST /api/computers/placement` returns `409 no_eligible_computer` (US-015.EC-1, UT-106).
 */
export function describeNoEligibleComputerReason(
  projectName: string | null | undefined,
  reasons?: Record<string, string>
): string {
  const label = projectName?.trim() || "This project";
  if (!reasons || Object.keys(reasons).length === 0) {
    return `${label} isn't ready on any eligible computer. Pick another project or check Load balancing in Settings.`;
  }

  const values = new Set(Object.values(reasons));
  if (values.has("cpu_busy") || values.has("memory_busy")) {
    return `Every eligible computer for ${label} is currently at capacity. Wait a moment or pick a specific computer.`;
  }
  if (
    values.has("manual_only") &&
    !values.has("project_not_found") &&
    !values.has("computer_offline")
  ) {
    return `Every connected computer is set to Manual only. Change load balancing weights in Settings or pick a specific computer.`;
  }
  if (values.has("computer_offline") && !values.has("project_not_found")) {
    return `${label} is only on computers that are currently offline. Pick another project or reconnect a computer.`;
  }
  return `${label} isn't available on any connected computer eligible for Auto. Pick another project or choose a computer directly.`;
}

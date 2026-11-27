import type {
  ComputerFixture,
  ComputerStatus,
  RemoteScope,
} from "../mocks/remote-computers-types";

/**
 * How one computer's session-list read went (`model.ts` visual contract).
 * Only `ok` and `offline` carry rows in the UI; every other state shows its
 * own note or skeleton and never fabricates a zero count (`S1`–`S6`, `S11`).
 */
export type ComputerRead =
  | "ok"
  | "connecting"
  | "failed"
  | "offline"
  | "offline_empty"
  | "needs_pro"
  | "incompatible";

export interface ComputerReadInput {
  status: ComputerStatus;
  /** TanStack facets query status when the computer is connected. */
  queryStatus?: "pending" | "error" | "success";
  /** True when cached session rows exist in memory for this computer (Rule 23). */
  hasCachedRows?: boolean;
  /** True when this entry represents the local daemon. */
  isLocal?: boolean;
}

/**
 * Pure mapper from computer registry status + query state + in-memory cache
 * presence into one `ComputerRead` state (`T-F4`).
 */
export function mapComputerRead({
  status,
  queryStatus,
  hasCachedRows = false,
  isLocal = false,
}: ComputerReadInput): ComputerRead {
  if (isLocal) {
    if (queryStatus === "error" && !hasCachedRows) return "failed";
    return "ok";
  }
  switch (status) {
    case "connecting":
      return "connecting";
    case "offline":
      return hasCachedRows ? "offline" : "offline_empty";
    case "needs_pro":
      return "needs_pro";
    case "incompatible":
    case "needs_attention":
      return "incompatible";
    case "disabled":
      return "offline_empty";
    case "connected":
      if (queryStatus === "error") return "failed";
      if (queryStatus === "pending") return "connecting";
      return "ok";
  }
}

const READ_DETAIL: Partial<Record<ComputerRead, string>> = {
  connecting: "Connecting…",
  needs_pro: "Needs Pro",
  incompatible: "Update needed",
};

/** Subtitle shown next to a computer's name in section and band headers. */
export function computerReadDetail(
  computer: Pick<ComputerFixture, "routeInUse" | "lastConnectedLabel"> | null | undefined,
  read: ComputerRead,
  isLocal = false
): string {
  if (isLocal) return "This computer";
  if (read === "offline" || read === "offline_empty") {
    return `Offline · ${computer?.lastConnectedLabel || "never"}`;
  }
  return READ_DETAIL[read] ?? computer?.routeInUse?.label ?? "Connected";
}

/** True when this computer's header and totals carry a known session count (`S11`). */
export function isComputerCounted(read: ComputerRead): boolean {
  return read === "ok" || read === "offline";
}

/**
 * Computers omitted from aggregate totals and named in `Not counted: <names>` (`R9`).
 * Plan-limited (`needs_pro`) computers are intentionally excluded from the active total.
 */
export function isComputerUncounted(read: ComputerRead): boolean {
  return read !== "ok" && read !== "offline" && read !== "needs_pro";
}

/** Names of computers that did not answer, in section order, for the footer (`R9`). */
export function uncountedComputerNames(
  sources: readonly { name: string; read: ComputerRead }[]
): string[] {
  const names: string[] = [];
  for (const source of sources) {
    if (isComputerUncounted(source.read)) names.push(source.name);
  }
  return names;
}

/** True when the operator may mutate sessions and folders on this computer (`F11`). */
export function computerCanOperateSessions(
  scopes: readonly RemoteScope[] | readonly string[] | undefined,
  isLocal: boolean
): boolean {
  if (isLocal) return true;
  return Boolean(scopes?.includes("sessions:operate"));
}

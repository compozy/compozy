/**
 * What the List and the Kanban read about each computer in view: whether its list loaded, the
 * words its header shows, and its rows. One computer section renders one daemon's list.
 */
import type {
  ComputerFixture,
  LocalComputerFixture,
} from "@/systems/computers/mocks/remote-computers-fixtures";
import { sessionBadgeSignal } from "@/systems/session/lib/session-badge";
import type { SessionListLeadingSignal } from "@/systems/session/components/session-list/session-list-disclosure-mark";
import type { SessionPayload } from "@/systems/session/types";

import { PROJECTS, type ComputerSession } from "./fixtures";

/** How a computer's list read went. Only `ok` and `offline` carry rows. */
export type ComputerRead =
  | "ok"
  | "connecting"
  | "failed"
  | "offline"
  | "offline_empty"
  | "needs_pro"
  | "incompatible";

export interface RailComputer {
  /** `local`, or the remote computer id. */
  key: string;
  name: string;
  computer: LocalComputerFixture | ComputerFixture;
  read: ComputerRead;
  /** Where it is, in the words Run on uses. */
  detail: string;
  sessions: readonly SessionPayload[];
  /** The section's pager has a next page. */
  more?: boolean;
}

/** How remote computers share the window: merged, one section each, or one computer only. */
export type RailLayout = "together" | "separate" | "scoped";

function readOf(computer: ComputerFixture, rows: number): ComputerRead {
  switch (computer.status) {
    case "connecting":
      return "connecting";
    case "offline":
      return rows > 0 ? "offline" : "offline_empty";
    case "needs_pro":
      return "needs_pro";
    case "incompatible":
      return "incompatible";
    default:
      return "ok";
  }
}

const DETAIL: Partial<Record<ComputerRead, string>> = {
  connecting: "Connecting…",
  needs_pro: "Needs Pro",
  incompatible: "Update needed",
};

function detailOf(computer: ComputerFixture, read: ComputerRead): string {
  if (read === "offline" || read === "offline_empty") {
    return `Offline · ${computer.lastConnectedLabel}`;
  }
  return DETAIL[read] ?? computer.routeInUse?.label ?? "Connected";
}

/** This computer first, then each remote computer, with a read override per state. */
export function railComputers(
  local: LocalComputerFixture,
  computers: readonly ComputerFixture[],
  entries: readonly ComputerSession[],
  overrides: Readonly<Record<string, { read?: ComputerRead; more?: boolean }>> = {}
): RailComputer[] {
  const rowsOf = (key: string) =>
    entries.flatMap(item => (item.computerId === key ? [item.session] : []));
  const head: RailComputer = {
    key: "local",
    name: local.name,
    computer: local,
    read: "ok",
    detail: "This computer",
    sessions: rowsOf("local"),
    ...overrides.local,
  };
  return [
    head,
    ...computers.map(computer => {
      const sessions = rowsOf(computer.id);
      const read = overrides[computer.id]?.read ?? readOf(computer, sessions.length);
      const carries = read === "ok" || read === "offline";
      return {
        key: computer.id,
        name: computer.name,
        computer,
        read,
        detail: detailOf(computer, read),
        sessions: carries ? sessions : [],
        more: overrides[computer.id]?.more,
      };
    }),
  ];
}

export function leadingOf(sessions: readonly SessionPayload[]): SessionListLeadingSignal {
  let needsYou = 0;
  let working = 0;
  for (const session of sessions) {
    const signal = sessionBadgeSignal(session.badge);
    if (signal.attention === "needs-you") needsYou += 1;
    else if (signal.state === "running") working += 1;
  }
  return { needsYou, working, slot: "session-group" };
}

export function totalsOf(sessions: readonly SessionPayload[]) {
  const { needsYou, working } = leadingOf(sessions);
  return { needsYou, working, idle: sessions.length - needsYou - working };
}

/** Rows by project, in the catalog's project order. */
export function byProject(sessions: readonly SessionPayload[]): [string, SessionPayload[]][] {
  const order = Object.keys(PROJECTS);
  const map = new Map<string, SessionPayload[]>();
  for (const session of sessions) {
    const key = session.workspace_id ?? "";
    map.set(key, [...(map.get(key) ?? []), session]);
  }
  return [...map.entries()].sort(([a], [b]) => order.indexOf(a) - order.indexOf(b));
}

export function matches(session: SessionPayload, query: string): boolean {
  return query === "" || (session.name ?? "").toLowerCase().includes(query.toLowerCase());
}

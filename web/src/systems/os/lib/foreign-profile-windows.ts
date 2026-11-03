import { windowDataScope, type DataScope } from "@/systems/workspace";

import { parseAgentWindowLocation } from "../apps/agents/agent-window-location";
import { matchSessionInstance, matchTerminalInstance } from "./app-catalog";
import type { OsWindow } from "./os-types";

/**
 * The profile-owned item a window shows. Each kind names what its owner read needs: a session
 * and an automation are found by id across profiles; a terminal is found in the catalog of the
 * scope the window reads; an agent window is pinned to the profile it was opened for.
 */
export type ShownProfileItem =
  | { kind: "session"; sessionId: string }
  | { kind: "terminal"; terminalId: string; scope: DataScope }
  | { kind: "agent"; name: string; profile: string; scope: DataScope }
  | { kind: "automation-job"; jobId: string }
  | { kind: "automation-trigger"; triggerId: string };

/** A window and the item it showed when it was inspected. */
export interface ShownItemWindow {
  windowId: string;
  item: ShownProfileItem;
}

/**
 * Who owns an item: a profile id where the read labels the row, a profile name where only the
 * window's own pin names it. `null` when the item is gone; a lookup that could not read the
 * owner throws instead.
 */
export type ProfileOwnerRef = { id: string } | { name: string };

export type ProfileItemOwnerLookup = (item: ShownProfileItem) => Promise<ProfileOwnerRef | null>;

/** The profile the operator switched into, by both identities an owner ref can carry. */
export interface SweepTargetProfile {
  id: string;
  name: string;
}

const AUTOMATION_DETAIL_PATTERN = /^\/automations\/(jobs|triggers)\/([^/]+)$/;

/** The foreign windows a desk inspection found, and whether every owner read answered. */
export interface ForeignProfileWindows {
  foreign: ShownItemWindow[];
  /** `false` when an owner read failed: those windows stay, so the desk needs another look. */
  complete: boolean;
}

/**
 * The windows of a profile's desk that show another profile's item.
 *
 * Under All profiles the client works on the destination profile's desk, so an item of any
 * profile opened there lands in that desk. Once the operator switches to one profile, those
 * windows are not part of it. Only a confirmed foreign owner counts: a gone item stays, because
 * its owning surface already handles that; an item whose owner read failed stays too, and the
 * inspection reports itself incomplete so the caller can look again.
 */
export async function foreignProfileWindows(
  windows: Readonly<Record<string, OsWindow>>,
  partition: string,
  target: SweepTargetProfile,
  ownerOf: ProfileItemOwnerLookup
): Promise<ForeignProfileWindows> {
  const shown = Object.values(windows).flatMap(win => {
    const item = shownProfileItem(win, partition);
    return item === null ? [] : [{ windowId: win.id, item }];
  });
  let complete = true;
  const owners = await Promise.all(
    shown.map(async ({ item }) => {
      try {
        return await ownerOf(item);
      } catch {
        complete = false;
        return null;
      }
    })
  );
  return { foreign: shown.filter((_, index) => isForeignOwner(owners[index], target)), complete };
}

function isForeignOwner(
  owner: ProfileOwnerRef | null | undefined,
  target: SweepTargetProfile
): boolean {
  if (owner === null || owner === undefined) return false;
  return "id" in owner ? owner.id !== target.id : owner.name !== target.name;
}

/**
 * The profile-owned item a window shows, or `null` for a window that shows none. `partition` is
 * the desk's workspace partition: an unbound window reads the scope of the desk it sits on.
 */
export function shownProfileItem(
  win: OsWindow | undefined,
  partition: string
): ShownProfileItem | null {
  if (win === undefined) return null;
  try {
    const { pathname, search } = win.route;
    const sessionId = matchSessionInstance(pathname);
    if (sessionId !== null) return { kind: "session", sessionId };
    if (win.app === "terminal") {
      const terminalId = matchTerminalInstance(pathname);
      const scope = windowDataScope(partition, search.workspace);
      return terminalId === null || scope === null ? null : { kind: "terminal", terminalId, scope };
    }
    if (win.app === "agents") return shownAgent(win, partition);
    if (win.app === "automations") {
      const match = AUTOMATION_DETAIL_PATTERN.exec(pathname);
      if (match === null) return null;
      const id = decodeURIComponent(match[2]);
      return match[1] === "jobs"
        ? { kind: "automation-job", jobId: id }
        : { kind: "automation-trigger", triggerId: id };
    }
    return null;
  } catch {
    // A malformed escape in one route must not abort the sweep of every other window.
    return null;
  }
}

/**
 * Only a pinned agent window belongs to a profile. An unpinned one reads whichever profile the
 * client acts as, so it always shows the desk's own profile and never needs to leave it.
 */
function shownAgent(win: OsWindow, partition: string): ShownProfileItem | null {
  const location = parseAgentWindowLocation(win.route);
  if (location.kind === "catalog") return null;
  const profile = location.search.profile;
  const scope = windowDataScope(partition, location.search.workspace);
  if (profile === undefined || scope === null) return null;
  return { kind: "agent", name: location.name, profile, scope };
}

/** Two inspections show the same item: a close is only licensed for the one that was inspected. */
export function sameShownItem(left: ShownProfileItem | null, right: ShownProfileItem): boolean {
  return left !== null && JSON.stringify(left) === JSON.stringify(right);
}

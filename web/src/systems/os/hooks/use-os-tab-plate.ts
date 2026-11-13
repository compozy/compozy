import { pendingInteractions, type SessionPayload } from "@/systems/session";

import type { OsWindow } from "../lib/os-types";
import { sessionSignal, tabSignal } from "../lib/tab-signal";
import { TAB_MAX_WIDTH, tabPresentation } from "../lib/tab-width";
import { ensureWindowSlotStore } from "../lib/window-slot-registry";
import { pinnedTabWidth } from "./use-os-tab-bar";
import { useOsWindowTitle } from "./use-os-window-title";
import { useWindowMemberSlot } from "./use-window-member-slot";

/** Interactions waiting on the operator across the tab's session windows. */
function pendingCount(
  members: readonly OsWindow[],
  sessionOf: (win: OsWindow) => SessionPayload | undefined
): number {
  return members.reduce((sum, member) => {
    const session = sessionOf(member);
    return sum + (member.app === "session" && session ? pendingInteractions(session).length : 0);
  }, 0);
}

export interface OsTabPlateInput {
  members: readonly OsWindow[];
  pinned: boolean;
  active: boolean;
  sessionOf: (win: OsWindow) => SessionPayload | undefined;
  width: number | undefined;
}

/**
 * What one tab plate shows: the lead window's title and crumbs, the label naming the split, the
 * loudest signal and where it sits, the waiting count, and the parts its width keeps.
 */
export function useOsTabPlate({ members, pinned, active, sessionOf, width }: OsTabPlateInput) {
  const lead = members[0]!;
  const title = useOsWindowTitle(lead, sessionOf(lead));
  const crumbs = useWindowMemberSlot(ensureWindowSlotStore(lead.id))?.crumbs;
  const signal = tabSignal(members.map(member => sessionSignal(sessionOf(member))));
  const pending = pendingCount(members, sessionOf);
  const label = members.length > 1 ? `${title} and ${members.length - 1} more` : title;
  const plateWidth = pinned ? pinnedTabWidth(members.length) : (width ?? TAB_MAX_WIDTH);
  const show = tabPresentation(plateWidth, active);
  const iconOnly = !pinned && !show.title;
  // The inline signal moves to the tile's corner once the tab is too narrow to show it.
  const cornerSignal = Boolean(signal) && (pinned || !show.signal);
  return { lead, title, crumbs, signal, pending, label, plateWidth, show, iconOnly, cornerSignal };
}

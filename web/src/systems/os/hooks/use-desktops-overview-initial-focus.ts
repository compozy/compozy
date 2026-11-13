import { useRef, useState } from "react";

import type {
  DesktopOverviewAction,
  DesktopOverviewFocusSegment,
  DesktopsOverviewState,
} from "../components/desktops-overview";

function nearestHiddenDesktopId(
  segment: DesktopOverviewFocusSegment | null | undefined
): string | null {
  if (!segment) return null;
  if (segment.direction === "later") return segment.hiddenDesktopIds[0] ?? null;
  return segment.hiddenDesktopIds[segment.hiddenDesktopIds.length - 1] ?? null;
}

/**
 * The desktops overview's initial focus: the nearest requested hidden desktop, else the current
 * one; none when a desktop menu action opened the overview, since that focuses inside its card.
 */
export function useDesktopsOverviewInitialFocus(
  open: boolean,
  desktopAction: DesktopOverviewAction | null,
  initialFocusSegment: DesktopOverviewFocusSegment | null | undefined,
  ready: Extract<DesktopsOverviewState, { status: "ready" }> | null
) {
  const initialFocusRef = useRef<HTMLButtonElement | null>(null);
  // The card consumes the action once applied, so the overview keeps the one it opened with: the
  // dialog's initial focus must not move to a thumbnail while the card's action holds focus.
  const [openingAction, setOpeningAction] = useState(desktopAction);
  if (desktopAction !== null && desktopAction !== openingAction) setOpeningAction(desktopAction);
  if (!open && openingAction !== null) setOpeningAction(null);
  // A menu action focuses inside its card (the rename field, the close confirm), not the thumbnail.
  const requestedFocusDesktopId = openingAction
    ? null
    : (nearestHiddenDesktopId(initialFocusSegment) ?? ready?.activeDesktopId ?? null);
  const initialFocusDesktopId =
    ready?.desktops.some(desktop => desktop.id === requestedFocusDesktopId) === true
      ? requestedFocusDesktopId
      : null;
  return { initialFocusRef, initialFocusDesktopId };
}
